package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xnu/mcp-core/internal/config"
	"github.com/0xnu/mcp-core/internal/registry"
	"github.com/0xnu/mcp-core/pkg/protocol"
)

func TestRegistry_StdioBackendLifecycle(t *testing.T) {
	reg, _ := startStdioBackendWithPython(t, "echo", "echo_server.py")
	defer func() { _ = reg.StopAll() }()

	t.Run("initial_state", func(t *testing.T) {
		backends := reg.All()
		if len(backends) != 1 {
			t.Fatalf("expected 1 backend, got %d", len(backends))
		}
		handle := backends[0]
		if handle.Name() != "echo" {
			t.Fatalf("expected name 'echo', got '%s'", handle.Name())
		}
		if !handle.IsHealthy() {
			t.Fatal("backend should be healthy after start")
		}
	})

	t.Run("tools", func(t *testing.T) {
		handle := reg.Get("echo")
		tools, err := handle.Tools()
		if err != nil {
			t.Fatalf("get tools: %v", err)
		}
		if len(tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(tools))
		}

		toolNames := map[string]bool{}
		for _, tool := range tools {
			toolNames[tool.Name] = true
		}
		if !toolNames["echo"] || !toolNames["add"] {
			t.Fatal("expected echo and add tools")
		}
	})

	t.Run("tool_calls", func(t *testing.T) {
		handle := reg.Get("echo")
		assertToolCall(t, handle, "echo", `{"text":"hello"}`, "Echo: hello")
		assertToolCall(t, handle, "add", `{"a":3,"b":4}`, "7")
	})
}

func startStdioBackendWithPython(tb testing.TB, name, fixture string) (*registry.Registry, context.Context) {
	tb.Helper()
	skipWithoutPython(tb)
	echoPath := fixturePath(tb, fixture)
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    name,
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 5 * time.Second,
			},
		},
	}
	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	tb.Cleanup(cancel)
	if err := reg.StartAll(ctx); err != nil {
		tb.Fatalf("start backends: %v", err)
	}
	return reg, ctx
}

func assertToolCall(tb testing.TB, handle *registry.BackendHandle, tool, args, expected string) {
	tb.Helper()
	req := protocol.NewToolCallRequest(json.RawMessage(`"1"`), tool, json.RawMessage(args))
	resp, err := handle.Forward(req)
	if err != nil {
		tb.Fatalf("forward %s: %v", tool, err)
	}
	if resp.Error != nil {
		tb.Fatalf("%s error: %s", tool, resp.Error.Message)
	}
	var result protocol.ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		tb.Fatalf("parse %s result: %v", tool, err)
	}
	if len(result.Content) == 0 {
		tb.Fatalf("%s: expected content", tool)
	}
	if result.Content[0].Text != expected {
		tb.Fatalf("%s: expected '%s', got '%s'", tool, expected, result.Content[0].Text)
	}
}

func TestRegistry_ToolNotFound(t *testing.T) {
	skipWithoutPython(t)

	echoPath := fixturePath(t, "echo_server.py")
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "echo",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 5 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = reg.StartAll(ctx)
	defer func() { _ = reg.StopAll() }()

	handle := reg.Get("echo")
	if handle == nil {
		t.Fatal("echo backend not found")
	}

	req := protocol.NewToolCallRequest(
		json.RawMessage(`"1"`),
		"nonexistent_tool",
		json.RawMessage(`{}`),
	)

	resp, err := handle.Forward(req)
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if resp != nil {
		t.Fatal("expected nil response on error")
	}
}

func TestRegistry_MultipleRequestsSequential(t *testing.T) {
	skipWithoutPython(t)

	echoPath := fixturePath(t, "echo_server.py")
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "echo",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 10 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_ = reg.StartAll(ctx)
	defer func() { _ = reg.StopAll() }()

	handle := reg.Get("echo")
	if handle == nil {
		t.Fatal("echo backend not found")
	}

	for i := range 5 {
		id := fmt.Sprintf(`"seq-%d"`, i)
		text := fmt.Sprintf(`{"text":"request %d"}`, i)
		req := protocol.NewToolCallRequest(
			json.RawMessage(id),
			"echo",
			json.RawMessage(text),
		)
		resp, err := handle.Forward(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if resp.Error != nil {
			t.Fatalf("request %d error: %s", i, resp.Error.Message)
		}
	}
}

func TestConfig_LoadAndValidate(t *testing.T) {
	yaml := `
hub:
  host: 127.0.0.1
  port: 9020
backends:
  - name: test
    type: stdio
    command: echo
    timeout: 5s
`
	cfg, err := config.LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Core.Port != 9020 {
		t.Fatalf("expected port 9020, got %d", cfg.Core.Port)
	}
	if len(cfg.Backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "test" {
		t.Fatalf("expected name 'test', got '%s'", cfg.Backends[0].Name)
	}
}

func TestConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "empty backends",
			yaml:    "hub:\n  host: 127.0.0.1\n  port: 9020\nbackends: []",
			wantErr: "at least one backend",
		},
		{
			name:    "missing command for stdio",
			yaml:    "hub:\n  host: 127.0.0.1\n  port: 9020\nbackends:\n  - name: test\n    type: stdio",
			wantErr: "command is required",
		},
		{
			name:    "invalid backend type",
			yaml:    "hub:\n  host: 127.0.0.1\n  port: 9020\nbackends:\n  - name: test\n    type: invalid\n    command: echo",
			wantErr: "type must be 'stdio' or 'sse'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.LoadBytes([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing '%s', got '%s'", tt.wantErr, err.Error())
			}
		})
	}
}

func TestProtocol_DetectRequest(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)
	if !protocol.IsRequest(data) {
		t.Fatal("expected request detection")
	}
	if protocol.IsResponse(data) {
		t.Fatal("not a response")
	}
	if protocol.IsNotification(data) {
		t.Fatal("not a notification")
	}
}

func TestProtocol_DetectNotification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if !protocol.IsNotification(data) {
		t.Fatal("expected notification detection")
	}
	if protocol.IsRequest(data) {
		t.Fatal("not a request")
	}
}

func TestProtocol_DetectResponse(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":"1","result":{}}`)
	if !protocol.IsResponse(data) {
		t.Fatal("expected response detection")
	}
	if protocol.IsRequest(data) {
		t.Fatal("not a request")
	}
}

func TestProtocol_ErrorResponse(t *testing.T) {
	resp := protocol.NewErrorResponse(json.RawMessage(`"1"`), -32601, "Method not found")
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected code -32601, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Method not found" {
		t.Fatalf("expected 'Method not found', got '%s'", resp.Error.Message)
	}
}

func TestProtocol_MarshalRoundtrip(t *testing.T) {
	req := protocol.NewRequest(json.RawMessage(`"1"`), "tools/list", nil)
	data, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := protocol.ParseRequest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Method != "tools/list" {
		t.Fatalf("expected 'tools/list', got '%s'", parsed.Method)
	}
}

func TestProtocol_ToolCallHelper(t *testing.T) {
	req := protocol.NewToolCallRequest(
		json.RawMessage(`"1"`),
		"echo",
		json.RawMessage(`{"text":"hi"}`),
	)
	if req.Method != "tools/call" {
		t.Fatalf("expected 'tools/call', got '%s'", req.Method)
	}
	var params protocol.CallToolRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("parse params: %v", err)
	}
	if params.Name != "echo" {
		t.Fatalf("expected 'echo', got '%s'", params.Name)
	}
}

func TestProtocol_ListToolsHelper(t *testing.T) {
	req := protocol.NewListToolsRequest(json.RawMessage(`"1"`), "")
	if req.Method != "tools/list" {
		t.Fatalf("expected 'tools/list', got '%s'", req.Method)
	}
}

func TestAggregatedTools(t *testing.T) {
	skipWithoutPython(t)

	echoPath := fixturePath(t, "echo_server.py")
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "echo-1",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 5 * time.Second,
			},
			{
				Name:    "echo-2",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 5 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_ = reg.StartAll(ctx)
	defer func() { _ = reg.StopAll() }()

	allTools := reg.AggregatedTools()

	if len(allTools) != 2 {
		t.Fatalf("expected 2 unique tools, got %d", len(allTools))
	}
}

func BenchmarkBackendForward(b *testing.B) {
	skipWithoutPython(b)

	echoPath := filepath.Join("..", "fixtures", "echo_server.py")
	if _, err := os.Stat(echoPath); os.IsNotExist(err) {
		echoPath = fixturePath(b, "echo_server.py")
	}

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "echo",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{echoPath},
				Timeout: 10 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = reg.StartAll(ctx)
	defer func() { _ = reg.StopAll() }()

	handle := reg.Get("echo")
	if handle == nil {
		b.Fatal("echo backend not found")
	}

	b.ResetTimer()

	for range b.N {
		req := protocol.NewToolCallRequest(
			json.RawMessage(`"bench"`),
			"echo",
			json.RawMessage(`{"text":"benchmark"}`),
		)
		resp, err := handle.Forward(req)
		if err != nil {
			b.Fatalf("forward: %v", err)
		}
		if resp.Error != nil {
			b.Fatalf("error: %s", resp.Error.Message)
		}
	}
}

func fixturePath(_ requireHelper, name string) string {
	path := filepath.Join("fixtures", name)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return filepath.Join("..", "..", "test", "integration", "fixtures", name)
}

func buildGoFixture(t requireHelper) string {
	srcPath := fixturePath(t, "echo_server.go")
	binPath := srcPath + ".bin"

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, srcPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Go fixture: %v\n%s", err, string(out))
	}

	return binPath
}

type requireHelper interface {
	Fatalf(format string, args ...any)
	Skip(args ...any)
}

func skipWithoutPython(t interface{ Skip(...any) }) {
	_, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found, skipping integration test")
	}
}

func skipWithoutNode(t interface{ Skip(...any) }) {
	_, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found, skipping integration test")
	}
}

func TestRegistry_GoBackend(t *testing.T) {
	goPath := buildGoFixture(t)
	defer os.Remove(goPath)

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "go-echo",
				Type:    "stdio",
				Command: goPath,
				Timeout: 5 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := reg.StartAll(ctx); err != nil {
		t.Fatalf("start Go backend: %v", err)
	}
	defer func() { _ = reg.StopAll() }()

	handle := reg.Get("go-echo")
	if handle == nil {
		t.Fatal("Go backend not found")
	}
	if !handle.IsHealthy() {
		t.Fatal("Go backend should be healthy")
	}

	tools, err := handle.Tools()
	if err != nil {
		t.Fatalf("Go list tools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("Go: expected 2 tools, got %d", len(tools))
	}

	req := protocol.NewToolCallRequest(
		json.RawMessage(`"g1"`),
		"echo",
		json.RawMessage(`{"text":"from Go"}`),
	)
	resp, err := handle.Forward(req)
	if err != nil {
		t.Fatalf("Go forward: %v", err)
	}
	var result protocol.ToolResult
	_ = json.Unmarshal(resp.Result, &result)
	if result.Content[0].Text != "Echo: from Go" {
		t.Fatalf("Go: expected 'Echo: from Go', got '%s'", result.Content[0].Text)
	}
}

func TestRegistry_JSBackend(t *testing.T) {
	skipWithoutNode(t)

	jsPath := fixturePath(t, "echo_server.js")
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "js-echo",
				Type:    "stdio",
				Command: "node",
				Args:    []string{jsPath},
				Timeout: 5 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := reg.StartAll(ctx); err != nil {
		t.Fatalf("start JS backend: %v", err)
	}
	defer func() { _ = reg.StopAll() }()

	handle := reg.Get("js-echo")
	if handle == nil {
		t.Fatal("JS backend not found")
	}
	assertBackendHealthy(t, handle)
	assertToolsCount(t, handle, 2)

	assertToolCall(t, handle, "echo", `{"text":"from JS"}`, "Echo: from JS")
	assertToolCall(t, handle, "add", `{"a":10,"b":20}`, "30")
}

func assertBackendHealthy(tb testing.TB, handle *registry.BackendHandle) {
	tb.Helper()
	if !handle.IsHealthy() {
		tb.Fatal("backend should be healthy")
	}
}

func assertToolsCount(tb testing.TB, handle *registry.BackendHandle, expected int) {
	tb.Helper()
	tools, err := handle.Tools()
	if err != nil {
		tb.Fatalf("list tools: %v", err)
	}
	if len(tools) != expected {
		tb.Fatalf("expected %d tools, got %d", expected, len(tools))
	}
}

func TestRegistry_MultiLanguageBackends(t *testing.T) {
	goPath := buildGoFixture(t)
	defer os.Remove(goPath)
	skipWithoutNode(t)
	skipWithoutPython(t)

	pyPath := fixturePath(t, "echo_server.py")
	jsPath := fixturePath(t, "echo_server.js")

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Name:    "go-echo",
				Type:    "stdio",
				Command: goPath,
				Timeout: 10 * time.Second,
			},
			{
				Name:    "py-echo",
				Type:    "stdio",
				Command: "python3",
				Args:    []string{pyPath},
				Timeout: 10 * time.Second,
			},
			{
				Name:    "js-echo",
				Type:    "stdio",
				Command: "node",
				Args:    []string{jsPath},
				Timeout: 10 * time.Second,
			},
		},
	}

	reg := registry.NewRegistry(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := reg.StartAll(ctx); err != nil {
		t.Fatalf("start backends: %v", err)
	}
	defer func() { _ = reg.StopAll() }()

	assertBackendCount(t, reg, 3)

	allTools := reg.AggregatedTools()
	if len(allTools) != 2 {
		t.Fatalf("expected 2 unique aggregated tools, got %d", len(allTools))
	}

	for _, name := range []string{"go-echo", "py-echo", "js-echo"} {
		assertMultiLangBackend(t, reg, name)
	}

	t.Logf("All %d language backends passed: Go, Python, JavaScript", reg.BackendCount())
}

func assertBackendCount(tb testing.TB, reg *registry.Registry, expected int) {
	tb.Helper()
	if reg.BackendCount() != expected {
		tb.Fatalf("expected %d backends, got %d", expected, reg.BackendCount())
	}
}

func assertMultiLangBackend(tb testing.TB, reg *registry.Registry, name string) {
	tb.Helper()
	handle := reg.Get(name)
	if handle == nil {
		tb.Fatalf("backend %s not found", name)
	}
	assertBackendHealthy(tb, handle)
	assertToolCall(tb, handle, "echo", `{"text":"multi-language"}`, "Echo: multi-language")
	assertToolCall(tb, handle, "add", `{"a":100,"b":200}`, "300")
}
