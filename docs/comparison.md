# Comparison

## vs Individual MCP Servers

| Aspect | Individual Servers | mcp-core |
|--------|-------------------|---------|
| Process count | N processes | 1 process |
| RAM usage | ~70-110MB each | ~12MB overhead |
| Config format | Per-tool (JSON) | Single YAML |
| Observability | None built-in | /health, /metrics, corectl |
| Health recovery | Manual restart | Auto-restart |
| Circuit breaking | None | Per-backend circuit breaker |
| Tool aggregation | Manual | Automatic |

## vs Agent Frameworks (LangChain, CrewAI, Google ADK)

| Aspect | Agent Frameworks | mcp-core |
|--------|-----------------|---------|
| Purpose | Agent orchestration | Tool infrastructure |
| Client coupling | Tight | Loose (any MCP client) |
| Language lock-in | Python/JS | Language-agnostic |
| Deployment | Complex | Single binary |
| Learning curve | Framework-specific | Standard MCP protocol |

## Why mcp-core?

mcp-core occupies infrastructure territory, not framework territory. It does not replace your agent framework or your MCP servers. It makes them work better together.
