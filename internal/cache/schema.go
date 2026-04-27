package cache

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/0xnu/mcp-core/pkg/protocol"
)

type SchemaCache struct {
	mu       sync.RWMutex
	entries  map[string]*SchemaEntry
	ttl      time.Duration
	maxSize  int
	maxEntry int
}

type SchemaEntry struct {
	Tools     []protocol.ToolDefinition
	CachedAt  time.Time
	ExpiresAt time.Time
}

func NewSchemaCache(ttl time.Duration, maxSize, maxEntry int) *SchemaCache {
	return &SchemaCache{
		entries:  make(map[string]*SchemaEntry),
		ttl:      ttl,
		maxSize:  maxSize,
		maxEntry: maxEntry,
	}
}

func (sc *SchemaCache) Get(backend string) ([]protocol.ToolDefinition, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, ok := sc.entries[backend]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	result := make([]protocol.ToolDefinition, len(entry.Tools))
	copy(result, entry.Tools)
	return result, true
}

func (sc *SchemaCache) Set(backend string, tools []protocol.ToolDefinition) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if len(sc.entries) >= sc.maxSize {
		sc.evict()
	}

	now := time.Now()
	entrySize := estimateSize(tools)
	if entrySize > sc.maxEntry {
		return
	}

	sc.entries[backend] = &SchemaEntry{
		Tools:     tools,
		CachedAt:  now,
		ExpiresAt: now.Add(sc.ttl),
	}
}

func (sc *SchemaCache) Invalidate(backend string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.entries, backend)
}

func (sc *SchemaCache) InvalidateAll() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = make(map[string]*SchemaEntry)
}

func (sc *SchemaCache) evict() {
	var oldest string
	var oldestTime time.Time

	for key, entry := range sc.entries {
		if oldest == "" || entry.CachedAt.Before(oldestTime) {
			oldest = key
			oldestTime = entry.CachedAt
		}
	}

	if oldest != "" {
		delete(sc.entries, oldest)
	}
}

func (sc *SchemaCache) Stats() CacheStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var totalSize int
	for _, entry := range sc.entries {
		totalSize += estimateSize(entry.Tools)
	}

	return CacheStats{
		Entries:   len(sc.entries),
		TotalSize: totalSize,
		MaxSize:   sc.maxSize,
	}
}

func estimateSize(tools []protocol.ToolDefinition) int {
	data, _ := json.Marshal(tools) //nolint:errchkjson
	return len(data)
}

type CacheStats struct {
	Entries   int `json:"entries"`
	TotalSize int `json:"totalSize"`
	MaxSize   int `json:"maxSize"`
}
