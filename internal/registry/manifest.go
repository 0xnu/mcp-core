package registry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/0xnu/mcp-core/internal/config"
	"github.com/0xnu/mcp-core/pkg/protocol"
)

type stdioClient struct {
	mu      sync.RWMutex
	cmd     *exec.Cmd
	cfg     config.BackendConfig
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	scanner *bufio.Scanner
	pending map[string]chan *protocol.Response
	timeout time.Duration
}

func newStdioClient(cfg config.BackendConfig) *stdioClient {
	return &stdioClient{
		cfg:     cfg,
		pending: make(map[string]chan *protocol.Response),
		timeout: cfg.Timeout,
	}
}

func (c *stdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cmd = exec.CommandContext(ctx, c.cfg.Command, c.cfg.Args...) //nolint:gosec // command from config, user-controlled

	for k, v := range c.cfg.Env {
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
		return fmt.Errorf("start process: %w", err)
	}

	c.scanner = bufio.NewScanner(c.stdout)
	c.scanner.Buffer(make([]byte, 65536), 65536)

	go c.readLoop()
	go c.logStderr()

	return nil
}

func (c *stdioClient) readLoop() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
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

func (c *stdioClient) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	scanner.Buffer(make([]byte, 65536), 65536)
	for scanner.Scan() {
		log.Printf("[backend:%s] %s", c.cfg.Name, scanner.Text())
	}
}

func (c *stdioClient) sendRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
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

	timeout := c.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
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

func generateID() json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`"%d"`, time.Now().UnixNano()))
}

func (c *stdioClient) ListTools(ctx context.Context) ([]protocol.ToolDefinition, error) {
	req := protocol.NewListToolsRequest(generateID(), "")
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("backend error: %s", resp.Error.Message)
	}
	var result protocol.ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse list_tools result: %w", err)
	}
	return result.Tools, nil
}

func (c *stdioClient) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolResult, error) {
	req := protocol.NewToolCallRequest(generateID(), name, args)
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tool error: %s", resp.Error.Message)
	}
	var result protocol.ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse call_tool result: %w", err)
	}
	return &result, nil
}

func (c *stdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

type DiscoveredManifest struct {
	Source     string   `json:"source"`
	ServerCmd  string   `json:"serverCmd"`
	ServerArgs []string `json:"serverArgs"`
}

func DiscoverFromConfig(_ string) (*DiscoveredManifest, error) {
	return nil, errors.New("discovery not yet implemented")
}

func ParseManifest(data []byte) (*DiscoveredManifest, error) {
	var manifest DiscoveredManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &manifest, nil
}

func DetectCommand(command string) (string, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("command not found: %s", command)
	}
	return path, nil
}

func (r *Registry) Reload(cfg *config.Config) error {
	if err := r.StopAll(); err != nil {
		return fmt.Errorf("stop backends: %w", err)
	}

	r.cfg = cfg

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.InitBackends(ctx); err != nil {
		return fmt.Errorf("reload backends: %w", err)
	}

	return nil
}

type BackendMetadata struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Healthy   bool      `json:"healthy"`
	ToolCount int       `json:"toolCount"`
	StartedAt time.Time `json:"startedAt"`
	ErrCount  int       `json:"errorCount"`
}

func (r *Registry) Metadata() []BackendMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta := make([]BackendMetadata, 0, len(r.backends))
	for _, h := range r.backends {
		h.mu.RLock()
		meta = append(meta, BackendMetadata{
			Name:      h.name,
			Type:      h.cfg.Type,
			Healthy:   h.healthy,
			ToolCount: len(h.tools),
			StartedAt: h.startedAt,
			ErrCount:  h.errCount,
		})
		h.mu.RUnlock()
	}
	return meta
}

func ParseCommandLine(input string) (string, []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
