# 2026-05-03 WebUI Branding & Deploy Flow (ceo_ip)

## 概览
今天主要完成两类工作：
1. 本地 Docker 开发/发布流程脚本化（保留数据与配置的无损发布）。
2. WebUI 品牌与视觉改造（CEO IP 助手主题）。

## 1) Docker 流程脚本化

### 新增脚本
- `scripts/dev-docker-local.sh`

### 关键能力
- `start`: build + up + migrate + health check
- `deploy`: 无损发布（仅重建/替换 goclaw，保留现有 volumes、数据、配置）
- `rollback <git-ref>`: 切换到指定版本并执行无损发布
- 其他辅助命令：`status`, `logs`, `down`, `reset`, `endpoints`, `prepare-env`

### 发布验证
- 执行了 `deploy`，容器重建并通过 `/health`。
- 数据库 schema 检查结果：无需迁移（current=required）。

## 2) WebUI 品牌改造

### 文档
- 重写品牌需求文档：`docs/webui-branding-change-ceo-ip.md`

### 视觉与品牌改动（ui/web）
- 品牌文案：统一为 `CEO IP 助手`
- Logo：在核心入口中隐藏（登录、setup、about、加载态、侧边栏标题位）
- 语言：移除越语，仅保留中/英（`en`, `zh`）
- 配色：
  - Light 模式背景改为纯白
  - 组件区保持浅蓝
  - 侧边栏改为深蓝底、白字、蓝色悬停态
- 侧边栏可读性修复：
  - 顶部标题、分组标题、连接状态、tenant/role 信息统一白字系

### 关键文件
- `ui/web/src/index.css`
- `ui/web/src/lib/constants.ts`
- `ui/web/src/i18n/index.ts`
- `ui/web/src/components/layout/sidebar.tsx`
- `ui/web/src/components/layout/sidebar-group.tsx`
- `ui/web/src/components/layout/connection-status.tsx`
- `ui/web/src/components/layout/about-dialog.tsx`
- `ui/web/src/pages/login/login-layout.tsx`
- `ui/web/src/pages/setup/setup-layout.tsx`
- `ui/web/src/routes.tsx`
- `ui/web/src/i18n/locales/{en,zh,vi}/*.json`（品牌文案替换）

## 3) 注意事项
- 技术兼容标识（如 `X-GoClaw-*`, `goclaw:*`, `GOCLAW_*`）未做破坏性变更。
- 本地环境中 `pnpm` 不可直接使用时，已通过 Docker 构建链完成前端构建与部署验证。

## 4) 后续建议
- 如需进一步统一视觉，可继续清理组件内个别硬编码 `violet-*` class。
- 在真实用户分辨率下做一轮浅色主题可读性验收（表格、badge、告警色对比度）。
