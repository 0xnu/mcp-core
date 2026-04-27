# Architecture

## Overview

mcp-core is a transparent SSE proxy that sits between MCP clients and backend MCP servers. Clients connect to a single SSE endpoint; mcp-core routes requests to the appropriate backend, merges tool schemas, and provides unified observability.

## Components

```
┌─────────────┐     ┌──────────┐     ┌──────────────────┐
│ MCP Client  │────▶│ mcp-core │────▶│ MCP Server (Go)  │
│ (Claude,    │     │ :9020    │     ├──────────────────┤
│  Cursor,    │     │          │     │ MCP Server (Py)  │
│  VS Code)   │     │          │     ├──────────────────┤
│             │     │          │     │ MCP Server (JS)  │
└─────────────┘     └──────────┘     └──────────────────┘
```

### Gateway Layer (`internal/core/`)
- **Server**: HTTP/SSE listener on port 9020. Handles SSE stream creation, message dispatch. Serves endpoints at `/health`, `/metrics`, `/reload`, `/drain`, `/shutdown`.
- **Router**: Maintains a tool-to-backend index. Routes `tools/call` by extracting the tool name from request params. Wires circuit breakers into every dispatch.
- **RouterTelemetry**: Tracks total requests, failed requests, and circuit breaker trips — exposed via `/health` and `/metrics`.
- **Stream**: Represents a single SSE connection. Receives JSON-RPC messages via POST, sends responses via SSE.
- **Lifecycle**: Coordinates startup, graceful shutdown, signal handling, and periodic health checks with auto-restart.

### Registry Layer (`internal/registry/`)
- **Registry**: Manages backend handles. Supports add, remove, get, start all, stop all.
- **BackendHandle**: Wraps a single MCP server process. Tracks health, tool list, error count, connection time.
- **Health Checker**: Periodically pings backends via `tools/list`. Marks unhealthy after N consecutive failures. `HealthCheckAndRestart()` also calls `restartBackend()` which kills and respawns failed processes.

### Mux Layer (`internal/mux/`)
- **Pool**: Worker pool with max size. Acquire/release workers by ID.
- **Session**: Tracks session-to-backend affinity, idle time, metadata.
- **Circuit Breaker**: Three-state (closed/open/half-open) per-backend breaker. Trips after threshold failures. Called on every dispatch in the router.
- **BackpressureManager**: Manages breakers and rate limiters per backend.

### Cache Layer (`internal/cache/`)
- **SchemaCache**: TTL-based cache for tool schemas per backend. LRU eviction.
- **ResponseCache**: TTL-based cache for tool call responses. Hit tracking.
- **InvalidationManager**: Coordinates cache invalidation on backend changes or config reloads.

### Auth Layer (`internal/auth/`)
- **TokenExtractor**: Supports Bearer token, X-API-Key header, query parameter.
- **PolicyEngine**: Per-backend allow/deny/restricted policies.
- **ACL**: Tool-level access control lists.

### Telemetry Layer (`internal/telemetry/`)
- **Metrics**: Request counters, latency histogram, cache hit/miss tracking.
- **Traces**: Span-based tracing with context propagation.
- **Logging**: Structured JSON logger with configurable levels (debug, info, warn, error, fatal).

## HTTP Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/sse` | GET | Open SSE stream, receive `event: endpoint` |
| `/sse?session=` | POST | Send JSON-RPC message to a session |
| `/health` | GET | JSON status: backends, streams, version, request stats, circuit breaks |
| `/metrics` | GET | Prometheus text format metrics |
| `/reload` | POST | Hot-reload config, restart backends, rebuild tool index |
| `/drain` | POST | Remove a backend by name from rotation |
| `/shutdown` | POST | Graceful server shutdown |

## Data Flow

1. Client opens `GET /sse` and receives `event: endpoint` with a session-specific POST URL.
2. Client sends JSON-RPC requests via `POST /sse?session=...`.
3. mcp-core parses the request, extracts the method/tool name, checks circuit breaker, routes to the correct backend.
4. Backend processes the request, sends response via stdout.
5. mcp-core reads the response, sends it back through the SSE stream.
6. The `tools/list` method is handled by the core itself. It aggregates schemas from all backends.

## Resilience

- **Health checks** run every 15 seconds via `HealthCheckAndRestart()`.
- **Auto-restart**: unhealthy backends are killed and respawned with fresh config.
- **Circuit breaker**: a per-backend breaker trips after 5 consecutive failures, returning fast-fail until successful requests reset it.
- **Graceful shutdown**: SIGINT/SIGTERM triggers `Shutdown()` with a 10s timeout.
