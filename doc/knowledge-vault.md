# Knowledge Vault in GoClaw

## What is the Knowledge Vault?

GoClaw's Knowledge Vault is a document-centric memory layer that lets agents register workspace files, search them with hybrid retrieval, and connect them through explicit links.

It complements (not replaces) episodic memory and the knowledge graph:
- **Vault**: document storage, summaries, embeddings, links, and file-sync
- **Episodic**: session-level narrative memory
- **Knowledge Graph**: entity-relation memory

---

## Core Capabilities

- **Document Registry**
  - Tracks workspace documents in `vault_documents`
  - Maintains metadata (path, scope, hash, summary, embedding)

- **Hybrid Search (`vault_search`)**
  - Full-text + semantic retrieval in one query path
  - Unified ranking across **vault + episodic + knowledge graph**

- **Document Read (`vault_read`)**
  - Fetches complete document content by `doc_id`
  - Supports source-aware fallback when callers pass non-vault IDs

- **Wikilinks and Backlinks**
  - Parses `[[wikilinks]]` from document content
  - Resolves and syncs link edges in `vault_links`

- **Filesystem Sync Hooks**
  - Auto-registers/updates docs after write/read/media tool operations
  - Keeps DB metadata aligned with real workspace files

- **Async Enrichment Worker**
  - Summarizes content
  - Generates embeddings
  - Performs auto-linking / relationship classification

---

## Architecture Flow

```
Agent tools / file operations
        |
        v
VaultInterceptor (tool hooks: write/read/edit/media)
        |
        v
vault_documents upsert + hash tracking
        |
        +--> EnrichWorker (async)
               |- summary generation
               |- embedding update
               `- wikilink + semantic link sync

Agent query: vault_search
        |
        v
VaultSearchService fan-out (parallel):
  1) Vault store search
  2) Episodic search
  3) KG search
        |
        v
Weighted merge + normalized ranking
```

---

## Key Wiring in goclaw

- **Tool and interceptor wiring**
  - `cmd/gateway_vault_wiring.go`
  - Registers `vault_search` and `vault_read`
  - Injects a shared `VaultInterceptor` into file/media tools

- **Search orchestration**
  - `internal/vault/search.go`
  - `VaultSearchService` performs nil-safe fan-out and result merging

- **Enrichment pipeline**
  - `internal/vault/enrich_worker.go`
  - Background processing for summaries, embeddings, and links

- **Link processing**
  - `internal/vault/links.go`
  - Wikilink extraction and target resolution

- **HTTP endpoints**
  - Wired from `cmd/gateway_http_wiring.go`
  - Includes document list/get/search and graph endpoints

---

## Data Model (High Level)

- `vault_documents`
  - Canonical document records and retrieval metadata
- `vault_links`
  - Directed links between documents (supports backlink traversal)
- `vault_versions`
  - Version table reserved for document history evolution

Related integrations:
- `migrations/000038_vault_tables.*.sql` (base vault schema)
- `migrations/000048_vault_media_linking.*.sql` (media linkage)
- `migrations/000049_vault_path_prefix_index.*.sql` (path/prefix performance)
- `migrations/000056_vault_chat_id.*.sql` (chat association)

---

## Runtime Behavior and Safety

- Vault wiring is conditional: when `stores.Vault == nil`, vault tools are not activated
- Fan-out search is nil-safe: unavailable stores are skipped gracefully
- Scope ownership is enforced by DB constraints and tenant/agent scoping
- Interceptors only register supported files (extension and path safety rules)

---

## How to Use (Operator View)

1. Ensure migrations are applied.
2. Configure at least one provider that supports chat/embedding workflows.
3. Let agents operate on workspace docs (`write_file`, `edit`, media tools).
4. Use `vault_search` for discovery and `vault_read` for full content retrieval.
5. Query links/graph endpoints to inspect document relationships.

---

## Position in Memory Stack

- **L0/L1/L2 prompt memory** remains the conversational runtime memory path.
- Knowledge Vault adds durable, document-first recall and cross-document linking.
- Combined with episodic and KG fan-out, it gives agents broader and more precise retrieval for long-running projects.
