# Protocol

## Transport

mcp-core uses Server-Sent Events (SSE) for server-to-client communication and HTTP POST for client-to-server messages, following the MCP specification.

### Connect

Client opens a GET request to `/sse`:

```
GET /sse
Accept: text/event-stream
```

The server responds with a stream containing an `endpoint` event:

```
event: endpoint
data: /sse?session=session-1234567890
```

### Send Messages

Client sends JSON-RPC requests via POST to the endpoint:

```
POST /sse?session=session-1234567890
Content-Type: application/json

{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}
```

### Receive Responses

Responses are delivered through the SSE stream as raw JSON-RPC messages:

```
{"jsonrpc":"2.0","id":"1","result":{"tools":[...]}}
```

## JSON-RPC Methods

### tools/list

Returns the aggregated list of all tools from all connected backends. Handled by the core directly; no backend dispatch.

### tools/call

Forwards the tool call to the backend that provides the named tool. The core extracts the `name` field from params, looks up the backend via the tool index, checks the circuit breaker, checks health, then forwards.

### resources/list, resources/read

Forwarded to the appropriate backend via the tool index.

### prompts/list, prompts/get

Forwarded to the appropriate backend via the tool index.

## Management Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | JSON: status, version, backends count, streams count, request stats, circuit breaker trips |
| `/metrics` | GET | Prometheus-format metrics (backends, streams, requests, failures, circuit breaks, uptime) |
| `/reload` | POST | Hot-reload config. Reloads YAML, stops and starts all backends, rebuilds tool index. Returns `{"status":"ok"}` |
| `/drain` | POST | Remove a backend from rotation. Body: `{"backend":"name"}`. Removes handle and rebuilds tool index. |
| `/shutdown` | POST | Graceful server shutdown. Returns `{"status":"shutting down"}` and stops the HTTP server. |

## Health Response

```json
{
  "status": "ok",
  "version": "0.1.0-dev",
  "backends": 3,
  "streams": 0,
  "requestsTotal": 42,
  "requestsFailed": 1,
  "circuitBreaks": 0
}
```

## Error Codes

| Code | Meaning |
|------|---------|
| -32601 | Method or tool not found |
| -32602 | Invalid params |
| -32000 | Backend unavailable, unhealthy, circuit-broken, or returned an error |
