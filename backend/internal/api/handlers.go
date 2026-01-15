package api

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/kubrowser/kubrowser-backend/internal/k8s"
	"github.com/kubrowser/kubrowser-backend/internal/session"
	"github.com/kubrowser/kubrowser-backend/internal/terminal"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

// getRestartCount returns the total restart count for all containers in a pod.
func getRestartCount(pod *v1.Pod) int32 {
	var totalRestarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}
	return totalRestarts
}
