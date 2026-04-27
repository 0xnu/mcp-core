package cache

import "sync"

type InvalidationPolicy string

const (
	PolicyOnBackendChange InvalidationPolicy = "on_backend_change"
	PolicyOnToolCall      InvalidationPolicy = "on_tool_call"
	PolicyTTLOnly         InvalidationPolicy = "ttl_only"
)

type InvalidationManager struct {
	mu       sync.RWMutex
	schema   *SchemaCache
	response *ResponseCache
	policies map[string]InvalidationPolicy
}

func NewInvalidationManager(schema *SchemaCache, response *ResponseCache) *InvalidationManager {
	return &InvalidationManager{
		schema:   schema,
		response: response,
		policies: make(map[string]InvalidationPolicy),
	}
}

func (im *InvalidationManager) SetPolicy(backend string, policy InvalidationPolicy) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.policies[backend] = policy
}

func (im *InvalidationManager) OnBackendChange(backend string) {
	im.schema.Invalidate(backend)
	im.response.InvalidatePrefix(backend)
}

func (im *InvalidationManager) OnToolCall(backend, tool string) {
	im.response.InvalidatePrefix(backend + ":" + tool)
}

func (im *InvalidationManager) OnConfigReload() {
	im.schema.InvalidateAll()
	im.response.Clear()
}

type CacheWarmer struct {
	manager *InvalidationManager
}

func NewCacheWarmer(manager *InvalidationManager) *CacheWarmer {
	return &CacheWarmer{
		manager: manager,
	}
}

func (cw *CacheWarmer) WarmSchema(_ string, _ any) {
}
