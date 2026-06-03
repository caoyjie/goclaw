# Ark Middleware — Implementation Plan

## Overview

A Python FastAPI middleware service that bridges goclaw with Volcengine's Ark KnowledgeBase (VikingDB).
Users upload files through Filestash (S3-compatible UI over TOS), then explicitly index/deindex them
into VikingKnowledgeBase collections via the middleware REST API. The goclaw agent uses those
collections for RAG (future wiring).

---

## Architecture

```
Browser
├── Filestash UI (port 18792)     ← file manager over TOS bucket
└── Index Manager UI (future)     ← controls index/deindex per file

         ↓ Bearer token (goclaw session)

services/ark/  FastAPI (port 18791)
├── POST /v1/ark/auth/verify      → calls goclaw GET /v1/auth/me to validate token
├── GET/POST /v1/ark/collections  → list / create VikingDB collection (lazy)
├── GET /v1/ark/files             → list files in TOS bucket + index status
├── POST /v1/ark/files/{key}/index    → index a TOS file into VikingDB
├── DELETE /v1/ark/files/{key}/index  → deindex from VikingDB
└── GET /v1/ark/collections/{name}/search  → manual search (preview/test)

         ↓ AK/SK

Volcengine TOS bucket             ← source of truth for raw files
Volcengine VikingKnowledgeBase    ← search index (vectors)

         ↓ goclaw-net (Docker bridge)

goclaw (port 18790)
└── GET /v1/auth/me               ← NEW endpoint (validates token, returns user+tenant+role)
```

---

## Folder / Collection Mapping

```
TOS bucket/
├── products/       → VikingDB collection: "products"
│   ├── manual.pdf
│   └── spec.docx
├── support/        → VikingDB collection: "support"
│   └── faq.md
└── internal/       → VikingDB collection: "internal"
    └── policy.pdf
```

- Top-level folder name = VikingDB collection name (flat, no tenant prefix)
- Files at bucket root (no folder) go into collection `"default"`
- Collections are created lazily on first index request
- Multi-tenant isolation: enforced by IAM policy (Phase 2), not by collection naming

---

## IAM Design (Phase 2 — deferred)

AWS-style policy model. Defer full implementation; Phase 1 uses a simple role check
from goclaw's existing `role` field returned by `GET /v1/auth/me`.

**Phase 1 auth (simplified):**
- `role == "admin"` or `role == "owner"` → full CRUD
- `role == "operator"` → index/deindex docs, list collections, search
- `role == "viewer"` → list and search only, no index/deindex

**Phase 2 (future):**
Full AWS-style policy engine with `ark_principals`, `ark_policies`, `ark_policy_attach` tables.
System user can grant/revoke per-action per-resource policies to other users/roles.

---

## Supported File Types

Volcengine VikingKnowledgeBase accepts: `pdf`, `docx`, `doc`, `txt`, `md`, `html`, `pptx`, `xlsx`.
Middleware validates extension before calling `AddDoc`. Other extensions visible in Filestash
but rejected with a clear 422 error on index attempt.

---

## State Store

Uses **goclaw's existing PostgreSQL** (shared `DATABASE_URL`).
Middleware owns a separate migration set in `services/ark/migrations/`.

### Table: `ark_files`

```sql
CREATE TABLE ark_files (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       UUID        NOT NULL,
    tos_key         TEXT        NOT NULL,          -- e.g. "products/manual.pdf"
    collection_name TEXT        NOT NULL,          -- derived from folder: "products"
    doc_id          TEXT,                          -- VikingDB doc_id after indexing
    status          TEXT        NOT NULL DEFAULT 'stored',
                                                   -- stored | indexing | indexed | deindexing | error
    error_msg       TEXT,
    indexed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, tos_key)
);
CREATE INDEX ark_files_tenant_collection ON ark_files (tenant_id, collection_name);
```

---

## Required Credentials / Config

All passed as environment variables. No secrets in code.

| Variable              | Description                                      | Required |
|-----------------------|--------------------------------------------------|----------|
| `DATABASE_URL`        | Shared goclaw PostgreSQL connection string       | Yes      |
| `GOCLAW_URL`          | Internal URL of goclaw, e.g. `http://goclaw:18790` | Yes   |
| `GOCLAW_GATEWAY_TOKEN`| goclaw gateway token (for service-to-service auth) | Yes   |
| `ARK_AK`              | Volcengine Access Key ID                         | Yes      |
| `ARK_SK`              | Volcengine Secret Access Key                     | Yes      |
| `ARK_REGION`          | VikingKnowledgeBase region, default `cn-beijing` | No      |
| `ARK_HOST`            | VikingKnowledgeBase host (default SDK value)     | No      |
| `TOS_BUCKET`          | TOS bucket name                                  | Yes      |
| `TOS_ENDPOINT`        | TOS S3-compatible endpoint URL                   | Yes      |
| `TOS_REGION`          | TOS region, e.g. `cn-beijing`                    | Yes      |
| `ARK_MIDDLEWARE_PORT` | Port for this service, default `18791`           | No      |

> TOS AK/SK reuse the same `ARK_AK` / `ARK_SK` credentials (same Volcengine account).
> You will need to provide these when you set up the TOS bucket.

---

## Project Structure

```
goclaw/
└── services/
    ├── ark/
    │   ├── main.py                  ← FastAPI app entry point
    │   ├── routers/
    │   │   ├── collections.py       ← GET /v1/ark/collections, POST /v1/ark/collections
    │   │   ├── files.py             ← GET /v1/ark/files, POST/DELETE index endpoints
    │   │   └── search.py            ← GET /v1/ark/collections/{name}/search
    │   ├── core/
    │   │   ├── config.py            ← env var loading (pydantic-settings)
    │   │   ├── client.py            ← VikingKnowledgeBaseService singleton
    │   │   ├── tos.py               ← boto3 S3 client pointed at TOS endpoint
    │   │   ├── db.py                ← asyncpg connection pool
    │   │   └── auth.py              ← token passthrough → goclaw /v1/auth/me
    │   ├── migrations/
    │   │   └── 001_ark_files.sql    ← ark_files table + index
    │   ├── Dockerfile
    │   ├── requirements.txt
    │   └── .env.example
    └── comfyui/
        └── .gitkeep                 ← placeholder for future ComfyUI service
```

---

## Phase 1 Implementation Steps

### Step 0 — Scaffold

- [ ] Create `services/ark/` directory structure
- [ ] Write `requirements.txt`:
  - `fastapi`, `uvicorn[standard]`, `pydantic-settings`
  - `volcengine` (VikingKnowledgeBase SDK)
  - `boto3` (TOS via S3-compatible API)
  - `asyncpg` (PostgreSQL async driver)
  - `httpx` (async HTTP client for goclaw auth roundtrip)
- [ ] Write `.env.example` with all variables from the table above
- [ ] Write `Dockerfile` (python:3.13-slim base, non-root user)
- [ ] Write `services/docker-compose.ark.yml` fragment (joins `goclaw-net`)

### Step 1 — Add `GET /v1/auth/me` to goclaw

- [ ] Create `internal/http/auth_me.go` in goclaw
- [ ] Register `GET /v1/auth/me` using existing `requireAuth` middleware
- [ ] Response: `{ "user_id": string, "tenant_id": string, "role": string }`
- [ ] Add to goclaw's HTTP mux registration in `cmd/gateway.go` or equivalent

### Step 2 — Core infrastructure (middleware)

- [ ] `core/config.py` — load and validate all env vars at startup, fail fast if missing
- [ ] `core/db.py` — asyncpg pool, run `migrations/001_ark_files.sql` on startup
- [ ] `core/auth.py` — `verify_token(token) → AuthUser` calls `GET /v1/auth/me`, caches result 60s
- [ ] `core/client.py` — VikingKnowledgeBaseService singleton (initialized from config)
- [ ] `core/tos.py` — boto3 S3 client pointed at TOS endpoint

### Step 3 — Collections router

- [ ] `GET /v1/ark/collections` — list all VikingDB collections (calls `list_collections`)
- [ ] `POST /v1/ark/collections` — create a collection with name + optional description
  - Requires `operator` role or above
  - VikingDB collection version: `UltimateVersion` (most capable)

### Step 4 — Files router

- [ ] `GET /v1/ark/files` — list TOS bucket contents merged with `ark_files` DB state
  - Response includes: `tos_key`, `size`, `last_modified`, `collection_name`, `status`, `doc_id`
  - Optional query param: `?collection=products` to filter by folder
- [ ] `POST /v1/ark/files/{key}/index` — index a file into VikingDB
  - Validate file extension is in supported list
  - Derive `collection_name` from top-level folder of `key`
  - Create VikingDB collection if it doesn't exist (lazy)
  - Call `collection.add_doc(add_type="tos", tos_path=key)`
  - Upsert `ark_files` row with `status="indexing"`, then update to `status="indexed"` + `doc_id`
  - Requires `operator` role or above
- [ ] `DELETE /v1/ark/files/{key}/index` — deindex from VikingDB
  - Lookup `doc_id` from `ark_files`
  - Call `collection.delete_doc(doc_id)`
  - Update `ark_files` row `status="stored"`, clear `doc_id`
  - Requires `operator` role or above

### Step 5 — Search router

- [ ] `GET /v1/ark/collections/{name}/search?q=...` — semantic search for preview/testing
  - Calls `search_collection(collection_name, query)`
  - Returns list of matching chunks with score
  - Requires `viewer` role or above

### Step 6 — Docker compose fragment

```yaml
# services/docker-compose.ark.yml
services:
  ark:
    build:
      context: services/ark
      dockerfile: Dockerfile
    ports:
      - "${ARK_MIDDLEWARE_PORT:-18791}:18791"
    env_file:
      - path: .env
        required: false
      - path: services/ark/.env
        required: false
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - GOCLAW_URL=http://goclaw:18790
      - GOCLAW_GATEWAY_TOKEN=${GOCLAW_GATEWAY_TOKEN}
      - ARK_AK=${ARK_AK}
      - ARK_SK=${ARK_SK}
      - TOS_BUCKET=${TOS_BUCKET}
      - TOS_ENDPOINT=${TOS_ENDPOINT}
      - TOS_REGION=${TOS_REGION}
    depends_on:
      - goclaw
    networks:
      - goclaw-net
    restart: unless-stopped

  filestash:
    image: machines/filestash
    ports:
      - "18792:8334"
    volumes:
      - filestash-data:/app/data
    networks:
      - goclaw-net
    restart: unless-stopped

volumes:
  filestash-data:
```

> Filestash is configured via its web UI on first launch at `http://localhost:18792`.
> Point it at your TOS bucket using S3 backend with `TOS_ENDPOINT`, `TOS_BUCKET`, `ARK_AK`, `ARK_SK`.

---

## What You Need to Provide (Before Implementation)

1. **Volcengine AK/SK** — Access Key ID and Secret for an account with VikingKnowledgeBase + TOS access
2. **TOS bucket** — Create a bucket in Volcengine TOS console, note the bucket name, endpoint URL, and region
3. **VikingKnowledgeBase project** — Confirm the project name in Ark console (default is `"default"`)

These go into `.env` (gitignored) at the goclaw project root. I will provide the exact keys to fill in.

---

## Out of Scope (Future Phases)

- Full AWS-style IAM policy engine (Phase 2)
- Index Manager web UI inside goclaw dashboard
- ComfyUI cloud integration (`services/comfyui/`)
- Auto-sync on TOS event notifications (webhook trigger)
- Goclaw agent wiring to use these VikingDB collections for RAG
- File re-indexing / version tracking
