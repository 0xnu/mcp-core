## mcp-core

### One Binary. Every Tool. Zero Friction.

[![Release](https://img.shields.io/github/release/0xnu/mcp-core.svg)](https://github.com/0xnu/mcp-core/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/0xnu/mcp-core)](https://goreportcard.com/report/github.com/0xnu/mcp-core)
[![Go Reference](https://pkg.go.dev/badge/github.com/0xnu/mcp-core.svg)](https://pkg.go.dev/github.com/0xnu/mcp-core)
[![Docker](https://img.shields.io/docker/pulls/0xnu20/mcp-core?label=Docker%20Pulls)](https://hub.docker.com/r/0xnu20/mcp-core)
[![License](https://img.shields.io/github/license/0xnu/mcp-core)](/LICENSE)

mcp-core is a high-performance consolidation gateway for the Model Context Protocol (MCP) ecosystem. Written in Go and distributed as a single statically-linked binary, it replaces the growing sprawl of individual MCP server processes with a unified, observable, and self-healing proxy layer.

### The Problem It Solves

**Before mcp-core:**

```
Developer's Machine
├── npx @modelcontextprotocol/server-filesystem    (Node,   ~80MB RAM)
├── npx @modelcontextprotocol/server-github        (Node,   ~90MB RAM)
├── npx @modelcontextprotocol/server-postgres      (Node,  ~110MB RAM)
├── npx @modelcontextprotocol/server-brave-search  (Node,   ~85MB RAM)
├── uvx mcp-server-git                             (Python, ~70MB RAM)
└── ...and Claude/VSCode connects to each one individually

Total: 5+ processes, ~435MB RAM, 5 different config formats
       5 startup times, 5 health states to debug
```

**After mcp-core:**

```
Developer's Machine
├── mcp-core    (Go binary, ~12MB RAM, single process)
│   ├── → filesystem backend
│   ├── → github backend
│   ├── → postgres backend
│   ├── → brave-search backend
│   └── → git backend
│
└── Claude/VSCode connects to ONE endpoint: http://localhost:9020

Total: 1 process to manage, ~12MB overhead, 1 config file
       1 health check, 1 log file, cached schemas
```

### Quick Start

#### 1. Install

**macOS / Linux (one-liner):**
```bash
curl -fsSL https://raw.githubusercontent.com/0xnu/mcp-core/main/install.sh | bash
# Downloads mcp-core and corectl to /usr/local/bin
# Total: ~8MB download, single statically-linked Go binary each
```

**Windows (PowerShell):**
```powershell
powershell -c "irm https://raw.githubusercontent.com/0xnu/mcp-core/main/install.ps1 | iex"
# Downloads mcp-core.exe and corectl.exe to %LOCALAPPDATA%\mcp-core
# Adds them to your user PATH automatically
```

**Via Go toolchain (any platform):**
```bash
go install github.com/0xnu/mcp-core/cmd/mcp-core@latest
go install github.com/0xnu/mcp-core/cmd/corectl@latest
```

**Docker:**
```bash
docker pull 0xnu20/mcp-core
docker run -p 9020:9020 \
  -v $(pwd)/mcp-core.yaml:/etc/mcp-core/mcp-core.yaml \
  0xnu20/mcp-core:latest
```

#### 2. Run

```bash
# Run with an echo test backend (Python)
./build/mcp-core -config configs/test.yaml

# In another terminal, connect any MCP client to:
# http://localhost:9020/sse

# Or run all language backends simultaneously:
./build/mcp-core -config configs/language-test.yaml
```

### Client Setup

See [Client Guide](./docs/client_guide.md) for connecting:

- **VS Code**: `.vscode/mcp.json` with SSE
- **Cursor**: `.cursor/mcp.json` with SSE
- **Zed**: `~/.config/zed/mcp.json` with SSE
- **Claude Desktop**: `claude_desktop_config.json` with stdio

### Multi-Language Support

mcp-core is language-agnostic. The project ships with identical MCP echo servers in 4 languages to prove it:

| Language | Runtime | File |
|----------|---------|------|
| **Go** | Compiled binary | [echo_server](test/integration/fixtures/echo_server.go) |
| **Python** | python3 | [echo_server.py](test/integration/fixtures/echo_server.py) |
| **JavaScript** | node | [echo_server.js](test/integration/fixtures/echo_server.js) |
| **TypeScript** | npx tsx | [echo_server.ts](test/integration/fixtures/echo_server.ts) |

All four run simultaneously in `configs/language-test.yaml` and work identically through the hub.

### CLI

```bash
corectl init       # Auto-discover MCP servers, scaffold config
corectl start      # Launch daemon
corectl status     # Dashboard: backends, streams, requests, circuit breaks
corectl trace      # Live request/response inspection via SSE
corectl validate   # Check config syntax and backend reachability
corectl reload     # Hot-reload config without dropping connections
corectl drain      # Gracefully remove a backend from rotation
corectl stop       # Graceful shutdown via /shutdown endpoint
corectl version    # Show version
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sse` | GET | Open an SSE stream for MCP communication |
| `/sse?session=` | POST | Send JSON-RPC messages to a session |
| `/health` | GET | Health status with backends, streams, request stats |
| `/metrics` | GET | Prometheus-format metrics |
| `/reload` | POST | Hot-reload configuration |
| `/drain` | POST | Remove a backend from rotation |
| `/shutdown` | POST | Graceful server shutdown |

### Features

- **Single binary**: No npm, pip, or Docker required. One statically-linked Go binary (~12MB).
- **Process consolidation**: Replace 5+ MCP server processes with one hub.
- **Language-agnostic**: Backends in Go, Python, JavaScript, TypeScript, or any language with stdio.
- **Framework-agnostic**: Works with Claude Desktop, Cursor, VS Code, Zed, any MCP client.
- **Auto-recovery**: Health checking with automatic restart of failed backends.
- **Circuit breaking**: Prevents cascading failures: wired into every dispatch path.
- **Schema caching**: TTL-based caching of tool schemas with LRU eviction.
- **Prometheus metrics**: Exposed at `/metrics` in Prometheus text format.
- **Config hot-reload**: Reload backends without dropping SSE connections via `corectl reload`.
- **Graceful drain**: Remove backends from rotation with `corectl drain <name>`.
- **Live tracing**: Inspect requests/responses in real time with `corectl trace`.
- **Structured logging**: JSON output with configurable levels.

### Configuration

See [minimal](./configs/minimal.yaml), [full](./configs/full.yaml), [production](./configs/production.yaml), and [language-test](./configs/language-test.yaml).

### Documentation

- [Architecture](./docs/architecture.md): Component design and data flow
- [Client Guide](./docs/client_guide.md): VS Code, Cursor, Zed, Claude Desktop setup
- [Protocol](./docs/protocol.md): SSE transport, JSON-RPC methods, error codes
- [Deployment](./docs/deployment.md): Binary install, Docker, production tips
- [Comparison](./docs/comparison.md): vs individual servers vs agent frameworks

### License

This project is licensed under the [MIT License](./LICENSE).

### Copyright

(c) 2026 [Finbarrs Oketunji](https://finbarrs.eu).
