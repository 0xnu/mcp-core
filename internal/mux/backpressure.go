package mux

import (
	"sync"
	"sync/atomic"
)

type CircuitBreaker struct {
	mu        sync.RWMutex
	failures  atomic.Int64
	threshold int
	state     CircuitState
}

type CircuitState int

const (
	StateClosed   CircuitState = 0
	StateOpen     CircuitState = 1
	StateHalfOpen CircuitState = 2
)

func NewCircuitBreaker(threshold int) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		state:     StateClosed,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures.Store(0)
	cb.state = StateClosed
}

func (cb *CircuitBreaker) Failure() {
	failures := cb.failures.Add(1)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if failures >= int64(cb.threshold) {
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures.Store(0)
	cb.state = StateClosed
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

type RateLimiter struct {
	mu     sync.Mutex
	rate   int
	burst  int
	tokens int
}

func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{
		rate:   rate,
		burst:  burst,
		tokens: burst,
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

type BackpressureManager struct {
	breakers map[string]*CircuitBreaker
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

func NewBackpressureManager() *BackpressureManager {
	return &BackpressureManager{
		breakers: make(map[string]*CircuitBreaker),
		limiters: make(map[string]*RateLimiter),
	}
}

func (bm *BackpressureManager) GetBreaker(name string) *CircuitBreaker {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if cb, ok := bm.breakers[name]; ok {
		return cb
	}

	cb := NewCircuitBreaker(5)
	bm.breakers[name] = cb
	return cb
}

func (bm *BackpressureManager) GetRateLimiter(name string) *RateLimiter {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if rl, ok := bm.limiters[name]; ok {
		return rl
	}

	rl := NewRateLimiter(100, 200)
	bm.limiters[name] = rl
	return rl
}

func (bm *BackpressureManager) Remove(name string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.breakers, name)
	delete(bm.limiters, name)
}
