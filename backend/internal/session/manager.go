package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents a terminal session.
type Session struct {
	ID        string
	PodName   string
	CreatedAt time.Time
	LastUsed  time.Time
	UserID    string
	Active    bool   // Whether there's an active WebSocket connection
	ExecLock  bool   // Whether an exec is currently running
}

// Manager handles session tracking and management.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewManager creates a new session manager.
func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}
}

// CreateSession creates a new session.
func (m *Manager) CreateSession(podName, userID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:        uuid.New().String(),
		PodName:   podName,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		UserID:    userID,
		Active:    false,
		ExecLock:  false,
	}

	m.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if exists {
		session.LastUsed = time.Now()
	}
	return session, exists
}

// DeleteSession removes a session.
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
}

// ListSessions returns all sessions.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// SetActive marks a session as active (has WebSocket connection).
func (m *Manager) SetActive(sessionID string, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.Active = active
		if active {
			session.LastUsed = time.Now()
		} else {
			session.ExecLock = false
		}
	}
}

// TryLockExec attempts to lock the exec for a session. Returns true if successful.
func (m *Manager) TryLockExec(sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		if session.ExecLock {
			return false // Already locked
		}
		session.ExecLock = true
		return true
	}
	return false
}

// UnlockExec releases the exec lock for a session.
func (m *Manager) UnlockExec(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.ExecLock = false
	}
}

// CleanupStaleSessions removes sessions that have exceeded the timeout.
func (m *Manager) CleanupStaleSessions(ctx context.Context, shouldDelete func(sessionID string) bool) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var deleted []string
	now := time.Now()

	for id, session := range m.sessions {
		// Don't cleanup active sessions
		if session.Active {
			continue
		}
		if now.Sub(session.LastUsed) > m.timeout {
			if shouldDelete == nil || shouldDelete(id) {
				delete(m.sessions, id)
				deleted = append(deleted, id)
			}
		}
	}

	return deleted
}

// GetSessionByPodName finds a session by pod name.
func (m *Manager) GetSessionByPodName(podName string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if session.PodName == podName {
			session.LastUsed = time.Now()
			return session, true
		}
	}
	return nil, false
}
