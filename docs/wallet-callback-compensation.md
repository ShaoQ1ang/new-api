# NewAPI Wallet 回调补偿设计

## 范围

NewAPI 对 Wallet 的 Reserve、Confirm、Cancel 默认同步调用。同步成功的请求不进入后台补偿；失败或结果不确定的请求持久化到 `wallet_usage_callbacks`，**默认不自动重试**。不引入 MQ，也不新增 Outbox 表。业务固定价覆盖计费见 [business-covered-billing.md](business-covered-billing.md)。

Callback 调用 Facade 的 `/api/v1/callback/wallet/api-platform/usage/{reserve,confirm,cancel}`，并使用
`api_platform_user_id` 传递 NewAPI 本地用户 ID。Facade 负责将该 ID 解析为 Wallet
所需的 IAM `user_id`。

三个回调的 `api_platform_user_id` 均使用正整数的十进制 **JSON 字符串**（例如
`"42"`），不是 JSON number；`estimate_amount` / `final_amount` 也保持十进制字符串。
内部用户 ID 仍来自已验证的真实用户 Token 归属及其持久化回调记录，不接受调用者自报
`user_id`，Facade 仍按严格 decimal ID 规则解析。ID 大于 JavaScript 安全整数范围时也
必须逐位保真；零/负身份不会创建回调记录或发送 Reserve。

2026-09-05 本地联调发现：旧回调把该 ID 编码为 JSON number，导致 Facade 在请求
解码阶段返回 400，NewAPI fail-closed 向业务返回 500，尚未调用模型。修复仅调整
Reserve / Confirm / Cancel 的 JSON 编码并增加真实 HTTP 原始 JSON 契约测试；
不放宽 Facade 校验、不迁移持久化 ID 类型、不恢复已拒绝的历史回调。已退款的业务
必须通过后续明确授权的新操作验收，不能借修复重放原订单。

本地验证记录：`go test ./service ./model ./relay/common -count=1` 通过；
实际 TLS HTTP 测试覆盖三种回调的字符串编码、零值拒绝及超过 JavaScript 安全
整数范围的 ID。2026-09-05 使用 `../k8s-deploy/local/billing/Build-NewApiBackend.ps1`
增量构建并部署 `lingentic/new-api:local-billing-20260905-2`，复用现有前端资源。
仅重建本地 `new-api` 容器，其环境和挂载不变，控制服务、PostgreSQL 和 Redis
容器未重建；`/api/status` 与容器健康检查通过，私有 TLS 回调入口可达。
原失败回调仍为 `REJECTED` 且未重试；上述检查未发起模型或收费业务。

## 状态

```text
RESERVE_PENDING -> RESERVED
RESERVE_PENDING -> CANCEL_PENDING -> CANCELLED
RESERVE_PENDING -> REJECTED
RESERVED -> CONFIRM_PENDING -> CONFIRMED
RESERVED -> CANCEL_PENDING -> CANCELLED
CONFIRM_PENDING/CANCEL_PENDING -> REJECTED（Wallet 明确拒绝）
```

不设置 `*_MANUAL_REQUIRED` 状态。`failure_code` 保存稳定错误分类，`last_error` 保存诊断信息；不存原始响应正文或认证凭证。失败回调保留待人工核对状态。Confirm 和 Cancel 的终止意图通过条件更新竞争，后来的相反意图不能覆盖已经选定的目标，Confirm 金额也不可改写。

## Reserve 失败补偿

明确的业务错误（例如余额不足、钱包不存在、幂等冲突或参数错误）直接进入 `REJECTED`，不调用 Cancel。

网络错误、超时、HTTP 408/429/5xx 表示 Reserve 结果不确定。在 fail-closed 模式下，当前 API 请求立即失败，记录进入 `CANCEL_PENDING`，但 `reserved_at_ms` 保持为 0。默认不立即重试、不自动 Cancel。经授权的运维手动调用内部 `ReconcileWalletUsageCallback(ctx, apiRequestID)`，使用相同 `api_request_id` 先核对 Reserve：

```text
Reserve 不确定失败
-> CANCEL_PENDING, reserved_at_ms=0
-> 重试同一个 Reserve
-> Wallet 幂等返回 Reserve 成功
-> 写 reserved_at_ms
-> 调用 Cancel
-> CANCELLED
```

因此 Cancel 只会在 Wallet 明确确认 Reserve 成功后发送，不需要创建空的 `CANCELLED` Wallet Operation。若补偿中的 Reserve 明确返回业务拒绝，则 NewAPI 进入 `REJECTED` 并终止补偿。

## Worker

- `WALLET_CALLBACK_AUTO_RETRY_ENABLED=false` 是默认值，当前业务必须保持关闭。关闭时不启动扫描、不消费待处理记录；服务重启也不会恢复自动处理。
- 以下为保留给其他用途的显式可选 Worker；固定价覆盖模式拒绝与自动重试开关同时启用。
- 每 1 秒扫描一次，也可以由同步失败事件立即唤醒。
- 每批最多 100 条，最大并发数为 8。
- Worker 只扫描 `RESERVE_PENDING`、`CONFIRM_PENDING`、`CANCEL_PENDING`。
- 使用 `next_retry_at_ms` 作为到期时间和短租约。通过带状态和到期条件的 UPDATE 抢占，只有 `RowsAffected = 1` 才能处理。
- 同步调用前写入 `WALLET_CALLBACK_TIMEOUT_MS + 2s` 的保护期限，防止 Worker 抢到仍在执行的同步请求。
- Worker 租约默认 30 秒，并且不会短于同步超时加安全余量。进程崩溃后任务会在租约到期后重新可见。

## 熔断

网络错误、超时、HTTP 408/429/5xx 打开全局 Wallet 熔断器。熔断期间每 5 秒只处理最早的一条任务作为探测，避免 Wallet 故障时持续放大流量。探测成功或收到明确 4xx 响应，说明 Wallet 已可达，立即关闭熔断并唤醒 Worker 排空积压。

以上自动探测仅在显式启用 Worker 时发生；默认关闭时仅保存熔断状态，不发送探测请求。

## 人工恢复边界

内部恢复函数只接受已有 `CONFIRM_PENDING` / `CANCEL_PENDING` 意图，使用数据库条件租约防止并发执行。同一调用只尝试一次既定账务操作，不重跑模型、不创建新订单。终态保持不变。没有挂载面向公网或普通成员的恢复接口；后续管理面接入必须先鉴权和审计，不能允许调用方改写用户、金额、请求 ID 或操作方向。

## 数据库迁移

现有部署执行：

```sql
ALTER TABLE wallet_usage_callbacks
    ADD COLUMN IF NOT EXISTS failure_code VARCHAR(64) NOT NULL DEFAULT '';
```

迁移脚本位于 `deploy/newapi-local/migrations/20260822_add_wallet_callback_failure_code.postgres.sql`。数据库迁移由部署过程手动执行，业务代码不负责执行该脚本。
