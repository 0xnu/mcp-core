# Deployment

## Install from Release

```bash
# Download the latest release
curl -L https://github.com/0xnu/mcp-core/releases/latest/download/mcp-core-linux-amd64.tar.gz | tar xz

# Install
sudo install mcp-core /usr/local/bin/
sudo install corectl /usr/local/bin/
```

## Configuration

Create `/etc/mcp-core/mcp-core.yaml` or `~/.config/mcp-core/mcp-core.yaml`. See example configs:

- [minimal.yaml](../configs/minimal.yaml): Single backend, quickstart
- [full.yaml](../configs/full.yaml): Multiple backends with health checks, cache, auth
- [production.yaml](../configs/production.yaml): Production-ready with auth, larger buffers, env vars
- [language-test.yaml](../configs/language-test.yaml): Multi-language demo (Go, Python, JavaScript)

```yaml
core:
  host: 127.0.0.1
  port: 9020

backends:
  - name: filesystem
    type: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "."]
```

## Production Recommendations

- Run behind a reverse proxy (nginx, Caddy) for TLS termination.
- Set `MCP_CORE_TOKEN` for API authentication.
- Configure logging to a centralized system (syslog, file).
- Use `corectl status` and `/metrics` for monitoring.
