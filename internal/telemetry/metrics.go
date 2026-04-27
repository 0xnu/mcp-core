package telemetry

import (
	"expvar"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu              sync.RWMutex
	RequestsTotal   int64
	RequestsActive  int64
	RequestsFailed  int64
	RequestDuration *Histogram
	CacheHits       int64
	CacheMisses     int64
	BackendErrors   *expvar.Map
	ToolCalls       *expvar.Map
	startTime       time.Time
}

type Histogram struct {
	mu      sync.Mutex
	buckets []int64
	counts  []int64
	total   int64
}

func NewHistogram(buckets []int64) *Histogram {
	return &Histogram{
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1),
	}
}

func (h *Histogram) Record(value int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.total++

	for i, b := range h.buckets {
		if value <= b {
			h.counts[i]++
			return
		}
	}

	h.counts[len(h.counts)-1]++
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestDuration: NewHistogram([]int64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}),
		BackendErrors:   new(expvar.Map),
		ToolCalls:       new(expvar.Map),
		startTime:       time.Now(),
	}
}

func (m *Metrics) IncRequests() {
	atomic.AddInt64(&m.RequestsTotal, 1)
}

func (m *Metrics) IncActive() {
	atomic.AddInt64(&m.RequestsActive, 1)
}

func (m *Metrics) DecActive() {
	atomic.AddInt64(&m.RequestsActive, -1)
}

func (m *Metrics) IncFailed() {
	atomic.AddInt64(&m.RequestsFailed, 1)
}

func (m *Metrics) RecordDuration(d time.Duration) {
	m.RequestDuration.Record(d.Milliseconds())
}

func (m *Metrics) IncCacheHit() {
	atomic.AddInt64(&m.CacheHits, 1)
}

func (m *Metrics) IncCacheMiss() {
	atomic.AddInt64(&m.CacheMisses, 1)
}

func (m *Metrics) RecordToolCall(tool string) {
	if m.ToolCalls != nil {
		m.ToolCalls.Add(tool, 1)
	}
}

func (m *Metrics) RecordBackendError(backend string) {
	if m.BackendErrors != nil {
		m.BackendErrors.Add(backend, 1)
	}
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MetricsSnapshot{
		RequestsTotal:  atomic.LoadInt64(&m.RequestsTotal),
		RequestsActive: atomic.LoadInt64(&m.RequestsActive),
		RequestsFailed: atomic.LoadInt64(&m.RequestsFailed),
		CacheHits:      atomic.LoadInt64(&m.CacheHits),
		CacheMisses:    atomic.LoadInt64(&m.CacheMisses),
		Uptime:         time.Since(m.startTime),
	}
}

type MetricsSnapshot struct {
	RequestsTotal  int64         `json:"requestsTotal"`
	RequestsActive int64         `json:"requestsActive"`
	RequestsFailed int64         `json:"requestsFailed"`
	CacheHits      int64         `json:"cacheHits"`
	CacheMisses    int64         `json:"cacheMisses"`
	Uptime         time.Duration `json:"uptime"`
}

type Counter struct {
	name  string
	value atomic.Int64
}

func NewCounter(name string) *Counter {
	return &Counter{name: name}
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Add(n int64) {
	c.value.Add(n)
}

func (c *Counter) Value() int64 {
	return c.value.Load()
}
