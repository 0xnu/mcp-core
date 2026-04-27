package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0xnu/mcp-core/pkg/protocol"
)

type Proxy struct {
	client  *http.Client
	timeout time.Duration
}

func NewProxy(timeout time.Duration) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

func (p *Proxy) ForwardToSSE(ctx context.Context, url string, req *protocol.Request) (*protocol.Response, error) {
	data, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return protocol.ParseResponse(body)
}

func (p *Proxy) ForwardToStdio(_ context.Context, stdin io.Writer, stdout io.Reader, req *protocol.Request) (*protocol.Response, error) {
	data, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data = append(data, '\n')

	if _, err := stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	buf := make([]byte, 65536)
	n, err := stdout.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return protocol.ParseResponse(buf[:n])
}

func ConvertToolCall(raw json.RawMessage) (*protocol.CallToolRequest, error) {
	var call protocol.CallToolRequest
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, err
	}
	return &call, nil
}

func ConvertListTools(raw json.RawMessage) (*protocol.ListToolsRequest, error) {
	var list protocol.ListToolsRequest
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return &list, nil
}
