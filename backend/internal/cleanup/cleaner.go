package cleanup

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/kubrowser/kubrowser-backend/internal/k8s"
	"github.com/kubrowser/kubrowser-backend/internal/session"
)

// Cleaner handles cleanup of stale pods and sessions.
type Cleaner struct {
	logger     *logrus.Logger
	podManager *k8s.PodManager
	sessionMgr *session.Manager
	stopChan   chan struct{}
	interval   time.Duration
	timeout    time.Duration
}

// NewCleaner creates a new cleanup service.
func NewCleaner(logger *logrus.Logger, podManager *k8s.PodManager, sessionMgr *session.Manager, interval, timeout time.Duration) *Cleaner {
	return &Cleaner{
		logger:     logger,
		podManager: podManager,
		sessionMgr: sessionMgr,
		interval:   interval,
		timeout:    timeout,
		stopChan:   make(chan struct{}),
	}
}

// Start starts the cleanup goroutine.
func (c *Cleaner) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup(ctx)
		case <-c.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the cleanup service.
func (c *Cleaner) Stop() {
	close(c.stopChan)
}

// cleanup performs the actual cleanup of stale sessions and pods.
func (c *Cleaner) cleanup(ctx context.Context) {
	c.logger.Debug("Starting cleanup cycle")

	deletedSessions := c.sessionMgr.CleanupStaleSessions(ctx, func(sessionID string) bool {
		sess, exists := c.sessionMgr.GetSession(sessionID)
		if !exists {
			return true
		}

		// Delete the pod associated with the session.
		deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := c.podManager.DeletePod(deleteCtx, sess.PodName); err != nil {
			c.logger.WithError(err).WithField("pod_name", sess.PodName).Error("Failed to delete pod during cleanup")
			return false
		}

		c.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"pod_name":   sess.PodName,
		}).Info("Cleaned up stale session and pod")

		return true
	})

	if len(deletedSessions) > 0 {
		c.logger.WithField("count", len(deletedSessions)).Info("Cleanup completed")
	}
}
