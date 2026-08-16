# AIGC 统一模型与生成 API 设计

## 决策

NewAPI 保留统一模型目录和统一生成接口，不按文本、图片、视频、音乐拆分外部路径：

```text
GET  /v1/aigc/models
POST /v1/aigc/generations
GET  /v1/aigc/generations/:id
```

管理员模型 Profile 同样使用统一资源：

```text
/api/aigc/models
```

媒体类型由模型 Profile 和生成请求中的 `type` 明确表达。统一接口成立的前提是四类模型全部迁移到 NewAPI 的 Profile、Resolver、执行器、渠道和计费链路；AIGC 后端不得保留本地模型配置或供应商直连分支。

## 系统边界

### AIGC 后端

AIGC 后端负责面向产品的会话、Turn、素材上传、结果入库和资产管理。前端提交生成的入口保持：

```text
POST /api/conversations/:conversation_id/turns
```

AIGC 后端将 Turn 转换成标准 GenerationRequest，使用当前用户的 NewAPI Token 调用 NewAPI。它不负责选择供应商渠道、解释模型能力或计算价格。

### NewAPI 后端

NewAPI 是模型能力和执行事实源，负责：

- 管理文本、图片、视频、音乐 Profile。
- 根据用户分组、Profile 状态和渠道可用性解析最终上游模型。
- 校验模式、输入素材、输出规格和显式零值。
- 调用现有同步 Relay 或异步 Task Workflow。
- 执行预扣、结算、退款和计费审计。
- 按幂等键保存生成记录，并提供统一状态查询。

## 模型目录

### Relay 模型目录

```text
GET /v1/aigc/models
GET /v1/aigc/models?type=text
GET /v1/aigc/models?type=image
GET /v1/aigc/models?type=video
GET /v1/aigc/models?type=music
```

该接口只返回当前 Token 所属用户分组可用、已发布且存在可用渠道的模型。无 `type` 参数时返回全部类型。

### 管理接口

```text
GET    /api/aigc/models
POST   /api/aigc/models
GET    /api/aigc/models/:id
PUT    /api/aigc/models/:id
DELETE /api/aigc/models/:id

POST   /api/aigc/models/:id/validate
POST   /api/aigc/models/:id/publish
POST   /api/aigc/models/:id/disable
POST   /api/aigc/models/import

GET    /api/aigc/upstream-models
GET    /api/aigc/upstream-models/*model
```

列表接口允许使用 `type`、`status`、`p`、`page_size` 筛选。创建和更新 Profile 时保留 `model_type`，服务端只接受 `text`、`image`、`video`、`music`。

## 统一生成协议

### 提交

```text
POST /v1/aigc/generations
Authorization: Bearer <user-token>
Content-Type: application/json
```

通用请求结构：

```json
{
  "idempotency_key": "turn-123",
  "model": "wan-2.7",
  "type": "video",
  "prompt": "camera moves forward",
  "mode": "first_frame",
  "inputs": {
    "images": [
      {"role": "first_frame", "url": "https://example.com/first.png"}
    ]
  },
  "output": {
    "resolution": "720p",
    "aspect_ratio": "16:9",
    "duration": 5,
    "generate_audio": false
  },
  "options": {
    "negative_prompt": "blur",
    "enhance": false,
    "private": false,
    "ai_mark": false
  },
  "parameters": {}
}
```

`type` 必须与 Profile 的 `model_type` 一致。`idempotency_key` 必填；同一用户用相同幂等键提交不同请求时返回冲突。

### 查询

```text
GET /v1/aigc/generations/:generation_id
Authorization: Bearer <user-token>
```

统一响应结构：

```json
{
  "id": "aigc_gen_xxx",
  "idempotency_key": "turn-123",
  "status": "completed",
  "progress": 100,
  "model": "wan-2.7",
  "type": "video",
  "created_at": 1786890000,
  "outputs": [
    {
      "id": "task_xxx",
      "type": "video",
      "url": "https://newapi.example/v1/videos/task_xxx/content"
    }
  ],
  "usage": {"quota": 250000}
}
```

状态为 `queued`、`processing`、`completed` 或 `failed`。失败时返回结构化 `error.code`、`error.message` 和 `error.retryable`。

## 各类型执行路径

| 模型类型 | NewAPI 执行器 | 最终 Relay |
|---|---|---|
| `text` | Sync Executor | 文本/Chat Relay |
| `image` | Sync Executor | `/v1/images/generations` 或 `/v1/images/edits` |
| `video` | Task Executor | `/v1/videos` Task Workflow |
| `music` | Task Executor | 音乐 Task Workflow |

同步类型可以在提交响应中直接完成；异步类型先返回 queued/processing，由 AIGC 后端轮询统一查询接口。调用方不需要了解最终渠道使用同步还是异步协议。

## 迁移状态

当前代码已经完成四类执行迁移：

- 文本和图片通过 NewAPI Sync Workflow 执行。
- 视频通过 NewAPI Task Workflow 执行。
- 音乐通过 NewAPI Task Workflow 执行。
- AIGC 后端通过统一 NewAPI client 提交、轮询和下载结果。
- AIGC 本地模型平面及旧生成入口已退休。

视频模型已完成 11 个模型、35 个模式的旧版金样回归。图片、文本和音乐仍需各自保留覆盖模型能力、上游请求、计费、Trace 和结果入库的回归矩阵，防止只完成接口迁移而遗漏供应商语义。

## 必须保持的约束

1. `model`、`type`、Profile `model_type` 三者必须一致。
2. `generate_audio:false`、`duration:0` 等显式零值不能在 AIGC 到 NewAPI 的标准协议中丢失；是否发送给供应商由具体协议决定。
3. 请求 Trace ID 必须贯穿 AIGC、NewAPI、渠道 Mock 和结果下载。
4. 已预扣费用的请求发生错误或 panic 时必须退款。
5. Profile 解析失败不得创建孤立上游任务。
6. AIGC 后端只消费统一协议，不根据公共模型 ID 编写供应商特判。
7. `/v1/aigc/generations` 是唯一标准生成入口；暂不新增按媒体类型拆分的别名。

## 暂不实施

本阶段不增加以下接口：

```text
/v1/aigc/video/generations
/v1/aigc/image/generations
/v1/aigc/text/generations
/v1/aigc/music/generations
```

只有当不同媒体类型出现无法由标准请求和 Profile 表达的认证、流式传输或生命周期差异时，才重新评估拆分路径。单纯为了前端页面分类不应复制生成 API。
