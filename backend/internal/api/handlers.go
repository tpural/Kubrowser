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
	"github.com/kubrowser/kubrowser-backend/internal/k8s"
	"github.com/kubrowser/kubrowser-backend/internal/session"
	"github.com/kubrowser/kubrowser-backend/internal/terminal"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, validate origin
	},
}

// Handlers holds handler dependencies.
type Handlers struct {
	logger       *logrus.Logger
	podManager   *k8s.PodManager
	sessionMgr   *session.Manager
	terminalExec *terminal.Executor
}

// NewHandlers creates a new handlers instance.
func NewHandlers(logger *logrus.Logger, podManager *k8s.PodManager, sessionMgr *session.Manager, terminalExec *terminal.Executor) *Handlers {
	return &Handlers{
		logger:       logger,
		podManager:   podManager,
		sessionMgr:   sessionMgr,
		terminalExec: terminalExec,
	}
}

// NewHandlersWithConfig creates handlers with REST config for terminal executor.
func NewHandlersWithConfig(logger *logrus.Logger, podManager *k8s.PodManager, sessionMgr *session.Manager, namespace string) *Handlers {
	terminalExec := terminal.NewExecutor(podManager.GetClient(), podManager.GetConfig(), namespace)
	return NewHandlers(logger, podManager, sessionMgr, terminalExec)
}

// HandleWebSocket handles WebSocket connections for terminal access.
func (h *Handlers) HandleWebSocket(c *gin.Context) {
	sessionID := c.Query("session_id")
	reconnect := c.Query("reconnect") == "true"

	var sess *session.Session
	var exists bool
	var ws *websocket.Conn
	var err error

	// Upgrade to WebSocket first so we can send status updates
	ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}
	defer ws.Close()

	if sessionID != "" && reconnect {
		sess, exists = h.sessionMgr.GetSession(sessionID)
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
	} else {
		// Send status checklist while creating pod
		sendStatusUpdate := func(message string) {
			ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			ws.WriteMessage(websocket.TextMessage, []byte(message))
		}

		// Show initial status header
		sendStatusUpdate("\r\n\x1b[36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\r\n")
		sendStatusUpdate("\x1b[34;1mKubrowser is starting up\x1b[0m\r\n")
		sendStatusUpdate("\x1b[36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\r\n\r\n")
		
		// Track start time
		startTime := time.Now()
		
		// Get username from query parameter or header (for now)
		// TODO: Get from authentication token when auth is implemented
		username := c.Query("username")
		if username == "" {
			username = c.GetHeader("X-Username")
		}
		if username == "" {
			username = "anonymous"
		}
		
		// Create new session with status updates
		newSessionID := generateSessionID()
		pod, err := h.podManager.CreatePodWithStatus(c.Request.Context(), newSessionID, username, startTime, func(status string) {
			sendStatusUpdate(status)
		})
		if err != nil {
			h.logger.WithError(err).Error("Failed to create pod")
			duration := time.Since(startTime)
			sendStatusUpdate(fmt.Sprintf("\r\n\x1b[31m[✗] Failed to create pod: %s (took %v)\x1b[0m\r\n", err.Error(), duration.Round(time.Millisecond)))
			ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Failed to create pod"))
			return
		}

		sess = h.sessionMgr.CreateSession(pod.Name, username)
		sessionID = sess.ID

		// Calculate total duration
		duration := time.Since(startTime)
		
		// Mark as ready - overwrite the "Starting terminal session" line
		sendStatusUpdate("\r\x1b[K\x1b[32m[✓] Terminal session ready\x1b[0m\r\n")
		
		// Separator line
		sendStatusUpdate("\r\n\x1b[36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\r\n")
		
		// Show summary
		sendStatusUpdate(fmt.Sprintf("\x1b[36m✓ Ready in %v\x1b[0m\r\n", duration.Round(time.Millisecond)))
		sendStatusUpdate(fmt.Sprintf("\x1b[90mPod: %s | Namespace: %s\x1b[0m\r\n\r\n", pod.Name, pod.Namespace))
	}

	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"pod_name":   sess.PodName,
	}).Info("WebSocket connection established")

	// Try to lock exec for this session (prevent multiple execs to same pod)
	if !h.sessionMgr.TryLockExec(sessionID) {
		h.logger.WithField("session_id", sessionID).Warn("Session exec already locked, rejecting connection")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Session already has an active connection"))
		return
	}
	defer func() {
		h.sessionMgr.UnlockExec(sessionID)
		h.sessionMgr.SetActive(sessionID, false)
	}()

	// Mark session as active
	h.sessionMgr.SetActive(sessionID, true)

	// Send session ID to client (for new sessions) as text message
	// Do this BEFORE starting terminal stream so stdinStream can skip it
	if !reconnect {
		sessionMsg := fmt.Sprintf(`{"type":"session","session_id":"%s","pod_name":"%s"}`, sessionID, sess.PodName)
		ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := ws.WriteMessage(websocket.TextMessage, []byte(sessionMsg)); err != nil {
			h.logger.WithError(err).Error("Failed to send session ID")
			return
		}
		// Give client time to receive and process session message
		time.Sleep(200 * time.Millisecond)
	}

	// Stream terminal I/O
	// Use background context so connection doesn't close when HTTP request ends
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		// Ensure pod is deleted when function exits (defer ensures this runs)
		h.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"pod_name":   sess.PodName,
		}).Info("Handler function exiting, ensuring pod deletion")
	}()

	// Monitor WebSocket connection and cancel context when it closes
	// This ensures the exec stream terminates immediately when browser disconnects
	ws.SetCloseHandler(func(code int, text string) error {
		h.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"pod_name":   sess.PodName,
			"code":       code,
			"text":       text,
		}).Info("WebSocket close handler triggered, canceling exec stream")
		cancel()
		return nil
	})

	// Monitor WebSocket connection state by checking write capability
	// This doesn't interfere with reads (which happen in stdinStream)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Try to write a ping frame to check if connection is still alive
				// This won't interfere with the terminal executor's ping mechanism
				deadline := time.Now().Add(500 * time.Millisecond)
				if err := ws.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
					// Connection is closed
					h.logger.WithFields(logrus.Fields{
						"session_id": sessionID,
						"pod_name":   sess.PodName,
						"error":      err.Error(),
					}).Info("WebSocket ping failed, connection closed, canceling exec stream")
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Update session LastUsed periodically while connection is active
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if sess, exists := h.sessionMgr.GetSession(sessionID); exists {
					sess.LastUsed = time.Now()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	containerName := "kubectl"
	
	// Check if pod is still running before exec
	pod, err := h.podManager.GetPod(ctx, sess.PodName)
	if err != nil {
		h.logger.WithError(err).WithField("pod_name", sess.PodName).Error("Failed to get pod")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Pod not found"))
		return
	}
	
	if pod.Status.Phase != v1.PodRunning {
		h.logger.WithField("pod_name", sess.PodName).WithField("phase", pod.Status.Phase).Error("Pod is not running")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Pod not running"))
		return
	}

	// Stream terminal - this blocks until connection closes or context is canceled
	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"pod_name":   sess.PodName,
	}).Info("Starting terminal stream")
	
	streamErr := h.terminalExec.StreamTerminal(ctx, ws, sess.PodName, containerName)
	
	// Log when stream terminates
	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"pod_name":   sess.PodName,
		"error":      streamErr,
		"error_type": fmt.Sprintf("%T", streamErr),
	}).Info("Terminal stream ended")
	
	// Handle stream errors gracefully
	if streamErr != nil {
		// Log all errors for debugging, but mark expected ones as debug level
		if streamErr == context.Canceled {
			h.logger.WithFields(logrus.Fields{
				"session_id": sessionID,
				"pod_name":   sess.PodName,
			}).Info("Stream terminated due to context cancellation (expected)")
		} else if streamErr == io.EOF {
			h.logger.WithFields(logrus.Fields{
				"session_id": sessionID,
				"pod_name":   sess.PodName,
			}).Info("Stream terminated with EOF (expected)")
		} else if streamErr == context.DeadlineExceeded {
			h.logger.WithFields(logrus.Fields{
				"session_id": sessionID,
				"pod_name":   sess.PodName,
			}).Info("Stream terminated due to deadline exceeded (expected)")
		} else {
			h.logger.WithError(streamErr).
				WithField("session_id", sessionID).
				WithField("pod_name", sess.PodName).
				Error("Terminal stream error")
		}
	}
	
	// Delete pod when WebSocket disconnects
	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"pod_name":   sess.PodName,
	}).Info("WebSocket disconnected, deleting pod")
	
	deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer deleteCancel()
	
	if err := h.podManager.DeletePod(deleteCtx, sess.PodName); err != nil {
		h.logger.WithError(err).
			WithField("session_id", sessionID).
			WithField("pod_name", sess.PodName).
			Error("Failed to delete pod on disconnect")
	} else {
		h.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"pod_name":   sess.PodName,
		}).Info("Pod deleted successfully (or already deleted)")
	}

	// Session cleanup happens above (pod deletion)
}

// HandleResize handles terminal resize requests.
func (h *Handlers) HandleResize(c *gin.Context) {
	sessionID := c.Param("session_id")
	sess, exists := h.sessionMgr.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	var req struct {
		Width  uint16 `json:"width"`
		Height uint16 `json:"height"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.terminalExec.ResizeTerminal(ctx, sess.PodName, "kubectl", req.Width, req.Height); err != nil {
		h.logger.WithError(err).Error("Failed to resize terminal")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resize terminal"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleSessionInfo returns session information.
func (h *Handlers) HandleSessionInfo(c *gin.Context) {
	sessionID := c.Param("session_id")
	sess, exists := h.sessionMgr.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sess.ID,
		"pod_name":   sess.PodName,
		"created_at": sess.CreatedAt,
		"last_used":  sess.LastUsed,
	})
}

// HandleDeleteSession deletes a session and its associated pod.
func (h *Handlers) HandleDeleteSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	sess, exists := h.sessionMgr.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.podManager.DeletePod(ctx, sess.PodName); err != nil {
		h.logger.WithError(err).Error("Failed to delete pod")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod"})
		return
	}

	h.sessionMgr.DeleteSession(sessionID)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

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
	for _, ns := range namespaces.Items {
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

	// Use metav1.NamespaceAll to list pods from all namespaces
	// Handle both "*" and URL-encoded "%2A"
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
		"namespace":        namespace,
		"list_namespace":    listNamespace,
		"pods_count":       len(pods.Items),
	}).Info("Successfully listed pods")

	podList := make([]gin.H, 0, len(pods.Items))
	for _, pod := range pods.Items {
		// Get pod status
		status := string(pod.Status.Phase)
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				ready = true
				break
			}
		}

		podList = append(podList, gin.H{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"status":    status,
			"ready":     ready,
			"age":       time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
			"restarts":  getRestartCount(&pod),
		})
	}

	c.JSON(http.StatusOK, gin.H{"pods": podList})
}

// HandleListNodes lists all Kubernetes nodes.
func (h *Handlers) HandleListNodes(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	nodes, err := h.podManager.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		h.logger.WithError(err).Error("Failed to list nodes")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}

	h.logger.WithField("nodes_count", len(nodes.Items)).Info("Successfully listed nodes")

	nodeList := make([]gin.H, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		// Get node status
		ready := false
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				if condition.Status == v1.ConditionTrue {
					ready = true
					status = "Ready"
				} else {
					status = "NotReady"
				}
				break
			}
		}

		// Get internal IP
		internalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		// Get external IP (if available)
		externalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeExternalIP {
				externalIP = addr.Address
				break
			}
		}

		// Calculate uptime from creation timestamp and format as days/hours
		uptimeDuration := time.Since(node.CreationTimestamp.Time)
		days := int(uptimeDuration.Hours() / 24)
		hours := int(uptimeDuration.Hours()) % 24
		var uptimeStr string
		if days > 0 {
			uptimeStr = fmt.Sprintf("%dd %dh", days, hours)
		} else {
			uptimeStr = fmt.Sprintf("%dh", hours)
		}

		// Get node role from labels
		// In Kubernetes, control-plane/master labels often exist but have empty values
		// The presence of the label itself indicates the role
		role := "worker"
		if node.Labels != nil {
			// Check for control-plane role (newer Kubernetes versions)
			// Label exists (value can be empty or "true")
			if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
				role = "control-plane"
			} else if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
				// Check for master role (older Kubernetes versions)
				role = "master"
			} else if val, ok := node.Labels["kubernetes.io/role"]; ok && val != "" {
				// Check for generic role label
				role = val
			} else {
				// Check all labels for any node-role pattern
				for key := range node.Labels {
					if strings.HasPrefix(key, "node-role.kubernetes.io/") {
						parts := strings.Split(key, "/")
						if len(parts) > 1 {
							// Extract role from label key (e.g., "control-plane" from "node-role.kubernetes.io/control-plane")
							roleName := parts[1]
							if roleName != "" {
								role = roleName
								break
							}
						}
					}
				}
			}
		}

		// Get node info
		kubeletVersion := node.Status.NodeInfo.KubeletVersion
		osImage := node.Status.NodeInfo.OSImage
		containerRuntime := node.Status.NodeInfo.ContainerRuntimeVersion
		architecture := node.Status.NodeInfo.Architecture
		operatingSystem := node.Status.NodeInfo.OperatingSystem

		// Get CPU and memory capacity
		cpuCapacity := node.Status.Capacity[v1.ResourceCPU]
		memoryCapacity := node.Status.Capacity[v1.ResourceMemory]
		cpuAllocatable := node.Status.Allocatable[v1.ResourceCPU]
		memoryAllocatable := node.Status.Allocatable[v1.ResourceMemory]

		// Convert memory to GB
		memoryCapacityGB := float64(memoryCapacity.Value()) / (1024 * 1024 * 1024)
		memoryAllocatableGB := float64(memoryAllocatable.Value()) / (1024 * 1024 * 1024)

		// Get labels
		labels := make(map[string]string)
		if node.Labels != nil {
			for k, v := range node.Labels {
				labels[k] = v
			}
		}

		// Get taints
		taints := make([]string, 0)
		for _, taint := range node.Spec.Taints {
			taints = append(taints, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}

		// Ensure role is never empty
		if role == "" {
			role = "worker"
		}
		
		nodeList = append(nodeList, gin.H{
			"name":              node.Name,
			"status":            status,
			"ready":             ready,
			"role":              role,
			"internalIP":        internalIP,
			"externalIP":        externalIP,
			"uptime":            uptimeStr,
			"kubeletVersion":    kubeletVersion,
			"osImage":           osImage,
			"containerRuntime": containerRuntime,
			"architecture":      architecture,
			"operatingSystem":   operatingSystem,
			"cpuCapacity":       cpuCapacity.String(),
			"memoryCapacity":    fmt.Sprintf("%.2f GB", memoryCapacityGB),
			"cpuAllocatable":    cpuAllocatable.String(),
			"memoryAllocatable": fmt.Sprintf("%.2f GB", memoryAllocatableGB),
			"labels":            labels,
			"taints":            taints,
			"age":               time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
		})
		
		// Log role for debugging
		h.logger.WithFields(logrus.Fields{
			"node": node.Name,
			"role": role,
		}).Debug("Node role extracted")
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodeList})
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
		// For non-following logs, use a timeout
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Get pod to find container name if not provided
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

	// Use first container if not specified
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	// Parse tail lines
	tailLinesInt := int64(100)
	if tailLines != "" {
		if parsed, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
			tailLinesInt = parsed
		}
	}

	// Get logs
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
	
	// Stream logs to response
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
				// For follow mode, EOF might be temporary, continue reading
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

	// Upgrade to WebSocket
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

	// Set WebSocket close handler to cancel context
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

	// Get pod to find container name if not provided
	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
	}).Info("Fetching pod information")
	
	pod, err := h.podManager.GetClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		h.logger.WithError(err).Error("Failed to get pod")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Pod not found"))
		return
	}

	h.logger.WithFields(logrus.Fields{
		"pod":       podName,
		"namespace": namespace,
		"phase":     pod.Status.Phase,
	}).Info("Pod found, checking status")

	// Check if pod is running
	if pod.Status.Phase != v1.PodRunning {
		h.logger.WithField("phase", pod.Status.Phase).Error("Pod is not running")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("Pod is not running (phase: %s)", pod.Status.Phase)))
		return
	}

	// Use first container if not specified
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	if containerName == "" {
		h.logger.Error("No container found in pod")
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "No containers found in pod"))
		return
	}

	// Check container image to provide better error messages
	var containerImage string
	for _, container := range pod.Spec.Containers {
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

	// Stream exec - reuse terminal executor but with different namespace
	executor := terminal.NewExecutor(h.podManager.GetClient(), h.podManager.GetConfig(), namespace)
	if err := executor.StreamTerminal(ctx, ws, podName, containerName); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"pod":       podName,
			"namespace": namespace,
			"container": containerName,
		}).Error("Exec stream error")
		
		// Provide user-friendly error messages
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such file or directory") || strings.Contains(errMsg, "exec:") || strings.Contains(errMsg, "no shell found") {
			errMsg = fmt.Sprintf("Container has no shell (image: %s). Use 'kubectl debug' for distroless containers.", containerImage)
		}
		
		// Send clean error message to client before closing
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Exec error: %s", errMsg)))
	} else {
		h.logger.WithFields(logrus.Fields{
			"pod":       podName,
			"namespace": namespace,
			"container": containerName,
		}).Info("Exec stream completed successfully")
	}
}

// getRestartCount returns the total restart count for all containers in a pod.
func getRestartCount(pod *v1.Pod) int32 {
	var totalRestarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}
	return totalRestarts
}

func generateSessionID() string {
	return "session-" + time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
