package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/0xnu/mcp-core/internal/config"
	"github.com/0xnu/mcp-core/pkg/protocol"
)

type BackendHandle struct {
	name      string
	cfg       config.BackendConfig
	client    backendClient
	tools     []protocol.ToolDefinition
	healthy   bool
	mu        sync.RWMutex
	cancel    context.CancelFunc
	startedAt time.Time
	errCount  int
}

type backendClient interface {
	Connect(ctx context.Context) error
	ListTools(ctx context.Context) ([]protocol.ToolDefinition, error)
	CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolResult, error)
	Close() error
}

type Registry struct {
	mu       sync.RWMutex
	backends map[string]*BackendHandle
	cfg      *config.Config
	baseCtx  context.Context
}

func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		backends: make(map[string]*BackendHandle),
		cfg:      cfg,
	}
}

func (r *Registry) SetBaseContext(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseCtx = ctx
}

func (r *Registry) InitBackends(ctx context.Context) error {
	for i := range r.cfg.Backends {
		bCfg := r.cfg.Backends[i]
		if err := r.AddBackend(ctx, bCfg); err != nil {
			log.Printf("failed to add backend %s: %v", bCfg.Name, err)
		}
	}
	return nil
}

func (r *Registry) AddBackend(ctx context.Context, cfg config.BackendConfig) error {
	cli := newStdioClient(cfg)

	ctx, cancel := context.WithCancel(ctx)

	if err := cli.Connect(ctx); err != nil {
		cancel()
		return fmt.Errorf("connect backend %s: %w", cfg.Name, err)
	}

	tools, err := cli.ListTools(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("list tools for %s: %w", cfg.Name, err)
	}

	handle := &BackendHandle{
		name:      cfg.Name,
		cfg:       cfg,
		client:    cli,
		tools:     tools,
		healthy:   true,
		cancel:    cancel,
		startedAt: time.Now(),
	}

	r.mu.Lock()
	r.backends[cfg.Name] = handle
	r.mu.Unlock()

	log.Printf("registered backend %s with %d tools", cfg.Name, len(tools))
	return nil
}

func (r *Registry) StartAll(ctx context.Context) error {
	return r.InitBackends(ctx)
}

func (r *Registry) StopAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, handle := range r.backends {
		if err := handle.Close(); err != nil {
			log.Printf("error closing backend %s: %v", name, err)
		}
		delete(r.backends, name)
	}
	return nil
}

func (r *Registry) Get(name string) *BackendHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.backends[name]
}

func (r *Registry) All() []*BackendHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*BackendHandle, 0, len(r.backends))
	for _, h := range r.backends {
		result = append(result, h)
	}
	return result
}

func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok := r.backends[name]; ok {
		h.Close()
		delete(r.backends, name)
		log.Printf("removed backend: %s", name)
	}
}

func (r *Registry) HealthCheckAll() {
	r.mu.RLock()
	handles := make([]*BackendHandle, 0, len(r.backends))
	for _, h := range r.backends {
		handles = append(handles, h)
	}
	r.mu.RUnlock()

	for _, h := range handles {
		h.HealthCheck()
	}
}

func (r *Registry) HealthCheckAndRestart() {
	r.mu.RLock()
	type handleEntry struct {
		handle *BackendHandle
		cfg    config.BackendConfig
	}
	entries := make([]handleEntry, 0, len(r.backends))
	for _, h := range r.backends {
		entries = append(entries, handleEntry{handle: h, cfg: h.cfg})
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		entry.handle.HealthCheck()

		if !entry.handle.IsHealthy() {
			log.Printf("auto-restarting unhealthy backend: %s", entry.handle.name)
			if err := r.restartBackend(entry.handle.name, entry.cfg); err != nil {
				log.Printf("auto-restart failed for %s: %v", entry.handle.name, err)
			}
		}
	}
}

func (r *Registry) restartBackend(name string, cfg config.BackendConfig) error {
	r.mu.Lock()
	existing, ok := r.backends[name]
	if ok {
		existing.Close()
		delete(r.backends, name)
	}
	r.mu.Unlock()

	baseCtx := context.Background()
	r.mu.RLock()
	if r.baseCtx != nil {
		baseCtx = r.baseCtx
	}
	r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(baseCtx, 15*time.Second)
	defer cancel()

	cli := newStdioClient(cfg)

	connCtx, connCancel := context.WithCancel(ctx)
	if err := cli.Connect(connCtx); err != nil {
		connCancel()
		return fmt.Errorf("reconnect: %w", err)
	}

	tools, err := cli.ListTools(connCtx)
	if err != nil {
		connCancel()
		return fmt.Errorf("list tools after restart: %w", err)
	}

	handle := &BackendHandle{
		name:      cfg.Name,
		cfg:       cfg,
		client:    cli,
		tools:     tools,
		healthy:   true,
		cancel:    connCancel,
		startedAt: time.Now(),
	}

	r.mu.Lock()
	r.backends[cfg.Name] = handle
	r.mu.Unlock()

	log.Printf("auto-restarted backend %s with %d tools", cfg.Name, len(tools))
	return nil
}

func (r *Registry) AggregatedTools() []protocol.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []protocol.ToolDefinition
	seen := make(map[string]bool)

	for _, handle := range r.backends {
		handle.mu.RLock()
		for _, tool := range handle.tools {
			if !seen[tool.Name] {
				all = append(all, tool)
				seen[tool.Name] = true
			}
		}
		handle.mu.RUnlock()
	}

	return all
}

func (r *Registry) BackendCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.backends)
}

func (bh *BackendHandle) Name() string {
	return bh.name
}

func (bh *BackendHandle) Tools() ([]protocol.ToolDefinition, error) {
	bh.mu.RLock()
	defer bh.mu.RUnlock()
	return bh.tools, nil
}

func (bh *BackendHandle) IsHealthy() bool {
	bh.mu.RLock()
	defer bh.mu.RUnlock()
	return bh.healthy
}

func (bh *BackendHandle) HealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := bh.client.ListTools(ctx)
	bh.mu.Lock()
	defer bh.mu.Unlock()

	if err != nil {
		bh.errCount++
		if bh.errCount >= 3 {
			bh.healthy = false
			log.Printf("backend %s marked unhealthy after %d failures", bh.name, bh.errCount)
		}
	} else {
		bh.errCount = 0
		bh.healthy = true
	}
}

func (bh *BackendHandle) Forward(req *protocol.Request) (*protocol.Response, error) {
	switch req.Method {
	case "tools/call":
		var callReq protocol.CallToolRequest
		if err := json.Unmarshal(req.Params, &callReq); err != nil {
			return nil, fmt.Errorf("parse call_tool params: %w", err)
		}
		result, err := bh.client.CallTool(context.Background(), callReq.Name, callReq.Arguments)
		if err != nil {
			return nil, err
		}
		resultJSON, _ := json.Marshal(result) //nolint:errchkjson
		return &protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      req.ID,
			Result:  resultJSON,
		}, nil

	case "tools/list":
		tools, err := bh.client.ListTools(context.Background())
		if err != nil {
			return nil, err
		}
		resultJSON, _ := json.Marshal(protocol.ListToolsResult{Tools: tools}) //nolint:errchkjson
		return &protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      req.ID,
			Result:  resultJSON,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}
}

func (bh *BackendHandle) Close() error {
	if bh.cancel != nil {
		bh.cancel()
	}
	return bh.client.Close()
}
