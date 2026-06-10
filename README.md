# observer-mcp

MCP server for the Observer test observability platform using a data-direct path to PostgreSQL (authoritative) and MongoDB (live step buffers).

## Why data-direct

This server is optimized for high-value analytical and debugging tools that can query storage directly without API translation overhead.

- PostgreSQL is used for durable test run analytics.
- MongoDB is used only for transient live step buffer workflows.

## Current capabilities

- `list_test_runs`: list recent runs with optional status filter.
- `get_run_details`: fetch run metadata and aggregate counters from `run_stats`.
- `analyze_failure_patterns`: summarize top failure signatures from `test_attempts`.
- `get_live_step_buffer`: inspect transient documents in `live_step_buffers`.

## Configuration

Environment variables:

- `POSTGRES_DSN` or `DATABASE_URL` (required)
- `MONGODB_URI` or `MONGO_URI` (optional but required for `get_live_step_buffer`)
- `MCP_MONGO_DATABASE` (default: `observer`)
- `MCP_MONGO_COLLECTION` (default: `live_step_buffers`)
- `MCP_HEALTH_ADDR` (default: `:9090`)
- `MCP_READ_TIMEOUT_SECONDS` (default: `15`)

## Local development

```bash
make tidy
make build
make test
```

## Running

The server uses MCP stdio transport and also exposes a health endpoint for Kubernetes probes.

```bash
POSTGRES_DSN='postgres://observer:password@localhost:5432/observer?sslmode=disable' \
MONGODB_URI='mongodb://observer:password@localhost:27017/observer?authSource=admin' \
./bin/observer-mcp
```

## Local integration with Observer docker compose

If you run Observer with `make web-dev-mode`, you can run observer-mcp in the same docker network and reuse the same postgres/mongo resources.

1. Start Observer in the `observer` repository.
2. From this repository, run `make mcp-up-observer`.
3. Tail logs with `make mcp-logs-observer`.
4. Stop with `make mcp-down-observer`.

By default, the shared network is `observer_observer-net`.
Override with `OBSERVER_DOCKER_NETWORK=<network-name>` if your compose project name differs.

## Helm deployment model

This repository includes a standalone Helm chart under `charts/observer-mcp`.

Default behavior is to reuse the same runtime secret as Observer in the same namespace:

- secret name: `observer-runtime-env`
- required keys: `POSTGRES_DSN`, `MONGODB_URI`

See `docs/architecture.md` and `docs/deployment-plan.md` for rollout guidance.
