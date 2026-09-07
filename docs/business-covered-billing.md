# NewAPI 固定业务操作覆盖计费

## 范围与责任

Business Reporting 对报告分析、报告汇总、行业采集、行业资讯汇总和知识入库五类任务调用 Wallet 固定价 `Charge`。NewAPI 不再扣第二份用户/token 额度，只将模型实际用量归集到已付款的父订单。File、Knowledge 等受信任子服务沿用相同订单上下文；浏览器不能构造该上下文。知识入库为 100105，金额尚未确定，不预设价格或执行实际收费。

Wallet 是订单归属、付款/退款、`usage_closed` 的唯一校验方。新 reserve 必须引用有效、归属当前用户、已付款且未关闭的订单。任务完成后 BR 必须关闭用量入口，失败按既有订单退款。NewAPI 不以本地金额或请求头声明替代 Wallet 授权。

## 双重认证与配置

用户身份始终来自原有 `Authorization: Bearer <用户 NewAPI Key>`。业务凭证另用：

| 项 | 约定 |
| --- | --- |
| `X-Business-Order` | 长度 1–128，仅 ASCII 字母、数字、`-_.:`；禁止重复值 |
| `X-Business-Billing-Authorization` | `Bearer <BUSINESS_BILLING_CALLER_TOKEN>`；禁止重复值 |
| `BUSINESS_BILLING_CALLER_TOKEN` | 至少 32 字节高熵服务密钥，由 Secret 管理，不进用户配置或日志 |
| `WALLET_CALLBACK_ENABLED` | 必须为 `true` |
| `WALLET_CALLBACK_FAIL_CLOSED` | 必须为 `true` |
| `WALLET_CALLBACK_BASE_URL` | 已验证证书的 HTTPS origin；禁止明文 HTTP、URL 凭证、路径、query、fragment |
| `WALLET_CALLBACK_TOKEN` | 至少 32 字节，独立于调用方密钥；以 `Authorization: Bearer ...` 发给 Facade |
| `WALLET_CALLBACK_AUTO_RETRY_ENABLED` | 默认 `false`；覆盖计费要求保持关闭 |

部署必须提供私有 TLS 入口、正确 CA 信任及网络访问控制，公网不能访问 Wallet 回调。不得用 `InsecureSkipVerify`、放宽证书检查或回退 HTTP 解决本地环境配置问题。本文不写入任何实际 Secret，也没有自动启用部署环境。

使用 SHA-256 固定长度摘要加 constant-time 比较校验专用工作负载凭证；这不替代用户 Token 校验。订单在验证后进入请求上下文，原始业务订单/凭证头立刻移除。上游通配、正则与显式 header override 也不允许传递这些头。模型供应商只收到其本身的认证信息。

当前覆盖接口仅支持 POST `/v1/chat/completions`、`/v1/responses`、`/v1/messages`、`/v1/embeddings`。Responses 的实际上游 body（含透传和渠道参数覆盖）禁止 `background=true`。未完成完整同步账务链路的其他接口（含异步/实时）拒绝覆盖计费，不悄悄回退普通计费。缺失或错误的服务凭证、回调配置、认证用户、订单归属均 fail closed。

Embeddings 复用 Controller.Relay 的先 reserve 后上游调用及失败 cancel，成功走 EmbeddingHelper 的 PostTextConsumeQuota/SettleBilling/confirm；即使免费模型也必须先验证父单。EmbeddingHelper 在发出请求前要求 BUSINESS_INCLUDED 会话已建立，结算层禁止任何覆盖调用降级到 legacy quota 路径。知识切块批次各自形成一次同步模型用量，所有批次归集同一个根入库订单，不增加固定操作扣费。

`TestCoveredEmbeddingWalletLifecycle` 使用隔离 SQLite 和 TLS Wallet 契约替身验证成功 confirm、失败 cancel、幂等终态及本地 user/token 余额不变；模型边界另验证缺 reserve 时禁止联系 Provider。该测试不是实际模型、钱包或企业数据联调，本次没有部署或启用知识入库价格。

## 用量生命周期

1. 同步 reserve：通过 Facade 将真实 Token 的 NewAPI 用户 ID 映射为 IAM 用户；Wallet 验证父订单。
2. 上游调用只执行一次，不自动切渠道重试。免费模型/零预估也必须 reserve；为满足 Wallet 正数契约，零预估使用 1 micro-CNY 作为内部记录占位，不扣余额。
3. 成功后 confirm 记录真实内部用量；失败后 cancel 记录使用终止。两者与 reserve 均用同一个服务端请求 ID。
4. CoveredFunding 不修改本地 user/token quota；后续额度补充、通知及违规附加计费不会产生额外扣款。负数预估与结算拒绝。

回调只接受 HTTP 200、合法 `{data: ...}` 和精确对应金额/请求 ID。reserve 必须返回 `funding_source=3`（BUSINESS_INCLUDED）；confirm 的 `balance_delta_amount` 和 cancel 的 `returned_amount` 必须为字符串 `0`。若返回普通钱包资金来源或错误金额，即使 HTTP 200 也拒绝。金额遵循 Facade 十进制字符串契约。响应限制 16 KiB，禁止所有重定向，不把原始响应体写日志。

## 并发与故障

- `api_request_id` 唯一约束防止重复本地账务记录；Wallet 的同 ID 幂等保障不重复记账。
- BillingSession 与 callback session 各自串行化同一请求生命周期；DB 条件更新让 Confirm/Cancel 首个意图胜出，禁止相互覆盖或改变已选金额。
- 网络异常、超时、无效响应不会当作授权成功。reserve 不确定立即拒绝模型调用，保留 `cancel_pending`；confirm/cancel 异常持久化错误供人工核对，不自动重跑模型或重新扣业务费。
- 进程退出后不会自动恢复账务调用。受授权的内部管理组件可调用 `ReconcileWalletUsageCallback`，用原请求 ID 和原意图进行一次处理；公共 HTTP 管理入口尚未新增。
- 已存在回调记录、数据库与余额保留；启用前需同时上线 Wallet 父订单关闭校验、Facade 回调认证和 BR 固定价编排，不能单独打开 NewAPI 开关。

## 验证

`go test ./service ./middleware ./relay/channel ./model ./controller ./router ./relay -count=1`（可按本次账务测试名缩小执行范围）。回归覆盖伪造/重复头、非法配置、真实 Token 归属、零预估、无本地二次扣款、严格响应、禁止重定向、默认不重试、并发结算意图和上游凭证剥离。未在本次代码变更中调用真实钱包、修改余额或部署。
