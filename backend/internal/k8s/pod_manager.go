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

// CreatePod creates a new pod with kubectl installed.
// The pod will be automatically cleaned up after the specified timeout.
func (pm *PodManager) CreatePod(ctx context.Context, sessionID string) (*v1.Pod, error) {
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
			ServiceAccountName: pm.serviceAccount,
			Containers: []v1.Container{
				{
					Name:  "kubectl",
					Image: pm.image,
					// Keep container running
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"trap 'exit 0' SIGTERM; while true; do sleep 30; done"},
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

	createdPod, err := pm.client.CoreV1().Pods(pm.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// Wait for pod to be ready
	if err := pm.waitForPodReady(ctx, podName); err != nil {
		_ = pm.DeletePod(ctx, podName)
		return nil, fmt.Errorf("pod failed to become ready: %w", err)
	}

	return createdPod, nil
}

// waitForPodReady waits for the pod to be in Ready state.
func (pm *PodManager) waitForPodReady(ctx context.Context, podName string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := pm.client.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
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
