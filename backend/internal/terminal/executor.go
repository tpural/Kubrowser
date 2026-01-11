package terminal

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 120 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Executor handles terminal command execution via Kubernetes Exec API.
type Executor struct {
	client    kubernetes.Interface
	config    *rest.Config
	namespace string
}

// NewExecutor creates a new terminal executor.
func NewExecutor(client kubernetes.Interface, config *rest.Config, namespace string) *Executor {
	return &Executor{
		client:    client,
		config:    config,
		namespace: namespace,
	}
}

// StreamTerminal streams terminal I/O between WebSocket and Kubernetes pod.
func (e *Executor) StreamTerminal(ctx context.Context, ws *websocket.Conn, podName, containerName string) error {
	// Set WebSocket options
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Create exec request
	req := e.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(e.namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh"}, // Simple shell, no -i flag
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(e.config, "POST", req.URL())
	if err != nil {
		return err
	}

	// Create streams
	stdin := &stdinStream{ws: ws, sessionSent: false, buffer: nil, ctx: ctx}
	stdout := &stdoutStream{ws: ws}
	stderr := stdout

	// Start ping goroutine
	go e.pingTicker(ws)

	// Execute - this will block until the stream ends
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    true,
	})
	
	// Log when exec stream ends
	if err != nil {
		// Log the error type for debugging
		if err == context.Canceled {
			// Context was canceled, this is expected when WebSocket closes
		} else if err == io.EOF {
			// EOF is expected when connection closes
		}
	}
	
	// Handle common errors gracefully
	if err != nil {
		// Exit code 137 = SIGKILL (process killed) - this is often normal when connection closes
		if err.Error() == "command terminated with exit code 137" {
			return io.EOF // Treat as normal EOF
		}
		// Don't wrap context errors
		if err == context.Canceled || err == context.DeadlineExceeded {
			return err
		}
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("exec stream error: %w", err)
	}
	
	return nil
}

// pingTicker sends ping messages to keep the connection alive.
func (e *Executor) pingTicker(ws *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
			break
		}
	}
}

// stdinStream reads from WebSocket and writes to stdin.
type stdinStream struct {
	ws           *websocket.Conn
	sessionSent  bool
	buffer       []byte
	ctx          context.Context
}

func (s *stdinStream) Read(p []byte) (int, error) {
	// Check if context is canceled (browser disconnected)
	select {
	case <-s.ctx.Done():
		return 0, io.EOF
	default:
	}

	// First, drain any buffered data
	if len(s.buffer) > 0 {
		n := copy(p, s.buffer)
		s.buffer = s.buffer[n:]
		return n, nil
	}

	for {
		// Check context before each read
		select {
		case <-s.ctx.Done():
			return 0, io.EOF
		default:
		}

		// Set read deadline to detect closed connections quickly
		// Use a shorter deadline so we detect browser closes faster
		s.ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		
		messageType, message, err := s.ws.ReadMessage()
		if err != nil {
			// Check if context was canceled
			if s.ctx.Err() != nil {
				return 0, io.EOF
			}
			// Check if it's a close error (browser disconnected)
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return 0, io.EOF
			}
			// Any read error from WebSocket likely means connection is closed
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return 0, io.EOF
			}
			// For timeout errors, continue loop to check context
			if websocket.IsUnexpectedCloseError(err) {
				// Reset deadline and continue
				s.ws.SetReadDeadline(time.Time{})
				continue
			}
			// For other errors, check if it's a network/connection error
			if err == io.EOF || err.Error() == "use of closed network connection" {
				return 0, io.EOF
			}
			// For timeout, continue to check context
			if err.Error() == "i/o timeout" {
				continue
			}
			return 0, err
		}
		
		// Skip the first text message if it looks like session info JSON
		if messageType == websocket.TextMessage && !s.sessionSent {
			if len(message) > 0 && message[0] == '{' && len(message) < 200 {
				// Likely session info, skip it and mark as sent
				s.sessionSent = true
				continue
			}
		}
		
		// Process both text (from xterm) and binary messages as terminal input
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			n := copy(p, message)
			if n < len(message) {
				// Buffer remaining data for next read
				s.buffer = message[n:]
			}
			return n, nil
		}
	}
}

// stdoutStream reads from stdout/stderr and writes to WebSocket.
type stdoutStream struct {
	ws *websocket.Conn
}

func (s *stdoutStream) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.ws.SetWriteDeadline(time.Now().Add(writeWait))
	// Send as binary message for terminal output
	if err := s.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ResizeTerminal resizes the terminal TTY.
func (e *Executor) ResizeTerminal(ctx context.Context, podName, containerName string, width, height uint16) error {
	req := e.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(e.namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh"},
			Stdin:     false,
			Stdout:    false,
			Stderr:    false,
			TTY:       true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(e.config, "POST", req.URL())
	if err != nil {
		return err
	}

	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		TerminalSizeQueue: &terminalSizeQueue{
			width:  int(width),
			height: int(height),
		},
		Tty: true,
	})
}

// terminalSizeQueue implements remotecommand.TerminalSizeQueue.
type terminalSizeQueue struct {
	width  int
	height int
	sent   bool
}

func (t *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	if t.sent {
		return nil
	}
	t.sent = true
	return &remotecommand.TerminalSize{
		Width:  uint16(t.width),
		Height: uint16(t.height),
	}
}

// Ensure stdinStream implements io.Reader.
var _ io.Reader = (*stdinStream)(nil)

// Ensure stdoutStream implements io.Writer.
var _ io.Writer = (*stdoutStream)(nil)
