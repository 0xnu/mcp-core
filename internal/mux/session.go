package mux

import (
	"sync"
	"time"
)

type Session struct {
	ID        string
	Backend   string
	CreatedAt time.Time
	LastUsed  time.Time
	mu        sync.RWMutex
	metadata  map[string]string
}

func NewSession(id, backend string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Backend:   backend,
		CreatedAt: now,
		LastUsed:  now,
		metadata:  make(map[string]string),
	}
}

func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastUsed = time.Now()
}

func (s *Session) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
}

func (s *Session) GetMeta(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.metadata[key]
	return v, ok
}

func (s *Session) Age() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.CreatedAt)
}

func (s *Session) Idle() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastUsed)
}

type SessionAffinity struct {
	mu       sync.RWMutex
	bindings map[string]string
}

func NewSessionAffinity() *SessionAffinity {
	return &SessionAffinity{
		bindings: make(map[string]string),
	}
}

func (sa *SessionAffinity) Bind(sessionID, backend string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.bindings[sessionID] = backend
}

func (sa *SessionAffinity) Lookup(sessionID string) (string, bool) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	backend, ok := sa.bindings[sessionID]
	return backend, ok
}

func (sa *SessionAffinity) Unbind(sessionID string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	delete(sa.bindings, sessionID)
}

func (sa *SessionAffinity) UnbindBackend(backend string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	for sid, be := range sa.bindings {
		if be == backend {
			delete(sa.bindings, sid)
		}
	}
}

func (sa *SessionAffinity) Count() int {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return len(sa.bindings)
}
