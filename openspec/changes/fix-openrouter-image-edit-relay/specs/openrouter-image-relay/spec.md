## ADDED Requirements

### Requirement: 使用 OpenRouter 统一图片端点
New API MUST（必须）将 OpenRouter 图片生成和图片编辑两种 Relay 模式发送到供应商的 `/v1/images` 端点。

#### Scenario: 路由图片生成请求
- **WHEN** 图片生成请求选择了 OpenRouter 渠道
- **THEN** New API 将请求发送到 OpenRouter `/v1/images` 端点

#### Scenario: 路由图片编辑请求
- **WHEN** 图片编辑请求选择了 OpenRouter 渠道
- **THEN** New API 将请求发送到 OpenRouter `/v1/images` 端点

### Requirement: 将通用编辑图片转换为 OpenRouter 引用
New API MUST（必须）在发送上游请求前，将有序的通用图片编辑输入转换成有序的 OpenRouter `input_references`。

#### Scenario: 转换 URL 图片输入
- **WHEN** OpenRouter 图片编辑请求包含带 URL 的通用 `images` 对象
- **THEN** New API 发送等价且顺序不变的 `image_url` 输入引用，并删除 `images`

#### Scenario: 转换 multipart 图片输入
- **WHEN** OpenRouter 图片编辑请求包含上传的图片文件
- **THEN** New API 以 JSON 发送等价且顺序不变的、带 MIME 类型的 data URL 输入引用

#### Scenario: 保留原生输入引用
- **WHEN** OpenRouter 图片请求已经包含 `input_references`，并且不包含 `images`
- **THEN** New API 保持这些引用的顺序和内容不变

### Requirement: 删除 OpenRouter 不支持的图片字段
New API MUST（必须）从 OpenRouter 图片请求中删除 `response_format`，同时保留供应商支持的显式可选值。

#### Scenario: 删除响应格式
- **WHEN** OpenRouter 图片请求包含 `response_format`
- **THEN** New API 发送上游请求时不包含 `response_format`

#### Scenario: 保留有效图片数量
- **WHEN** OpenRouter 图片请求显式设置 1 到 10 之间的 `n`
- **THEN** New API 在上游请求中保留该显式数量

### Requirement: 拒绝有歧义或无效的 OpenRouter 图片请求
当输入表示冲突或请求图片数量超过供应商限制时，New API MUST（必须）在发送上游请求前拒绝该 OpenRouter 图片请求。

#### Scenario: 图片输入发生冲突
- **WHEN** OpenRouter 图片请求同时包含 `images` 和 `input_references`
- **THEN** New API 返回客户端请求错误，并且不发送上游请求

#### Scenario: 图片数量超过供应商限制
- **WHEN** OpenRouter 图片请求显式设置的 `n` 大于 10
- **THEN** New API 返回客户端请求错误，并且不发送上游请求

#### Scenario: 图片编辑包含不支持的 mask
- **WHEN** OpenRouter multipart 图片编辑请求包含 `mask` 文件
- **THEN** New API 返回客户端请求错误，并且不发送上游请求

### Requirement: 保持其他渠道行为不变
New API MUST（必须）只对 OpenRouter 渠道应用 OpenRouter 图片标准化。

#### Scenario: 通过其他 OpenAI 兼容渠道编辑图片
- **WHEN** 图片编辑请求选择了非 OpenRouter 的 OpenAI 兼容渠道
- **THEN** New API 保持该渠道已有的图片编辑 URL 和请求转换行为
