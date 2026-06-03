# Fast Local Dev: Modify & Test with Docker

How to apply code changes and test them locally as fast as possible.

## Overview

The stack has two independent parts that can be rebuilt separately:

- **Backend (Go)** — compiled into `goclaw` binary, runs in Docker on port 18790
- **Frontend (React/Vite)** — built by pnpm, either embedded into the binary or served by Vite dev server

The key insight: **the frontend never needs to be inside Docker during development.** Vite's dev server proxies all API and WebSocket traffic to the running backend container, so you get hot module replacement with zero Docker involvement for UI changes.

---

## One-Time Setup

### 1. Start all services (first time only)

```bash
./scripts/dev-docker-local.sh start
```

This builds everything from scratch and starts postgres, redis, otel, browser, and goclaw. Takes ~5 min. Only needed once — after this, services stay running between reboots unless you explicitly stop them.

### 2. Configure UI dev server

```bash
cd ui/web
pnpm install
cat > .env.local <<'EOF'
VITE_BACKEND_HOST=localhost
VITE_BACKEND_PORT=18790
EOF
```

`.env.local` is git-ignored. After this, `pnpm dev` always points at the Docker backend automatically.

---

## Daily Iteration Loop

### Backend (Go) change

```bash
./scripts/dev-docker-local.sh quick --no-ui
```

- Skips the entire Node/pnpm Docker stage (`ENABLE_EMBEDUI=false`)
- Skips fetching base images from registry (uses cache)
- Rebuilds only the Go binary, restarts only the `goclaw` container (`--no-deps`)
- Runs DB migrations, health check, prints status

**Rebuild time: ~20–40s**

### Frontend (React/Vite) change

```bash
# Keep this running in a dedicated terminal all day
cd ui/web && pnpm dev
# Open http://localhost:5173
```

- Vite proxies `/ws`, `/v1`, `/health` → `http://localhost:18790`
- HMR updates the browser on every file save
- No Docker rebuild, no container restart

**Feedback time: instant (HMR)**

### Both changed at the same time

```bash
# Terminal 1 — rebuild Go, no UI embed
./scripts/dev-docker-local.sh quick --no-ui

# Terminal 2 — UI dev server (keep running, auto-reconnects after backend restart)
cd ui/web && pnpm dev
```

---

## Command Reference

| Command | What it does | When to use | Time |
|---|---|---|---|
| `quick --no-ui` | Rebuild Go only, restart goclaw | Go source changed, UI unchanged or running via pnpm dev | ~20–40s |
| `quick` | Rebuild Go + embed UI, restart goclaw | Go + UI, want single-port result | ~30–60s |
| `deploy` | Like `quick` but pulls latest base images, also starts comfyui | Pre-release / CI-like build | ~2–3 min |
| `start` | Full build of all services from scratch | First time, or after `down` | ~5+ min |
| `migrate` | Run DB migrations only | Schema changed, binary already up to date | seconds |
| `logs` | Tail goclaw container logs | Debugging runtime behavior | — |
| `status` | Show all container states | Quick sanity check | — |
| `down` | Stop all services | Done for the day | — |

---

## Docker Layer Cache — What Gets Reused

The Dockerfile has three stages. Docker caches each layer independently:

```
Stage 0 — web-builder (Node/pnpm)
  COPY pnpm-lock.yaml  →  pnpm install      ← cached unless lockfile changes
  COPY ui/web/         →  pnpm build         ← rebuilds on any UI source change

Stage 1 — builder (Go)
  COPY go.mod go.sum   →  go mod download    ← cached unless go.mod/go.sum changes
  COPY . .             →  go build           ← rebuilds on any Go source change

Stage 2 — runtime (Alpine)
  COPY --from=builder  →  final image        ← rebuilds only when binary changes
```

### Cache behavior by change type

| What changed | Cached | Rebuilds | Time |
|---|---|---|---|
| Go source only | pnpm install + go mod download | Go compile only | ~20–40s |
| `go.mod` / `go.sum` | pnpm install | go mod download + compile | ~2–3 min |
| `ui/web/` source | go mod download | pnpm build + Go compile | ~1–2 min |
| `pnpm-lock.yaml` | go mod download | pnpm install + build + compile | ~3–5 min |
| Nothing (re-run) | Everything | Nothing | ~5s |

`quick --no-ui` short-circuits Stage 0 entirely by passing `--build-arg ENABLE_EMBEDUI=false`, which skips the `COPY --from=web-builder` instruction in Stage 1. This saves the pnpm build time even when `ui/web/` source has changed.

---

## Build Args

Passed via `docker compose build --build-arg KEY=value goclaw` or set in `docker-compose.yml`:

| Arg | Default | Effect |
|---|---|---|
| `ENABLE_EMBEDUI` | `false` | `true` = embed `ui/web/dist` into binary (required for production) |
| `ENABLE_OTEL` | `false` | `true` = compile with OpenTelemetry tracing |
| `ENABLE_PYTHON` | `true` | `false` = skip Python runtime in final image |
| `ENABLE_SANDBOX` | `false` | `true` = install docker-cli for code sandbox |
| `ENABLE_FULL_SKILLS` | `false` | `true` = pre-install all skill Python/Node deps |
| `VERSION` | git tag or `dev` | Sets binary version string |

For local dev, defaults are fine. Never set `ENABLE_FULL_SKILLS=true` locally — it downloads hundreds of MB.

---

## Vite Proxy Configuration

Defined in [ui/web/vite.config.ts](../../ui/web/vite.config.ts):

```
/ws    → ws://localhost:18790   (WebSocket RPC)
/v1    → http://localhost:18790  (HTTP API)
/health → http://localhost:18790 (health check)
```

`VITE_BACKEND_HOST` and `VITE_BACKEND_PORT` env vars override the target. Set them in `ui/web/.env.local` (git-ignored).

---

## Troubleshooting

**`quick` fails with "image not found"**
Run `./scripts/dev-docker-local.sh start` first to do the initial build.

**UI dev server shows blank page / 502**
The backend container is not running. Check with `./scripts/dev-docker-local.sh status` and restart with `./scripts/dev-docker-local.sh quick --no-ui`.

**WebSocket disconnects after `quick`**
Expected — the container restarted. The UI auto-reconnects within a few seconds.

**Migration fails on `quick`**
The `upgrade` container runs the migration. If it fails, check logs:
```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.upgrade.yml logs upgrade
```

**Go build cache stale / unexpected behavior**
Force a clean Go build inside Docker:
```bash
docker compose build --no-cache goclaw
```
