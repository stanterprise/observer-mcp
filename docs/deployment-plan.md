# Deployment Plan

## Goal

Deploy observer-mcp as a sibling release to Observer in the same namespace so it can consume existing runtime configuration and secrets.

## Prerequisites

- Observer distributed deployment is running.
- Shared runtime secret exists in namespace (default: `observer-runtime-env`).
- Secret includes at least:
  - `POSTGRES_DSN`
  - `MONGODB_URI`

## Helm strategy

Deploy as independent chart release:

- chart: `observer-mcp/charts/observer-mcp`
- namespace: same as observer release
- values override:
  - `runtime.existingSecret: observer-runtime-env`
  - image tag pointing to published observer-mcp image

Example:

```bash
helm upgrade --install observer-mcp ./charts/observer-mcp \
  --namespace observer \
  --set image.repository=ghcr.io/stanterprise/observer-mcp \
  --set image.tag=latest \
  --set runtime.existingSecret=observer-runtime-env
```

## Suggested helm-infra integration

Add a parallel Helm release in the same environment helmfile where Observer is installed.

Recommended values file keys:

```yaml
runtime:
  existingSecret: observer-runtime-env

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

## Rollout steps

1. Build and publish `observer-mcp` container image.
2. Add Helm release entry in helmfile for target environment.
3. Deploy with one replica.
4. Validate pod health endpoint and log startup.
5. Verify MCP tools from an MCP-compatible client.

## Operational notes

- If Mongo is unavailable, PostgreSQL tools continue working.
- Data schema compatibility follows Observer database migrations.
- Keep chart pinned to image tags to avoid accidental drift.
