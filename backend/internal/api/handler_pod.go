package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubrowser/kubrowser-backend/internal/terminal"
)

// HandleListNamespaces lists all available Kubernetes namespaces.
func (h *Handlers) HandleListNamespaces(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	namespaces, err := h.podManager.GetClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		h.logger.WithError(err).Error("Failed to list namespaces")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list namespaces"})
		return
	}

	namespaceList := make([]gin.H, 0, len(namespaces.Items))
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		namespaceList = append(namespaceList, gin.H{
			"name":   ns.Name,
			"status": ns.Status.Phase,
		})
	}

	c.JSON(http.StatusOK, gin.H{"namespaces": namespaceList})
}

// HandleListPods lists pods in a namespace or all namespaces.
func (h *Handlers) HandleListPods(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Handle both "*" and URL-encoded "%2A".
	listNamespace := namespace
	if namespace == "*" || namespace == "%2A" || namespace == "all" || namespace == "" {
		listNamespace = metav1.NamespaceAll
		h.logger.WithField("requested_namespace", namespace).Info("Listing pods from all namespaces")
	} else {
		h.logger.WithField("namespace", namespace).Info("Listing pods from namespace")
	}

	pods, err := h.podManager.GetClient().CoreV1().Pods(listNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		h.logger.WithError(err).WithField("namespace", namespace).Error("Failed to list pods")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"namespace":      namespace,
		"list_namespace": listNamespace,
		"pods_count":     len(pods.Items),
	}).Info("Successfully listed pods")

	podList := make([]gin.H, 0, len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		// Get pod status.
		status := string(pod.Status.Phase)
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				ready = true
				break
			}
		}

		podList = append(podList, gin.H{
			"name":        pod.Name,
			"namespace":   pod.Namespace,
			"status":      status,
			"ready":       ready,
			"age":         time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
			"restarts":    getRestartCount(pod),
			"node":        pod.Spec.NodeName,
			"podIP":       pod.Status.PodIP,
			"qosClass":    string(pod.Status.QOSClass),
			"labels":      pod.Labels,
			"annotations": pod.Annotations,
			"containers":  getContainerInfo(pod),
		})
	}

	c.JSON(http.StatusOK, gin.H{"pods": podList})
}

// HandleGetPod gets a specific pod by name.
func (h *Handlers) HandleGetPod(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	pod, err := h.podManager.GetClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
			return
		}
		h.logger.WithError(err).Error("Failed to get pod")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pod"})
		return
	}

	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			ready = true
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"status":    string(pod.Status.Phase),
		"ready":     ready,
		"age":       time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
		"restarts":  getRestartCount(pod),
		"labels":    pod.Labels,
		"node":      pod.Spec.NodeName,
	})
}

// HandleDeletePod deletes a pod by name.
func (h *Handlers) HandleDeletePod(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err := h.podManager.GetClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
			return
		}
		h.logger.WithError(err).Error("Failed to delete pod")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "pod": podName})
}

// HandlePodLogs streams logs from a pod.
func (h *Handlers) HandlePodLogs(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")
	containerName := c.DefaultQuery("container", "")
	tailLines := c.DefaultQuery("tail", "100")
	follow := c.DefaultQuery("follow", "true") == "true"

	ctx := c.Request.Context()
	if !follow {
		// For non-following logs, use a timeout.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Get pod to find container name if not provided.
	pod, err := h.podManager.GetClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
			return
		}
		h.logger.WithError(err).Error("Failed to get pod")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pod"})
		return
	}

	// Use first container if not specified.
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	// Parse tail lines.
	tailLinesInt := int64(100)
	if tailLines != "" {
		if parsed, parseErr := strconv.ParseInt(tailLines, 10, 64); parseErr == nil {
			tailLinesInt = parsed
		}
	}

	// Get logs.
	podLogOpts := &v1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
		TailLines: &tailLinesInt,
	}

	req := h.podManager.GetClient().CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to stream logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream logs"})
		return
	}
	defer stream.Close()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")

	// Stream logs to response.
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := stream.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				break
			}
			c.Writer.Flush()
		}
		if err != nil {
			if err == io.EOF {
				if !follow {
					break
				}
				// For follow mode, EOF might be temporary, continue reading.
				time.Sleep(100 * time.Millisecond)
				continue
			}
			h.logger.WithError(err).Error("Error reading logs")
			break
		}
	}
}

// HandlePodExec handles WebSocket connections for exec into a pod.
func (h *Handlers) HandlePodExec(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")
	containerName := c.DefaultQuery("container", "")

	// Upgrade to WebSocket.
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}
	defer ws.Close()

	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
	}).Info("WebSocket upgraded successfully for exec")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set WebSocket close handler to cancel context.
	ws.SetCloseHandler(func(code int, text string) error {
		h.logger.WithFields(logrus.Fields{
			"pod":       podName,
			"namespace": namespace,
			"code":      code,
			"reason":    text,
		}).Info("WebSocket close handler triggered, canceling exec stream")
		cancel()
		return nil
	})

	// Get pod to find container name if not provided.
	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
	}).Info("Fetching pod information")

	pod, err := h.podManager.GetClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		h.logger.WithError(err).Error("Failed to get pod")
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Pod not found"))
		return
	}

	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
		"phase":     pod.Status.Phase,
	}).Info("Pod found, checking status")

	// Check if pod is running.
	if pod.Status.Phase != v1.PodRunning {
		h.logger.WithField("phase", pod.Status.Phase).Error("Pod is not running")
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("Pod is not running (phase: %s)", pod.Status.Phase)))
		return
	}

	// Use first container if not specified.
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	if containerName == "" {
		h.logger.Error("No container found in pod")
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "No containers found in pod"))
		return
	}

	// Check container image to provide better error messages.
	var containerImage string
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Name == containerName {
			containerImage = container.Image
			break
		}
	}
	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
		"container": containerName,
		"image":     containerImage,
	}).Info("Container image info")

	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
		"container": containerName,
	}).Info("Starting exec session")

	// Stream exec - reuse terminal executor but with different namespace.
	executor := terminal.NewExecutor(h.podManager.GetClient(), h.podManager.GetConfig(), namespace)
	if err := executor.StreamTerminal(ctx, ws, podName, containerName); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"pod":       podName,
			"namespace": namespace,
			"container": containerName,
		}).Error("Exec stream error")

		// Provide user-friendly error messages.
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such file or directory") ||
			strings.Contains(errMsg, "exec:") ||
			strings.Contains(errMsg, "no shell found") {
			errMsg = fmt.Sprintf("Container has no shell (image: %s). Use 'kubectl debug' for distroless containers.", containerImage)
		}

		// Send clean error message to client before closing.
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Exec error: %s", errMsg)))
	} else {
		h.logger.WithFields(logrus.Fields{
			"pod":       podName,
			"namespace": namespace,
			"container": containerName,
		}).Info("Exec stream completed successfully")
	}
}

func getContainerInfo(pod *v1.Pod) []gin.H {
	containers := make([]gin.H, 0, len(pod.Spec.Containers))
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]
		restartCount := int32(0)
		state := "Unknown"

		// Find status for this container.
		for j := range pod.Status.ContainerStatuses {
			status := &pod.Status.ContainerStatuses[j]
			if status.Name == c.Name {
				restartCount = status.RestartCount
				if status.State.Running != nil {
					state = "Running"
				} else if status.State.Waiting != nil {
					state = "Waiting"
					if status.State.Waiting.Reason != "" {
						state = fmt.Sprintf("Waiting (%s)", status.State.Waiting.Reason)
					}
				} else if status.State.Terminated != nil {
					state = "Terminated"
					if status.State.Terminated.Reason != "" {
						state = fmt.Sprintf("Terminated (%s)", status.State.Terminated.Reason)
					}
				}
				break
			}
		}

		containers = append(containers, gin.H{
			"name":     c.Name,
			"image":    c.Image,
			"restarts": restartCount,
			"state":    state,
		})
	}
	return containers
}
