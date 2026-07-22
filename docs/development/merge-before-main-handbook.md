# Merge 手册：`feat/merge-before-main` → 对齐 `main`

> 目标：把当前产品线与上游 `main` 收敛，**共享代码一律以 `main` 为准**；仅保留本分支相对 `main` 的**新增能力**（业务 + 本地 `deploy/`），再把这些能力 re-apply 到 `main` 底座上。  
> 本文是操作手册，不是设计说明书。具体子能力细节见 `docs/development/2026-*.md`。

---

## 0. 快照（写入手册时）

| 项 | 值 |
|---|---|
| 当前分支 | `feat/merge-before-main` |
| HEAD | `8b573da2d`（2026-07-17）`fix: route local compose traffic through seedance gateway` |
| `main` | `5a6c53d49`（2026-07-18）`feat: standardize OpenAI Models label usage across components` |
| merge-base | `0977965d9`（2026-07-03）`fix: handle ollama non-stream tool calls (#5865)` |
| 分叉规模 | **main 独有 ~107 commits** / **本分支独有 ~318 commits** |
| 路径 diff（三路） | ~1655 files，+414k / -98k 行量级 |

> 执行前请重新 `git fetch` 并更新本节 SHA；数字会随远端变动。

---

## 1. 总原则（必须遵守）

### 1.1 一句话

```text
结果树 = main 的全量代码
        + 本分支「新增能力」清单（见 §2）
        − 禁止用本分支旧实现覆盖 main 的共享模块
```

### 1.2 冲突裁决

| 场景 | 裁决 |
|---|---|
| 同文件两边都改、且**不属于** §2 新增能力 | **`main` 赢**（`--ours`/`--theirs` 以实际 merge 方向为准，语义上取 main） |
| 本分支新增文件/目录（§2 有登记） | **保留并迁入** main 底座 |
| main 新增、本分支没有 | **完整吃进**（如 authz、system-info/task、unset price、cache billing 等） |
| 工具链 / 依赖 / 格式化 / UI 基建 | **一律 main**（`go.mod`、`web/default` 工具链、data-table、theme 等） |
| 拿不准是否「新增能力」 | **先按 main**，再开 issue/后续 PR 补 port，禁止凭感觉整文件盖回旧实现 |

### 1.3 明确禁止

1. 在旧 `web/default` 上硬 `merge main` 解 900+ 文件冲突当常规路径。  
2. 用本分支旧 `components/ui`、`data-table`、theme 覆盖 main。  
3. 整文件覆盖 i18n locales（只合 key）。  
4. 为省事丢掉 main 的安全/计费/权限修复（用户硬删清认证、配额、cache 计费、tool call 去重等）。  
5. 把 classic 自动续费/Playground 大改与 default 基建重构塞进同一个「一口气合完」PR 且不拆验证。

### 1.4 推荐拓扑（分支）

```text
main  (upstream tip)
  └─ merge/align-main                 ← 工作分支：先吃满 main
        ├─ port/backend-new-features  ← 可选拆分：后端新增能力
        ├─ port/default-ui            ← 可选拆分：default 独有页
        └─ port/classic-ui            ← 可选拆分：classic 业务 UI
```

也可单分支分 commit 阶段，但**提交必须按阶段可回滚**。

---

## 2. 本分支「新增能力」白名单（唯一可压过 main 的部分）

> **除此清单外，默认全部以 main 为准。**  
> 「deploy 新增」在此手册中 = **本产品线已落地/在落地的新增能力**（含 `deploy/newapi-local` 与业务代码），不是「只有 deploy 目录」。

### 2.1 能力总表

| ID | 能力 | 后端 | default UI | classic UI | deploy/docs | 优先级 |
|---|---|---|---|---|---|---|
| F01 | Skill Hub | ✅ | ✅ | ✅（若有） | docs / scripts | P0 |
| F02 | Client Releases + OSS 直传 | ✅ | ✅ | 视情况 | docs | P0 |
| F03 | 短信登录（Aliyun SMS） | ✅ | ✅ | 视情况 | docs | P0 |
| F04 | Chat Model 列表 / think effort / speed | ✅ | ✅ | ✅ | docs | P0 |
| F05 | 支付宝：充值 + 订阅支付 | ✅ | ✅ 增量 | ✅ | docs | P0 |
| F06 | Stripe / 支付宝 **自动续费** | ✅ | 仅购买侧小修 | ✅ 主战场 | docs | P0 |
| F07 | `plan_kind` 等订阅计划扩展 | ✅ | 视情况 | ✅ | docs | P0 |
| F08 | Playground 增强（历史/规则/文件/图视频） | ✅ 部分 | ❌ 跟 main | ✅ 主战场 | docs | P1 |
| F09 | 视频：秒级计费、阿里视频族、豆包/条件价 | ✅ | 少 | ✅ | docs / mock | P1 |
| F10 | Seedance 本地 / 网关 / compat | ✅ 路由相关 | — | — | ✅ **deploy 核心** | P1 |
| F11 | 支付快照 / 订单状态等辅助 | ✅ | 少 | 少 | docs | P2 |
| F12 | 本地部署编排 `deploy/newapi-local` | — | — | — | ✅ **整包保留** | P1 |
| F13 | 英文优先 i18n / 杂项产品文案 | 部分 | 增量 key | 增量 | docs | P2 |

### 2.2 关键路径索引（迁入时按此找代码）

#### F01 Skill Hub

- Backend: `controller/skill_hub.go`, `model/skill_hub.go`, `service/skill_hub_oss.go`（及测试）
- Router: `router/api-router.go` 中 `/skill-hub`、`/admin/skill-hub`
- default: `web/default/src/features/skill-hub/**`, `routes/_authenticated/skill-hub/**`
- classic: 检索 `SkillHub` / `skill-hub`（以当时树为准）
- docs: `docs/skill-hub.md`, `docs/development/2026-07-13-skill-hub-*.md`, `docs/development/2026-07-15-skill-hub-tag-query-postgresql.md`
- scripts: `scripts/skill-hub-batch-upload/**`（若存在）

#### F02 Client Releases

- Backend: `controller/client_release.go`, `model/client_release.go`, `service/client_release_oss.go`, `service/oss_endpoint.go`
- Router: `/client-releases`, `/admin/client-releases`
- default: `web/default/src/features/client-releases/**`, `routes/_authenticated/client-releases/**`
- docs: `docs/client-releases.md`

#### F03 SMS

- Backend: `service/sms.go`, `common/sms.go`（及测试）, user phone 字段相关 model
- Router: `POST /sms/verification`, `POST /user/phone/login`（以 `api-router.go` 为准）
- default: `features/auth/**`（SMS 登录块）, `system-settings/integrations/aliyun-sms-settings-section.tsx`, users 表手机号
- docs: `docs/development/2026-06-24-aliyun-sms-login.md`

#### F04 Chat Model

- Backend: `controller/chat_model.go`, `model/chat_model.go`, `dto/chat_model.go`
- Router: `/chat-models`, `GET .../chat-models`（self）
- default: `features/models/**`（`chat-models-table.tsx` 等）
- classic: think effort / speed 相关提交（如 `#11`）
- docs: `docs/development/2026-07-13-chat-model-capabilities.md`

#### F05 / F06 / F07 支付与自动续费

- Backend（示意，以树为准）:
  - `controller/topup_alipay.go`
  - `controller/subscription_payment_alipay.go`
  - `controller/subscription_payment_alipay_auto_renew.go`
  - `controller/subscription_payment_stripe.go`（recurring 增量）
  - `service/alipay*.go`, `service/alipay_auto_renew_*.go`, `service/alipay_pending_task.go`
  - `model/alipay_pending_task.go` 及 auto-renew / plan_kind 相关 model
  - `setting/payment_alipay.go`
- Router: `/alipay/*`, subscription `alipay`/`stripe` auto-renew checkout, notify
- default: `subscriptions/api|types|purchase-dialog`（`ApiSuccess`/`checkout_url`/`trade_no`）, `wallet/*` 支付宝增量, billing 设置中的 Alipay 段
- classic: 自动续费计划配置、Stripe/Alipay 管理与测试设置（主 UI）
- docs:
  - `docs/development/2026-07-10-stripe-auto-renew-subscription-notes.md`
  - `docs/development/2026-07-13-alipay-auto-renew-design.md`
  - `docs/development/2026-07-14-subscription-plan-kind.md`

#### F08 Playground（以 classic 为准）

- Backend: `controller/playground_history.go`, `model/playground_conversation.go` 等
- classic: Playground composer / sidebar / model rules / 文件限制 / 图视频模式（大量 commits）
- default playground: **不要用本分支旧实现盖 main**；能力保留在 classic，除非单独立项搬迁
- docs: `docs/development/2026-07-09-classic-playground-pdf-attachments.md`, `2026-07-10-classic-playground-model-rules.md`

#### F09 视频计费与模型族

- Backend:  
  - `setting/ratio_setting/video_seconds_price.go`  
  - `setting/ratio_setting/task_condition_price.go`（若仍启用）  
  - `relay/channel/task/ali/*`, `relay/.../video_billing*.go`, doubao 相关  
  - `cmd/ali-video-mock/**`
- classic: 秒级定价编辑、任务账单展示
- docs: `docs/development/2026-07-07-ali-video-model-family-support.md` 及视频相关 design/spec

#### F10 / F12 Seedance 与本地部署

- `deploy/newapi-local/**`（**整包保留**，main 无对等物则直接带上）
- seedance-compat、gateway nginx、compose、metadata、smoke json
- 与路由相关的 compose/网关修正（如 local traffic → seedance gateway）
- 根目录若有 `docker-compose.local.yml` / `docker-compose.test.yml` 等本地编排：**保留并检查与 main 的 `docker-compose.yml` 不互相覆盖错文件**

#### F11 辅助支付

- `controller/payment_snapshot.go`, `controller/order_status.go` 等  
- 无对应 main 实现则迁入；与 main 支付流水线冲突时 **接口语义跟 main，扩展字段跟 F05/F06**

### 2.3 非白名单 = 必须跟 main 的典型区

| 区域 | 说明 |
|---|---|
| `web/default` 基建 | `components/ui`, `data-table`, `styles`, package/lock, oxlint/oxfmt, rsbuild |
| `web/default` 共享业务页 | dashboard, pricing（除你们未登记的增量）, channels 列表基建, profile, keys, redemption, rankings, home, setup… |
| main 独有后端 | `service/authz/**`, `model/authz_*`, `casbin`, `system_info`/`system_task`, channel authz 路由等 |
| 上游计费/协议修复 | cache_write、Responses→Chat tool call 去重、配额溢出、硬删清认证等 |
| 工具与元数据 | `.github`（除你们专有 workflow 需评估）、通用 `makefile` 目标以 main 为底再补本地目标 |

---

## 3. 推荐执行流程（分阶段）

### Phase A — 准备（不改业务语义）

1. `git fetch origin`  
2. 确认 `main` / 当前分支 tip，更新本文 §0  
3. 从当前 tip **打 tag 或备份分支**：  
   `git branch backup/merge-before-main-8b573da2d`  
4. 导出白名单补丁包（可选但强烈建议）：

```bash
# 示例：按路径导出，便于 main 底座上 re-apply
git diff main...HEAD -- <whitelist-paths> > /tmp/ours-features.patch
```

更稳妥：对 F01–F12 每个能力建「文件清单」文本（`git diff --name-only main...HEAD -- <paths>`），提交到 `docs/development/merge-filelists/`（可选后续步骤）。

5. 工作分支：

```bash
git checkout -b merge/align-main origin/main   # 或 main
# 若必须从当前分支出发：先 merge main，但 default 仍建议 Phase C 整树替换
```

### Phase B — 后端：main 底座 + 白名单能力

**顺序：**

1. 保证工作树基于 **最新 main**。  
2. **迁入白名单后端路径**（§2.2），优先顺序：  
   `F03 SMS → F05 支付宝基础 → F01 SkillHub → F02 ClientRelease → F04 ChatModel → F06/F07 自动续费 → F09 视频 → F08 playground API → F11`  
3. **Router**：以 **main 的 `router/` 为底**，把白名单路由块手工合并进 `api-router.go` / `relay-router.go`。  
   - main 若有 `authz-router` / `channel-router`：**保留 main**，不要删掉换回旧单文件结构。  
4. **`go.mod` / `go.sum`**：以 main 为底，`go get`/`go mod tidy` 只为白名单依赖补齐。  
5. **`main.go` / 启动任务**：以 main 为底，仅注册自动续费 charge task、SMS 等白名单初始化。  
6. **Model 迁移**：确认 SQLite / MySQL / PostgreSQL 三库可启动；禁止引入仅单库语法（见 `AGENTS.md` Rule 2）。  
7. 编译与测试：

```bash
go test ./...
# 至少覆盖：skill_hub / client_release / sms / alipay / subscription auto-renew / video_seconds
```

**冲突时：** 非白名单符号、中间件、auth、日志框架 → **main**；白名单类型字段可扩展 main 结构体，避免复制整份旧 `user.go`。

### Phase C — `web/default`：整树 main + 白名单 port

**这是降低冲突的关键步骤。**

```bash
# 在基于 main 的工作分支上：
git checkout main -- web/default
# 安装并确认「纯 main default」可 build
cd web/default && bun install && bun run build
```

然后 **仅 port 白名单 UI**：

| 顺序 | 内容 |
|---|---|
| 1 | 拷贝目录：`features/skill-hub`, `features/client-releases` + 对应 `routes` |
| 2 | 接线：`use-sidebar-data` / `use-sidebar-config` / maintenance sidebar modules（**在 main 文件上改**） |
| 3 | SMS：在 main 的 auth / system-settings 上打增量 |
| 4 | Chat models：在 main 的 models 上打增量 |
| 5 | 订阅/钱包：只合支付响应与支付宝方式（小 diff） |
| 6 | i18n：只合新增英文 key，再 `bun run i18n:sync`（若适用） |
| 7 | `routeTree.gen.ts`：按 main 工具链重新生成，禁止长期手改 |

**不要 port：** 旧 data-table 路径、旧 playground 零散文件、全量 locales、全量 system-settings。

### Phase D — `web/classic`：main 底座 + 白名单业务

1. classic **共享壳**（布局、通用组件若与 main 冲突）→ **main**。  
2. 再 port / 保留：  
   - Playground 全家桶（F08）  
   - 自动续费管理与计划配置（F06/F07）  
   - 视频秒价与任务展示（F09）  
   - SkillHub / Chat model 管理入口（与 default 双端时 API 对齐）  
3. 独立冒烟：Playground 发图/视频、订阅续费配置保存、视频价编辑。

> classic 变更量大时，允许 **单独 PR**：`port/classic-ui`，不阻塞后端 + default 先合。

### Phase E — deploy / docs / agents

| 路径 | 策略 |
|---|---|
| `deploy/newapi-local/**` | **整包保留**（F12） |
| `cmd/ali-video-mock/**` | 保留（F09） |
| `docker-compose.local.yml` / `test.yml` | 保留；与 main 的 compose **并存不覆盖** |
| `docs/development/2026-*.md` 及产品 docs | 保留；与 main 文档冲突时 **并存** 或改名，不删 main 文档 |
| `.agents/skills/**` | 本分支新增 skill **保留**；与 main 同路径则 **main 为底再补** |
| 根 `README*` | **main 为底**，仅追加本产品部署入口链接（指向 `deploy/newapi-local`） |

### Phase F — 回归清单

#### 后端

- [ ] `go test ./...` 通过（或已知跳过列表书面记录）  
- [ ] 三库至少一种本地库迁移成功（优先 Postgres 与 SQLite）  
- [ ] Skill Hub CRUD / 批量删除导出 / 直传  
- [ ] Client Release 上传与发布  
- [ ] SMS 验证码 + 登录（开关关时不影响原登录）  
- [ ] Chat models API + think/speed 字段  
- [ ] 支付宝充值 notify；订阅支付宝支付  
- [ ] Stripe / 支付宝自动续费 checkout + webhook/扣款任务（staging）  
- [ ] 视频任务计费（秒级）与阿里视频模型路由  
- [ ] Seedance 本地链路（若环境具备）  
- [ ] main 回归：渠道上游模型拉取、用户硬删、cache 计费、tool call 流式不重复  

#### default UI

- [ ] `bun run build`  
- [ ] 侧栏：Skill Hub、Client Releases 可见且权限正确  
- [ ] SMS 登录 UI  
- [ ] Chat models 管理  
- [ ] 订阅购买（checkout_url / trade_no）  
- [ ] Channels / Pricing / Dashboard **表现与 main 一致**（无旧皮肤回退）  

#### classic UI

- [ ] Playground 主路径  
- [ ] 自动续费计划配置  
- [ ] 视频定价编辑  

#### deploy

- [ ] `deploy/newapi-local` compose 仍可起  
- [ ] 网关 / seedance-compat 文档与脚本路径有效  

---

## 4. 冲突速查表

| 冲突路径模式 | 取谁 | 备注 |
|---|---|---|
| `web/default/src/components/ui/**` | **main** | 永不取旧分支 |
| `web/default/src/components/data-table/**` | **main** | 结构已变 |
| `web/default/src/styles/**` | **main** | |
| `web/default/package.json` / lock | **main** | 再补白名单依赖（通常无） |
| `web/default/src/features/skill-hub/**` | **ours 迁入** | main 无则直接加 |
| `web/default/src/features/client-releases/**` | **ours 迁入** | |
| `web/default/src/features/channels/**` | **main** | 除非白名单 API 必须接线 |
| `web/default/src/features/subscriptions/**` | **main + 小 diff** | 只合 F05/F06 字段 |
| `web/default/src/features/playground/**` | **main** | 能力在 classic |
| `web/classic/**` Playground / 续费 | **ours 逻辑** | 壳冲突时先 main 再 port |
| `router/api-router.go` | **main 为底 + 白名单路由块** | 禁止整文件 ours |
| `router/authz-router.go` 等 main 新路由 | **main** | 本分支若删除过须恢复 |
| `service/authz/**` | **main** | |
| `controller/user.go` / `model/user.go` | **main 为底** | 只合 phone/SMS 字段 |
| `controller/subscription*.go` | **main 为底 + F05/F06** | 逐函数合，禁整文件覆盖 |
| `go.mod` | **main 为底 + tidy** | |
| `deploy/newapi-local/**` | **ours** | |
| `common/**` 非 SMS | **main** | SMS 相关文件迁入 |
| i18n `en.json`/`zh.json` | **main + key 合并** | |

---

## 5. 分 PR / 分 commit 建议

| 顺序 | 单元 | 内容 | 可单独上线 |
|---|---|---|---|
| 1 | merge main 空窗 | 仅对齐 main，CI 绿 | 是（若暂不启白名单） |
| 2 | backend P0 | F01–F05 + 路由 | 是 |
| 3 | backend P0 续费 | F06–F07 | 视支付配置 |
| 4 | default port | Phase C | 是 |
| 5 | classic port | Phase D | 是 |
| 6 | video + seedance | F09–F10 + deploy | 视环境 |
| 7 | docs/agents 收尾 | 文档与手册更新 | 是 |

每个单元：`build + 相关 test + §3 Phase F 子集`。

---

## 6. 回滚策略

| 级别 | 动作 |
|---|---|
| 单能力回滚 | `git revert` 对应 port commit；DB 迁移需单独 down/兼容 |
| default 整树失败 | 再执行一次 `git checkout main -- web/default`，重新 port |
| 全盘失败 | 回到 `backup/merge-before-main-*` tag/分支 |
| 生产 | 功能开关（SMS、自动续费 task、SkillHub 侧栏模块）优先于硬回滚 |

---

## 7. 决策记录（默认值）

| 议题 | 默认决策 |
|---|---|
| default 是否跟 classic 功能对等 | **否**；续费管理与 Playground 以 classic 为主，default 只 port 已列白名单 |
| 本分支旧 default 皮肤/表格 | **放弃** |
| main 的 authz / system-info | **完整保留** |
| deploy/newapi-local | **完整保留** |
| 冲突默认 | **main** |
| 白名单与 main 行为冲突（如支付回调） | **先保证 main 不崩，再以适配层兼容双方字段**；禁止静默丢掉 main 校验 |

---

## 8. 操作口令（给执行者）

```text
1. 备份当前 tip
2. 从 main 开 merge/align-main
3. 后端：main + §2 白名单（router 手合）
4. default：checkout main -- web/default → 只 port F01–F05 UI
5. classic：main 壳 + F06–F09 UI（可另 PR）
6. deploy/docs：整包保留新增
7. 按 §3 Phase F 打勾
8. 更新本文 §0 SHA 与已知遗留
```

---

## 9. 相关文档

| 文档 | 用途 |
|---|---|
| `docs/development/ai-development-checklist.md` | 开发前后检查 |
| `docs/development/2026-06-24-aliyun-sms-login.md` | SMS |
| `docs/development/2026-07-10-stripe-auto-renew-subscription-notes.md` | Stripe 续费 |
| `docs/development/2026-07-13-alipay-auto-renew-design.md` | 支付宝续费 |
| `docs/development/2026-07-13-chat-model-capabilities.md` | Chat model |
| `docs/development/2026-07-13-skill-hub-*.md` | Skill Hub |
| `docs/development/2026-07-14-subscription-plan-kind.md` | plan_kind |
| `docs/development/2026-07-07-ali-video-model-family-support.md` | 阿里视频 |
| `docs/client-releases.md` | Client release |
| `deploy/newapi-local/README.md` | 本地部署 |
| `AGENTS.md` | JSON / 三库 / 双前端等硬规则 |

---

## 10. 遗留与后续（merge 后填）

- [ ] 实际 merge 使用的 main SHA：________  
- [ ] 未迁入的白名单项（若有）及原因：________  
- [ ] 已知测试跳过：________  
- [ ] 生产开关默认值：________  

---

**维护：** 每次对齐 `main` 或增删白名单能力时，更新 §0、§2、§10，并保留决策日期。

---

## Execution log (automated)

- Branch: \merge/align-main- Strategy: main base + product whitelist re-applied
- Backup branches: \ackup/merge-before-main-8b573da2d\, \ackup/merge-align-wip-ca5500b07- \go build .\ status: OK (placeholder \web/*/dist/index.html\ required for embed)
- Follow-ups: re-port video-seconds billing into elay/relay_task.go\; default UI sidebar/i18n wiring smoke test; classic already taken from feature wholesale

