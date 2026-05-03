# Core Logic for Building the Topic Pool

One sentence: Centered on five major sources - "Haier industry lines + social hot topics + platform trending charts + Mr. Zhou's schedule + key milestones" - build a topic pool that can be automatically updated, filtered, and accumulated, with Agents responsible for web-wide information collection, cleaning, association, and storage.

## Five Topic Sources (Clearly Defined)

1. Haier industry lines: smart home, AI appliances, big health, industrial internet, overseas markets, new-category/new products (such as Rubik's Cube Refrigerator, Seeker series), major projects/signings/strategic releases.
2. Social hot topics: hot issues across livelihood, technology, health, consumption, home, workplace, etc. (such as elderly care, AI life, appliance replacement, healthy home living).
3. Platform trending charts: real-time trending lists, vertical hot searches, and topic rankings from Weibo/Douyin/Xiaohongshu/Zhihu/Bilibili/36Kr, etc.
4. Mr. Zhou (Zhou Yunjie) schedule and speeches: public events, meetings, livestreams, interviews, Two Sessions remarks, personal account updates, key quote highlights, and related hot topics (such as "Haier Wishing Pool" interactions).
5. Key milestones: festivals/solar terms, appliance industry nodes (315, AWE, 618, Double 11), industry anniversaries, corporate anniversaries, product launch dates, and policy rollout dates.

## Core Capabilities Required for Agents

- Real-time web-wide collection: multi-platform trending charts, news sources, industry sites, official WeChat/Douyin accounts, Mr. Zhou's public accounts, and overseas media when needed. It should also integrate with existing information-capture modules to automatically obtain social hot topics, industry trending lists, Haier strategy, product technology, authoritative industry knowledge, and key milestone/schedule materials, then complete precise matching between materials and scripts.
- Structured information cleaning: deduplication, low-quality filtering, ad removal; extraction of title, summary, keywords, popularity, timestamp, and source.
- Association matching (most critical): automatically associate "hot topics/milestones" with "Haier industry lines/Mr. Zhou's schedule" and output directly usable topics.
- Knowledge integration: automatically enrich with Haier-related product knowledge, technical parameters, cases, past coverage, and authoritative endorsements.
- Scheduled updates + push: generate daily/weekly topic lists; support keyword subscriptions, hot-topic alerts, and schedule reminders. Design an intelligent Agent with dedicated script generation capability, clearly defining core Agent capabilities, script-generation rules, account-adapted script templates, and the full execution workflow for direct production use.

## Script Generation and Account Capabilities

### 1. Account Style Replication Capability

Deeply learn the account's historical content style: formal and concise language, calm and confident tone, high-level positioning, no marketing rhetoric, emphasis on industry insight/corporate responsibility/era-level thinking, strict and progressive logic, and strict avoidance of vulgarity, exaggeration, and fragmented expression, fully aligned with an entrepreneur personal-IP tone.

### 2. Professional Script Generation Capability

Automatically generate standardized video scripts, including full elements such as video topic, core proposition, duration, voice-over copy, visual adaptation, and tags, while controlling both content altitude (industry/era/corporate-level positioning) and depth (deeply mining industry essence, development logic, and core value) to meet communication goals.

### 3. Flexible Adaptation Capability

Support automatic switching of script frameworks across different content scenarios (hot-topic interpretation, industry thinking, corporate philosophy, milestone reflections, technical insights). Allow duration adjustment as needed (60s short-frequency / 90s in-depth format), and support script optimization, iteration, and batch generation.

## Peer IP Accounts

- Perceive the status of other CEO accounts, such as click volume, view volume, and video content summaries.

## Detailed Rules for Data Analysis Capabilities

### 1. Self-owned Account Data Analysis (All Platforms)

Capture multi-dimensional metrics: plays, completion rate, likes, reposts, comments, favorites, follower growth, follower loss, audience profile, comment sentiment, viral topics, and low-performance content with automated analysis:
- Viral commonalities: copy structure, topic direction, tone and style, video duration, entry angle.
- Weakness diagnosis: low completion rate, weak opening hook, overly vertical topics, insufficient emotional resonance.
- User preferences: viewpoints preferred by audiences, high-frequency comment keywords, rejected content.
- Content review: weekly summary indicating which topic directions should continue and which should be eliminated directly.

### 2. Benchmark Entrepreneur Account Deconstruction (Lei Jun / Yu Chengdong, etc.)

Continuously capture full dimensions of benchmark accounts:
1. Topic direction: daily topics tend toward technology, industry trends, industrial thinking, life values, and business thinking.
2. Copy style: accessibility level of tone, sharpness of viewpoints, empathy intensity, and colloquial clarity of wording.
3. Video format: duration, camera style, voice-over rhythm, opening phrasing.
4. Data performance: average plays, viral frequency, interaction scale, fan stickiness, and comment direction.
5. Persona positioning: approachable / hardcore professional / high-level thinker.

### 3. Automated Horizontal Comparison Output

1. Our strengths
- High-level copy perspective, industrial positioning, official corporate tone, depth of thought, sound values, and stable premium tone.
2. Our weaknesses
- Relatively weak mass empathy, flat traffic-pulling openings, overly vertical topic circles, insufficient viewpoint sharpness, and limited lifestyle-oriented content.
3. Benchmark account strengths
- Pain-point-focused openings, direct and sharp viewpoints, more lifestyle content, stronger empathy, easy-to-understand language, and very fast hot-topic alignment.
4. Optimization recommendations
- Based on gaps, reversely optimize topic selection, script openings, copy tone, and content mix ratio.

## Value-Added Items

- Intelligent material matching and editing: automatic visual matching, automatic editing, music, subtitles, and effects packaging.
- Final video output: one-click generation of complete videos aligned with the account's quality tone.

## Summary of Five Core Modules

1. Web-wide information collection: capture and consolidate social hot topics, industry public opinion, industry intelligence, and corporate updates.
2. Customized style scripts: replicate Mr. Zhou's account tone and produce entrepreneur voice-over scripts with altitude and depth.
3. Dual-dimension data analysis:
- Full-platform self-owned account review, viral deconstruction, and user profiling.
- Benchmarking against entrepreneur IPs such as Lei Jun/Yu Chengdong, horizontal strengths-vs-weaknesses analysis, and optimization strategy output.
4. Intelligent material matching and editing: automatic visual matching, automatic editing, music, subtitles, and effects packaging.
5. Final output: one-click generation of complete videos aligned with the account's quality tone.

## Supplement

### I. Intelligent Comment-Section Processing Capability

1. Automatically capture all comments, direct messages, and highly liked comments across platforms.
2. Category labeling: positive praise, objective questions, neutral suggestions, negative doubts, conflict-stirring narrative steering, malicious public opinion.
3. Comment sentiment analysis: automatically identify sentiment tendencies, summarize high-frequency questions, and extract key public concerns.
4. High-quality comment accumulation: extract high-value viewpoints and audience demands, then feed them back into subsequent topic pools.

### II. Public Opinion Early Warning and Risk Control Capability

1. Real-time monitoring of negative terms, controversial statements, and derivative public narratives.
2. Automatic alerts for sensitive words, negative directionality, malicious narrative steering, and concentrated smearing.
3. Tiered risk assessment: low risk / medium risk / high risk.
4. Public opinion brief output: risk points, propagation trends, distribution platforms, mitigation suggestions, and copywriting red-flag guidance.
5. Synchronously feed into script writing: avoid controversial topics and revise expression caliber.

## Complete Closed-Loop Pipeline

Information collection (topic discovery) -> customized script generation -> AI automatic editing -> post-publication public-opinion monitoring -> benchmark analysis/data review iteration.
