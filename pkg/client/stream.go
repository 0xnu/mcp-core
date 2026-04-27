package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

type SSEStream struct {
	events chan SSEEvent
	done   chan struct{}
}

func NewSSEStream() *SSEStream {
	return &SSEStream{
		events: make(chan SSEEvent, 256),
		done:   make(chan struct{}),
	}
}

func (s *SSEStream) Events() <-chan SSEEvent {
	return s.events
}

func (s *SSEStream) Done() <-chan struct{} {
	return s.done
}

func (s *SSEStream) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func ParseSSE(r io.Reader, out chan<- SSEEvent) error {
	scanner := bufio.NewScanner(r)
	var event SSEEvent

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if event.Data != "" {
				out <- event
			}
			event = SSEEvent{}
			continue
		}

		if after, ok := strings.CutPrefix(line, "event: "); ok {
			event.Event = after
		} else if after, ok := strings.CutPrefix(line, "data: "); ok {
			event.Data = after
		} else if after, ok := strings.CutPrefix(line, "id: "); ok {
			event.ID = after
		}
	}

	return scanner.Err()
}

type SSEClient struct {
	url    string
	client *http.Client
	events chan SSEEvent
	done   chan struct{}
}

func NewSSEClient(url string) *SSEClient {
	return &SSEClient{
		url:    url,
		client: &http.Client{},
		events: make(chan SSEEvent, 256),
		done:   make(chan struct{}),
	}
}

func (s *SSEClient) Connect() error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}
	resp, err := s.client.Do(req) //nolint:bodyclose // closed in goroutine
	if err != nil {
		return fmt.Errorf("connect SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	go func() {
		defer resp.Body.Close()
		_ = ParseSSE(resp.Body, s.events)
		close(s.done)
	}()

	return nil
}

func (s *SSEClient) Events() <-chan SSEEvent {
	return s.events
}

func (s *SSEClient) Done() <-chan struct{} {
	return s.done
}

func (s *SSEClient) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

type MessageWriter struct {
	mu       sync.Mutex
	endpoint string
	client   *http.Client
}

func NewMessageWriter(endpoint string) *MessageWriter {
	return &MessageWriter{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

func (w *MessageWriter) WriteMessage(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, w.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("post message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func EncodeSSEMessage(event string, data []byte) ([]byte, error) {
	var buf bytes.Buffer

	if event != "" {
		fmt.Fprintf(&buf, "event: %s\n", event)
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		fmt.Fprintf(&buf, "data: %s\n", line)
	}
	buf.WriteString("\n")

	return buf.Bytes(), nil
}

func DecodeSSEMessage(raw []byte) (*SSEEvent, error) {
	var event SSEEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))

	for scanner.Scan() {
		line := scanner.Text()

		if after, ok := strings.CutPrefix(line, "event: "); ok {
			event.Event = after
		} else if after, ok := strings.CutPrefix(line, "data: "); ok {
			event.Data = after
		} else if after, ok := strings.CutPrefix(line, "id: "); ok {
			event.ID = after
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &event, nil
}

func MarshalJSONEvent(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return data, nil
}
