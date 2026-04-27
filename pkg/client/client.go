package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/0xnu/mcp-core/pkg/protocol"
)

type MCPClient struct {
	mu      sync.RWMutex
	config  ClientConfig
	backend BackendConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	reader  *bufio.Scanner
	pending map[string]chan *protocol.Response
	info    BackendInfo
	cancel  context.CancelFunc
}

func NewMCPClient(backend BackendConfig, opts ...ClientOption) *MCPClient {
	cfg := DefaultClientConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &MCPClient{
		config:  cfg,
		backend: backend,
		pending: make(map[string]chan *protocol.Response),
		info: BackendInfo{
			Config: backend,
			Status: StatusDisconnected,
		},
	}
}

func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, c.cancel = context.WithCancel(ctx)

	switch c.backend.Type {
	case BackendTypeStdio:
		return c.connectStdio(ctx)
	case BackendTypeSSE:
		return c.connectSSE(ctx)
	default:
		return fmt.Errorf("unsupported backend type: %s", c.backend.Type)
	}
}

func (c *MCPClient) connectStdio(ctx context.Context) error {
	c.cmd = exec.CommandContext(ctx, c.backend.Command, c.backend.Args...) //nolint:gosec // command from config, user-controlled

	for k, v := range c.backend.Env {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdout = stdout

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	c.stderr = stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start backend: %w", err)
	}

	c.reader = bufio.NewScanner(c.stdout)
	c.reader.Buffer(make([]byte, c.config.BufferSize), c.config.BufferSize)

	go c.readLoop()
	go c.logStderr()

	c.info.Status = StatusConnected
	c.info.ConnectedAt = time.Now()

	return nil
}

func (c *MCPClient) connectSSE(_ context.Context) error {
	return errors.New("SSE backend not yet implemented")
}

func (c *MCPClient) readLoop() {
	for c.reader.Scan() {
		line := c.reader.Bytes()
		if len(line) == 0 {
			continue
		}

		resp, err := protocol.ParseResponse(line)
		if err != nil {
			continue
		}

		c.mu.RLock()
		ch, ok := c.pending[string(resp.ID)]
		c.mu.RUnlock()

		if ok {
			ch <- resp
		}
	}
}

func (c *MCPClient) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("[backend:%s] %s", c.backend.Name, scanner.Text())
	}
}

func (c *MCPClient) SendRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	id := string(req.ID)
	ch := make(chan *protocol.Response, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	data, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data = append(data, '\n')

	c.mu.RLock()
	stdin := c.stdin
	c.mu.RUnlock()

	if stdin == nil {
		return nil, errors.New("backend not connected")
	}

	if _, err := stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	timeout := c.config.RequestTimeout
	if c.backend.Timeout > 0 {
		timeout = c.backend.Timeout
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout after %v", timeout)
	}
}

func (c *MCPClient) ListTools(ctx context.Context) ([]protocol.ToolDefinition, error) {
	id := json.RawMessage(fmt.Sprintf(`"%d"`, time.Now().UnixNano()))
	req := protocol.NewListToolsRequest(id, "")

	resp, err := c.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("backend error: %s", resp.Error.Message)
	}

	var result protocol.ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	c.mu.Lock()
	c.info.Tools = result.Tools
	c.mu.Unlock()

	return result.Tools, nil
}

func (c *MCPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolResult, error) {
	id := json.RawMessage(fmt.Sprintf(`"%d"`, time.Now().UnixNano()))
	req := protocol.NewToolCallRequest(id, name, args)

	resp, err := c.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tool error: %s", resp.Error.Message)
	}

	var result protocol.ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

func (c *MCPClient) Info() BackendInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info
}

func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill backend: %w", err)
		}
	}

	c.info.Status = StatusDisconnected

	return nil
}

func WithMaxRetries(n int) ClientOption {
	return func(c *ClientConfig) {
		c.MaxRetries = n
	}
}

func WithRequestTimeout(d time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.RequestTimeout = d
	}
}

func WithBufferSize(n int) ClientOption {
	return func(c *ClientConfig) {
		c.BufferSize = n
	}
}

type MultiClient struct {
	mu      sync.RWMutex
	clients map[string]*MCPClient
}

func NewMultiClient() *MultiClient {
	return &MultiClient{
		clients: make(map[string]*MCPClient),
	}
}

func (mc *MultiClient) Register(name string, client *MCPClient) {
	mc.mu.Lock()
	mc.clients[name] = client
	mc.mu.Unlock()
}

func (mc *MultiClient) Get(name string) (*MCPClient, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	c, ok := mc.clients[name]
	return c, ok
}

func (mc *MultiClient) All() []*MCPClient {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make([]*MCPClient, 0, len(mc.clients))
	for _, c := range mc.clients {
		result = append(result, c)
	}
	return result
}

func (mc *MultiClient) CloseAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	for name, c := range mc.clients {
		c.Close()
		delete(mc.clients, name)
	}
}

func ParseJSON(data []byte) json.RawMessage {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	return json.RawMessage(data)
}
