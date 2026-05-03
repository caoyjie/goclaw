# 模块 1：数据采集

## 职责

按计划从五类选题来源采集原始数据，并存储到火山引擎 TOS 原始数据层。该模块仅负责采集器能力：采集、最小规范化、入库，不做其他处理。

**六类数据源：**
1. **平台热榜数据** - 通过 TrendRadar MCP（微博、抖音、小红书、知乎、Bilibili、36Kr 等）
2. **社交热点 / 行业资讯** - 通过 TrendRadar MCP（RSS 与新闻站点）
3. **CEO 日程与发言** - 由海尔内部数据流推送
4. **评论数据** - 通过第三方评论 API（如 Apify、平台开放 API）
5. **关键里程碑事件** - 手工导入或日历事件流
6. **管理员上传文件** - 管理员通过火山引擎对象存储管理控制台直接上传文档、CSV、PDF、表格

---

## 清晰边界 - 本模块不做什么

- 不做 NLP，不做情感分析，不做关键词提取
- 不做相关性打分，不按海尔相关性过滤
- 不做内容级去重（同一热点跨采集窗口重复由模块 3 处理）
- 不做同源响应内原始条目去重
- 不做选题生成或内容决策
- 不做 Agent 执行（这是数据流水线任务，不是 Agent 任务）
- 不做租户维度识别（这是系统级任务）

---

## 存储

**原始数据目标：Volcengine TOS**

所有采集数据以原始 JSON 写入 TOS。写入具备幂等性：同一任务运行总是写入同一个确定路径（见下方“去重机制”）。

TOS 是 **统一原始数据层**，同时承载 Airflow 采集数据与管理员上传文件。无论数据来源如何，模块 3 都从 TOS 读取。

**路径规范 - Airflow 采集数据：**
```text
tos://ceo-ip-raw/
  trending/{platform}/{YYYY-MM-DD}/{run_id}.json
  comments/{platform}/{YYYY-MM-DD}/{run_id}.json
  ceo_schedule/{YYYY-MM-DD}/{run_id}.json
  milestones/{YYYY-MM-DD}/{run_id}.json
```

**路径规范 - 管理员上传：**
```text
tos://ceo-ip-raw/
  uploads/{category}/{filename}
```

- `category` 待定，由管理员定义（如 product、ceo、brand、industry、competitor、other）
- 接受所有文件格式：PDF、Word（.docx）、Excel（.xlsx）、CSV、纯文本、中英文混合
- 原始层保留全部版本，重复上传不覆盖；版本规范待定
- 上传内容去重由下游模块 3（NLP 黄金层）处理，不在本模块处理

**为什么用 TOS，而不是 GoClaw Vault：**
- 原始数据量大且噪声高，不适合放 Vault
- 大多数原始条目会在模块 3 处理后被丢弃
- Vault 只接收清洗富化后的高价值内容（模块 3 输出），不是原始输入

---

## 调度与编排

**编排器：Airflow（系统级）**

模块 1 任务由 Airflow 管理，不由 GoClaw cron 管理，原因如下：
- GoClaw cron 是租户级；模块 1 是无租户上下文的系统级任务
- 模块 1 -> 3 -> 5 是依赖流水线，Airflow 负责依赖链管理
- Airflow 提供统一可视化看板，便于追踪全部模块任务

**GoClaw 在这里的职责：** 不参与调度。GoClaw 是内容生成（模块 6+）的 Agent 执行引擎，不是数据流水线编排器。

**建议调度频率：**
- 热点数据：每 1 小时
- 评论数据：每 2 小时
- CEO 日程：海尔数据流事件触发
- 里程碑：每日同步

---

## 故障处理与告警

当采集任务失败（API 宕机、限流、网络错误）时：

1. Airflow 将任务标记失败，并按退避策略重试
2. 持续失败（超过重试上限）后触发告警：
   - **GoClaw 告警事件** - GoClaw 路由到已配置消息渠道（Telegram、飞书、Zalo、Discord，按部署配置）
   - **GoClaw Web UI 广播** - 在 Web 聊天界面对全体用户推送警告消息
3. 模块 1 不管理告警路由，只向 GoClaw 发出一次事件，由 GoClaw 负责分发

---

## 管理员上传流程

管理员通过 **火山引擎对象存储控制台** 直接上传文件，不经过 GoClaw UI。

**访问控制：**
- 仅管理员角色持有 TOS bucket 凭证
- 不提供 GoClaw 上传页，不提供数据浏览页，文件管理全部在对象存储控制台完成

**上传后摄取触发：**
TOS 事件通知 -> 火山引擎消息队列（如 RocketMQ） -> Airflow DAG 逐文件消费处理。按顺序摄取可避免模块 3 被瞬时压垮。

**无需手工触发** - 上传到 TOS 后自动触发流水线。

---

TrendRadar 以 **现有形态直接使用**，通过其 MCP Server 接口调用，不改造 TrendRadar。

GoClaw（或 Airflow 任务）调用 TrendRadar MCP 工具：
- `search_tools` - 按关键词/平台查询热点数据
- `data_query` - 拉取已采集热点条目
- `storage_sync` - 需要时将 TrendRadar 本地存储同步到 TOS

TrendRadar 保持独立服务形态。模块 1 将其视为黑盒 MCP 工具提供方。

---

## 输出契约

写入 TOS 的每条原始数据遵循最小 schema：

```json
{
  "source": "weibo_trending | douyin_trending | ceo_schedule | comment | milestone | ...",
  "platform": "weibo",
  "collected_at": "2026-05-02T10:00:00Z",
  "run_id": "weibo_trending_20260502_1000",
  "raw": {}
}
```

`run_id` 为确定性值：`{source}_{platform}_{YYYYMMDD}_{HHMM}`，不是随机 UUID。

`raw` 字段保存未经修改的 API 原始响应。模块 3（NLP）从 TOS 读取并负责后续解析与富化。

---

## 去重机制

两层机制避免重复摄取：

**第 1 层：Airflow 任务级控制**
Airflow 按 DAG 运行窗口跟踪执行状态。若某窗口已在运行或已完成，不会重复触发，避免编排层重复调度。

**第 2 层：TOS 幂等写入**
`run_id` 是确定性值，由 `hash(source + platform + time_window)` 派生，不是随机 UUID。若同一窗口任务重复触发，会写入同一路径；第二次写入覆盖相同数据，不会累积重复文件。

**本模块不处理：**
内容级去重。相同热点在不同时间窗口重复出现（例如 10:00 和 11:00 都在热榜）属于预期重叠。内容/语义级去重由模块 3（NLP）在清洗阶段处理。

---

## 协作关系

| 模块 | 关系 |
|---|---|
| **模块 3（NLP）** | 下游消费者 - 读取 TOS 原始文件（采集 + 上传），进行清洗与富化 |
| **模块 4（知识库）** | 模块 1 不直接交互 |
| **Airflow** | 负责本模块调度与依赖链 |
| **GoClaw** | 接收失败告警事件，并路由到消息渠道与 Web UI |
| **TrendRadar** | 通过 MCP 调用的上游数据提供方 |
| **海尔内部数据流** | CEO 日程数据上游推送源 |
| **火山引擎对象存储控制台** | 管理员上传入口 - 不需要 GoClaw UI |
| **火山引擎消息队列** | 管理员上传文件的顺序处理队列 |

---

## Airflow 与 GoClaw 职责划分

| 事项 | 责任方 |
|---|---|
| 调度模块 1 定时任务 | Airflow |
| 失败重试 | Airflow |
| 模块 1 完成后触发模块 3 | Airflow |
| 流水线可视化看板 | Airflow |
| 告警消息路由（渠道 + Web UI） | GoClaw |
| Agent 任务（脚本生成、选题引擎） | GoClaw |
| 内容生产流水线（模块 6-8） | GoClaw |
