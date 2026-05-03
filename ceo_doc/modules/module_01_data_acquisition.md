# Module 1: Data Acquisition

## Responsibility

Collect raw data from all five topic sources on a scheduled basis and store it in Aliyun OSS as the raw data zone. This module is purely a collector — it acquires, normalizes minimally, and stores. Nothing more.

**Six data sources:**
1. **Platform trending charts** — via TrendRadar MCP (Weibo, Douyin, Xiaohongshu, Zhihu, Bilibili, 36Kr, etc.)
2. **Social hot topics / industry news** — via TrendRadar MCP (RSS feeds, news sites)
3. **CEO schedule and speeches** — pushed from Haier internal data feed
4. **Comment data** — via third-party comment APIs (e.g. Apify, platform APIs)
5. **Key milestones** — manually imported or calendar-fed events
6. **Admin-uploaded files** — documents, CSVs, PDFs, spreadsheets uploaded directly by admin users via Aliyun OSS console

---

## Clear Boundaries — What This Module Does NOT Do

- No NLP, no sentiment analysis, no keyword extraction
- No relevance scoring, no filtering by Haier relevance
- No content-level deduplication (same trending item across two run windows — Module 3 owns that)
- No dedup of raw items within the same source response
- No topic generation or content decisions
- No agent execution — this is a data pipeline job, not an agent task
- No tenant-level awareness — this is a system-wide job

---

## Storage

**Raw data destination: Aliyun OSS**

All collected data lands in OSS as raw JSON files. Writes are idempotent — same job run always writes to the same deterministic path (see Deduplication below).

OSS is the **unified raw data zone** for both Airflow-collected data and admin-uploaded files. Module 3 reads from OSS regardless of how data arrived.

**Path convention — Airflow collected:**
```
oss://ceo-ip-raw/
  trending/{platform}/{YYYY-MM-DD}/{run_id}.json
  comments/{platform}/{YYYY-MM-DD}/{run_id}.json
  ceo_schedule/{YYYY-MM-DD}/{run_id}.json
  milestones/{YYYY-MM-DD}/{run_id}.json
```

**Path convention — Admin uploads:**
```
oss://ceo-ip-raw/
  uploads/{category}/{filename}
```

- `category` values TBD — admin will define categories (e.g. product, ceo, brand, industry, competitor, other)
- All file formats accepted: PDF, Word (.docx), Excel (.xlsx), CSV, plain text, mixed Chinese/English
- Raw layer keeps all versions — no overwrite on re-upload; versioning convention TBD
- Deduplication of uploaded file content is handled downstream in Module 3 (NLP golden layer), not here

**Why OSS, not GoClaw Vault:**
- Raw data volume is too large and too noisy for the Vault
- Most raw items will be discarded after Module 3 processing
- The Vault receives only curated, enriched content — output of Module 3, not input

---

## Scheduling and Orchestration

**Orchestrator: Airflow (system-level)**

Module 1 jobs are managed by Airflow, not by GoClaw cron. This is because:
- GoClaw cron is tenant-scoped — Module 1 is a system-wide job with no tenant context
- Modules 1→3→5 form a dependent pipeline — Airflow manages the dependency chain
- Airflow provides one dashboard for visibility across all module jobs

**GoClaw's role here:** None for scheduling. GoClaw is the agent execution engine for content generation (Modules 6+), not the data pipeline orchestrator.

**Suggested schedule:**
- Trending data: every 1 hour
- Comment data: every 2 hours
- CEO schedule: event-driven push from Haier feed
- Milestones: daily sync

---

## Failure Handling and Alerting

When a collection job fails (API down, rate limit, network error):

1. Airflow marks the task as failed and retries with backoff
2. On sustained failure (exceeds retry limit), fire an alert:
   - **GoClaw alert event** — GoClaw routes to all configured message channels (Telegram, Feishu, Zalo, Discord — whichever the deployment uses)
   - **GoClaw web UI broadcast** — push warning message to all users in the web chat interface
3. Module 1 does not manage alert routing — it fires one event to GoClaw; GoClaw owns the dispatch

---

## Admin Upload Flow

Admin users upload files directly via **Aliyun OSS console** — no GoClaw UI involved.

**Access control:**
- Only admin-role users receive OSS bucket credentials
- No GoClaw upload page; no data browser page — all file management via OSS console

**Ingest trigger after upload:**
OSS event notification → **Aliyun MNS/RocketMQ queue** → Airflow DAG picks up and processes one file at a time. Files are ingested sequentially to avoid overwhelming Module 3.

**No manual trigger needed** — upload to OSS automatically kicks off the pipeline.

---



TrendRadar is used **as-is** via its existing MCP server interface. No modifications to TrendRadar.

GoClaw (or the Airflow job) calls TrendRadar MCP tools:
- `search_tools` — query trending data by keyword/platform
- `data_query` — retrieve collected trending items
- `storage_sync` — sync TrendRadar's local storage to OSS if needed

TrendRadar remains an independent service. Module 1 treats it as a black-box MCP tool provider.

---

## Output Contract

Each raw item written to OSS follows this minimal schema:

```json
{
  "source": "weibo_trending | douyin_trending | ceo_schedule | comment | milestone | ...",
  "platform": "weibo",
  "collected_at": "2026-05-02T10:00:00Z",
  "run_id": "weibo_trending_20260502_1000",
  "raw": { }
}
```

`run_id` is deterministic: `{source}_{platform}_{YYYYMMDD}_{HHMM}`. Not a random UUID.

The `raw` field is the unmodified API response. Module 3 (NLP) reads from OSS and owns all further parsing and enrichment.

---

## Deduplication

Two layers protect against duplicate ingestion:

**Layer 1: Airflow job-level**
Airflow tracks execution state per DAG run. If a run is already in-progress or completed for a given time window, it will not re-trigger. This prevents double-scheduling at the orchestration level.

**Layer 2: OSS idempotent writes**
`run_id` is deterministic — derived from `hash(source + platform + time_window)`, not a random UUID. If the same job fires twice for the same window, it writes to the identical OSS path. The second write overwrites with the same data — no duplicate files accumulate.

**What this module does NOT handle:**
Content-level dedup — the same trending item appearing across two different run windows (e.g. a story trending at 10:00 and again at 11:00) is intentional overlap. Module 3 (NLP) owns dedup at the content/semantic level during cleaning.

---

## Collaboration

| Module | Relationship |
|---|---|
| **Module 3 (NLP)** | Downstream consumer — reads raw OSS files (both collected and uploaded), cleans and enriches |
| **Module 4 (Knowledge Base)** | No direct interaction from Module 1 |
| **Airflow** | Owns scheduling and dependency chain for this module |
| **GoClaw** | Receives failure alert events; routes to message channels and web UI |
| **TrendRadar** | Upstream data provider called via MCP |
| **Haier internal feed** | Upstream push source for CEO schedule data |
| **Aliyun OSS console** | Admin upload interface — no GoClaw UI needed |
| **Aliyun MNS/RocketMQ** | Queue for sequential processing of admin-uploaded files |

---

## What Airflow Manages vs What GoClaw Manages

| Concern | Owner |
|---|---|
| Schedule Module 1 cron jobs | Airflow |
| Retry on failure | Airflow |
| Trigger Module 3 after Module 1 completes | Airflow |
| Pipeline visibility dashboard | Airflow |
| Alert message routing (channels + web UI) | GoClaw |
| Agent tasks (script gen, topic engine) | GoClaw |
| Content generation pipeline (Modules 6–8) | GoClaw |
