package registry

import (
	"context"
	"log"
	"sync"
	"time"
)

type HealthChecker struct {
	mu          sync.RWMutex
	interval    time.Duration
	timeout     time.Duration
	maxFailures int
	failures    map[string]int
}

func NewHealthChecker(interval, timeout time.Duration, maxFailures int) *HealthChecker {
	return &HealthChecker{
		interval:    interval,
		timeout:     timeout,
		maxFailures: maxFailures,
		failures:    make(map[string]int),
	}
}

func (hc *HealthChecker) Start(ctx context.Context, backends []*BackendHandle) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.checkAll(ctx, backends)
		}
	}
}

func (hc *HealthChecker) checkAll(ctx context.Context, backends []*BackendHandle) {
	var wg sync.WaitGroup

	for _, bh := range backends {
		wg.Add(1)
		go func(handle *BackendHandle) {
			defer wg.Done()
			hc.checkBackend(ctx, handle)
		}(bh)
	}

	wg.Wait()
}

func (hc *HealthChecker) checkBackend(ctx context.Context, bh *BackendHandle) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	_, err := bh.client.ListTools(checkCtx)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	if err != nil {
		hc.failures[bh.name]++
		log.Printf("health check failed for %s (%d/%d): %v",
			bh.name, hc.failures[bh.name], hc.maxFailures, err)

		if hc.failures[bh.name] >= hc.maxFailures {
			bh.mu.Lock()
			bh.healthy = false
			bh.mu.Unlock()
			log.Printf("backend %s marked unhealthy", bh.name)
		}
	} else {
		hc.failures[bh.name] = 0
		bh.mu.Lock()
		bh.healthy = true
		bh.mu.Unlock()
	}
}

func (hc *HealthChecker) Status(name string) (int, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	failures, ok := hc.failures[name]
	if !ok {
		return 0, true
	}
	return failures, failures < hc.maxFailures
}

func (hc *HealthChecker) Reset(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.failures, name)
}

func (hc *HealthChecker) HealthyBackends(backends []*BackendHandle) []*BackendHandle {
	var healthy []*BackendHandle
	for _, bh := range backends {
		if bh.IsHealthy() {
			healthy = append(healthy, bh)
		}
	}
	return healthy
}
