# CEO IP Agent System Module Design

Define the overall module decomposition, responsibility boundaries, and collaboration relationships of the CEO IP Agent system, as the foundational specification for system design, development, and subsequent expansion.

---
## II. Overall System Architecture

Centered on the goal of "automated entrepreneur IP production," the system builds a complete closed loop:
Data acquisition -> data processing -> semantic understanding -> topic decision-making -> content generation -> publishing feedback -> continuous optimization.

The system adopts an architecture of "business modules + Agent orchestration layer":
- Business modules: responsible for concrete capability implementation.
- Agent orchestration layer (GoClaw): responsible for process scheduling and automated execution.

Cloud baseline (fully migrated to Volcengine)
- LLM invocation: Volcengine Ark (Doubao family models).
- Enterprise knowledge base: Volcengine knowledge retrieval + RAG pipeline.
- Object storage: Volcengine TOS (all raw files, media assets, and derived artifacts).
- Database: Volcengine managed database services (relational + analytical scenarios).

---
## III. Module Decomposition

The system is divided into 9 core modules:
1. Data Acquisition Module
2. Data Storage Module
3. NLP Semantic Analysis Module
4. Enterprise Knowledge Base Module
5. Topic Engine Module
6. Script Generation Module
7. Content Production Module
8. Public Opinion Risk Control Module
9. Data Review Module

---
## IV. Module Designs

---
### 1. Data Acquisition Module

Responsibilities
Acquire data from multiple channels, including trending information, comment data, industry intelligence, and internal enterprise data.

Data Sources
- Social platforms: Douyin, Xiaohongshu, Weibo, Bilibili, Zhihu.
- Comments and DMs: account comments and user interactions.
- Industry intelligence: news websites and technology media.
- Enterprise data: product materials, strategic information, CEO schedule.
- Manual imports: WeChat Channels and private-domain data.

Core Capabilities
- Multi-platform crawling.
- Scheduled jobs.
- Keyword subscriptions.
- Retry-on-failure and rate-limit control.

---
### 2. Data Storage Module

Responsibilities
Store and manage data in layered zones to support analytics and queries.

Layered Structure
- Raw Zone: raw data (JSON/HTML).
- Clean Zone: cleaned structured data.
- Insight Zone: analysis result data.
- Content Zone: topics and scripts.

Volcengine implementation
- Object files (raw text, images, videos, intermediate artifacts): unified storage in Volcengine TOS.
- Structured business data (tasks, topics, scripts, workflow states): unified storage in Volcengine managed relational database.
- Analytical/aggregation data (performance statistics, trend summaries): can use Volcengine analytical database based on data volume and latency goals.
- Access in Go services follows Volcengine Go SDK conventions (credential management, region endpoint, client reuse, retry strategy).

---
### 3. NLP Semantic Analysis Module

Responsibilities
Perform semantic understanding and structured analysis on comments and public-opinion data.

Core Capabilities
- Sentiment analysis (positive/negative/neutral/mixed).
- Comment classification (question/challenge/suggestion/attack, etc.).
- Topic extraction.
- Keyword extraction.
- Risk identification (sensitive terms, public-opinion risks).
- User intent recognition.
- High-frequency question aggregation.

Model invocation (Volcengine)
- All NLP capabilities are implemented via Volcengine Ark large models (including classification, extraction, summarization, and risk reasoning).
- Prompt templates are versioned by task type (sentiment, intent, risk) and centrally managed.
- Add fallback strategy: primary model unavailable -> switch to backup model of same vendor.

Output
Structured semantic labels for downstream topic generation and risk control.

---
### 4. Enterprise Knowledge Base Module

Responsibilities
Accumulate enterprise-related knowledge as the basis for content generation and topic judgment.

Content Composition
- Industry knowledge (smart home, AI appliances, big health, etc.).
- Product knowledge (functions, parameters, use cases).
- CEO style library (historical remarks, language style).
- Brand expression guidelines (expression norms, forbidden terms).
- Industry knowledge (policies, trends).

Technical Implementation (Volcengine)
- Source documents and media are stored in Volcengine TOS.
- Indexing/vector retrieval relies on Volcengine knowledge retrieval capabilities and RAG workflow.
- Retrieval + generation path is unified as: query rewrite -> recall -> rerank -> context assembly -> Ark generation.
- Knowledge freshness strategy: incremental indexing + periodic full rebuild.

---
### 5. Topic Engine Module (Core Module)

Responsibilities
Combine hot topics, public-opinion insights, and enterprise knowledge to generate high-quality topics.

Inputs
- Hot-topic data.
- Comment insights.
- Enterprise knowledge.
- Industry trends.

Core Logic
- Hot topic to industry relevance mapping.
- User concern identification.
- Communication potential evaluation.
- Risk evaluation.
- CEO expression fit.

Outputs
- Topic title.
- Core proposition.
- Content angle.
- Risk alerts.
- Priority score.

---
### 6. Script Generation Module

Responsibilities
Generate standardized video scripts based on selected topics.

Output Content
- Video topic.
- Core viewpoints.
- Voice-over copy.
- Duration structure (60s / 90s).
- Tags.
- Publishing notes.

Requirements
- Align with CEO expression style.
- Emphasize industry-level altitude and logical depth.
- Avoid marketing-oriented expression.
- LLM generation is fully based on Volcengine Ark and uses knowledge-grounded prompts from the enterprise knowledge base.

---
### 7. Content Production Module (Video Generation)

Responsibilities
Transform scripts into publishable video content.

Submodules
1. Asset matching: match images, videos, and B-roll.
2. Storyboard generation: generate visual structure.
3. Video generation: editing, stitching, AI generation.
4. Packaging: subtitles, music, covers, visual effects.

Outputs
- Complete video files.
- Asset packages.

Storage Convention (Volcengine)
- All produced files are written to TOS with standardized directory rules for traceability and reuse.

---
### 8. Public Opinion Risk Control Module

Responsibilities
Identify content risks to avoid brand and public-opinion issues.

Core Capabilities
- Sensitive-term detection.
- Negative public-opinion identification.
- Risk grading (low / medium / high / critical).
- Comment-section anomaly monitoring.
- Script content review.

Outputs
- Risk reports.
- Revision recommendations.
- Block/pass decisions.

---
### 9. Data Review Module

Responsibilities
Analyze post-publication performance and optimize system strategy.

Analysis Dimensions
- Views, likes, comments, reposts, favorites.
- Completion rate, follower growth.
- Comment sentiment.
- Viral-content characteristics.
- Root causes of low performance.

Outputs
- Weekly/daily reports.
- Viral pattern summaries.
- Content optimization recommendations.

---
## V. Agent Orchestration Layer (GoClaw)

Positioning
GoClaw serves as the system orchestration layer and is not a business module.

Responsibilities
- Schedule execution across modules.
- Orchestrate multi-Agent workflows.
- Manage task states.
- Control execution order and dependency relationships.
- Unify Volcengine client lifecycle management (Ark, TOS, DB service SDK clients).

Sample Workflow
Crawler Agent
-> NLP Analysis Agent
-> Topic Agent
-> Script Agent
-> Risk Control Agent
-> Content Generation Agent
-> Reporting Agent

---
## VI. Module Relationship Summary

Data acquisition -> data storage -> NLP analysis -> topic engine -> script generation -> content production -> publishing -> data review -> feedback into topic generation.

The enterprise knowledge base runs through the full process, while the public-opinion risk-control module provides end-to-end protection.

All model invocation, knowledge retrieval, object storage, and database capabilities are standardized on Volcengine.

---
## VII. Implementation Priorities

Phase 1 (MVP)
- Data Acquisition Module.
- NLP Semantic Analysis Module.
- Enterprise Knowledge Base Module.
- Topic Engine Module.

Phase 2
- Script Generation Module.
- Public Opinion Risk Control Module.

Phase 3
- Content Production Module.
- Data Review Module.

---
## VIII. Summary

The system's core capability is:
Understand data -> generate insights -> convert into topics -> produce content -> optimize via data feedback.

The goal is to achieve automated, systematized, and sustainably optimized entrepreneur IP content production.

The entire technical baseline is unified on Volcengine to ensure consistent architecture, O&M simplicity, and cost/permission governance.
