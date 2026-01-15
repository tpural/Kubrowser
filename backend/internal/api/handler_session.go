package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

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
