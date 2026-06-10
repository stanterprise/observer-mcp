# Architecture

## Scope

observer-mcp is a separate service process that exposes MCP tools for AI agents and directly queries Observer data stores.

## Data authority

- PostgreSQL is canonical for durable run data and analytical queries.
- MongoDB is limited to transient live buffer reads (`live_step_buffers`).

## Runtime topology

- Deployed in the same namespace as Observer.
- Reads database connection env from the shared runtime secret (`observer-runtime-env` by default).
- No public ingress required.

## Component layout

```text
cmd/server/main.go            # process boot, health server, MCP server
internal/config               # env parsing and defaults
internal/db                   # postgres + mongo clients
internal/mcp                  # MCP protocol transport + method handlers
internal/tools                # tool registry and query implementations
charts/observer-mcp           # Helm chart for Kubernetes deployment
```

## MCP surface

Implemented methods:

- `initialize`
- `tools/list`
- `tools/call`

Implemented tools:

- `list_test_runs`
- `get_run_details`
- `analyze_failure_patterns`
- `get_live_step_buffer`

## Security model

- Runtime credentials sourced via Kubernetes secrets only.
- Service account token mount disabled by default in chart.
- Container runs as non-root with dropped capabilities.
- Read-only filesystem enabled by default.

## Expansion path

Planned next tools:

1. `get_success_rate` with rolling windows.
2. `search_runs_by_metadata` using JSONB filters.
3. `inspect_test_attempt` for deep failure triage.
4. `get_slowest_tests` for performance regression spotting.
