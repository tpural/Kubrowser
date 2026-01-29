package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kubrowser/kubrowser-backend/internal/session"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

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

		// Get username from authentication context
		username, exists := c.Get("user")
		if !exists {
			// This shouldn't happen since auth middleware sets it, but fallback just in case
			username = "anonymous"
		}
		usernameStr, ok := username.(string)
		if !ok {
			usernameStr = "anonymous"
		}

		newSessionID := generateSessionID()
		pod, err := h.podManager.CreatePodWithStatus(c.Request.Context(), newSessionID, usernameStr, startTime, func(status string) {
			sendStatusUpdate(status)
		})
		if err != nil {
			h.logger.WithError(err).Error("Failed to create pod")
			duration := time.Since(startTime)
			sendStatusUpdate(fmt.Sprintf("\r\n\x1b[31m[✗] Failed to create pod: %s (took %v)\x1b[0m\r\n", err.Error(), duration.Round(time.Millisecond)))
			ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Failed to create pod"))
			return
		}

		sess = h.sessionMgr.CreateSession(pod.Name, usernameStr)
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
		// DO NOT delete pod here. We want it to persist for reconnection.
		// The background reaper will clean it up if not used.
		h.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"pod_name":   sess.PodName,
		}).Info("Handler function exiting")
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

		// Update heartbeat one last time before disconnect
		if err := h.podManager.UpdatePodHeartbeat(context.Background(), sess.PodName); err != nil {
			h.logger.WithError(err).Warn("Failed to update heartbeat on disconnect")
		}

		cancel()
		return nil
	})

	// Monitor WebSocket connection state by checking write capability
	// This doesn't interfere with reads (which happen in stdinStream)
	go func() {
		// Wait a bit before starting pings to allow session message to be sent first
		time.Sleep(500 * time.Millisecond)

		ticker := time.NewTicker(1 * time.Second)
		heartbeatTicker := time.NewTicker(30 * time.Second) // Heartbeat every 30s
		defer ticker.Stop()
		defer heartbeatTicker.Stop()

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
			case <-heartbeatTicker.C:
				// Keep pod alive
				if err := h.podManager.UpdatePodHeartbeat(ctx, sess.PodName); err != nil {
					h.logger.WithError(err).Warn("Failed to update pod heartbeat")
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

	containerName := "terminal"

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
	}).Info("WebSocket disconnected")

	// We no longer delete the pod here. It will be reaped if not reconnected.

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

	if err := h.terminalExec.ResizeTerminal(ctx, sess.PodName, "terminal", req.Width, req.Height); err != nil {
		h.logger.WithError(err).Error("Failed to resize terminal")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resize terminal"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
