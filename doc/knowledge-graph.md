# Knowledge Graph in GoClaw

## What is the Knowledge Graph?

GoClaw's knowledge graph (KG) is a **structured, queryable store of entities and their relationships** built automatically from conversation memory. Instead of raw text, conversations are parsed by an LLM into named entities (people, organizations, projects, tasks, concepts, etc.) and typed relations between them (`works_at`, `depends_on`, `manages`, …). The result is a directed graph that the agent can traverse at query time, answering multi-hop relationship questions that plain semantic memory search cannot answer well.

---

## Architecture Overview

```
conversation/memory write
        │
        ▼
  KGExtractFunc (gateway_managed.go)
        │ async, best-effort
        ▼
  Extractor.Extract()          ← LLM call (temp=0.2, max_tokens=8192)
        │                        chunks input at 12 000 chars
        │                        retries on "length" finish_reason (8 000 char retry)
        │                        normalises external_id, entity_type, relation_type (lowercase)
        │                        dedupes entities by external_id (higher confidence wins)
        ▼
  KnowledgeGraphStore.IngestExtraction()
        │   upsert kg_entities (UNIQUE: agent_id, user_id, external_id)
        │   upsert kg_relations
        ▼
  DedupAfterExtraction()
        │   similarity > 0.98 + name match → auto-merge
        │   similarity > 0.90             → flag as kg_dedup_candidates (status='pending')
        ▼
  (background) BackfillKGEmbeddings()
        │   vector(1536) per entity via configured embedding provider
        │   HNSW index for ANN search
        ▼
  Agent query via knowledge_graph_search tool
        │
        ├─ query="*"           → ListEntities (up to 30)
        ├─ entity_id provided  → Traverse() recursive CTE (default depth 2, max 5)
        │                        fallback: ListRelations direct connections (1-hop)
        └─ text query          → SearchEntities() hybrid FTS + vector
```

---

## Database Schema

### Core tables

```sql
-- migrations/000013_knowledge_graph.up.sql
CREATE TABLE kg_entities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id     VARCHAR(255) NOT NULL DEFAULT '',   -- '' = agent-shared scope
    external_id VARCHAR(255) NOT NULL,              -- LLM-assigned stable ID, lowercased
    name        TEXT NOT NULL,
    entity_type VARCHAR(100) NOT NULL,              -- person / org / project / etc.
    description TEXT DEFAULT '',
    properties  JSONB DEFAULT '{}',
    source_id   VARCHAR(255) DEFAULT '',
    confidence  FLOAT NOT NULL DEFAULT 1.0,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(agent_id, user_id, external_id)
);

CREATE TABLE kg_relations (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id         UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id          VARCHAR(255) NOT NULL DEFAULT '',
    source_entity_id UUID NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    relation_type    VARCHAR(200) NOT NULL,         -- lowercased, e.g. "works_at"
    target_entity_id UUID NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    confidence       FLOAT NOT NULL DEFAULT 1.0,
    properties       JSONB DEFAULT '{}',
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(agent_id, user_id, source_entity_id, relation_type, target_entity_id)
);
```

### Embeddings (migration 000025)

```sql
ALTER TABLE kg_entities ADD COLUMN IF NOT EXISTS embedding vector(1536);
CREATE INDEX IF NOT EXISTS idx_kg_entity_vec ON kg_entities USING hnsw(embedding vector_cosine_ops);
```

### Full-text search + dedup (migration 000031)

```sql
-- tsvector auto-generated from name + description
ALTER TABLE kg_entities ADD COLUMN IF NOT EXISTS tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', name || ' ' || COALESCE(description, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_kg_entities_tsv ON kg_entities USING GIN (tsv);

-- Dedup candidates flagged by medium-similarity matches
CREATE TABLE IF NOT EXISTS kg_dedup_candidates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id    VARCHAR(255) NOT NULL DEFAULT '',
    entity_a_id UUID NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    entity_b_id UUID NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    similarity FLOAT NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending',   -- pending / merged / dismissed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_a_id, entity_b_id)
);
```

---

## How to Enable the Knowledge Graph

### 1. Edition requirement

KG requires the **Standard edition** (PostgreSQL + pgvector). It is **not available** in the Lite/desktop edition.

| Edition | `KGEnabled` | Notes |
|---|---|---|
| Standard (PG) | `true` | Full KG features |
| Lite (SQLite) | `false` | `KnowledgeGraph` store is `nil`; tool registered but returns "not enabled" message |

### 2. PostgreSQL + pgvector

The `embedding vector(1536)` column and HNSW index require the [pgvector](https://github.com/pgvector/pgvector) extension to be installed in your PostgreSQL instance.

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

Run all migrations to get the full schema:

```bash
./goclaw migrate up
```

### 3. Configure an embedding provider

Embeddings are computed via an LLM provider. The same provider registry used for chat also serves embeddings. Any provider that exposes an embeddings API (e.g. OpenAI, Azure OpenAI) can be used. Configure it through the admin UI or directly in the `llm_providers` table with your API key.

At startup, `buildKGExtractFunc` in `cmd/gateway_managed.go` wires the embedding provider into `PGKnowledgeGraphStore.SetEmbeddingProvider(embProvider)` and immediately runs a background `BackfillKGEmbeddings()` to compute vectors for any existing entities that lack them.

### 4. Enable extraction per agent via built-in tool settings

Extraction is **off by default**. It is controlled by the `knowledge_graph_search` entry in the `builtin_tools` table, read fresh on every memory-write event:

| Setting | Type | Default | Meaning |
|---|---|---|---|
| `extract_on_memory_write` | bool | `false` | Enable automatic extraction when memory is written |
| `extraction_provider` | string | `""` | Provider name (must match an `llm_providers` row) |
| `extraction_model` | string | `""` | Model ID to use for extraction |
| `min_confidence` | float64 | `0.75` | Entities/relations below this score are discarded |

These settings are edited via the admin UI under **Agent → Built-in Tools → knowledge_graph_search**.

### 5. Per-agent scope: shared vs per-user

By default KG is **per-user**: `user_id` is set to the sender's ID in every entity row, so each user has an isolated sub-graph. To make the entire agent share one graph (all users see each other's entities), set `ShareKnowledgeGraph: true` in the agent's `WorkspaceSharingConfig`:

```json
{
  "workspace_sharing": {
    "share_knowledge_graph": true
  }
}
```

When `ShareKnowledgeGraph` is `true`, the agent loop calls `store.WithSharedKG(ctx)` before any KG operation, which causes `KGUserID()` to return `""` (agent-scoped) instead of the sender's user ID.

---

## Graph Traversal

Traversal uses a **WITH RECURSIVE CTE** in PostgreSQL (`internal/store/pg/knowledge_graph_traversal.go`):

- Default `maxDepth = 3` (overrides 0 or negative inputs)
- Tool cap: max 5 (user-provided `max_depth` capped at 5)
- Statement timeout: `SET LOCAL statement_timeout = '5000'` (5 s)
- **Bidirectional**: follows both outgoing and incoming edges; reverse relations are prefixed with `~` (e.g. `~works_at`)
- **Cycle prevention**: `NOT e.id::text = ANY(path)` in the recursive step
- Start entity is excluded from results (`depth > 1`)
- Shared vs per-user scope controlled by `IsSharedKG(ctx)`

---

## Hybrid Search

`SearchEntities` in `internal/store/pg/knowledge_graph.go` combines:

1. **FTS** — `tsv @@ plainto_tsquery('simple', $query)` on the GIN-indexed `tsv` column
2. **Vector ANN** — cosine distance on the HNSW `embedding` column (when embeddings are available)

Results are merged and ranked by combined score.

---

## Entity Deduplication

After each ingestion batch, `DedupAfterExtraction` compares the newly upserted entity IDs against the existing graph using their embeddings:

| Similarity | Action |
|---|---|
| > 0.98 and names match | Auto-merge (one entity survives) |
| > 0.90 | Flag as `kg_dedup_candidates` (`status='pending'`) for human review |

The admin can resolve candidates via the HTTP API (`POST /v1/kg/dedup/{id}/merge` or `/dismiss`) or the UI.

A full scan can be triggered manually via `POST /v1/kg/agents/{agentId}/dedup/scan`.

---

## Agent Tool: `knowledge_graph_search`

The tool is registered at startup (`cmd/gateway_setup.go:88`) and available to all agents on the Standard edition. When no KG store is wired (Lite/SQLite), the tool returns `"Knowledge graph is not enabled for this agent."`.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Text search; `"*"` lists all entities |
| `entity_type` | string | no | Post-filter by type (`person`, `organization`, `project`, …) |
| `entity_id` | string | no | UUID to traverse from |
| `max_depth` | number | no | Traversal depth, default 2, max 5 |

**Modes:**

- `entity_id` provided → deep traversal (falls back to direct 1-hop connections, then text search)
- `query="*"` → list all (cap 30)
- text query → `SearchEntities` hybrid FTS+vector, optional type filter, shows relations per entity (cap 5)

The system prompt includes a `HasKnowledgeGraph` hint (`internal/agent/systemprompt.go:65`) when the tool is registered, guiding the agent to use `knowledge_graph_search` for relationship questions alongside `memory_search`.

---

## HTTP API

Registered via `server.SetKnowledgeGraphHandler(httpapi.NewKnowledgeGraphHandler(...))` in `cmd/gateway.go:482`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/kg/agents/{agentId}/entities` | List entities |
| `GET` | `/v1/kg/agents/{agentId}/entities/{id}` | Get entity |
| `PUT` | `/v1/kg/agents/{agentId}/entities` | Upsert entity |
| `DELETE` | `/v1/kg/agents/{agentId}/entities/{id}` | Delete entity |
| `GET` | `/v1/kg/agents/{agentId}/traverse/{id}` | Traverse from entity |
| `POST` | `/v1/kg/agents/{agentId}/extract` | Manual LLM extraction |
| `GET` | `/v1/kg/agents/{agentId}/stats` | Graph statistics |
| `POST` | `/v1/kg/agents/{agentId}/dedup/scan` | Scan for duplicates |
| `GET` | `/v1/kg/agents/{agentId}/dedup/candidates` | List dedup candidates |
| `POST` | `/v1/kg/agents/{agentId}/dedup/{id}/merge` | Merge two entities |
| `POST` | `/v1/kg/agents/{agentId}/dedup/{id}/dismiss` | Dismiss candidate |
| `GET` | `/v1/kg/agents/{agentId}/graph` | Full graph dump (entity + relation list) |

The `handleExtract` endpoint triggers the same `Extractor.Extract()` → `IngestExtraction()` → `DedupAfterExtraction()` pipeline as the automatic trigger, with shared-KG scoping applied via `store.WithSharedKG(ctx)` where configured.

---

## Extraction Pipeline Detail

`internal/knowledgegraph/extractor.go`:

- Input is chunked at `maxChunkChars = 12 000` on paragraph boundaries
- LLM call: `temperature=0.2`, `max_tokens=8192`
- If finish_reason is `"length"`, retries with `retryMaxChars = 8 000`
- `sanitizeJSON` repairs common LLM JSON artefacts (malformed decimals like `.5`, trailing commas)
- `mergeResults` deduplicates across chunks: entities by `external_id` (higher confidence wins), relations by `(source, type, target)` tuple
- `external_id`, `entity_type`, and `relation_type` are normalised to lowercase

---

## Multi-Tenant Isolation

KG is fully tenant-scoped via the standard `scopeClause()` helpers (see [multi-tenancy.md](multi-tenancy.md)). `kg_entities` and `kg_relations` carry `agent_id` (not a direct `tenant_id`); tenant isolation is enforced through the foreign key chain `agent_id → agents.tenant_id`. `kg_dedup_candidates` has a direct `tenant_id` column for efficient scoping.

---

## Key Code References

| File | Purpose |
|---|---|
| `migrations/000013_knowledge_graph.up.sql` | Base schema: `kg_entities`, `kg_relations` |
| `migrations/000025_kg_entity_embeddings.up.sql` | `embedding vector(1536)` + HNSW index |
| `migrations/000031_kg_fts_and_dedup.up.sql` | `tsv` FTS column, `kg_dedup_candidates` table |
| `internal/store/knowledge_graph_store.go` | `KnowledgeGraphStore` interface |
| `internal/store/pg/knowledge_graph.go` | Entity CRUD + hybrid `SearchEntities` |
| `internal/store/pg/knowledge_graph_relations.go` | Relation CRUD, `IngestExtraction`, `PruneByConfidence`, `Stats` |
| `internal/store/pg/knowledge_graph_traversal.go` | `Traverse()` WITH RECURSIVE CTE |
| `internal/store/pg/knowledge_graph_dedup.go` | `DedupAfterExtraction`, `ScanDuplicates`, `MergeEntities` |
| `internal/store/pg/knowledge_graph_embedding.go` | `BackfillKGEmbeddings`, `EmbedEntity` |
| `internal/knowledgegraph/extractor.go` | LLM extraction pipeline |
| `internal/tools/knowledge_graph.go` | `knowledge_graph_search` tool (agent-facing) |
| `internal/agent/systemprompt.go` | `HasKnowledgeGraph` system-prompt hint |
| `internal/agent/loop_utils.go` | `shouldShareKnowledgeGraph()` |
| `internal/store/context.go` | `SharedKGKey`, `WithSharedKG`, `IsSharedKG`, `KGUserID` |
| `internal/store/agent_store.go` | `WorkspaceSharingConfig.ShareKnowledgeGraph` |
| `internal/http/knowledge_graph.go` | `KnowledgeGraphHandler` |
| `internal/http/knowledge_graph_handlers.go` | HTTP handler implementations |
| `cmd/gateway_managed.go` | `buildKGExtractFunc` — memory-write trigger wiring |
| `cmd/gateway_setup.go` | Tool registration, embedding provider wiring |
| `cmd/gateway.go` | `SetKnowledgeGraphHandler` HTTP registration |
| `internal/edition/edition.go` | `KGEnabled` flag (Standard=true, Lite=false) |
| `internal/store/sqlitestore/factory.go` | SQLite factory — `KnowledgeGraph` intentionally `nil` |
