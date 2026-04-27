package mux

import (
	"context"
	"sync"
)

type Pool struct {
	mu      sync.RWMutex
	workers map[string]*Worker
	maxSize int
}

type Worker struct {
	ID     string
	pool   *Pool
	ctx    context.Context
	cancel context.CancelFunc
}

func NewPool(maxSize int) *Pool {
	return &Pool{
		workers: make(map[string]*Worker),
		maxSize: maxSize,
	}
}

func (p *Pool) Acquire(id string) *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.workers[id]; ok {
		return w
	}

	if len(p.workers) >= p.maxSize {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored in Worker, called on Release
	w := &Worker{
		ID:     id,
		pool:   p,
		ctx:    ctx,
		cancel: cancel,
	}
	p.workers[id] = w
	return w
}

func (p *Pool) Release(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.workers[id]; ok {
		w.cancel()
		delete(p.workers, id)
	}
}

func (p *Pool) Get(id string) *Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workers[id]
}

func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

func (p *Pool) ReleaseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, w := range p.workers {
		w.cancel()
		delete(p.workers, id)
	}
}

type StreamPool struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStreamPool() *StreamPool {
	return &StreamPool{
		sessions: make(map[string]*Session),
	}
}

func (sp *StreamPool) Add(session *Session) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.sessions[session.ID] = session
}

func (sp *StreamPool) Remove(id string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	delete(sp.sessions, id)
}

func (sp *StreamPool) Get(id string) *Session {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.sessions[id]
}

func (sp *StreamPool) Count() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return len(sp.sessions)
}
