package k8s

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	HeartbeatAnnotation = "kubrowser.io/last-heartbeat"
)

// PodManager handles creation and deletion of temporary Kubernetes pods.
type PodManager struct {
	client         kubernetes.Interface
	config         *rest.Config
	namespace      string
	image          string
	serviceAccount string
	limits         ResourceLimits
}

// ResourceLimits holds CPU and memory limits.
type ResourceLimits struct {
	CPU    string
	Memory string
}

// NewPodManager creates a new PodManager instance.
func NewPodManager(kubeconfigPath, namespace, image, serviceAccount string, limits ResourceLimits) (*PodManager, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &PodManager{
		client:         clientset,
		config:         config,
		namespace:      namespace,
		image:          image,
		serviceAccount: serviceAccount,
		limits:         limits,
	}, nil
}

// StatusCallback is called to report pod creation status updates
type StatusCallback func(message string)

// StatusCallbackWithDuration is called to report pod creation status updates with duration
type StatusCallbackWithDuration func(message string, duration time.Duration)

// sanitizeUsername sanitizes a username to be Kubernetes-compliant.
// Kubernetes names must be lowercase alphanumeric characters, '-', or '.', and must start/end with alphanumeric.
// Max length is 63 characters.
func sanitizeUsername(username string) string {
	// Default to "anonymous" if empty
	if username == "" {
		username = "anonymous"
	}

	// Convert to lowercase
	username = strings.ToLower(username)

	// Replace invalid characters with hyphens (keep alphanumeric, dots, and hyphens)
	re := regexp.MustCompile(`[^a-z0-9.-]`)
	username = re.ReplaceAllString(username, "-")

	// Remove leading/trailing dots and hyphens
	username = strings.Trim(username, "-.")

	// Ensure it starts and ends with alphanumeric
	if len(username) > 0 && !isAlphanumeric(rune(username[0])) {
		username = "u" + username
	}
	if len(username) > 0 && !isAlphanumeric(rune(username[len(username)-1])) {
		username = username + "0"
	}

	// Truncate to 63 characters (Kubernetes limit)
	if len(username) > 63 {
		username = username[:63]
		// Ensure it still ends with alphanumeric after truncation
		if !isAlphanumeric(rune(username[len(username)-1])) {
			username = username[:62] + "0"
		}
	}

	// If empty after sanitization, use default
	if username == "" {
		username = "anonymous"
	}

	return username
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// CreatePod creates a new pod with kubectl installed.
// The pod will be automatically cleaned up after the specified timeout.
func (pm *PodManager) CreatePod(ctx context.Context, sessionID string) (*v1.Pod, error) {
	return pm.CreatePodWithStatus(ctx, sessionID, "", time.Now(), nil)
}

// CreatePodWithStatus creates a new pod with kubectl installed and reports status updates.
// username is sanitized and included in the pod name for easier management.
// Pod name format: kubrowser-{username}
// Note: This creates one pod per username. If a pod already exists for this user, it will be deleted first.
func (pm *PodManager) CreatePodWithStatus(ctx context.Context, sessionID, username string, startTime time.Time, statusCallback StatusCallback) (*v1.Pod, error) {
	// Sanitize username for Kubernetes naming requirements
	sanitizedUsername := sanitizeUsername(username)

	// Generate pod name: kubrowser-{username}
	podName := fmt.Sprintf("kubrowser-%s", sanitizedUsername)

	// Ensure pod name doesn't exceed 63 characters (Kubernetes limit)
	if len(podName) > 63 {
		// Truncate username if needed (leave room for "kubrowser-" prefix which is 10 chars)
		maxUsernameLen := 63 - 10
		if maxUsernameLen > 0 && len(sanitizedUsername) > maxUsernameLen {
			sanitizedUsername = sanitizedUsername[:maxUsernameLen]
			podName = fmt.Sprintf("kubrowser-%s", sanitizedUsername)
		}
	}

	// Create PVC for user's home directory if it doesn't exist
	pvcName := fmt.Sprintf("kubrowser-home-%s", sanitizedUsername)
	if err := pm.ensureHomePVC(ctx, pvcName, sanitizedUsername, statusCallback); err != nil {
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("\r\x1b[K\x1b[31m[✗] Failed to create home storage: %v\x1b[0m\r\n", err))
		}
		return nil, fmt.Errorf("failed to create home PVC: %w", err)
	}

	// Check if a pod with this name already exists and reuse it if possible
	existingPod, err := pm.FindExistingPod(ctx, username)
	if err == nil && existingPod != nil {
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("\r\x1b[K\x1b[32m[✓] Found existing session for %s\x1b[0m\r\n", sanitizedUsername))
		}
		// Update heartbeat to ensure it doesn't get reaped immediately
		_ = pm.UpdatePodHeartbeat(ctx, existingPod.Name)
		return existingPod, nil
	}

	// Check if a pod with this name already exists and wait for it to be fully deleted

	existingPod, err = pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil && existingPod != nil {
		// Pod exists, delete it first and wait for it to be fully gone
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Cleaning up existing pod for user %s...\x1b[0m", sanitizedUsername))
		}

		// Only delete if not already terminating
		if existingPod.DeletionTimestamp == nil {
			deletePolicy := metav1.DeletePropagationForeground
			deleteErr := pm.client.CoreV1().Pods(pm.namespace).Delete(ctx, podName, metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			if deleteErr != nil && !errors.IsNotFound(deleteErr) {
				if statusCallback != nil {
					statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[!] Warning: Failed to delete existing pod: %v\x1b[0m\r\n", deleteErr))
				}
			}
		}

		// Wait for pod to be fully deleted (up to 60 seconds)
		waitStart := time.Now()
		waitErr := wait.PollUntilContextTimeout(ctx, 1*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
			_, getErr := pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
			if errors.IsNotFound(getErr) {
				return true, nil // Pod is fully deleted
			}
			if getErr != nil {
				return false, getErr
			}
			// Pod still exists, update status
			if statusCallback != nil {
				elapsed := time.Since(waitStart)
				statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Waiting for previous session to terminate... (%v)\x1b[0m", elapsed.Round(time.Millisecond)))
			}
			return false, nil
		})

		if waitErr != nil {
			if statusCallback != nil {
				statusCallback("\r\x1b[K\x1b[31m[✗] Timeout waiting for old pod to terminate\x1b[0m\r\n")
			}
			return nil, fmt.Errorf("timeout waiting for existing pod to terminate: %w", waitErr)
		}

		if statusCallback != nil {
			statusCallback("\r\x1b[K\x1b[32m[✓] Previous session cleaned up\x1b[0m\r\n")
		}
	}

	cpuQuantity, err := resource.ParseQuantity(pm.limits.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU limit: %w", err)
	}

	memoryQuantity, err := resource.ParseQuantity(pm.limits.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory limit: %w", err)
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pm.namespace,
			Labels: map[string]string{
				"app":        "kubrowser",
				"session-id": sessionID,
				"username":   sanitizedUsername,
				"managed-by": "kubrowser-backend",
			},
			Annotations: map[string]string{
				HeartbeatAnnotation: time.Now().Format(time.RFC3339),
			},
		},
		Spec: v1.PodSpec{
			Hostname:           "kubrowser",
			ServiceAccountName: pm.serviceAccount,
			Containers: []v1.Container{
				{
					Name:            "terminal",
					Image:           pm.image,
					ImagePullPolicy: v1.PullIfNotPresent,
					// The entrypoint.sh in the custom image handles user creation
					Env: []v1.EnvVar{
						{
							Name:  "KUBROWSER_USER",
							Value: sanitizedUsername,
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    cpuQuantity,
							v1.ResourceMemory: memoryQuantity,
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    cpuQuantity,
							v1.ResourceMemory: memoryQuantity,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "home",
							MountPath: fmt.Sprintf("/home/%s", sanitizedUsername),
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "home",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: fmt.Sprintf("kubrowser-home-%s", sanitizedUsername),
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	if statusCallback != nil {
		statusCallback("\r\x1b[K\x1b[33m[ ] Creating pod...\x1b[0m")
	}

	createdPod, err := pm.client.CoreV1().Pods(pm.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if statusCallback != nil {
			statusCallback("\r\x1b[K\x1b[31m[✗] Failed to create pod\x1b[0m\r\n")
		}
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	if statusCallback != nil {
		elapsed := time.Since(startTime)
		statusCallback(fmt.Sprintf("\r\x1b[K\x1b[32m[✓] Pod created (%v)\x1b[0m\r\n", elapsed.Round(time.Millisecond)))
		statusCallback("\r\x1b[K\x1b[33m[ ] Waiting for pod to be ready...\x1b[0m")
	}

	// Wait for pod to be ready
	if err := pm.waitForPodReady(ctx, podName, startTime, statusCallback); err != nil {
		if statusCallback != nil {
			statusCallback("\r\x1b[K\x1b[31m[✗] Pod failed to become ready\x1b[0m\r\n")
		}
		_ = pm.DeletePod(ctx, podName)
		return nil, fmt.Errorf("pod failed to become ready: %w", err)
	}

	if statusCallback != nil {
		elapsed := time.Since(startTime)
		statusCallback(fmt.Sprintf("\r\x1b[K\x1b[32m[✓] Pod is ready (%v)\x1b[0m\r\n", elapsed.Round(time.Millisecond)))
		statusCallback("\r\x1b[K\x1b[33m[ ] Starting terminal session...\x1b[0m")
	}

	return createdPod, nil
}

// waitForPodReady waits for the pod to be in Ready state.
func (pm *PodManager) waitForPodReady(ctx context.Context, podName string, startTime time.Time, statusCallback StatusCallback) error {
	lastPhase := ""
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Report phase changes - overwrite the waiting line with elapsed time
		if statusCallback != nil {
			elapsed := time.Since(startTime)
			if pod.Status.Phase != v1.PodPhase(lastPhase) {
				lastPhase = string(pod.Status.Phase)
				// Clear line and overwrite with phase info
				switch pod.Status.Phase {
				case v1.PodPending:
					statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Waiting for pod to be ready... (Pending, %v)\x1b[0m", elapsed.Round(time.Millisecond)))
				case v1.PodRunning:
					statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Waiting for pod to be ready... (Running, %v)\x1b[0m", elapsed.Round(time.Millisecond)))
				case v1.PodSucceeded, v1.PodFailed:
					statusCallback(fmt.Sprintf("\r\x1b[K\x1b[31m[✗] Pod phase: %s (%v)\x1b[0m", pod.Status.Phase, elapsed.Round(time.Millisecond)))
				}
			} else {
				// Update elapsed time periodically even if phase hasn't changed
				switch pod.Status.Phase {
				case v1.PodPending, v1.PodRunning:
					statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Waiting for pod to be ready... (%s, %v)\x1b[0m", pod.Status.Phase, elapsed.Round(time.Millisecond)))
				}
			}
		}

		// Check if pod is running first
		if pod.Status.Phase != v1.PodRunning {
			return false, nil
		}

		// Check if pod is ready
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	})
}

// DeletePod deletes a pod by name.
// Returns nil if pod is already deleted (not found).
func (pm *PodManager) DeletePod(ctx context.Context, podName string) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := pm.client.CoreV1().Pods(pm.namespace).Delete(ctx, podName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})

	// If pod is not found, it's already deleted - this is fine
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // Pod already deleted, treat as success
		}
		return err
	}

	return nil
}

// ensureHomePVC creates a PersistentVolumeClaim for the user's home directory if it doesn't exist.
func (pm *PodManager) ensureHomePVC(ctx context.Context, pvcName, username string, statusCallback StatusCallback) error {
	// Check if PVC already exists
	_, err := pm.client.CoreV1().PersistentVolumeClaims(pm.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		// PVC already exists
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("\r\x1b[K\x1b[32m[✓] Home storage ready for %s\x1b[0m\r\n", username))
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing PVC: %w", err)
	}

	if statusCallback != nil {
		statusCallback(fmt.Sprintf("\r\x1b[K\x1b[33m[ ] Creating home storage for %s...\x1b[0m", username))
	}

	// Create PVC
	storageSize := resource.MustParse("1Gi")
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: pm.namespace,
			Labels: map[string]string{
				"app":        "kubrowser",
				"username":   username,
				"managed-by": "kubrowser-backend",
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: storageSize,
				},
			},
		},
	}

	_, err = pm.client.CoreV1().PersistentVolumeClaims(pm.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	if statusCallback != nil {
		statusCallback(fmt.Sprintf("\r\x1b[K\x1b[32m[✓] Home storage created for %s\x1b[0m\r\n", username))
	}

	return nil
}

// GetPod retrieves a pod by name.
func (pm *PodManager) GetPod(ctx context.Context, podName string) (*v1.Pod, error) {
	return pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
}

// ListPods lists all pods managed by kubrowser.
func (pm *PodManager) ListPods(ctx context.Context) ([]v1.Pod, error) {
	list, err := pm.client.CoreV1().Pods(pm.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=kubrowser",
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListPodsByUsername lists all pods for a specific username.
func (pm *PodManager) ListPodsByUsername(ctx context.Context, username string) ([]v1.Pod, error) {
	sanitizedUsername := sanitizeUsername(username)
	list, err := pm.client.CoreV1().Pods(pm.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=kubrowser,username=%s", sanitizedUsername),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// IsPodReady checks if a pod is in Ready state.
func (pm *PodManager) IsPodReady(ctx context.Context, podName string) (bool, error) {
	pod, err := pm.GetPod(ctx, podName)
	if err != nil {
		return false, err
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}

// FindExistingPod checks for an existing running pod for the given username.
// Returns the pod if found and running, nil otherwise.
func (pm *PodManager) FindExistingPod(ctx context.Context, username string) (*v1.Pod, error) {
	sanitizedUsername := sanitizeUsername(username)
	podName := fmt.Sprintf("kubrowser-%s", sanitizedUsername)

	// Ensure pod name consistency with creation logic
	if len(podName) > 63 {
		maxUsernameLen := 63 - 10
		if maxUsernameLen > 0 && len(sanitizedUsername) > maxUsernameLen {
			sanitizedUsername = sanitizedUsername[:maxUsernameLen]
			podName = fmt.Sprintf("kubrowser-%s", sanitizedUsername)
		}
	}

	pod, err := pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	// Check if pod is usable (Running and not deleting)
	if pod.Status.Phase == v1.PodRunning && pod.DeletionTimestamp == nil {
		// Also verify it's ready
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return pod, nil
			}
		}
	}

	return nil, nil
}

// UpdatePodHeartbeat updates the last-heartbeat annotation on the pod.
func (pm *PodManager) UpdatePodHeartbeat(ctx context.Context, podName string) error {
	timestamp := time.Now().Format(time.RFC3339)

	// Create patch to update annotation
	patchData := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, HeartbeatAnnotation, timestamp))

	_, err := pm.client.CoreV1().Pods(pm.namespace).Patch(ctx, podName, types.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}

// CleanupStalePods deletes pods that haven't had a heartbeat for the specified duration.
func (pm *PodManager) CleanupStalePods(ctx context.Context, timeout time.Duration) error {
	pods, err := pm.ListPods(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	deletedCount := 0

	for _, pod := range pods {
		// Skip if already deleting
		if pod.DeletionTimestamp != nil {
			continue
		}

		lastHeartbeatStr, ok := pod.Annotations[HeartbeatAnnotation]
		if !ok {
			// No heartbeat? Assume it's old/legacy or just started.
			// Check creation timestamp as fallback
			if now.Sub(pod.CreationTimestamp.Time) > timeout {
				// Old pod with no heartbeat (or legacy), delete it
				_ = pm.DeletePod(ctx, pod.Name)
				deletedCount++
			}
			continue
		}

		lastHeartbeat, err := time.Parse(time.RFC3339, lastHeartbeatStr)
		if err != nil {
			// Invalid format? safe to ignore or delete? Let's check creation time as fallback
			if now.Sub(pod.CreationTimestamp.Time) > timeout {
				_ = pm.DeletePod(ctx, pod.Name)
				deletedCount++
			}
			continue
		}

		if now.Sub(lastHeartbeat) > timeout {
			// Stale pod
			fmt.Printf("Reaping stale pod: %s (last heartbeat: %s)\n", pod.Name, lastHeartbeatStr)
			if err := pm.DeletePod(ctx, pod.Name); err != nil {
				fmt.Printf("Failed to reap pod %s: %v\n", pod.Name, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		fmt.Printf("Reaper: Cleaned up %d stale pods\n", deletedCount)
	}

	return nil
}

// StartReaper starts a background goroutine to clean up stale pods.
func (pm *PodManager) StartReaper(ctx context.Context, checkInterval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		fmt.Printf("Pod Reaper started (Interval: %v, Timeout: %v)\n", checkInterval, timeout)

		for {
			select {
			case <-ticker.C:
				if err := pm.CleanupStalePods(ctx, timeout); err != nil {
					fmt.Printf("Reaper error: %v\n", err)
				}
			case <-ctx.Done():
				fmt.Println("Pod Reaper stopped")
				return
			}
		}
	}()
}

// GetClient returns the Kubernetes client.
func (pm *PodManager) GetClient() kubernetes.Interface {
	return pm.client
}

// GetConfig returns the REST config.
func (pm *PodManager) GetConfig() *rest.Config {
	return pm.config
}
