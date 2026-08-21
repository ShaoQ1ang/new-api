# NewAPI Wallet 回调补偿设计

## 范围

NewAPI 对 Wallet 的 Reserve、Confirm、Cancel 默认同步调用。同步成功的请求不进入后台补偿；只有失败或结果不确定的请求由 `wallet_usage_callbacks` 表驱动重试。不引入 MQ，也不新增 Outbox 表。

## 状态

```text
RESERVE_PENDING -> RESERVED
RESERVE_PENDING -> CANCEL_PENDING -> CANCELLED
RESERVE_PENDING -> REJECTED
RESERVED -> CONFIRM_PENDING -> CONFIRMED
RESERVED -> CANCEL_PENDING -> CANCELLED
CONFIRM_PENDING/CANCEL_PENDING -> REJECTED（Wallet 明确拒绝）
```

不设置 `*_MANUAL_REQUIRED` 状态。`failure_code` 保存稳定错误分类，`last_error` 保存详细响应。

## Reserve 失败补偿

明确的业务错误（例如余额不足、钱包不存在、幂等冲突或参数错误）直接进入 `REJECTED`，不调用 Cancel。

网络错误、超时、HTTP 408/429/5xx 表示 Reserve 结果不确定。在 fail-closed 模式下，当前 API 请求立即失败，记录进入 `CANCEL_PENDING`，但 `reserved_at_ms` 保持为 0。Worker 使用相同 `api_request_id` 先重试 Reserve：

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

- 每 1 秒扫描一次，也可以由同步失败事件立即唤醒。
- 每批最多 100 条，最大并发数为 8。
- Worker 只扫描 `RESERVE_PENDING`、`CONFIRM_PENDING`、`CANCEL_PENDING`。
- 使用 `next_retry_at_ms` 作为到期时间和短租约。通过带状态和到期条件的 UPDATE 抢占，只有 `RowsAffected = 1` 才能处理。
- 同步调用前写入 `WALLET_CALLBACK_TIMEOUT_MS + 2s` 的保护期限，防止 Worker 抢到仍在执行的同步请求。
- Worker 租约默认 30 秒，并且不会短于同步超时加安全余量。进程崩溃后任务会在租约到期后重新可见。

## 熔断

网络错误、超时、HTTP 408/429/5xx 打开全局 Wallet 熔断器。熔断期间每 5 秒只处理最早的一条任务作为探测，避免 Wallet 故障时持续放大流量。探测成功或收到明确 4xx 响应，说明 Wallet 已可达，立即关闭熔断并唤醒 Worker 排空积压。

## 数据库迁移

现有部署执行：

```sql
ALTER TABLE wallet_usage_callbacks
    ADD COLUMN IF NOT EXISTS failure_code VARCHAR(64) NOT NULL DEFAULT '';
```

迁移脚本位于 `deploy/newapi-local/migrations/20260822_add_wallet_callback_failure_code.postgres.sql`。数据库迁移由部署过程手动执行，业务代码不负责执行该脚本。
