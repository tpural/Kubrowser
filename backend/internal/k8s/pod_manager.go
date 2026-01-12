package k8s

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PodManager handles creation and deletion of temporary Kubernetes pods.
type PodManager struct {
	client        kubernetes.Interface
	config        *rest.Config
	namespace     string
	image         string
	serviceAccount string
	limits        ResourceLimits
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

// CreatePod creates a new pod with kubectl installed.
// The pod will be automatically cleaned up after the specified timeout.
func (pm *PodManager) CreatePod(ctx context.Context, sessionID string) (*v1.Pod, error) {
	return pm.CreatePodWithStatus(ctx, sessionID, time.Now(), nil)
}

// CreatePodWithStatus creates a new pod with kubectl installed and reports status updates.
func (pm *PodManager) CreatePodWithStatus(ctx context.Context, sessionID string, startTime time.Time, statusCallback StatusCallback) (*v1.Pod, error) {
	podName := fmt.Sprintf("kubrowser-%s-%d", sessionID, time.Now().Unix())

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
				"session-id":  sessionID,
				"managed-by":  "kubrowser-backend",
			},
		},
		Spec: v1.PodSpec{
			Hostname: "kubrowser",
			ServiceAccountName: pm.serviceAccount,
			Containers: []v1.Container{
				{
					Name:  "kubectl",
					Image: pm.image,
					// Keep container running
					// Create a user entry in /etc/passwd to prevent "I have no name!" message
					Command: []string{"/bin/sh", "-c"},
					Args: []string{
						"echo 'kubrowser:x:1000:1000:Kubrowser User:/home/kubrowser:/bin/bash' >> /etc/passwd 2>/dev/null || true; " +
							"mkdir -p /home/kubrowser 2>/dev/null || true; " +
							"trap 'exit 0' SIGTERM; while true; do sleep 30; done",
					},
					Env: []v1.EnvVar{
						{
							Name:  "USER",
							Value: "kubrowser",
						},
						{
							Name:  "HOME",
							Value: "/home/kubrowser",
						},
						{
							Name:  "PS1",
							Value: "kubrowser:\\w\\$ ",
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
					SecurityContext: &v1.SecurityContext{
						RunAsNonRoot: func() *bool { b := true; return &b }(),
						RunAsUser:    func() *int64 { u := int64(1000); return &u }(),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
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

// GetClient returns the Kubernetes client.
func (pm *PodManager) GetClient() kubernetes.Interface {
	return pm.client
}

// GetConfig returns the REST config.
func (pm *PodManager) GetConfig() *rest.Config {
	return pm.config
}
