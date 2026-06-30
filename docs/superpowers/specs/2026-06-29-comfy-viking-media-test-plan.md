# ComfyUI 生成媒体上传 TOS 的测试方案

## 背景

GoClaw 已能通过 ComfyUI MCP 生成图片，后续也需要覆盖视频。但当前问题是：MCP 产物里的下载 URL 或下载命令不能稳定发送到当前 Web chat，也不应该让模型自行拼接被脱敏的 URL 或执行 `curl`。

第一版方案采用后端 Remote Media Bridge：

1. 从 Comfy MCP `get_output` 结果中提取图片/视频产物。
2. 服务端安全下载远程产物到临时文件或流式 reader。
3. 上传到火山引擎 TOS 对象存储。
4. 返回 `RemoteMediaRef`，让 Web chat 展示预览和下载入口。
5. 清理本地临时文件，不写入长期本地 media 目录。

`byted-viking-cli` 可以作为可选上传路径验证，但第一版产品契约应该以 TOS object storage 为主，而不是 Viking Knowledge 导入结果。

## 术语

- `TOS`: Volcengine TOS object storage。本文统一使用 `TOS`，不是 `TOC`。
- `RemoteMediaRef`: 后端返回给会话/UI 的远程媒体引用。
- `Viking CLI`: GoClaw runtime 里的 `byted-viking-cli`，当前主要能力是 `knowledge add-file`。

## 主契约

桥接层成功时应返回：

```json
{
  "url": "https://...",
  "key": "goclaw-media/tenants/{tenant_id}/agents/{agent_id}/sessions/{session_key_hash}/2026/06/29/{media_id}.png",
  "mime_type": "image/png",
  "size": 123456,
  "expires_at": "2026-06-30T10:00:00Z"
}
```

字段要求：

- `url`: 可被 Web chat 预览或下载的 URL。私有 bucket 用 presigned URL，公开 bucket/CDN 用 public URL。
- `key`: TOS object key，用于审计、重新签名、清理。
- `mime_type`: 服务端嗅探后的 MIME，不只信任 HTTP header。
- `size`: 实际上传字节数。
- `expires_at`: presigned URL 的过期时间；public URL 可为空或省略。

## TOS Object Key 结构

第一版必须使用租户/会话隔离、日期分区的 object key：

```text
goclaw-media/
  tenants/{tenant_id}/
    agents/{agent_id}/
      sessions/{session_key_hash}/
        yyyy/mm/dd/
          {media_id}.{ext}
```

实际 key 示例：

```text
goclaw-media/tenants/8f3c.../agents/ceo-assistant/sessions/a91f2c7e/2026/06/29/0192f6e8-7f9e-7a80-bc83-2ff4c6bb7d0a.png
```

字段规则：

- `goclaw-media/`: 固定 prefix，便于生命周期规则、成本统计和权限隔离。
- `tenant_id`: 使用 UUID 或稳定租户 ID，作为对象存储隔离边界。
- `agent_id`: 优先使用 agent key 或安全的 agent identifier，便于排障和归因。
- `session_key_hash`: 对 session key 做 hash 后截断，例如前 8 到 16 个 hex 字符，避免泄漏原始 session key。
- `yyyy/mm/dd`: 使用上传发生时的 UTC 日期。
- `media_id`: 使用 UUIDv7 或项目现有 UUID 生成方式。
- `ext`: 只能来自服务端验证后的 MIME 映射，不能直接信任 Comfy 文件名。

禁止放入 object key：

- prompt 文本
- 用户名
- 原始 Comfy 文件名
- 原始 session key
- provider signed URL 参数
- `output.png` 这类非唯一名称

MIME 到扩展名映射：

| MIME | ext | kind |
|---|---|---|
| `image/png` | `.png` | `image` |
| `image/jpeg` | `.jpg` | `image` |
| `image/webp` | `.webp` | `image` |
| `video/mp4` | `.mp4` | `video` |

## 支持范围

第一版必须覆盖：

- `image/png`
- `image/jpeg`
- `image/webp`
- `video/mp4`

第一版可延后：

- `video/webm`
- `audio/*`
- 3D 文件
- 批量多文件结果
- 过期 URL 的重新签名接口

## 上传模式

### 必选：`tos_s3`

GoClaw 后端直接使用 TOS S3-compatible endpoint 上传对象。

测试重点：

- TOS endpoint、region、bucket、prefix 透传正确。
- private bucket 返回 presigned URL。
- public bucket/CDN 返回稳定 public URL。
- object key 符合 `goclaw-media/tenants/{tenant_id}/agents/{agent_id}/sessions/{session_key_hash}/yyyy/mm/dd/{media_id}.{ext}`。
- 上传失败时不会返回假成功。

### 可选：`viking_cli`

调用现有 `byted-viking-cli knowledge add-file --collection <name> --file <local_path>`。

测试重点：

- 只作为可插拔 uploader 验证，不作为主契约。
- 如果 CLI 只返回 `doc_id`，不得伪造成下载链接。
- 如果 CLI 返回可下载 URL，则转换为 `RemoteMediaRef`。
- CLI 配置缺失、collection 缺失、返回非 JSON 都要有明确降级。

## 安全测试

Remote Media Bridge 会服务端下载外部 URL，因此安全测试是阻断项。

必须覆盖：

- 拒绝非 `http/https` URL。
- 拒绝 `localhost`、`127.0.0.0/8`、`::1`。
- 拒绝 RFC1918 私网地址。
- 拒绝 link-local、metadata endpoint，例如 `169.254.169.254`。
- 拒绝 DNS 解析到私网地址。
- 拒绝重定向到私网地址。
- 限制最大重定向次数。
- 限制下载超时。
- 限制最大文件大小。
- 校验 `Content-Length` 与实际读取字节数。
- 不能只信任远端 `Content-Type`，必须进行 MIME sniffing。
- 下载失败时清理临时文件。

## 测试分层

### 1. 单元测试

只测桥接层纯逻辑，不访问真实外网，不调用真实 TOS 或 Viking CLI。

覆盖点：

- Comfy MCP `get_output` 结果解析。
- URL 与 `download_command` 提取。
- URL 安全校验。
- MIME 与大小校验。
- object key 生成。
- `RemoteMediaRef` 构造。
- TOS public/presigned URL 分支。
- Viking CLI stdout/stderr 解析。
- 错误归一化。

建议断言：

- 没有媒体产物时返回空结果，不影响最终回复。
- 图片和视频 MIME 均可通过。
- 非支持 MIME 被拒绝。
- 只有 `doc_id` 时不会伪造 `url`。
- `url_mode=presigned` 时必须有 `expires_at`。
- `url_mode=public` 时不能生成无意义过期时间。

### 2. 假 HTTP + 假 TOS 集成测试

用 `httptest.Server` 模拟 Comfy 下载 URL，用 fake uploader 记录上传输入。

覆盖点：

- 真实 HTTP 下载路径。
- MIME sniffing。
- 下载大小限制。
- 临时文件生命周期。
- upload reader 内容与下载内容一致。
- 上传失败时错误返回且无最终媒体引用。

建议断言：

- 上传收到的 bytes 与源文件完全一致。
- `key` 使用固定 prefix、tenant、agent、session hash、UTC 日期和 MIME 派生扩展名。
- `mime_type` 来自服务端检测结果。
- `size` 是实际字节数。
- 成功和失败路径都不会遗留临时文件。

### 3. 假 CLI 集成测试

用本地假 `viking-cli` 脚本替代真实二进制，只验证 GoClaw 调用路径。

覆盖点：

- `knowledge add-file` 命令被调用一次。
- `--collection`、`--file`、`--tos-bucket`、`--tos-endpoint` 按预期透传。
- 假 CLI 输出 JSON 时能提取 `download_url`、`uri`、`doc_id`。
- 假 CLI 输出非 JSON 文本时进入降级分支。
- CLI 返回非 0 时不会返回媒体链接。

### 4. Pipeline / Agent 集成测试

验证桥接层接入 agent tool loop 或 finalize 附近后，最终消息包含远程媒体引用。

覆盖点：

- Comfy MCP tool result 被识别。
- 桥接层上传后，assistant message 附带 media ref 或最终文本链接。
- 失败时 final content 包含可读错误或降级说明。
- 不能要求模型调用 `message(action=send)`。
- 不能要求模型执行 `curl`。

建议断言：

- `providers.Message.MediaRefs` 或对应远程媒体字段包含图片/视频结果。
- `state.Observe.FinalContent` 不包含脱敏 URL 拼接产物。
- 多轮工具调用时不会重复上传同一媒体。

### 5. Web UI 合约测试

现有 Web `MediaGallery` 已支持 `image`、`audio`、`video` kind，并假设 `item.path` 可直接预览/下载。远程媒体接入后要明确 UI 契约。

覆盖点：

- 图片 remote URL 可显示缩略图。
- 视频 remote URL 可用 `<video controls>` 播放。
- 下载按钮使用 remote URL。
- presigned URL 过期时显示可理解状态。
- `fileName`、`size`、`mimeType` 展示不破坏布局。

第一版如果不新增 `remoteMediaItems`，则必须规定：

- 远程 `url` 映射到现有 `MediaItem.path`。
- `key`、`expires_at` 暂时不进入 UI，或进入可选字段。
- 过期 URL 只提示重新生成，不做重新签名。

### 6. 真实环境冒烟测试

真实环境测试单独加 build tag 或手动脚本，不进入默认 `go test ./...`。

建议标签：

- `integration`
- `requires-network`
- `requires-tos`
- `requires-viking`，仅 Viking CLI 路径需要

TOS 主路径验收：

- 上传一个 PNG 到测试 bucket。
- 上传一个 MP4 到测试 bucket。
- object key 存在。
- object size 与源文件一致。
- object MIME 正确。
- 返回 URL 用 `GET` 打开是 `200`。
- 图片能被浏览器预览。
- 视频能被浏览器播放或至少能被 `<video>` 加载 metadata。

Viking CLI 可选路径验收：

- `knowledge add-file` 能上传本地文件。
- 返回值如果有 URL，则 URL 可打开。
- 返回值如果只有 `doc_id`，记录为“知识库导入成功，但不能满足媒体下载链接契约”。

## 重点场景

### 正常路径

- Comfy 输出一个 PNG。
- Comfy 输出一个 MP4。
- 成功下载到临时文件。
- 成功上传到 TOS。
- 返回 `RemoteMediaRef`。
- Web chat 能展示预览和下载入口。

### 下载失败

- 404。
- 超时。
- URL 过期。
- 重定向到非法地址。
- 返回 HTML 错误页但 header 伪装成图片。
- `Content-Length` 超过限制。
- 实际读取超过限制。

### 上传失败

- TOS 配置缺失。
- TOS 凭证错误。
- bucket 不存在。
- endpoint/region 不匹配。
- put object 超时。
- presign 失败。

### 输出不完整

- Comfy MCP 结果只有脱敏 URL。
- Comfy MCP 结果只有 `download_command`。
- Viking CLI 只有 `doc_id`。
- CLI 返回结构变更。

### 清理失败

- 临时文件删除失败。
- 上传成功但清理失败。
- 下载失败后清理失败。

清理失败不应阻断用户结果，但必须写结构化日志。

## 建议测试文件位置

- `internal/mediaremote/*_test.go`
- `internal/mediaremote/testdata/*`
- `internal/pipeline/*_test.go`
- `internal/agent/*_test.go`
- `ui/web/src/components/chat/**/__tests__/*`
- `ui/web/src/pages/chat/**/__tests__/*`

如果第一版暂时没有 `internal/mediaremote`，应先创建独立包再测，避免把对象存储、安全下载、CLI 调用散落在 pipeline 里。

## 验收标准

- 单元测试覆盖图片、视频、失败路径和 SSRF 阻断路径。
- 假 HTTP + 假 TOS 集成测试不依赖外网，能稳定通过。
- TOS 主路径真实冒烟测试至少覆盖一个 PNG 和一个 MP4。
- 返回的 URL 能被浏览器实际访问。
- Web chat 能展示图片缩略图和视频播放器。
- 失败路径不会泄漏临时文件、半成品链接或假成功状态。
- Viking CLI 只能作为可选 uploader；如果不能返回可下载 URL，不阻塞 TOS 主路径。

## 风险与约束

- `byted-viking-cli` 当前更像知识库导入工具，不保证返回稳定下载 URL。
- 直接复用 backup S3 配置会混淆备份权限和媒体权限，应使用独立媒体对象存储配置。
- presigned URL 会过期，第一版要么提示过期，要么后续增加重新签名接口。
- 视频文件通常更大，必须有独立大小限制和超时配置。
- 真实环境测试依赖外部网络和云资源，不能作为默认 CI 阻断项。

## 结论

第一版测试策略应该以 TOS object storage 的 `RemoteMediaRef` 契约为核心。Viking CLI 可以辅助验证现有 runtime 上传能力，但不能替代媒体交付契约。只有当图片/视频都能从 Comfy MCP 输出进入 TOS，并在 Web chat 中稳定预览或下载，才算这个问题完成。
