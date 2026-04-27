# Client Guide for Connecting to mcp-core

## Overview

mcp-core supports two integration modes:

- **SSE mode**: the client connects to `http://localhost:9020/sse` over HTTP. Used by VS Code, Cursor, Zed, and custom scripts.
- **Stdio mode**: the client spawns `mcp-core` as a subprocess and communicates over stdin/stdout. Used by Claude Desktop.

---

## VS Code (v1.98+)

Create `.vscode/mcp.json` in your project root:

```json
{
  "servers": {
    "mcp-core": {
      "type": "sse",
      "url": "http://localhost:9020/sse"
    }
  }
}
```

For it to work globally across all projects, place it at `~/.vscode/mcp.json` instead.

Make sure mcp-core is already running before VS Code connects:

```bash
mcp-core -config configs/minimal.yaml
```

After saving the file, open the command palette (`Cmd+Shift+P`) and run **"MCP: List Tools"** to verify the connection.

If your VS Code version doesn't support MCP natively, install the **Continue** extension (`continue.continue`) and configure it:

```json
{
  "models": [{ ... }],
  "experimental": {
    "mcpServers": {
      "mcp-core": {
        "type": "sse",
        "url": "http://localhost:9020/sse"
      }
    }
  }
}
```

---

## Cursor

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "mcp-core": {
      "type": "sse",
      "url": "http://localhost:9020/sse"
    }
  }
}
```

Cursor reads this file on startup. If mcp-core is already running, the tools appear in Cursor's AI chat panel immediately.

---

## Zed

Create `~/.config/zed/mcp.json`:

```json
{
  "mcpServers": {
    "mcp-core": {
      "type": "sse",
      "url": "http://localhost:9020/sse"
    }
  }
}
```

Then restart Zed or run **"Zed: Reload MCP Servers"** from the command palette.

---

## Claude Desktop

Claude Desktop uses stdio mode. It spawns mcp-core as a subprocess and does not connect to the HTTP server.

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "mcp-core": {
      "command": "/path/to/mcp-core",
      "args": ["-config", "/path/to/configs/minimal.yaml"]
    }
  }
}
```

After saving, restart Claude Desktop. All tools from all backends will appear in Claude's tool list.

Claude Desktop doesn't support SSE connections. It must spawn the binary as a stdio subprocess. Unlike VS Code and Cursor, mcp-core does not need to be pre-started; Claude starts and stops it automatically.

---

## Custom Client (HTTP)

Any tool or script that speaks MCP over SSE can connect directly:

```python
import json, requests

# 1. Open SSE stream to discover the POST endpoint
sse = requests.get("http://localhost:9020/sse", stream=True)
for line in sse.iter_lines():
    if line:
        decoded = line.decode()
        if decoded.startswith("data: "):
            endpoint = decoded.removeprefix("data: ")
            break

# 2. Send a tools/list
resp = requests.post(
    f"http://localhost:9020{endpoint}",
    json={"jsonrpc": "2.0", "id": "1", "method": "tools/list", "params": {}}
)
print(resp.status_code)  # 202 Accepted
```

---

## corectl — Interactive Client

corectl itself can act as a client for testing:

```bash
# Start mcp-core first, then:
corectl status    # Health check via /health
corectl trace     # Live SSE stream — prints requests/responses in real time
```

---

## Quick Reference

| Client | Config File | Mode | Key |
|--------|-------------|------|-----|
| VS Code | `.vscode/mcp.json` | SSE | `"type": "sse"`, `"url": "http://localhost:9020/sse"` |
| Cursor | `.cursor/mcp.json` | SSE | `"type": "sse"`, `"url": "http://localhost:9020/sse"` |
| Zed | `~/.config/zed/mcp.json` | SSE | `"type": "sse"`, `"url": "http://localhost:9020/sse"` |
| Claude Desktop | `claude_desktop_config.json` | Stdio | `"command": "/path/to/mcp-core"` |

**SSE clients**: mcp-core must be running first (`mcp-core`).  
**Stdio clients**: mcp-core is spawned automatically.
