# Architecture Review: Third-Party Services vs Self-Hosted

## Current Direction Assessment

Based on Module 1 design and overall module architecture, here's the third-party dependency map and migration path for data privacy compliance.

---

## Third-Party Services Used (Phase 1 - MVP)

| Module | Service | Purpose | Data Sensitivity | Migration Priority |
|---|---|---|---|---|
| **Module 1** | TrendRadar (self-hosted MCP) | Trending data collection | Low (public data) | P3 - Keep as-is |
| **Module 1** | Aliyun OSS | Raw data storage | Medium (contains CEO schedule) | P2 - Can stay (Aliyun is China-native) |
| **Module 1** | Airflow (Aliyun DataWorks) | Job orchestration | Low (metadata only) | P2 - Can stay |
| **Module 1** | Third-party comment APIs (Apify) | Comment scraping | Medium (public comments) | P3 - Keep as-is |
| **Module 3** | Aliyun Bailian / LLM API | NLP inference (sentiment, keywords) | **HIGH** (processes all content) | **P0 - Must migrate** |
| **Module 4** | Aliyun Bailian Knowledge Base | Enterprise KB + RAG | **CRITICAL** (product specs, CEO style, brand guidelines) | **P0 - Must migrate** |
| **Module 5** | LLM API (via GoClaw) | Topic generation | **HIGH** (strategic content decisions) | **P0 - Must migrate** |

---

## Data Privacy Risk Analysis

**Critical risk points:**
1. **Module 3 (NLP)** — all raw data (trending, comments, CEO schedule) flows through Bailian API for analysis
2. **Module 4 (Knowledge Base)** — Haier's proprietary knowledge (product specs, CEO style library, brand guidelines) stored in Bailian's managed service
3. **Module 5 (Topic Engine)** — strategic content decisions made via LLM API calls

**What leaks to third-party:**
- CEO schedule and speech content
- Haier product roadmap and technical specs
- Brand expression guidelines and forbidden terms
- Strategic topic decisions and content angles

**Compliance concern:**
If Haier's legal/compliance team requires **zero external data transmission** for proprietary content, Modules 3-5 cannot use cloud LLM APIs.

---

## Migration Path: Cloud → Self-Hosted

### Phase 1 (Current - MVP with Cloud Services)

**Use third-party to validate the system works:**
- Aliyun Bailian for NLP + Knowledge Base
- Fast iteration, zero infra burden
- Prove the pipeline end-to-end before investing in self-hosting

**Acceptable because:**
- This is a prototype/MVP phase
- Haier can use non-sensitive test data during validation
- Real CEO content and proprietary knowledge stay out until Phase 2

---

### Phase 2 (Hybrid - Sensitive Data Self-Hosted)

**Trigger:** Haier legal approves production use with real data.

**Migration strategy:**

| Module | Cloud (Phase 1) | Self-Hosted (Phase 2) | Notes |
|---|---|---|---|
| **Module 3 (NLP)** | Bailian API | Self-hosted LLM (Qwen, GLM) on Haier GPU cluster | Inference only, no training needed |
| **Module 4 (KB)** | Bailian KB | GoClaw Vault + pgvector + self-hosted embedding model | RAG stays in-house |
| **Module 5 (Topic)** | LLM API | Same self-hosted LLM as Module 3 | Agent calls local endpoint |

**What stays cloud:**
- Aliyun OSS (raw zone) — acceptable if Haier already uses OSS for other data
- Airflow orchestration — metadata only, no sensitive content
- TrendRadar — public trending data, no proprietary info

**Infrastructure needed for self-hosting:**
- GPU cluster for LLM inference (4x A100 or equivalent for Qwen-72B)
- PostgreSQL with pgvector for knowledge base
- Embedding model (e.g. bge-large-zh, m3e) for RAG
- Model serving layer (vLLM, TGI, or Xinference)

---

## Recommended Approach

**Start with cloud, design for migration:**

1. **Module interfaces are provider-agnostic**
   - Module 3 exposes `analyze(text) -> {sentiment, keywords, topics}`
   - Module 4 exposes `retrieve(query) -> [documents]`
   - Implementation (Bailian vs self-hosted) is swappable behind the interface

2. **Use Aliyun Bailian for Phase 1 MVP**
   - Validate the full pipeline with test data
   - Prove ROI before infrastructure investment
   - Iterate fast on prompts and logic

3. **Design Module 3/4/5 with abstraction layer**
   - `NLPProvider` interface: `BailianProvider` (Phase 1) vs `SelfHostedProvider` (Phase 2)
   - `KnowledgeBaseProvider` interface: `BailianKBProvider` vs `GoClaw VaultProvider`
   - Swap implementation via config, zero code change

4. **Migration trigger: legal approval + budget for GPU cluster**
   - When Haier legal says "no external LLM calls allowed"
   - When Haier allocates budget for self-hosted inference infra
   - Flip config from `provider: bailian` to `provider: self_hosted`

---

## Architecture Principle

**"Cloud-first for speed, self-host-ready for compliance"**

- All module designs use **interface-based providers**, not hard-coded Bailian calls
- Phase 1 uses Bailian to prove the system works
- Phase 2 swaps to self-hosted without rewriting modules
- Migration is a config change + infra setup, not a code rewrite

---

## Decision Points

**Before finalizing Module 2-9 designs, confirm:**

1. **Is Haier okay with Bailian for MVP?**
   - If yes → proceed with cloud-first design
   - If no → start with self-hosted from day 1 (slower, more expensive)

2. **What's the timeline for production use?**
   - If 6+ months → cloud MVP is safe, migrate later
   - If immediate production ��� must self-host from start

3. **Does Haier already have GPU infrastructure?**
   - If yes → self-hosting is cheaper than you think
   - If no → cloud buys time to build infra

**My recommendation:** Use Bailian for Phase 1 MVP, design all modules with swappable providers, migrate to self-hosted when legal requires it. This balances speed (validate the system works) with future compliance (migration path is clean).
