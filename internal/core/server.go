package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0xnu/mcp-core/internal/config"
)

type Server struct {
	httpServer *http.Server
	router     *Router
	cfg        *config.Config
	reloadFn   func(*config.Config) error
}

func NewServer(cfg *config.Config, router *Router) *Server {
	s := &Server{
		cfg:    cfg,
		router: router,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/drain", s.handleDrain)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	mux.HandleFunc("/", s.handleNotFound)

	s.httpServer = &http.Server{
		Addr:         cfg.ListenAddr(),
		Handler:      withLogging(mux),
		ReadTimeout:  cfg.Core.ReadTimeout,
		WriteTimeout: cfg.Core.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

func (s *Server) SetReloadHandler(fn func(*config.Config) error) {
	s.reloadFn = fn
}

func (s *Server) Start() error {
	addr := s.httpServer.Addr
	log.Printf("mcp-core listening on %s", addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Printf("shutting down mcp-core...")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSSEStream(w, r)
	case http.MethodPost:
		s.handleSSEMessage(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	endpoint := "/sse?session=" + sessionID
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpoint)
	flusher.Flush()

	stream := s.router.NewStream(sessionID, w, flusher)

	<-r.Context().Done()
	stream.Close()

	log.Printf("SSE stream closed: %s", sessionID)
}

func (s *Server) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")

	if sessionID == "" {
		http.Error(w, "session parameter required", http.StatusBadRequest)
		return
	}

	stream := s.router.GetStream(sessionID)
	if stream == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if err := stream.HandleMessage(r.Body); err != nil { //nolint:contextcheck // HandleMessage takes io.Reader, ctx available in stream
		log.Printf("message error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	router := s.router
	meta := struct {
		Status        string `json:"status"`
		Backends      int    `json:"backends"`
		Streams       int    `json:"streams"`
		Version       string `json:"version"`
		Requests      int64  `json:"requestsTotal"`
		Failed        int64  `json:"requestsFailed"`
		CircuitBreaks int64  `json:"circuitBreaks"`
	}{
		Status:        "ok",
		Backends:      router.BackendCount(),
		Streams:       router.StreamCount(),
		Version:       "0.1.0-dev",
		Requests:      router.telemetry.RequestsTotal,
		Failed:        router.telemetry.RequestsFailed,
		CircuitBreaks: router.telemetry.CircuitBreaks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"%s","version":"%s","backends":%d,"streams":%d,"requestsTotal":%d,"requestsFailed":%d,"circuitBreaks":%d}`,
		meta.Status, meta.Version, meta.Backends, meta.Streams, meta.Requests, meta.Failed, meta.CircuitBreaks)
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	router := s.router
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "# HELP mcp_hub_backends Number of registered backends\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_backends gauge\n")
	fmt.Fprintf(w, "mcp_hub_backends %d\n", router.BackendCount())

	fmt.Fprintf(w, "# HELP mcp_hub_streams Number of active SSE streams\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_streams gauge\n")
	fmt.Fprintf(w, "mcp_hub_streams %d\n", router.StreamCount())

	fmt.Fprintf(w, "# HELP mcp_hub_requests_total Total requests processed\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_requests_total counter\n")
	fmt.Fprintf(w, "mcp_hub_requests_total %d\n", router.telemetry.RequestsTotal)

	fmt.Fprintf(w, "# HELP mcp_hub_requests_failed Total failed requests\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_requests_failed counter\n")
	fmt.Fprintf(w, "mcp_hub_requests_failed %d\n", router.telemetry.RequestsFailed)

	fmt.Fprintf(w, "# HELP mcp_hub_circuit_breaks Total circuit breaker trips\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_circuit_breaks counter\n")
	fmt.Fprintf(w, "mcp_hub_circuit_breaks %d\n", router.telemetry.CircuitBreaks)

	fmt.Fprintf(w, "# HELP mcp_hub_uptime_seconds Server uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE mcp_hub_uptime_seconds gauge\n")
	fmt.Fprintf(w, "mcp_hub_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.reloadFn == nil {
		http.Error(w, `{"error":"reload not configured"}`, http.StatusNotImplemented)
		return
	}

	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"config load failed: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	if err := s.reloadFn(cfg); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"reload failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	s.cfg = cfg
	s.router.RebuildToolIndex()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","message":"config reloaded"}`)
}

func (s *Server) handleDrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Backend string `json:"backend"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid body: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	if req.Backend == "" {
		http.Error(w, `{"error":"backend name required"}`, http.StatusBadRequest)
		return
	}

	s.router.registry.Remove(req.Backend)
	s.router.RebuildToolIndex()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","message":"backend %s drained"}`, req.Backend)
}

func (s *Server) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"shutting down"}`)

	go func() { //nolint:contextcheck // no parent context available in goroutine
		time.Sleep(100 * time.Millisecond)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `{"error":"not found","path":"%s"}`, r.URL.Path) //nolint:gosec // XSS false positive for error response
}

var startTime = time.Now()

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start)) //nolint:gosec // log injection acceptable for local daemon
	})
}
