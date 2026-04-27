package cache

import (
	"encoding/json"
	"sync"
	"time"
)

type ResponseCache struct {
	mu       sync.RWMutex
	entries  map[string]*ResponseEntry
	ttl      time.Duration
	maxSize  int
	maxEntry int
}

type ResponseEntry struct {
	Data      json.RawMessage
	CachedAt  time.Time
	ExpiresAt time.Time
	Hits      int64
}

type ResponseCacheConfig struct {
	TTL      time.Duration
	MaxSize  int
	MaxEntry int
}

func NewResponseCache(cfg ResponseCacheConfig) *ResponseCache {
	return &ResponseCache{
		entries:  make(map[string]*ResponseEntry),
		ttl:      cfg.TTL,
		maxSize:  cfg.MaxSize,
		maxEntry: cfg.MaxEntry,
	}
}

func (rc *ResponseCache) Get(key string) (json.RawMessage, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	entry, ok := rc.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(rc.entries, key)
		return nil, false
	}

	entry.Hits++
	return entry.Data, true
}

func (rc *ResponseCache) Set(key string, data json.RawMessage) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if len(data) > rc.maxEntry {
		return
	}

	if len(rc.entries) >= rc.maxSize {
		rc.evict()
	}

	now := time.Now()
	rc.entries[key] = &ResponseEntry{
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now.Add(rc.ttl),
	}
}

func (rc *ResponseCache) Invalidate(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.entries, key)
}

func (rc *ResponseCache) InvalidatePrefix(prefix string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for key := range rc.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(rc.entries, key)
		}
	}
}

func (rc *ResponseCache) evict() {
	var oldest string
	var oldestTime time.Time

	for key, entry := range rc.entries {
		if oldest == "" || entry.CachedAt.Before(oldestTime) {
			oldest = key
			oldestTime = entry.CachedAt
		}
	}

	if oldest != "" {
		delete(rc.entries, oldest)
	}
}

func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries = make(map[string]*ResponseEntry)
}

func (rc *ResponseCache) Stats() ResponseCacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	var totalHits int64
	var totalSize int
	for _, entry := range rc.entries {
		totalHits += entry.Hits
		totalSize += len(entry.Data)
	}

	return ResponseCacheStats{
		Entries:   len(rc.entries),
		TotalHits: totalHits,
		TotalSize: totalSize,
		MaxSize:   rc.maxSize,
	}
}

type ResponseCacheStats struct {
	Entries   int   `json:"entries"`
	TotalHits int64 `json:"totalHits"`
	TotalSize int   `json:"totalSize"`
	MaxSize   int   `json:"maxSize"`
}

func Key(method string, params json.RawMessage) string {
	data, _ := json.Marshal(map[string]any{ //nolint:errchkjson
		"method": method,
		"params": params,
	})
	return string(data)
}
