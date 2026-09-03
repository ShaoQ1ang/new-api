# OpenRouter 视频模型 Handler 架构说明

本文档定义 `OpenRouter` 视频任务通道的推荐架构，目标是让同一个 channel 下支持多个不同视频模型族时，仍然保持：

- 入参转换清晰
- 结果转换清晰
- 计费提取清晰
- 新增模型族时尽量不改主流程

本文档是后续新增 `OpenRouter` 视频模型时的长期约定，不是一次性实现说明。

## 1. 设计目标

`OpenRouter` 作为聚合通道，一个 channel 下面可能同时出现多种视频模型族，例如：

- `openai/sora-*`
- `google/veo-*`
- `kling/*`
- `bytedance/seedance-*`
- `wan/*`

这些模型虽然都走同一个 `OpenRouter` 视频 API，但在以下方面通常存在差异：

- 请求字段命名不同
- 对图、视频、首尾帧、参考素材的约束不同
- 返回结果中可用 URL 或 usage 字段的位置不同
- 计费维度不同，例如：
  - 只看输出秒数
  - 同时看分辨率档位
  - 是否有声
  - 上游直接返回 provider usage 或 provider cost

因此，不应把所有模型族的逻辑堆进单个 adaptor 的 `switch model` 分支中。推荐做法是：

- 通道公共流程在 `TaskAdaptor`
- 模型族差异逻辑在 `Handler` 接口实现
- 通过工厂按模型名选择具体 handler

## 2. 总体分层

推荐目录结构：

```text
relay/channel/task/openrouter/
  adaptor.go
  factory.go
  types.go
  helpers.go
  seedance.go
  veo.go
  sora.go
```

当前分支采用的约束是：

- channel 内公共流程集中在 `adaptor.go`
- 工厂分发集中在 `factory.go`
- 通用接口、`BaseHandler`、通用响应转换集中在 `types.go`
- 通用 JSON 读取、状态映射、URL / usage / cost 提取集中在 `helpers.go`
- 每个模型族只保留一个实现文件，例如 `seedance.go`、`veo.go`

不再把单个模型族拆成 `normalize/request/billing` 多文件，避免新增模型族时横跳多个文件。

职责划分如下。

### 2.1 TaskAdaptor

`TaskAdaptor` 只负责 `OpenRouter` 通道的公共协议流程，不负责具体模型族差异。

它应当负责：

- 初始化 `ChannelType / baseURL / apiKey`
- 读取公共请求 `TaskSubmitReq`
- 通过工厂选择 handler
- 调用 handler 做校验、请求转换、结果解析、计费提取
- 发起 `POST /videos` 或 `POST /v1/videos`
- 发起 `GET /videos/{id}` 或 `GET /v1/videos/{id}`
- 处理通用鉴权头、超时、错误包装
- 返回统一的任务提交响应

它不应当负责：

- 按模型名写大量 `if / switch`
- 直接解析某个模型族的特殊 metadata
- 直接编写某个模型族的计费规则

### 2.2 Handler Factory

工厂只负责根据模型名返回合适的 handler。

推荐规则：

- 按模型前缀、命名空间或模型族特征匹配
- 优先匹配更具体的 handler
- 最后回退到默认 handler

工厂本身不应承担业务逻辑，只做分发。

### 2.3 Model Handler

每个模型族一个 handler，实现该模型族的差异化逻辑：

- 请求校验
- 入参转换
- 提交响应解析
- 查询响应解析
- OpenAI Video 统一输出转换
- 计费上下文提取
- 必要时的提交后或完成后计费修正

### 2.4 BaseHandler

`BaseHandler` 提供默认实现，减少每个模型族的重复代码。

适合下沉到 `BaseHandler` 的通用能力包括：

- 顶层字段读取
- duration / resolution / audio 的标准化
- 常见任务状态映射
- 常见 URL 提取
- 默认 usage 提取
- 默认 `ConvertToOpenAIVideo`
- 默认空实现：
  - `AdjustBillingOnSubmit`
  - `AdjustBillingOnComplete`

模型族 handler 只覆盖自己的差异部分。

## 3. 推荐接口

推荐为 `OpenRouter` 视频模型族定义统一接口：

```go
type ModelHandler interface {
    Match(model string) bool

    Validate(req *relaycommon.TaskSubmitReq) error

    BuildUpstreamRequest(
        info *relaycommon.RelayInfo,
        req *relaycommon.TaskSubmitReq,
    ) (any, error)

    EstimateBilling(
        info *relaycommon.RelayInfo,
        req *relaycommon.TaskSubmitReq,
    ) (*VideoBillingContext, error)

    ParseSubmitResponse(
        info *relaycommon.RelayInfo,
        body []byte,
    ) (*VideoSubmitResult, error)

    ParseFetchResponse(
        info *relaycommon.RelayInfo,
        body []byte,
    ) (*relaycommon.TaskInfo, error)

    ConvertToOpenAIVideo(task *model.Task) ([]byte, error)

    AdjustBillingOnSubmit(
        info *relaycommon.RelayInfo,
        body []byte,
    ) (*VideoBillingContext, error)

    AdjustBillingOnComplete(
        task *model.Task,
        result *relaycommon.TaskInfo,
    ) (*VideoBillingSettlement, error)
}
```

如果某个模型族没有提交后修正或完成后修正需求，可以由 `BaseHandler` 返回空值。

## 4. 请求转换分层

请求转换应拆成两层，而不是直接从 `TaskSubmitReq` 拼最终 JSON。

### 4.1 标准化层

先把公共请求归一成 channel 内部的标准视频请求结构，例如：

```go
type VideoNormalizedRequest struct {
    Model            string
    Prompt           string
    DurationSeconds  int
    ResolutionTier   string
    AspectRatio      string
    InputImages      []string
    InputVideos      []string
    FirstFrameURL    string
    LastFrameURL     string
    AudioEnabled     *bool
    ResponseFormat   string
    RawMetadata      map[string]any
}
```

这层的目标是统一语义，不关心上游最终字段名。

### 4.2 上游请求构造层

再由具体 handler 把标准化结构转换成该模型族所需的 OpenRouter 上游请求体。

例如：

- `SoraHandler` 负责 `sora` 族字段
- `VeoHandler` 负责 `veo` 族字段
- `KlingHandler` 负责 `kling` 族字段

这样做有两个好处：

- 计费提取可以复用标准化结果
- 以后改 OpenRouter 请求协议时，不需要重写计费层

## 5. 计费架构

计费应当和请求转换一样，分成结构化层，而不是把逻辑散落在 adaptor 或控制器里。

### 5.1 Billing Context

推荐定义统一的计费上下文：

```go
type VideoBillingContext struct {
    DurationSeconds int
    ResolutionTier  string
    AudioEnabled    *bool
    OtherRatios     map[string]float64

    ProviderUsage   map[string]any
    ProviderCostUSD *float64
}
```

它的职责是：

- 提供平台结算所需的标准字段
- 保留上游 usage / cost 观察信息
- 为后续 `provider_preferred` 结算模式留扩展口

### 5.2 EstimateBilling

`EstimateBilling` 只负责从请求里提取“预扣费所需信息”，不直接操作 HTTP，不直接写响应。

推荐做法：

- 从标准化请求中提取：
  - `DurationSeconds`
  - `ResolutionTier`
  - `AudioEnabled`
- 如果当前模型使用 `video_seconds` 计费：
  - 尽量返回标准化计费参数
- 如果当前模型仍使用 `base price * ratios`：
  - 返回 `OtherRatios`

### 5.3 AdjustBillingOnSubmit

如果 OpenRouter 的提交响应里已经包含更准确的 usage 或价格信息，则可在这里修正预扣费。

适合在这一层修正的内容：

- 提交响应里直接带 `seconds`
- 提交响应里直接带 `resolution`
- 提交响应里直接带 `estimated_cost`

### 5.4 AdjustBillingOnComplete

如果只有任务完成后才能拿到最终 usage 或 provider cost，则在轮询完成阶段修正。

适合在这一层处理的内容：

- 上游最终 usage 与预估时长不一致
- 上游最终分辨率回落或提升
- 上游返回最终 provider cost

### 5.5 平台价格与上游价格分离

推荐明确区分两个概念：

- 平台价格口径
  - 当前系统用于 quota 计算和用户扣费的价格
- 上游价格口径
  - OpenRouter 返回的 usage / estimated cost / final cost

不要把它们混在同一个临时 map 里。

推荐策略：

- 第一阶段先按平台价格体系结算
- 同时保留 OpenRouter usage / cost 到 task data 或 usage metadata
- 后续如果要支持 `provider_preferred` 再新增策略层，而不是重写 handler

## 6. 响应转换分层

响应转换也应当分层处理。

### 6.1 提交响应

`ParseSubmitResponse` 负责把 OpenRouter 提交响应解析成系统内部的提交结果：

- 上游 task id
- 初始状态
- 初始 progress
- polling url
- 可选结果 url
- 可选的 provider usage
- 可选的 provider cost

当前约定：

- 公共字段尽量由 `BaseHandler` 解析
- 常见 `usage` 保留到 `OpenAIVideo.metadata.usage`
- 常见 provider 成本保留到 `OpenAIVideo.metadata.provider_cost_usd`
- 只有模型族确实返回特殊结构时，才在具体 handler 覆盖

### 6.2 查询响应

`ParseFetchResponse` 负责把 OpenRouter 查询响应解析成统一 `TaskInfo`：

- 任务状态
- 视频 URL
- 失败原因
- 可选 usage / cost
- 可选 token 计费字段

当前约定：

- `BaseHandler` 负责提取常见 `status / error / output / usage`
- 若上游响应里有 `video_tokens / total_tokens / output_tokens / completion_tokens`，优先归一到 `TaskInfo.CompletionTokens` 和 `TaskInfo.TotalTokens`
- 如果未来某个模型族要按更特殊的 usage 字段结算，再由该 handler 覆盖
- 当前 `OpenRouter` adaptor 已支持在任务完成时显式重算最终 quota：
  - 如果任务使用 `video_seconds` 计费，优先按轮询返回的实际 `duration/seconds` 结合 `video_seconds_unit_price` 重算
  - 否则优先使用 `ConditionalInputPrice`
  - 再回退到 `ModelRatio * GroupRatio * OtherRatios * TotalTokens`

### 6.3 OpenAI Video 输出

`ConvertToOpenAIVideo` 负责把落库后的 task data 转成统一的 `/v1/videos/{task_id}` 输出。

这里建议：

- 通用字段由 `BaseHandler` 负责
- 模型族特有字段写入 `metadata`
- 不要把模型族专用字段塞进公共顶层字段

当前约定：

- `BaseHandler` 默认补齐 `task_id`、状态、progress、created/completed 时间
- 常见 `url / polling_url / usage / provider_cost_usd / error` 统一落到公开返回
- 只有模型族响应体与通用结构明显不一致时，才单独覆盖

## 7. 新增 OpenRouter 视频模型的接入步骤

以后新增一个 OpenRouter 视频模型族时，推荐按以下步骤执行。

1. 确认是否属于现有模型族。
2. 如果不是，新增一个新的 handler 文件。
3. 在 handler 内实现：
   - `Match`
   - `Validate`
   - `BuildUpstreamRequest`
   - `EstimateBilling`
   - 如无特殊需要，直接复用 `BaseHandler` 的 `ParseSubmitResponse`
   - 如无特殊需要，直接复用 `BaseHandler` 的 `ParseFetchResponse`
   - 如无特殊需要，直接复用 `BaseHandler` 的 `ConvertToOpenAIVideo`
   - 只有响应结构明显不同才覆盖
4. 在工厂里注册新 handler。
5. 补充该模型族的测试：
   - 请求转换
   - 查询结果解析
   - OpenAI Video 输出
   - 计费参数提取
6. 如果有新的计费维度：
   - 优先扩展 `VideoBillingContext`
   - 不要在 controller 或主 adaptor 中单独硬编码
7. 更新相关开发文档。

## 8. 禁止事项

为了保证后续模型扩展可维护，禁止以下做法：

- 在 `OpenRouter TaskAdaptor` 中直接按模型名堆大量 `if / else`
- 把某个模型族的 metadata 解析逻辑写进公共控制器
- 把计费规则散落到提交、轮询、日志显示等多个位置重复实现
- 用匿名 `map[string]any` 到处透传计费核心字段，而不定义结构化上下文
- 在新增模型族时修改其他 handler 的特化逻辑，除非确实要抽公共能力到 `BaseHandler`

## 9. 与现有 Ali 视频架构的关系

现有 `Ali` 视频 adaptor 已经体现出部分相同方向：

- 请求转换与计费估算分离
- 不同模型族在同一 channel 下走不同构造逻辑
- 主流程仍由统一 task 提交流程编排

但 `OpenRouter` 这次建议更进一步：

- 正式引入 handler 工厂
- 把模型族差异上升为显式接口
- 把计费上下文从临时 ratio 拼装提升为结构化对象

这样后续视频模型继续增加时，主流程仍能保持稳定。

## 10. 实施建议

落地实现时，建议先按以下顺序推进：

1. 先搭 `openrouter` 目录骨架
2. 先实现 `factory + base handler + default types`
3. 先接入一个首个模型族作为模板
4. 再接入其他模型族
5. 最后再决定是否启用 provider usage / provider cost 作为正式结算依据

如果实现过程中发现多个视频 channel 都需要相同的模型族 handler 分层，可在后续把这套模式进一步抽象为跨 channel 的通用视频模型 handler 框架；但第一阶段应优先保证 `OpenRouter` 自身边界清晰。

## 11. 当前已接入模型族

截至 `2026-07-10`，当前分支已先接入两个 `OpenRouter` 视频模型族：

- `bytedance/seedance-*`
- `google/veo-*`

### 11.1 Seedance

当前实现已经覆盖：

- `text-to-video`
- `first_frame`
- `first_frame + last_frame`
- 图片参考输入
- 视频参考输入
- 音频参考输入（通过显式 `input_references` 或 metadata 参考字段）
- `generate_audio`
- `resolution`
- `aspect_ratio`
- passthrough: `watermark`、`req_key`

当前按已抓取的 `OpenRouter /videos/models` 元数据对齐：

- durations: `4-15`
- resolutions: `480p`、`720p`、`1080p`、`4K`
- aspect ratios: `1:1`、`3:4`、`9:16`、`4:3`、`16:9`、`21:9`、`9:21`
- frame images: `first_frame`、`last_frame`

### 11.2 Veo

当前实现已经覆盖：

- `text-to-video`
- `first_frame`
- `first_frame + last_frame`
- 图片参考输入
- `generate_audio`
- `resolution`
- `aspect_ratio`

当前按已抓取的 `OpenRouter /videos/models` 元数据对齐：

- durations: `4`、`6`、`8`
- resolutions: `720p`、`1080p`
- aspect ratios: `16:9`、`9:16`

当前实现对 `Veo` 做了更严格的能力约束：

- 非 `4/6/8` 的 duration 会归一到最近支持值
- 非 `720p/1080p` 的 resolution 会直接报错
- 非 `16:9/9:16` 的 aspect ratio 会直接报错

### 11.3 后续新增模型族要求

后续新增 `OpenRouter` 视频模型族时，至少要补：

- handler 匹配测试
- request 映射测试
- billing 标准化测试
- submit response 解析测试
- fetch response 解析测试
- `ConvertToOpenAIVideo` 测试
- 必要的能力约束测试
- 开发文档中的“当前已接入模型族”更新
