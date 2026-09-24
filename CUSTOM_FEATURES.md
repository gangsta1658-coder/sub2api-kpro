# Sub2API 自定义功能保护清单

更新时间：2026-08-22

## 文档目的

本文件记录 `kpro.eu.cc` 当前部署中不属于官方发行版的功能，以及升级时必须保留的代码、数据库结构和验收项目。后续升级到任意官方版本时，禁止直接用官方源码、官方镜像或全量构建产物覆盖当前自定义版本，必须按本文档执行合并和验收。

当前目标基线：官方 `v0.1.179` + Kpro 自定义提交 `c157c83801` + Glass Auth 提交 `dda9a0f230` + 移动端白边修复 + Codex 5h/7d 额度透支模块。

## 升级原则

1. 官方版本只作为上游基线；自定义功能必须做三方合并（旧自定义版本、官方新版本、合并结果）。
2. 下列重叠文件不能整文件覆盖。官方对同一文件的新增修复要保留，同时重新应用自定义代码块。
3. `deploy/.env`、`/app/data` 数据卷、数据库和 Redis 数据必须单独备份，不能由源码压缩包覆盖。
4. 数据库迁移必须按版本顺序执行。禁止删除 `daily_checkins`、`community_messages` 或 `accounts.extra.account_folder` 数据。
5. 新镜像必须使用明确的自定义标签，例如 `sub2api:0.1.177-custom-YYYYMMDD`，不能直接切换到没有自定义代码的 `weishaw/sub2api` 官方镜像。
6. 部署后必须完成本文档末尾的验收清单；验收失败时立即使用本次升级快照回滚。

本服务器为 ARM64。服务器没有可用的 Docker BuildKit/buildx，因此构建时使用源码根目录的 `Dockerfile.legacy` 兼容副本；该副本只移除 BuildKit 专用语法，保留官方 PostgreSQL 客户端层。构建时必须显式使用 `TARGETOS=linux`、`TARGETARCH=arm64` 和目标 `VERSION`，不能把 ARM64 主机误构建成 amd64 镜像。

## 自定义功能清单

### 1. 账号文件夹归类

用途：在账号管理页按文件夹查看和管理账号，便于分类、批量移动和整理。

- 页面：`/admin/accounts`
- 后端接口：`GET /api/v1/admin/accounts/folders`
- 账号归类字段：`accounts.extra.account_folder`
- 未归类筛选值：`__uncategorized__`
- 空文件夹名称保存在浏览器 `localStorage` 的 `account-empty-folders-v1` 中。
- 支持创建、重命名、删除、选择文件夹、批量移动账号和未归类筛选。
- 重命名或删除文件夹时，账号的 `extra.account_folder` 必须同步更新，不能只修改前端显示。

核心实现：

- `backend/internal/service/account_service.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/service/admin_account.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/views/admin/AccountsView.vue`
- 账号相关的 `frontend/src/i18n/locales/zh/admin/`、`frontend/src/i18n/locales/en/admin/` 翻译文件

### 2. 每日签到和日历

用途：认证用户每天领取余额奖励，并在签到页查看日期和当日状态。

- 页面：`/check-in`
- 接口：`GET /api/v1/user/check-in`、`POST /api/v1/user/check-in`、`GET /api/v1/user/check-in/calendar`
- 奖励：每天固定增加 `0.20` 美元余额。
- 日期以服务端配置时区的日历日期为准。
- 同一用户同一天只能成功领取一次；并发请求也只能发放一次奖励。
- 页面显示当前服务端月份的签到日历、今日是否已签到、奖励金额、当前余额和领取按钮；日历只读查询历史记录，不改变领取逻辑。

数据库：

- 迁移：`backend/migrations/192_daily_checkins.sql`
- 表：`daily_checkins`
- 唯一约束：`(user_id, checkin_date)`
- 余额更新和签到记录插入必须保持原子性。

核心实现：

- `backend/internal/repository/user_repo.go` 中的签到查询和领取逻辑
- `backend/internal/repository/user_repo.go` 中的当月签到日期查询
- `backend/internal/repository/user_repo_daily_checkin_test.go`
- `backend/internal/service/user_service.go` 中的 `DailyCheckIn` 和奖励逻辑
- `backend/internal/service/user_daily_checkin_test.go`
- `backend/internal/handler/user_handler.go`
- `backend/internal/server/routes/user.go`
- `frontend/src/api/user.ts`
- `frontend/src/views/user/CheckInView.vue`
- `frontend/src/router/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh/dashboard.ts`、`frontend/src/i18n/locales/en/dashboard.ts`

### 3. 公共交流区

用途：认证用户在交流区发布文字、图片和链接，并查看其他用户的消息。

- 页面：`/community`
- 列表：`GET /api/v1/chat/messages`
- 发布：`POST /api/v1/chat/messages`
- 撤回：`DELETE /api/v1/chat/messages/:id`
- 图片：`GET /api/v1/chat/media/:filename`
- 页面每 3 秒增量轮询新消息，并按消息 ID 去重。
- 支持 JPEG、PNG、GIF、WebP；单张图片不超过 5 MiB，尺寸不超过 `8000 x 8000`。
- 消息作者可在发送后两分钟内撤回自己的消息，不能撤回其他作者或超时消息。
- 链接必须以安全的外部链接形式展示；消息旁显示作者名称和数字 ID。
- 图片文件保存在持久化数据卷的 `data/community` 目录。

数据库：

- 迁移：`backend/migrations/193_community_messages.sql`
- 表：`community_messages`
- 消息通过 `user_id` 关联 `users`，使用 `deleted_at` 软删除。

核心实现：

- `backend/internal/repository/chat_repo.go`
- `backend/internal/service/chat_service.go`
- `backend/internal/handler/user_handler.go` 中的聊天处理逻辑
- `backend/internal/server/routes/user.go`
- `backend/migrations/193_community_messages.sql`
- `frontend/src/api/chat.ts`
- `frontend/src/views/user/CommunityView.vue`
- `frontend/src/router/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh/dashboard.ts`、`frontend/src/i18n/locales/en/dashboard.ts`

### 4. 自定义导航、翻译和品牌资源

以下内容属于自定义界面的一部分，升级时不能因官方页面重构而删除：

- 侧边栏中的“每日签到”和“交流区”入口。
- `/check-in`、`/community` 路由及页面标题/描述元数据。
- 中文和英文的签到、交流区、账号文件夹翻译。
- `favicon.png` 及项目现有自定义品牌资源。
- `assets/partners/logos/AICodeMirror.jpg`、`assets/partners/logos/anpin.jpg`、`assets/partners/logos/unity2.png` 三个自定义合作方 logo。
- `.monkeycode/specs/daily-check-in/` 和 `.monkeycode/specs/public-community-chat/` 中的需求与设计记录。

### 5. 签到与交流区功能开关

用途：管理员可在“系统设置 > 功能开关”分别启用或关闭每日签到和交流区。

- 设置键：`daily_check_in_enabled`、`community_enabled`。
- 两个开关默认开启；设置缺失或临时读取失败时保持开启，避免升级后误隐藏已有功能。
- 保存后刷新公共设置缓存，侧边栏会同步隐藏对应入口：每日签到在用户/管理员个人菜单中隐藏，交流区在用户和管理员主菜单中隐藏。
- 关闭后前端路由重定向到仪表盘，后端签到、交流消息和交流图片接口返回 `404`，不能仅通过直接 URL 或 API 绕过开关。

核心实现：

- `backend/internal/service/setting_features.go`
- `backend/internal/server/middleware/feature_gate.go`
- `backend/internal/server/routes/user.go`
- `frontend/src/stores/app.ts`
- `frontend/src/utils/featureFlags.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/router/index.ts`
- `frontend/src/views/admin/SettingsView.vue`

### 6. 自定义 OpenAI 兼容平台账号

用途：在账号管理中创建只使用 API Key 的自定义平台账号，供 Kpro 的兼容端点接入。

- 账号通过 `extra.custom_platform`、平台名称和可选 logo 标记。
- logo 只接受 PNG/JPEG/WebP/GIF data URL，大小上限 96 KiB。
- 创建、编辑、列表展示和平台图标都必须保留，不能因官方平台目录重构而把自定义账号当成未知平台删除。

核心实现：

- `backend/internal/service/custom_platform.go`
- `backend/internal/service/admin_account.go`
- `backend/internal/handler/admin/account_handler.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/types/index.ts`

### 7. 站点 IP/CIDR 黑名单

用途：管理员在系统设置中维护站点级 IPv4/IPv6 地址或 CIDR 网段，网关中间件在请求入口统一拦截。

- 黑名单条目持久化在系统设置中，支持添加、删除和规范化校验。
- 匹配 IPv4、IPv6 和 CIDR；无效条目必须返回明确的 4xx，不得写入半截配置。
- 中间件只阻断匹配来源，不能绕过认证，也不能阻断管理员合法配置接口。

核心实现：

- `backend/internal/handler/admin/setting_handler_ip_blacklist.go`
- `backend/internal/server/middleware/ip_blacklist.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/setting_update.go`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/api/__tests__/settings.ipBlacklist.spec.ts`

### 8. Codex quota overdraft

用途：为符合条件的 Codex OAuth 账号提供 5h/7d 周期的额度探测、透支调度和后台状态展示。

- 生产开关：`GATEWAY_CODEX_QUOTA_OVERDRAFT_ENABLED`，当前镜像默认开启，可显式设为 `false` 暂停。
- 透支状态保存在现有账号 `extra` JSONB 中，不删除或重建账号数据。
- 图片、Compact、Embedding、Count Tokens、Live 和 Shadow 等不适用端点继续走官方原有路径。

核心实现：

- `backend/internal/service/openai_codex_quota_overdraft.go`
- `backend/internal/service/openai_codex_quota_overdraft_probe.go`
- `backend/internal/repository/account_repo_codex_overdraft.go`
- `frontend/src/components/account/CodexOverdraftStatus.vue`
- `frontend/src/components/account/CodexOverdraftStats.vue`
- `CODEX_QUOTA_OVERDRAFT_CUSTOMIZATION.md`

### 9. Glass Auth 登录、注册和邮箱验证页

用途：统一认证入口的玻璃视觉，并在移动端保持整屏深色背景。

- 基础提交：`dda9a0f2307e81e44b87c70fa19658fd07aca3ca`，父提交：`c157c838013ae7c76ed432e34a6d3d9491c7f893`。
- 页面：`/login`、`/register`、`/verify-email`。
- 移动端修复使用 `100dvh`、`html/body/#app` 黑色背景和 `overscroll-behavior-y: none`，390x844 视口上下不露白边。
- 受保护文件：
  - `frontend/src/main.ts`
  - `frontend/src/components/layout/GlassAuthLayout.vue`
  - `frontend/src/styles/glass-auth.css`
  - `frontend/src/views/auth/LoginView.vue`
  - `frontend/src/views/auth/RegisterView.vue`
  - `frontend/src/views/auth/EmailVerifyView.vue`

验收：`pnpm exec vue-tsc --noEmit`、认证相关测试 11/11、Vite 构建、390x844 移动端截图和公网 `/health` 均通过。

### 10. 可重放 custom layer

完整 patch 和脚本位于仓库根目录 `custom/`：

- `custom/manifest.json`：不可变提交、功能清单、保护路径和运行时排除项。
- `custom/patches/001-kpro-custom-v0.1.179.patch`：官方 v0.1.179 到 overdraft + 全部 Kpro 功能的合并补丁。
- `custom/patches/002-glass-auth.patch`：认证页面。
- `custom/patches/003-glass-auth-mobile.patch`：移动端白边修复。
- `custom/scripts/apply-customizations.*`：按依赖顺序执行 `git apply --check --3way`，遇到冲突立即停止。

后续升级必须在独立 staging 中应用 custom layer，不能直接用官方镜像或官方源码覆盖生产版本；数据库、Redis、`deploy/.env` 和 `/app/data` 永远单独备份。服务器上的独立归档位于 `/home/ubuntu/custom/sub2api-kpro-layer-20260822`，生产源码目录只保留本记录文件，不作为 Git source of truth。

### 11. Agnes 图像模型生图识别与路由支持

用途：允许通过 OpenAI 标准兼容生图端点 `/v1/images/generations` 和 `/images/generations` 直接请求 `agnes-image-*` 系列生图模型（如 `agnes-image-2.0-flash`、`agnes-image-2.1-flash`、`agnes-image-2.5-flash`），不再被网关的模型白名单拦截报错 400。

- 端点：`POST /v1/images/generations`、`POST /images/generations`
- 模型识别前缀：`agnes-image-`
- 规则：在 `isOpenAIImageGenerationModel()` 中加入对 `agnes-image-` 前缀的判定，使包含该前缀的请求能够合法通过图像请求校验，并正常透传至对应的 OpenAI 兼容上游（如 Agnes 平台）。
- 受保护文件：
  - `backend/internal/service/openai_images.go`
  - `backend/internal/service/openai_images_test.go`
- 验收标准：
  - 单元测试 `TestOpenAIGatewayServiceParseOpenAIImagesRequest_AllowsAgnesImageModels` 通过；
  - 非图像模型（如 `gpt-5.4`）仍然被正确拦截并返回 400；
  - 真实请求 `/v1/images/generations` 使用 `agnes-image-2.5-flash` 时返回 200 并成功输出 1024x1024 及 4K 图片。

## 受保护文件处理方式

以下文件与官方 `v0.1.173` 存在重叠修改，不能使用“官方文件覆盖当前文件”的方式升级：

- 后端：`openai_images.go`、`user_repo.go`、`user_service.go`、`user_handler.go`、`routes/user.go`、`account_repo.go`、`account_service.go`、`admin_account.go`、`account_handler.go`、`routes/admin.go`
- 前端：`AccountsView.vue`、`CheckInView.vue`、`CommunityView.vue`、`AppSidebar.vue`、`router/index.ts`、`api/user.ts`、`api/admin/accounts.ts`、`api/chat.ts`
- 翻译：`frontend/src/i18n/locales/zh/dashboard.ts`、`frontend/src/i18n/locales/en/dashboard.ts` 以及账号管理相关翻译文件
- 数据库：`backend/migrations/192_daily_checkins.sql`、`backend/migrations/193_community_messages.sql`
- 规格与品牌：`.monkeycode/specs/`、`favicon.png`

官方新版本若修改了这些文件，应先保留官方新增内容，再将自定义区块合并进去。尤其要检查官方新增的 usage log 字段和其他迁移，不能因为保留自定义文件而丢失官方迁移 `194`、`195` 或之后的迁移。

## 标准升级流程

1. 记录当前镜像标签、容器健康状态、源码校验和和当前发行版本。
2. 备份 `deploy/.env`、源码压缩包、当前镜像、PostgreSQL dump、`deploy_sub2api_data` 卷及校验和。
3. 下载目标官方 tag，解压到独立 staging 目录，不能直接覆盖线上目录。
4. 对照本文档合并自定义实现；优先使用代码合并工具，禁止只复制编译后的 `dist`。
5. 检查 `backend/migrations` 的完整连续性，确认 `192`、`193` 和官方新增迁移都存在且只执行一次。
6. 使用目标版本号构建自定义镜像，并在镜像中确认嵌入的前端包含三个自定义页面和账号文件夹文案。
7. 停机窗口内替换应用容器，保留原容器镜像和数据卷；等待健康检查通过后再删除旧容器。
8. 完成验收清单，发现任一关键项失败就回滚，不在故障状态继续在线修改。

## 发布验收清单

- `GET /`、`/health`、`/admin/accounts`、`/check-in`、`/community` 返回 `200`。
- 登录后可以打开账号管理并看到文件夹列表，创建/重命名/移动账号后刷新仍然保留。
- 登录后每日签到页显示日历/日期和余额；重复领取不会再次增加 `0.20` 美元。
- 登录后交流区可以加载、发送文字、发送合规图片，并能在两分钟内撤回自己的消息。
- 未认证请求 `/api/v1/admin/accounts/folders`、`/api/v1/user/check-in`、`/api/v1/chat/messages` 应返回 `401`，不能变成公开接口。
- `daily_checkins`、`community_messages` 表和历史数据仍存在，`data/community` 文件可读取。
- 容器为 `healthy`，重启次数没有异常增长；日志没有 panic、fatal、migration error 或持续 `5xx`。
- 官方新功能的目标版本和迁移也能正常工作。

## 回滚原则

升级前生成带时间戳的快照目录，并记录镜像、源码、数据库 dump 的 SHA-256。回滚时恢复原源码/Compose 配置，切回原镜像，必要时再恢复同一快照的数据库；不要把新版本数据库 dump 和旧版本源码混用而不检查迁移兼容性。

当前已知可用快照：

- `/home/ubuntu/backups/sub2api-upgrade-20260808-132426`：`0.1.171` 自定义版本
- `/home/ubuntu/backups/sub2api-before-rollback-20260808-224205`：此前 `0.1.172` 尝试升级前快照
- `/home/ubuntu/backups/sub2api-post-upgrade-0.1.172-custom-20260808-155326`：当前已验证的 `0.1.172` 自定义版本
- `/home/ubuntu/backups/sub2api-feature-toggles-predeploy-20260808-185527`：本次功能开关部署前快照
- `/home/ubuntu/backups/sub2api-feature-toggles-postdeploy-20260808-191643`：本次功能开关部署后快照；镜像 `sub2api:0.1.172-custom-feature-toggles-20260809`
- `/home/ubuntu/backups/sub2api-pre-v0.1.173-20260809-124959`：升级到 `0.1.173` 前完整快照，包含源码、旧镜像、PostgreSQL 导出/数据卷、Redis 数据、应用数据和 `deploy/.env`。
- `/home/ubuntu/backups/sub2api-post-v0.1.173-20260809-130207`：`0.1.173` 自定义版本完整快照；镜像 `sub2api:0.1.173-custom-kpro-20260809`。
- `/home/ubuntu/backups/sub2api-pre-overdraft-20260816-221329`：合并 `v0.1.177` 与 Codex 透支模块前完整快照。
- `/home/ubuntu/staging/sub2api-overdraft-kpro-20260817-fix1`：当前源码 staging；生产镜像 `sub2api:0.1.177-custom-kpro-overdraft-20260817-fix1`。

每次新的升级都必须创建新的快照，并把快照路径写入本文件的升级记录。

## 升级记录

| 日期 | 目标版本 | 结果 | 说明 |
| --- | --- | --- | --- |
| 2026-08-08 | 0.1.171 | 已回滚并验证 | 保留账号文件夹、每日签到、交流区 |
| 2026-08-08 | 0.1.172 | 已合并部署并验证 | 镜像 `sub2api:0.1.172-custom-20260808-150204`；快照 `/home/ubuntu/backups/sub2api-upgrade-0.1.172-custom-20260808-150204` |
| 2026-08-09 | 0.1.172 | 已部署并验证 | 镜像 `sub2api:0.1.172-custom-feature-toggles-20260809`；staging `/home/ubuntu/staging/sub2api-feature-toggles-20260809-021310`；部署前快照 `/home/ubuntu/backups/sub2api-feature-toggles-predeploy-20260808-185527`；部署后快照 `/home/ubuntu/backups/sub2api-feature-toggles-postdeploy-20260808-191643` |
| 2026-08-09 | 0.1.173 | 已部署并验证 | 镜像 `sub2api:0.1.173-custom-kpro-20260809`；staging `/home/ubuntu/staging/sub2api-upgrade-v0.1.173-20260809-121017/merged-v0.1.173-custom-cherry`；部署前快照 `/home/ubuntu/backups/sub2api-pre-v0.1.173-20260809-124959`；部署后快照 `/home/ubuntu/backups/sub2api-post-v0.1.173-20260809-130207`；后端测试、前端 76 项测试、ARM64 构建和公网验收均通过。 |
| 2026-08-17 | 0.1.177 + overdraft | 已部署并验证 | 镜像 `sub2api:0.1.177-custom-kpro-overdraft-20260817-fix1`；staging `/home/ubuntu/staging/sub2api-overdraft-kpro-20260817-fix1`；部署前快照 `/home/ubuntu/backups/sub2api-pre-overdraft-20260816-221329`；生产 Compose 默认开启透支，可显式设置 `GATEWAY_CODEX_QUOTA_OVERDRAFT_ENABLED=false` 暂停；后端定向测试、前端类型检查、ARM64 构建和公网健康检查通过。 |
| 2026-08-22 | 0.1.179 + Glass Auth + mobile fix | 已部署并验证 | 镜像 `sub2api:0.1.179-custom-kpro-glass-auth-mobile-20260822`；staging `/home/ubuntu/staging/sub2api-glass-auth-20260822`；移动端 390x844 无上下白边；`vue-tsc --noEmit`、认证测试 11/11、健康检查通过；custom layer 见仓库根目录 `custom/`。 |
| 2026-09-24 | 0.2.7 + Agnes Image | 已部署并验证 | 镜像 `sub2api:0.2.7-agnes-image-20260923`；支持 `agnes-image-*` 生图识别与路由；后端定向测试、ARM64 构建和真实生图（1024x1024 / 4K）验收均通过。 |
