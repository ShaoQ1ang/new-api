# AIGC 模型能力迁移到 New API 设计

## 1. 文档状态

- 状态：已确认总体边界，作为详细实施规划。
- 日期：2026-08-15。
- 涉及仓库：`new-api`、`aigc`。
- 核心结论：AIGC 前端继续只请求 AIGC 后端；AIGC 后端保留业务能力，但不再拥有模型适配、模型能力配置和模型分配；这些能力统一迁入 New API。

本文中的“AIGC”指创作台产品及其现有后端，“New API”指统一渠道、模型路由、鉴权和计费平台。

## 2. 背景与问题

当前 AIGC 后端同时承担两类职责：

1. 创作业务职责：会话、Turn、任务、作品、素材、上传、灵感、SSE 和对象存储。
2. 模型平台职责：模型目录、能力配置、业务模式到上游模型的映射、供应商协议适配、异步任务轮询以及部分价格展示适配。

第二类职责与 New API 已有的渠道管理、模型路由、用户组、Token、计费、任务和日志能力重叠。每新增一个模型或供应商，通常需要同时修改 New API 和 AIGC，容易出现以下问题：

- 同一个模型在两边维护不同的 ID、能力和启用状态。
- AIGC 需要理解 OpenAI、Gemini、OpenRouter、Suno 和各视频供应商协议。
- 模型是否可用、用户是否可见、应如何计费可能由两边分别判断。
- AIGC 管理后台需要从 New API 同步模型，再维护一份本地发布快照。
- 异步任务、错误码、重试和幂等逻辑按供应商分散在 AIGC Provider 中。

本次迁移的目的不是下线 AIGC 后端，而是将其收敛为稳定的创作业务服务和前端 BFF。

## 3. 目标

### 3.1 必须实现

- AIGC 前端请求地址、登录方式和核心业务协议保持不变。
- New API 新增独立的 `aigc/` 后端目录，集中实现 AIGC 模型能力。
- New API 成为 AIGC 模型目录、能力配置、模式映射和模型发布状态的唯一事实来源。
- New API 提供面向 AIGC 后端的服务间模型目录和生成协议。
- New API 管理后台新增 AIGC 模型分配页面。
- AIGC 后端通过用户对应的 New API Token 调用服务间接口，使用户组、额度、计费和日志归属真实用户。
- AIGC 删除供应商级模型适配和本地模型发布配置，只保留 New API Client。
- AIGC 继续拥有会话、Turn、业务任务、作品、素材、上传、灵感和永久对象存储。
- 支持 SQLite、MySQL 和 PostgreSQL。

### 3.2 非目标

- 不将 AIGC 创作台前端迁入 New API Web。
- 不将 AIGC 会话、作品、素材或灵感数据迁入 New API。
- 不让浏览器直接调用 New API 的 AIGC 服务间接口。
- 不修改已经废弃的 `web/default`；New API 管理界面只修改 `web/classic`。
- 不在 `aigc/` 下重新实现渠道选择、供应商适配或计费引擎。
- 不在第一阶段重做 AIGC 页面交互和视觉设计。
- 不要求一次性切换所有模型类型。

## 4. 已确认架构

```text
AIGC Frontend
    |
    | existing /api/* contracts
    v
AIGC Backend
    |- auth/session bridge
    |- conversations and turns
    |- business job state and SSE
    |- assets, uploads and inspirations
    |- permanent object storage
    `- New API AIGC Client
            |
            | user-bound New API token
            v
New API /aigc module
    |- public model catalog
    |- capability validation
    |- public model -> upstream model routing
    |- normalized generation protocol
    |- idempotency envelope
    `- admin model assignment API
            |
            v
Existing New API relay/task/billing/channel stack
            |
            v
Upstream providers
```

### 4.1 所有权边界

| 能力 | AIGC | New API |
| --- | --- | --- |
| 前端 BFF 接口 | 是 | 否 |
| 登录态桥接和用户 New API Token | 是 | 签发及校验 |
| 会话、Turn、业务 Job | 是 | 否 |
| 永久 Asset 和对象存储 | 是 | 否 |
| 灵感内容 | 是 | 否 |
| AIGC 公共模型 ID 和展示名称 | 只消费 | 是 |
| 模型能力和业务模式 | 只消费 | 是 |
| 公共模型到真实模型映射 | 否 | 是 |
| 用户组模型可见性 | 透传用户身份 | 是 |
| 渠道选择和失败重试 | 否 | 是 |
| 供应商协议 | 否 | 是，复用 `relay/` |
| 上游异步任务 | 只保存外部任务 ID | 是 |
| 计费、额度和消费日志 | 展示或代理查询 | 是 |

### 4.2 两层任务的含义

两边可以各有一条任务记录，但含义必须不同：

- AIGC Job 是产品业务任务，关联 Conversation、Turn 和最终 Asset，驱动前端 SSE。
- New API AIGC Request 是模型调用信封，负责幂等、路由快照、真实用户计费归属和上游任务引用。

AIGC Job 不保存供应商私有字段。New API Request 不保存 Conversation 业务内容，也不成为作品系统。

## 5. New API 包规划

### 5.1 后端目录

```text
aigc/
├── router/
│   ├── api.go                 # /api/aigc 管理接口
│   └── relay.go               # /v1/aigc 服务间接口
├── handler/
│   ├── admin_models.go
│   ├── models.go
│   └── generations.go
├── dto/
│   ├── model.go
│   ├── generation.go
│   ├── input.go
│   ├── output.go
│   └── error.go
├── entity/
│   ├── model_profile.go
│   └── request.go
├── repository/
│   ├── model_profile.go
│   └── request.go
├── service/
│   ├── catalog.go
│   ├── generation.go
│   ├── availability.go
│   └── idempotency.go
├── capability/
│   ├── common.go
│   ├── image.go
│   ├── video.go
│   ├── music.go
│   └── text.go
├── execution/
│   ├── executor.go            # 执行端口
│   ├── text.go                # 调用现有 Relay 能力
│   ├── image.go
│   ├── video.go               # 调用现有视频 Task 能力
│   └── music.go               # 调用现有音乐 Task 能力
└── testdata/
    └── contracts/
```

目录名称允许在实施时按项目现有命名小幅调整，但依赖方向不能改变。

### 5.2 依赖方向

```text
router -> handler -> service -> repository -> entity
                         |
                         `-> execution port -> existing relay/task services

dto and capability are leaf packages
```

约束：

- `aigc/service` 不导入 Gin。
- `aigc/entity` 不导入根目录 `model` 包，避免迁移注册产生循环依赖。
- `aigc/repository` 可以使用根 `model.DB`，但不得包含协议转换。
- `aigc/execution` 不通过 `localhost` HTTP 回调 New API 自己。
- 不调用 `controller.Relay` 并截获 HTTP Response；应抽取可测试的 Relay/Task 服务入口。
- 供应商名称、URL、请求字段和响应解析不得进入 `aigc/capability` 或 `aigc/service`。
- 所有 JSON 编解码继续使用 New API 的 `common.Marshal`、`common.Unmarshal` 等包装函数。

### 5.3 路由注册

根 `router.SetRouter` 增加两处注册：

```text
aigc/router.SetApiRouter(router)
aigc/router.SetRelayRouter(router)
```

管理接口加入 `/api` 路由体系，复用 `AdminAuth`、权限和 API 限流。服务间接口加入 `/v1` 路由体系，复用 `TokenAuth`、模型限流、用户组选择、请求日志和性能检查。

### 5.4 数据库迁移注册

`aigc/entity` 只包含纯 GORM Entity。根 `model/main.go` 可以导入该叶子包并将 Entity 加入 `AutoMigrate`，不会形成循环依赖。

迁移必须满足：

- 三种数据库使用相同 GORM Schema。
- JSON 配置使用 `TEXT`，业务层负责序列化，不使用数据库专属 JSON 类型。
- 不依赖数据库生成的枚举类型。
- 唯一索引长度兼容 MySQL。
- 创建默认值由代码负责，避免跨数据库重复迁移。

## 6. 数据模型

### 6.1 `AigcModelProfile`

建议表名：`aigc_model_profiles`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint/int | 内部主键 |
| `public_model_id` | varchar(128) | AIGC 对外稳定模型 ID，唯一 |
| `display_name` | varchar(128) | 创作台展示名称 |
| `model_type` | varchar(16) | `text/image/video/music` |
| `description` | text | 模型说明 |
| `status` | int | `draft/published/disabled` |
| `groups_json` | text | 可见 New API 用户组；空数组表示按真实模型能力自动开放 |
| `config_json` | text | 强类型能力和路由配置 |
| `config_version` | int | 乐观锁及目录缓存版本 |
| `created_time` | bigint | 创建时间 |
| `updated_time` | bigint | 更新时间 |

状态语义：

- `draft`：仅管理员可见，不能生成。
- `published`：满足配置、上游能力和用户组条件时可见。
- `disabled`：保留配置和历史引用，但立即停止新请求。

不提供物理删除已发布模型的常规入口。未被使用的草稿可以删除。

### 6.2 `AigcRequest`

建议表名：`aigc_requests`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint/int | 内部主键 |
| `request_id` | varchar(191) | AIGC Turn ID/幂等键 |
| `generation_id` | varchar(191) | New API 对外任务 ID，唯一 |
| `user_id` | int | 真实 New API 用户 |
| `token_id` | int | 实际调用 Token |
| `group_name` | varchar(64) | 提交时使用的用户组快照 |
| `public_model_id` | varchar(128) | AIGC 公共模型 ID |
| `upstream_model_id` | varchar(255) | 解析后的真实模型 ID |
| `model_type` | varchar(16) | 模型类型 |
| `mode` | varchar(40) | 解析后的业务模式 |
| `config_version` | int | 提交时解析使用的 Profile 版本 |
| `status` | varchar(24) | 标准状态 |
| `progress` | int | 标准化进度，范围 `0-100` |
| `native_task_id` | varchar(191) | New API 原生异步 Task ID，可空 |
| `request_digest` | varchar(64) | 规范化请求摘要 |
| `request_json` | text | 规范化请求快照，用于故障恢复和审计 |
| `execution_json` | text | 解析后的执行目标快照，运行中任务不随 Profile 更新漂移 |
| `result_json` | text | 结果描述，不保存永久作品二进制 |
| `error_code` | varchar(64) | 标准错误码 |
| `error_message` | text | 安全的用户错误信息 |
| `created_time` | bigint | 创建时间 |
| `updated_time` | bigint | 更新时间 |
| `finished_time` | bigint | 完成时间 |

唯一索引：`(user_id, request_id)`。相同用户和幂等键只能对应一个逻辑调用。

幂等规则：

- 相同 `request_id` 和相同 `request_digest` 返回原任务。
- 相同 `request_id` 但请求内容不同返回 `409 IDEMPOTENCY_CONFLICT`。
- 已完成任务不得重新经过计费和 Relay。
- 失败是否允许重新执行由明确的 Retry API 或新的 Turn ID 决定，不能靠重复 POST 隐式重跑。

## 7. 模型能力配置

### 7.1 公共原则

公共模型 ID 与真实模型 ID 分离。例如：

```text
public: happyhorse-1.1
text_to_video -> happyhorse-1.1-t2v
first_frame   -> happyhorse-1.1-i2v
reference     -> happyhorse-1.1-r2v
```

New API 负责将前台业务模式解析为真实模型。AIGC 后端和前端都不得知道上述真实 ID。

每份配置必须具备版本号。提交生成请求时保存所使用的配置版本和真实模型快照，管理员后续修改配置不能改变进行中的任务。

### 7.2 图片配置

```json
{
  "adapter": "image-relay",
  "modes": {
    "text_to_image": {
      "upstream_model_id": "gemini-3.1-flash-image",
      "output": {
        "sizes": ["1024x1024", "1536x1024"],
        "counts": [1, 2, 4],
        "default_size": "1024x1024",
        "default_count": 1
      }
    },
    "image_edit": {
      "upstream_model_id": "gemini-3.1-flash-image",
      "input": {
        "role": "source_image",
        "min": 1,
        "max": 4,
        "accept": ["image/jpeg", "image/png", "image/webp"]
      },
      "output": {
        "sizes": ["1024x1024"],
        "counts": [1],
        "default_size": "1024x1024",
        "default_count": 1
      }
    }
  }
}
```

`adapter` 表示 New API 内部标准执行器，不是供应商品牌。供应商差异由真实模型的渠道 Relay 处理。

### 7.3 视频配置

必须支持以下业务模式：

- `text_to_video`
- `first_frame`
- `first_last_frame`
- `reference`
- `video_extension`
- `video_edit`

模式配置包含输入角色限制和真实模型映射；输出规格独立表达分辨率、比例、时长和音频能力。不同输出规格可以覆盖真实模型目标，用于同一公共模型在 720p 和 1080p 时路由不同上游模型。

验证要求：

- 每个启用模式必须映射真实模型。
- 每个启用模式至少被一个输出规格覆盖。
- 同一模式、分辨率、比例、时长和音频选项只能命中一个规格。
- 所有输入角色的 `min/max` 合法且总量不超过系统限制。
- 视频时长使用 New API 现有最大时长常量。
- 目标真实模型必须在至少一个允许用户组中有可用 Ability。

### 7.4 音乐配置

第一版支持 `text_to_music`，配置是否允许纯音乐、默认值和输出曲目数范围。真实模型通过 New API 当前的音乐 Task Adapter 执行。

### 7.5 文本配置

文本模型配置只保留公共 ID、真实模型 ID、可选系统参数白名单及允许的最大输出长度。AIGC 业务层不再保存 OpenRouter 或 OpenAI 请求字段。

## 8. 模型可见性与发布判断

一个模型进入 AIGC 前台目录必须同时满足：

1. Profile 状态为 `published`。
2. 配置 JSON 可以按当前版本成功解析和验证。
3. 当前用户组在 Profile 允许组内，或 Profile 未显式限制组。
4. 当前用户组对该业务模式映射的真实模型存在启用 Ability。
5. 目标渠道处于启用状态。
6. 模型具备可用计费配置。

目录结果必须按当前用户 Token 的实际组计算，不能由 AIGC 传入任意组名绕过权限。

上游模型下线时不删除 Profile：

- 管理后台显示“上游不可用”。
- 用户目录立即隐藏。
- 已提交任务继续按快照执行或进入明确失败状态。
- 上游恢复后，无需重新发布即可恢复可见，除非管理员已停用 Profile。

## 9. 服务间 API

### 9.1 鉴权和公共 Header

所有 `/v1/aigc/*` 接口使用 New API `TokenAuth`。

```http
Authorization: Bearer <user-newapi-token>
X-Request-ID: <aigc-turn-id>
Idempotency-Key: <aigc-turn-id>
Content-Type: application/json
```

规则：

- `X-Request-ID` 用于全链路日志，不承担唯一性。
- `Idempotency-Key` 用于业务幂等，必须与 Body 中 `request_id` 一致。
- AIGC 不使用管理员或公共服务 Token 替用户调用，否则会破坏用户组、额度和日志归属。
- New API 不信任 AIGC 传入的 `user_id`、`group`、`quota` 或价格字段。

### 9.2 模型目录

```http
GET /v1/aigc/models?type=image
GET /v1/aigc/models/:public_model_id
```

响应只包含当前用户可见的公共能力，不返回真实模型 ID、渠道 ID、价格内部表达式或供应商密钥。

```json
{
  "object": "list",
  "data": [
    {
      "id": "happyhorse-1.1",
      "name": "HappyHorse 1.1",
      "type": "video",
      "description": "...",
      "config_version": 3,
      "capabilities": {
        "modes": [],
        "output_specs": []
      }
    }
  ]
}
```

响应支持 `ETag`。AIGC 后端可做短缓存，但收到配置版本变化后必须刷新；缓存不可跨用户组共享。

### 9.3 提交生成

```http
POST /v1/aigc/generations
```

```json
{
  "request_id": "turn-01J...",
  "model": "happyhorse-1.1",
  "type": "video",
  "prompt": "A train moving through snow",
  "mode": "first_frame",
  "inputs": {
    "images": [
      {"role": "first_frame", "url": "https://..."}
    ],
    "videos": [],
    "audios": []
  },
  "output": {
    "resolution": "1080p",
    "aspect_ratio": "16:9",
    "duration": 10,
    "generate_audio": true
  },
  "options": {
    "negative_prompt": "",
    "enhance": false,
    "private": true,
    "ai_mark": true
  }
}
```

处理顺序固定为：

1. 校验 Header 和 Body 幂等键。
2. 根据 Token 获取用户、Token 和实际使用组。
3. 加载已发布 Profile。
4. 校验请求能力和输入数量。
5. 解析真实模型、执行器和配置版本。
6. 创建或命中 `AigcRequest`。
7. 进入 New API 现有 Relay/Task、额度预扣和日志链路。
8. 返回标准任务信封。

```json
{
  "id": "aigc_gen_01J...",
  "request_id": "turn-01J...",
  "status": "submitted",
  "progress": 0,
  "model": "happyhorse-1.1",
  "type": "video",
  "created_at": 1786723200,
  "outputs": []
}
```

文本或图片上游同步完成时，接口可以直接返回 `completed`；AIGC Client 必须同时支持 `completed` 和异步状态，不能假设所有类型都需要轮询。

### 9.4 查询生成状态

```http
GET /v1/aigc/generations/:generation_id
```

统一状态：

```text
submitted -> queued -> processing -> completed
                                 `-> failed
                                 `-> canceled
```

状态转换只能前进，重复查询无副作用。上游的 `NOT_START/SUBMITTED/QUEUED/IN_PROGRESS/SUCCESS/FAILURE` 等状态在 New API 内部归一化。

完成响应示例：

```json
{
  "id": "aigc_gen_01J...",
  "request_id": "turn-01J...",
  "status": "completed",
  "progress": 100,
  "model": "happyhorse-1.1",
  "type": "video",
  "outputs": [
    {
      "id": "output-1",
      "type": "video",
      "url": "https://upstream-temporary-url/...",
      "content_type": "video/mp4",
      "width": 1920,
      "height": 1080,
      "duration": 10,
      "poster_url": "https://..."
    }
  ],
  "usage": {
    "quota": 12345
  }
}
```

`usage` 只用于展示和排障。AIGC 不根据该字段自行扣费。

### 9.5 取消

```http
POST /v1/aigc/generations/:generation_id/cancel
```

第一阶段可以只实现能力探测和标准返回：上游支持取消时执行取消；不支持时返回 `409 CANCEL_NOT_SUPPORTED`。AIGC 前端当前没有可靠取消语义时，不需要阻塞首轮迁移。

## 10. 管理 API

管理接口使用 New API Session 和管理员权限：

```http
GET    /api/aigc/models
GET    /api/aigc/models/:id
POST   /api/aigc/models
PUT    /api/aigc/models/:id
POST   /api/aigc/models/:id/validate
POST   /api/aigc/models/:id/publish
POST   /api/aigc/models/:id/disable
DELETE /api/aigc/models/:id              # 仅未发布草稿
POST   /api/aigc/models/import           # 一次性导入旧 model_access
GET    /api/aigc/upstream-models
GET    /api/aigc/upstream-models/:id
```

`upstream-models` 直接聚合当前 `models`、`abilities`、渠道状态、端点和价格，不再执行 AIGC 侧同步，也不落一份上游快照。

更新接口必须提交 `config_version`。版本不匹配返回 `409 CONFIG_VERSION_CONFLICT`，防止两个管理员相互覆盖。

发布操作必须重新执行完整校验，不能仅依赖前端表单校验。

## 11. New API 管理后台

New API 管理界面只在 `web/classic` 实现。`web/default` 已废弃，不新增页面、组件、路由、文案或兼容代码。

Classic 现有模型管理位于 `/console/models`，采用 `ModelPage` + Semi UI Tabs。AIGC 模型分配作为管理员专属 Tab 加入该页面，不新增一套平行导航。建议目录：

```text
web/classic/src/
├── pages/Model/index.jsx                         # 增加管理员专属 Tab
├── hooks/aigc-models/
│   └── useAigcModelsData.jsx
└── components/table/aigc-models/
    ├── index.jsx
    ├── AigcModelsTable.jsx
    ├── AigcModelsColumns.jsx
    ├── AigcModelsFilters.jsx
    ├── AigcModelsActions.jsx
    ├── modals/
    │   └── EditAigcModelModal.jsx
    └── editors/
        ├── ModeMappingEditor.jsx
        ├── OutputSpecEditor.jsx
        └── ValidationSummary.jsx
```

前端实现约束：

- 使用 Classic 当前的 React 18、JavaScript/JSX、Semi UI 和现有 Tailwind 工具类。
- 数据请求使用 `web/classic/src/helpers` 导出的 `API`，沿用 `{success, message, data}` 响应处理。
- 表格、分页、紧凑模式和移动端行为复用现有 Models Table、`CardPro`、`useTableCompactMode` 和 `createCardProPagination` 模式。
- 编辑能力较复杂时使用 Semi UI Modal 或 SideSheet，但不引入 Default 的 Base UI、React Query、Zustand 或 TypeScript Feature 结构。
- 页面只对管理员显示，沿用 `isAdmin()` 和现有管理权限路由；普通管理权限不能隐式获得 AIGC 模型发布能力。

页面功能：

- 按类型、状态、用户组和上游可用性筛选。
- 创建公共模型，设置展示名称和说明。
- 选择一个或多个允许用户组。
- 为各业务模式选择真实模型。
- 配置输入角色、数量、MIME 类型和组合策略。
- 配置分辨率、比例、时长、数量和音频选项。
- 查看真实模型的渠道数、端点、用户组和价格状态。
- 保存草稿、校验、发布、停用和复制配置。
- 清晰显示“缺渠道”“缺价格”“配置错误”“上游不可用”“已发布”等状态。

界面不直接编辑原始 JSON。可以提供只读 JSON 预览用于排障，但保存必须经过强类型表单和后端验证。

所有新文案使用 `useTranslation()`，加入 `web/classic/src/i18n/locales`。至少保证英文和简体中文完整，并从 `web/classic` 运行 `bun run i18n:sync`、`bun run i18n:lint`。

## 12. AIGC 后端收敛规划

### 12.1 保留的前端接口

以下接口继续由 AIGC 后端提供，前端无需改变地址：

```text
GET    /api/models
GET    /api/v1/models
GET    /api/v1/videos/models
GET    /api/conversations
POST   /api/conversations
GET    /api/conversations/:id/turns
POST   /api/conversations/:id/turns
GET    /api/conversations/:id/turns/:turn_id
DELETE /api/conversations/:id/turns/:turn_id
GET    /api/assets
GET    /api/assets/:id/content
POST   /api/uploads/grants
POST   /api/uploads/confirm
```

其中模型目录接口改为调用 New API 后按现有 AIGC 响应格式返回。会话和素材接口仍使用 AIGC 自己的数据库。

### 12.2 新增 Client 包

建议在 AIGC 中新增：

```text
backend/internal/adapter/newapi/aigc/
├── client.go
├── models.go
├── generations.go
├── errors.go
└── client_test.go
```

Client 职责仅包括：

- 使用当前用户对应的 New API Token。
- 发送模型目录和生成请求。
- 透传 `X-Request-ID` 和幂等键。
- 解码标准错误和任务状态。
- 设置连接、请求和空闲超时。
- 不在 Client 内判断具体供应商或真实模型 ID。

### 12.3 简化后的创建 Turn 流程

```text
1. 前端 POST AIGC /api/conversations/:id/turns
2. AIGC 校验会话归属和基础请求格式
3. AIGC 创建本地 Job/Turn，状态 queued
4. AIGC 使用 Turn ID 调 New API POST /v1/aigc/generations
5. New API 校验完整模型能力并提交 Relay/Task
6. AIGC 保存 generation_id
7. completed: AIGC 立即转存输出并完成 Asset
8. async: AIGC 查询 New API 状态并更新本地进度
9. AIGC 将临时输出转存永久对象存储
10. AIGC 完成本地 Job 并通过 SSE 通知前端
```

AIGC 保留本地 Job 恢复机制，但恢复动作只允许：

- 已有 `generation_id`：查询原 New API 任务，禁止重新提交。
- 尚无 `generation_id`：使用原 Turn ID 重复提交，由 New API 幂等层返回原任务。

### 12.4 迁移后删除的 AIGC 能力

最终删除或退役：

- 本地 `model_access` 和 `upstream_models` 发布配置。
- `/api/admin/models`、`/api/admin/models/sync` 和相关管理 UI。
- 图片、视频、音乐模型能力配置和默认配置表。
- `image_provider_*`、`video_provider_*`、`music_provider_*` 和供应商 Factory。
- AIGC 对 OpenAI、Gemini、OpenRouter、Suno、Ali Video 等请求体转换。
- AIGC 根据模型参数推断上游价格或真实模型 ID 的逻辑。

以下能力继续保留：

- New API 登录态和用户 Token 获取/刷新。
- Billing 页面对 New API 账户、订阅和日志接口的代理。
- AIGC 对输入素材的业务归属校验。
- AIGC 对生成结果的下载、永久转存、Asset 创建和清理。

## 13. 同步与异步执行

New API 的 AIGC 协议对 AIGC 后端提供统一任务信封，但内部不强制所有供应商采用同一种执行方式。

### 13.1 文本和图片

- 通过现有同步 Relay 执行。
- 可以在创建响应中直接返回 `completed`。
- 文本结果保存为标准文本 Output。
- 图片结果可能是 URL 或内联数据；New API 统一成标准 Output，AIGC 立即转存。

### 13.2 视频和音乐

- 通过 New API 现有 Task 体系提交。
- `AigcRequest.native_task_id` 引用原生 Task。
- 查询接口读取并映射原生 Task 状态。
- 上游轮询、结算、退款和失败原因解析继续由 New API 负责。

### 13.3 输出媒体生命周期

New API 不成为 AIGC 永久作品存储。

- 上游 URL 视为临时 URL。
- AIGC 必须在 New API 返回完成后立即流式下载并写入自己的对象存储。
- AIGC 只有在对象存储成功并创建 Asset 后才能将本地 Job 标记为 `completed`。
- 下载失败时保留 `generation_id`，重试只重新获取/下载结果，不重新生成。
- New API 不把大型媒体二进制长期写入 `aigc_requests.result_json`。

对于只返回内联图片且无法安全重放的供应商，第一轮迁移前必须确认以下方案之一：

1. New API Relay 可以请求 URL 响应；或
2. New API 在创建响应中把内联结果直接返回给 AIGC；或
3. 增加短期结果存储/内容读取端点。

该项是图片切流前的明确 Gate，不能在生产切换时临时决定。

## 14. 鉴权、权限与计费

### 14.1 用户身份

AIGC 当前登录流程继续维护用户与 New API Session/Token 的关联。调用 AIGC 模型接口时，AIGC 后端加载该用户的 New API Token。

禁止使用单一服务 Token 代替所有用户，原因包括：

- 用户组模型可见性失效。
- 用户额度和订阅无法正确扣减。
- 日志无法关联真实用户。
- 单 Token 限流会影响所有 AIGC 用户。

### 14.2 用户组

New API 根据 Token 和用户配置计算实际组。Profile 的 `groups_json` 只做进一步收窄，不能赋予用户原本没有的组。

真实模型必须在该组存在 Ability。模式映射涉及多个真实模型时，每个模式分别检查，不要求所有模式同时可用；目录可以只返回当前组可用的模式，但同一配置版本下必须有明确、稳定的裁剪规则。

推荐第一版采用“模型整体可见，模式按组裁剪”；如果裁剪后没有任何模式，则隐藏模型。

### 14.3 计费

- 计费只发生在现有 New API Relay/Task 路径。
- `aigc/service` 不计算金额，不接受客户端传入价格。
- `AigcRequest` 只保存最终 quota 引用或展示快照，不作为账本。
- 同步和异步路径都必须通过 New API 现有预扣、结算、退款和日志逻辑。
- 幂等命中不得再次预扣。
- AIGC 本地 Job 重试不得导致第二次模型调用。

涉及视频时长、图片数量、分辨率和其他倍率时，继续遵守 New API 现有计费边界、上限和饱和转换规则。

## 15. 错误协议

标准错误响应：

```json
{
  "error": {
    "code": "MODEL_MODE_NOT_SUPPORTED",
    "message": "该模型不支持首尾帧生成",
    "retryable": false,
    "request_id": "turn-01J..."
  }
}
```

建议错误码：

| HTTP | Code | Retryable | 含义 |
| --- | --- | --- | --- |
| 400 | `INVALID_REQUEST` | 否 | 请求结构错误 |
| 400 | `INVALID_INPUT_ROLE` | 否 | 输入角色错误 |
| 400 | `OUTPUT_NOT_SUPPORTED` | 否 | 输出规格不支持 |
| 401 | `UNAUTHORIZED` | 否 | Token 无效 |
| 403 | `MODEL_NOT_AVAILABLE_FOR_GROUP` | 否 | 用户组无权使用 |
| 404 | `MODEL_NOT_FOUND` | 否 | 公共模型不存在或未发布 |
| 409 | `IDEMPOTENCY_CONFLICT` | 否 | 同一幂等键请求内容不同 |
| 409 | `CONFIG_VERSION_CONFLICT` | 是 | 管理配置版本冲突 |
| 429 | `RATE_LIMITED` | 是 | 限流 |
| 402/429 | `INSUFFICIENT_QUOTA` | 否 | 额度不足，沿用项目约定状态码 |
| 502 | `UPSTREAM_REJECTED` | 视情况 | 上游拒绝 |
| 503 | `MODEL_CHANNEL_UNAVAILABLE` | 是 | 无可用渠道 |
| 504 | `UPSTREAM_TIMEOUT` | 是 | 上游超时 |

AIGC Client 将这些错误映射为现有 Job 的 `error_code`、`error` 和 `retryable` 字段，前端无需理解供应商原始错误。

内容安全拒绝应映射为稳定的不可重试错误，不把上游完整响应或敏感字段返回前端。

## 16. 可观测性

全链路统一使用 AIGC Turn ID：

```text
AIGC turn_id
  -> X-Request-ID
  -> Idempotency-Key
  -> New API AigcRequest.request_id
  -> Relay log request id
  -> upstream task metadata when supported
```

New API 日志至少包含：

- `request_id`
- `generation_id`
- `user_id`、`token_id`、`group`
- `public_model_id`
- `upstream_model_id`
- `config_version`
- `channel_id`
- `native_task_id`
- 标准状态和错误码

日志不得记录 Token、供应商 Key、完整签名 URL 或包含隐私内容的原始媒体。

建议指标：

- 按公共模型和模式统计请求量、成功率和延迟。
- 模型配置校验失败数。
- 无渠道和无价格次数。
- 幂等命中与冲突次数。
- AIGC 已完成但素材转存失败次数。
- New API 上游完成到 AIGC Asset 完成的耗时。

## 17. 缓存与一致性

- 已发布 Profile 可按 `config_version` 缓存在 New API 内存和 Redis。
- 发布、停用和更新配置后主动失效缓存。
- 模型目录缓存 Key 必须至少包含用户组和模型类型。
- Ability/渠道状态变化后，目录可见性必须跟随 New API 现有缓存刷新机制。
- AIGC BFF 可以短缓存目录，但不得缓存生成权限判断；New API 每次提交仍需重新校验。
- 任务提交保存路由快照，配置更新不影响运行中任务。

## 18. 分阶段实施计划

### Phase 0：契约冻结和基线

目标：先锁定现有行为，不改运行路径。

- 固化 AIGC 前端使用的模型目录和创建 Turn 请求样例。
- 为文本、图片、视频、音乐各选至少一个代表模型。
- 固化输入角色、输出规格、错误码和结果结构。
- 记录当前模型配置和公共模型 ID。
- 统计所有 AIGC Provider、Factory 和默认配置文件。
- 确认图片内联结果的交付方案。

完成标准：测试可以从现有 AIGC 请求生成预期标准执行规格。

### Phase 1：New API 模型配置面

- 创建 `aigc/` 包骨架。
- 新增 `AigcModelProfile` Entity、Repository 和跨数据库迁移。
- 迁移图片、视频、音乐和文本能力类型及校验规则。
- 实现上游模型聚合和可用性判断。
- 实现管理 API。
- 实现 New API 管理后台页面。
- 导入现有 AIGC 模型配置为初始 Profile。

此阶段不切换生产生成流量。

### Phase 2：目录接口和 AIGC 代理

- 实现 `/v1/aigc/models`。
- AIGC 新增 New API AIGC Client。
- AIGC `/api/models` 和视频模型目录改为调用 New API。
- 增加旧目录与新目录对比日志。
- 通过特性开关选择旧目录或新目录。

完成标准：相同用户组下，模型、模式和能力与预期一致。

### Phase 3：文本和图片生成

- 抽取可复用的同步 Relay 执行入口。
- 实现 `AigcRequest` 和幂等逻辑。
- 接通文本生成。
- 接通图片生成和编辑。
- 验证结果转存、图片数量、尺寸、输入上限和计费。
- AIGC 通过类型级特性开关切流。

完成标准：AIGC 不再直接构造文本/图片供应商请求。

### Phase 4：视频生成

- 将 AIGC 视频业务模式解析迁入 New API。
- 复用现有 `/v1/videos` Task 和各 Task Adaptor。
- 建立 `AigcRequest` 到原生 Task 的引用。
- 统一状态、进度、结果和错误映射。
- 验证服务重启后的任务恢复与查询。
- 切换一个低风险视频模型，再逐个模型迁移。

完成标准：AIGC 只持有 `generation_id`，不理解供应商 Task ID 或协议。

### Phase 5：音乐生成

- 将 Suno 模式和参数校验迁入 New API。
- 复用现有音乐 Task 提交和查询。
- 支持多曲目 Output。
- 验证多 Asset 转存的原子性和清理逻辑。

### Phase 6：清理 AIGC 模型层

- 停止写入 AIGC `model_access` 和 `upstream_models`。
- 移除 AIGC 模型管理入口。
- 删除供应商 Provider、Factory 和协议测试。
- 删除模型能力默认值和同步逻辑。
- 保留必要的兼容读取一个发布周期。
- 最后移除兼容表和环境变量。

## 19. 特性开关

建议 AIGC 使用以下类型级开关：

```text
AIGC_MODEL_CATALOG_SOURCE=local|newapi
AIGC_TEXT_EXECUTION=local|newapi
AIGC_IMAGE_EXECUTION=local|newapi
AIGC_VIDEO_EXECUTION=local|newapi
AIGC_MUSIC_EXECUTION=local|newapi
```

开关用于迁移和回滚，不成为长期产品配置。所有类型切换完成并稳定一个发布周期后删除。

禁止对同一个用户请求同时调用新旧执行链路进行结果对比，因为会产生双重生成和双重计费。允许对目录和纯校验结果进行影子对比。

## 20. 测试计划

### 20.1 New API 单元测试

- 四种模型配置解析、规范化和验证。
- 视频输出规格重叠检测。
- 输入数量、角色和 MIME 类型边界。
- 用户组与 Ability 交集。
- 上游渠道停用后的可见性。
- 幂等首次提交、重复命中和冲突。
- Profile 乐观锁。
- 标准状态和错误映射。

### 20.2 New API 集成测试

- Router -> Handler -> Service -> Repository。
- TokenAuth 后真实用户和组归属。
- 公共模型到真实模型再到渠道选择。
- 同步 Relay 的计费和日志只发生一次。
- 异步 Task 的预扣、结算、退款和恢复。
- SQLite、MySQL、PostgreSQL Migration。
- 管理员与普通用户权限隔离。

### 20.3 契约测试

在 `aigc/testdata/contracts` 保存不含敏感信息的请求和预期执行规格：

- 文本生成。
- 文生图。
- 多图编辑。
- 文生视频。
- 首帧和首尾帧。
- 参考图片/视频/音频组合。
- 视频续写和编辑。
- 音乐及纯音乐选项。

契约测试验证公共协议，不锁定供应商私有实现细节。

### 20.4 AIGC 测试

- 前端 API 合约保持不变。
- 模型目录代理和缓存按用户隔离。
- 创建 Turn 只提交一次 New API 请求。
- AIGC 重启后使用原幂等键恢复。
- New API 完成后素材转存成功才完成 Job。
- 临时 URL 下载失败可以重试且不重新生成。
- 多音乐 Asset 部分失败时执行清理。
- SSE 状态和现有前端兼容。

### 20.5 前端验证

- 只验证 `web/classic`，不构建或修改 `web/default`。
- 从 `web/classic` 运行 Prettier、ESLint、i18n Lint 和 Vite Build。
- 创建、编辑、校验、发布和停用完整流程。
- 长模型 ID、长错误信息和移动端不溢出。
- 管理员专属 Tab 权限与非管理员不可见性。
- AIGC 创作台回归测试，确认无需改请求地址。

## 21. 发布与回滚

### 21.1 发布顺序

1. 先发布 New API Schema 和只读目录能力。
2. 导入 Profile，但保持生成接口无流量。
3. 发布 New API 管理页面并完成配置校验。
4. 发布 AIGC Client 和关闭状态的特性开关。
5. 先切目录，再按文本、图片、视频、音乐依次切执行。
6. 每种类型至少观察一个完整任务周期。
7. 最后清理 AIGC 旧适配代码。

### 21.2 回滚原则

- Schema 和新表保留，不在紧急回滚时删除。
- 类型级开关可将新请求切回旧 Provider。
- 已经提交到 New API 的任务必须继续通过 New API 查询，不能切回旧 Provider 重新提交。
- 回滚只影响新 Turn，进行中 Turn 按其 `execution_source` 完成。
- AIGC Job 增加 `execution_source` 和 `external_generation_id`，确保恢复时选择正确链路。

### 21.3 数据迁移

现有 AIGC `model_access` 导入 New API 时：

- `model_id` -> `public_model_id`。
- `display_name` -> `display_name`。
- `model_type` -> `model_type`。
- `config_json` 经过新类型校验后写入。
- `enabled=true` 仅在完整校验通过时转为 `published`，否则导入为 `draft`。
- 迁移报告列出无法解析、真实模型不存在或缺少价格的配置。

迁移是一次性导入，不建立双向同步。

导入通过管理员接口 `POST /api/aigc/models/import` 执行，Body 使用 `items` 数组，每项兼容旧表的
`model_id`、`display_name`、`model_type`、`config_json` 和 `enabled` 字段，也可直接传对象形式的
`config`。接口幂等地跳过已存在的 `public_model_id`，不会覆盖管理员已经编辑的 Profile。响应逐项
返回 `imported`、`skipped` 或 `failed`，以及 `INVALID_JSON`、`INVALID_CONFIG`、
`UPSTREAM_MODEL_NOT_FOUND`、`UPSTREAM_MODEL_NOT_AVAILABLE_FOR_GROUP`、`PRICING_MISSING` 等报告项。
旧文本配置的根级 `upstream_model_id` 在导入时转换为新的 `text` 配置信封；不合法 JSON 不落库，
能力不完整、上游不可用或缺少价格的合法 JSON 保存为草稿。

## 22. 安全要求

- AIGC 传入的媒体 URL 必须经过 SSRF 防护；优先允许 AIGC 自有对象存储域名。
- New API 下载上游结果和 AIGC 下载 New API 结果都要限制大小、类型、重定向和超时。
- 管理 API 不返回渠道 Key 或私有配置。
- Profile 中不能保存供应商凭证。
- `result_json` 不保存永久可访问的敏感 URL；必要时在用户响应时生成短时地址。
- 错误响应不暴露上游密钥、Header 或完整请求体。
- 普通用户只能查询自己的 `generation_id`。

## 23. 性能和容量

- 模型目录不逐条查询 Ability，必须批量加载并构建索引。
- 管理列表分页，关联的渠道数、组和价格批量填充。
- 生成查询按 `generation_id` 和 `user_id` 建复合索引。
- 后台轮询复用 New API 现有 Task 批量轮询，不为每个 AIGC 请求建立独立常驻 Goroutine。
- AIGC 查询 New API 时使用退避，并尊重 `Retry-After` 或建议轮询间隔。
- 媒体下载和上传使用流式 IO，不把完整视频加载到内存。

## 24. 关键风险与应对

| 风险 | 影响 | 应对 |
| --- | --- | --- |
| 新旧路径重复提交 | 重复生成和扣费 | Turn ID 幂等键、唯一索引、禁止影子执行 |
| 模型组权限不一致 | 越权或模型消失 | 始终以 Token 实际组为准，契约测试多组场景 |
| 图片内联结果不可重放 | 网络失败后无法恢复 | 图片切流 Gate，确认 URL、直返或短期存储方案 |
| 临时媒体 URL 过期 | Asset 转存失败 | 完成后立即转存，只重试下载不重跑生成 |
| 视频任务状态重复维护 | 状态漂移 | New API Task 为上游真相，AigcRequest 仅投影 |
| 配置更新影响运行中任务 | 请求行为漂移 | 保存配置版本和真实模型快照 |
| 管理员并发覆盖 | 配置丢失 | `config_version` 乐观锁 |
| New API 单目录形成耦合 | 后续难维护 | 明确包依赖，供应商差异留在 `relay/` |
| 多数据库迁移差异 | 部署失败 | TEXT JSON、GORM、三库集成测试 |

## 25. 实施任务拆分

### New API 后端

1. `aigc/entity` 和数据库迁移。
2. `aigc/capability` 四类强类型配置和验证。
3. `aigc/repository` Profile 与 Request 持久化。
4. `aigc/service/catalog` 用户组可见性和上游可用性。
5. 管理 API 和 Profile 导入工具。
6. 服务间模型目录 API。
7. Relay/Task 可复用执行入口。
8. 幂等生成服务和标准状态。
9. 文本、图片、视频、音乐 Execution Adapter。
10. 标准错误、日志和指标。

### New API 前端

1. 在 Classic `/console/models` 增加管理员专属 AIGC 模型 Tab。
2. 模型列表与筛选。
3. 公共模型基本信息编辑。
4. 模式映射编辑器。
5. 输入规则和输出规格编辑器。
6. 上游模型选择与可用性提示。
7. 校验、发布、停用和版本冲突处理。
8. Classic i18n、Prettier、ESLint 和构建验证。
9. 确认 `web/default` 没有任何改动。

### AIGC 后端

1. New API AIGC Client。
2. 用户 Token 获取和刷新接入。
3. 模型目录代理。
4. Job 增加执行来源和 Generation ID。
5. 文本、图片、视频、音乐逐类切换。
6. 结果转存和恢复逻辑统一。
7. 移除模型管理和 Provider 层。

### AIGC 前端

原则上无协议改动，只进行回归测试。若 New API 返回了 AIGC 当前目录协议尚未表达的新能力，由 AIGC BFF 做兼容转换，不能要求前端在同一迁移中同时切协议。

## 26. 验收标准

满足以下条件才认为迁移完成：

- AIGC 前端所有请求仍只指向 AIGC 后端。
- AIGC 前端核心接口和 SSE 行为无回归。
- AIGC 后端不存在供应商请求 URL、私有请求 DTO 或 Provider Factory。
- 公共模型、能力、模式和真实模型映射只在 New API 配置。
- New API 管理后台可以完成模型创建、校验、发布、停用和组分配。
- 相同 Turn ID 的重复提交不会重复生成或扣费。
- 用户只能看到和调用其 New API 用户组允许的模型和模式。
- 文本、图片、视频和音乐都经过 New API 现有计费及日志链路。
- AIGC 重启和 New API 重启后，进行中的异步任务可以恢复。
- 生成媒体最终由 AIGC 对象存储持久化，New API 不成为作品库。
- SQLite、MySQL、PostgreSQL 测试通过。
- New API Go 测试和 `web/classic` Build 通过。
- `web/default` 保持不变。
- AIGC Go 测试及前端回归通过。

## 27. 推荐的首个实施切片

第一批实现只做“配置面 + 目录面”，不碰生产生成：

1. 建立 `aigc/` 包骨架和两张表。
2. 搬迁现有四类模型能力结构和校验。
3. 导入现有 AIGC 模型配置为草稿。
4. 完成 New API AIGC 模型管理页面。
5. 发布 `/v1/aigc/models`。
6. 让 AIGC `/api/models` 在测试环境代理新目录。
7. 对比不同用户组的目录结果。

这个切片能够先验证最关键的数据归属和管理流程，同时不引入生成、计费或媒体转存风险。完成后再进入文本和图片执行迁移。
