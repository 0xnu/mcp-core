package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/0xnu/mcp-core/internal/mux"
	"github.com/0xnu/mcp-core/internal/registry"
	"github.com/0xnu/mcp-core/pkg/protocol"
)

type Router struct {
	mu           sync.RWMutex
	registry     *registry.Registry
	streams      map[string]*Stream
	toolIndex    map[string]string
	backpressure *mux.BackpressureManager
	telemetry    *RouterTelemetry
}

type RouterTelemetry struct {
	RequestsTotal  int64
	RequestsFailed int64
	CircuitBreaks  int64
}

func NewRouter(reg *registry.Registry) *Router {
	return &Router{
		registry:     reg,
		streams:      make(map[string]*Stream),
		toolIndex:    make(map[string]string),
		backpressure: mux.NewBackpressureManager(),
		telemetry:    &RouterTelemetry{},
	}
}

func (r *Router) RebuildToolIndex() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.toolIndex = make(map[string]string)

	for _, be := range r.registry.All() {
		tools, err := be.Tools()
		if err != nil {
			continue
		}
		for _, tool := range tools {
			if _, exists := r.toolIndex[tool.Name]; !exists {
				r.toolIndex[tool.Name] = be.Name()
			}
		}
	}
}

func (r *Router) Route(toolName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	backend, ok := r.toolIndex[toolName]
	return backend, ok
}

func (r *Router) NewStream(id string, w http.ResponseWriter, flusher http.Flusher) *Stream {
	r.mu.Lock()
	defer r.mu.Unlock()

	stream := NewStream(id, w, flusher, r)
	r.streams[id] = stream
	return stream
}

func (r *Router) GetStream(id string) *Stream {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.streams[id]
}

func (r *Router) RemoveStream(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, id)
}

func (r *Router) BackendCount() int {
	return len(r.registry.All())
}

func (r *Router) StreamCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.streams)
}

func (r *Router) Dispatch(ctx *RequestContext) {
	r.telemetry.RequestsTotal++

	var toolName string

	switch ctx.Method {
	case "tools/call":
		var callReq protocol.CallToolRequest
		if err := json.Unmarshal(ctx.Params, &callReq); err != nil {
			r.telemetry.RequestsFailed++
			ctx.RespondError(protocol.ErrorObject{
				Code:    -32602,
				Message: "invalid params: " + err.Error(),
			})
			return
		}
		toolName = callReq.Name

	default:
		toolName = ctx.Method
	}

	backendName, ok := r.Route(toolName)
	if !ok {
		r.telemetry.RequestsFailed++
		ctx.RespondError(protocol.ErrorObject{
			Code:    -32601,
			Message: "tool not found: " + toolName,
		})
		return
	}

	breaker := r.backpressure.GetBreaker(backendName)
	if !breaker.Allow() {
		r.telemetry.CircuitBreaks++
		r.telemetry.RequestsFailed++
		ctx.RespondError(protocol.ErrorObject{
			Code:    -32000,
			Message: fmt.Sprintf("backend %s is circuit-broken", backendName),
		})
		return
	}

	be := r.registry.Get(backendName)
	if be == nil {
		r.telemetry.RequestsFailed++
		breaker.Failure()
		ctx.RespondError(protocol.ErrorObject{
			Code:    -32000,
			Message: "backend unavailable: " + backendName,
		})
		return
	}

	if !be.IsHealthy() {
		r.telemetry.RequestsFailed++
		breaker.Failure()
		ctx.RespondError(protocol.ErrorObject{
			Code:    -32000,
			Message: "backend unhealthy: " + backendName,
		})
		go be.HealthCheck()
		return
	}

	resp, err := be.Forward(ctx.RawRequest)
	if err != nil {
		r.telemetry.RequestsFailed++
		breaker.Failure()
		log.Printf("forward error to %s: %v", backendName, err)
		ctx.RespondError(protocol.ErrorObject{
			Code:    -32000,
			Message: "backend error: " + err.Error(),
		})
		return
	}

	breaker.Success()
	ctx.Respond(resp)
}

type RequestContext struct {
	ID         json.RawMessage
	Method     string
	Params     json.RawMessage
	RawRequest *protocol.Request
	stream     *Stream
}

func NewRequestContext(req *protocol.Request, stream *Stream) *RequestContext {
	return &RequestContext{
		ID:         req.ID,
		Method:     req.Method,
		Params:     req.Params,
		RawRequest: req,
		stream:     stream,
	}
}

func (ctx *RequestContext) Respond(resp *protocol.Response) {
	ctx.stream.SendResponse(resp)
}

func (ctx *RequestContext) RespondError(err protocol.ErrorObject) {
	resp := &protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      ctx.ID,
		Error:   &err,
	}
	ctx.stream.SendResponse(resp)
}

func (ctx *RequestContext) RespondResult(result json.RawMessage) {
	resp := &protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      ctx.ID,
		Result:  result,
	}
	ctx.stream.SendResponse(resp)
}

type Stream struct {
	mu      sync.Mutex
	id      string
	writer  io.Writer
	flusher http.Flusher
	router  *Router
	closed  bool
}

func NewStream(id string, w io.Writer, flusher http.Flusher, router *Router) *Stream {
	return &Stream{
		id:      id,
		writer:  w,
		flusher: flusher,
		router:  router,
	}
}

func (s *Stream) HandleMessage(body io.ReadCloser) error {
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if !protocol.IsRequest(data) {
		return nil
	}

	req, err := protocol.ParseRequest(data)
	if err != nil {
		return fmt.Errorf("parse request: %w", err)
	}

	ctx := NewRequestContext(req, s)

	if req.Method == "tools/list" {
		allTools := s.router.registry.AggregatedTools()
		ctx.RespondResult(mustMarshal(protocol.ListToolsResult{Tools: allTools}))
		return nil
	}

	s.router.Dispatch(ctx)
	return nil
}

func (s *Stream) SendResponse(resp *protocol.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	data, err := resp.Marshal()
	if err != nil {
		log.Printf("marshal response: %v", err)
		return
	}

	data = append(data, '\n')

	_, err = s.writer.Write(data)
	if err != nil {
		log.Printf("write response: %v", err)
		return
	}

	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.router.RemoveStream(s.id)
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
