# New API AIGC 服务接口

本文定义 AIGC Backend 调用 New API 的模型目录与生成协议。浏览器不直接调用这些接口。

## 鉴权

所有 `/v1/aigc/*` 接口使用用户自己的 New API Token：

```http
Authorization: Bearer <user-newapi-token>
Accept: application/json
```

New API 从 Token 得到 `user_id`、`token_id` 和实际用户组。请求方不能通过 Body 或 Query 指定这些身份字段。

## 接口列表

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/v1/aigc/models?type=text|image|video|music` | 查询当前用户可见模型 |
| POST | `/v1/aigc/generations` | 提交生成 |
| GET | `/v1/aigc/generations/{generation_id}` | 查询生成状态和结果 |

当前没有单模型详情或取消接口。

## 幂等协议

`POST /v1/aigc/generations` 只使用请求 Body 中的 `idempotency_key` 进行幂等判断：

```json
{
  "idempotency_key": "turn_01JXYZ"
}
```

规则：

1. `idempotency_key` 必填，去除首尾空格后长度为 1 到 200 个字符。
2. 幂等作用域是 `(user_id, idempotency_key)`。
3. 相同 Key、相同规范化请求返回原 Generation，不再次进入 Relay 或计费。
4. 相同 Key、不同请求摘要返回 HTTP 409 `IDEMPOTENCY_CONFLICT`。
5. 网络超时或响应丢失时，客户端必须使用相同 Key 和相同 Body 重放。
6. 原 Generation 明确失败后的业务重试必须使用新 Key；新请求重新读取当前 Published Profile，并重新选择当前模型与渠道。
7. 不使用 `Idempotency-Key` Header。`X-Request-ID` 如有发送只用于链路追踪，不参与幂等。

AIGC Backend 使用 AIGC Turn ID 作为 `idempotency_key`。

## 查询模型目录

```http
GET /v1/aigc/models?type=video HTTP/1.1
Authorization: Bearer sk-user-token
Accept: application/json
```

```json
{
  "object": "list",
  "data": [
    {
      "id": "video-cinema-pro",
      "name": "Cinema Pro",
      "type": "video",
      "description": "通用视频生成模型",
      "config_version": 7,
      "capabilities": {
        "modes": {
          "first_frame": {
            "inputs": {
              "first_frame": {"min": 1, "max": 1}
            }
          }
        },
        "output_specs": [
          {
            "id": "standard",
            "modes": ["first_frame"],
            "resolutions": ["720p", "1080p"],
            "aspect_ratios": ["16:9", "9:16"],
            "durations": [5, 8],
            "generate_audio": {
              "supported": true,
              "default": false
            }
          }
        ]
      }
    }
  ]
}
```

目录只返回当前 Token 用户组可见、已发布且存在可用渠道的能力。调用方传入的 Query 参数不能覆盖 Token 用户组。

## 提交生成

```http
POST /v1/aigc/generations HTTP/1.1
Authorization: Bearer sk-user-token
Content-Type: application/json
Accept: application/json
```

```json
{
  "idempotency_key": "turn_01JXYZ",
  "model": "video-cinema-pro",
  "type": "video",
  "prompt": "清晨薄雾中的城市天际线",
  "mode": "first_frame",
  "inputs": {
    "images": [
      {
        "role": "first_frame",
        "url": "https://aigc.example/internal/input/start-frame"
      }
    ],
    "videos": [],
    "audios": []
  },
  "output": {
    "resolution": "1080p",
    "aspect_ratio": "16:9",
    "duration": 8,
    "generate_audio": true
  },
  "options": {
    "negative_prompt": "抖动，字幕",
    "enhance": true,
    "private": false,
    "ai_mark": false
  },
  "parameters": {
    "instrumental": false
  }
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `idempotency_key` | 是 | 客户端幂等键，AIGC 使用 Turn ID |
| `model` | 是 | New API 公开模型 ID |
| `type` | 是 | `text`、`image`、`video` 或 `music` |
| `prompt` | 是 | 规范化后不能为空 |
| `mode` | 按类型 | Profile 中声明的业务模式 |
| `inputs` | 否 | 按 capability 约束的媒体角色和 URL |
| `output` | 按类型 | 图片或视频输出组合 |
| `options` | 否 | 通用生成选项 |
| `parameters` | 否 | 类型专属参数 |

`output.generate_audio` 使用三态语义：省略表示使用 Profile 默认值；显式 `false` 表示关闭；显式 `true` 要求能力声明支持。

处理顺序：

1. TokenAuth 确认用户、Token 和用户组。
2. 规范化请求并计算 SHA-256 请求摘要。
3. 按 `(user_id, idempotency_key)` 查询幂等记录。
4. 未命中时读取当前 Published Profile，校验 Group、Mode、输入和输出组合。
5. 解析当前 `upstream_model_id`，创建 Generation 记录。
6. 进入 New API Relay 完成渠道路由、供应商适配和计费。
7. 异步任务创建成功后回填 `native_task_id`。

## Generation 响应

```json
{
  "id": "aigc_gen_vD8uP4",
  "idempotency_key": "turn_01JXYZ",
  "status": "processing",
  "progress": 42,
  "model": "video-cinema-pro",
  "type": "video",
  "created_at": 1786848000,
  "outputs": [],
  "usage": {"quota": 0}
}
```

标准状态：

- `submitted`
- `queued`
- `processing`
- `completed`
- `failed`
- `canceled`

完成时 `progress` 必须为 100。媒体结果通过 `outputs[].url` 返回，调用方应及时下载；该 URL 不是产品永久资产地址。

## 查询 Generation

```http
GET /v1/aigc/generations/aigc_gen_vD8uP4 HTTP/1.1
Authorization: Bearer sk-user-token
Accept: application/json
```

查询按 `(user_id, generation_id)` 隔离。对于已有异步任务，New API 使用保存的 `native_task_id` 查询 Task，不重新读取 Model Profile，也不重新解析上游模型。管理员修改 Profile 只影响使用新 `idempotency_key` 提交的新 Generation。

## 错误协议

```json
{
  "error": {
    "code": "IDEMPOTENCY_CONFLICT",
    "message": "idempotency_key was already used with different input",
    "retryable": false,
    "idempotency_key": "turn_01JXYZ"
  }
}
```

| HTTP | Code | Retryable | 说明 |
| --- | --- | ---: | --- |
| 400 | `INVALID_REQUEST` | false | 请求结构错误或缺少幂等键 |
| 400 | `MODEL_MODE_NOT_SUPPORTED` | false | 模型不支持该 Mode |
| 400 | `INVALID_INPUT_ROLE` | false | 媒体角色或数量错误 |
| 400 | `OUTPUT_NOT_SUPPORTED` | false | 输出组合不受支持 |
| 401 | `UNAUTHORIZED` | false | Token 身份无效 |
| 403 | `MODEL_NOT_AVAILABLE_FOR_GROUP` | false | 当前用户组无权限 |
| 404 | `MODEL_NOT_FOUND` | false | 公开模型不存在或未发布 |
| 404 | `GENERATION_NOT_FOUND` | false | Generation 不存在或不属于当前用户 |
| 409 | `IDEMPOTENCY_CONFLICT` | false | 同一幂等键使用了不同请求 |
| 503 | `MODEL_CONFIGURATION_INVALID` | false | Profile 配置无效 |
| 503 | `MODEL_CHANNEL_UNAVAILABLE` | true | 当前用户组没有可用渠道 |

调用方只能对网络错误或 `retryable=true` 的临时错误进行原请求重放。明确失败后的重新生成必须使用新的 `idempotency_key`。
