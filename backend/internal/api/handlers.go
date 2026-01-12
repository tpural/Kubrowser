package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kubrowser/kubrowser-backend/internal/k8s"
	"github.com/kubrowser/kubrowser-backend/internal/session"
	"github.com/kubrowser/kubrowser-backend/internal/terminal"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

	if sessionID != "" && reconnect {
		sess, exists = h.sessionMgr.GetSession(sessionID)
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
	} else {
		// Create new session
		pod, err := h.podManager.CreatePod(c.Request.Context(), generateSessionID())
		if err != nil {
			h.logger.WithError(err).Error("Failed to create pod")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod"})
			return
		}

		sess = h.sessionMgr.CreateSession(pod.Name, "anonymous") // TODO: Get from auth
		sessionID = sess.ID
	}

	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"pod_name":   sess.PodName,
	}).Info("WebSocket connection established")

	// Upgrade to WebSocket first (before locking, so we can send close message)
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}
	defer ws.Close()

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
