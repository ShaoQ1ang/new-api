## 为什么要改

OpenRouter 使用统一的 `/api/v1/images` 端点处理图片生成和图片编辑，但当前适配器只改写了图片生成请求。AIGC 图片编辑请求因此会被转发到不存在的 `/api/v1/images/edits`，最终收到上游 404。

## 变更内容

- 将 OpenRouter 图片生成和图片编辑两种 Relay 模式都路由到供应商统一的 `/v1/images` 端点。
- 将通用 JSON 图片编辑请求中的 `images` 转换成 OpenRouter 的 `input_references`，并保持输入顺序。
- 转换后删除 OpenRouter 不支持的请求字段。
- 在请求发送到上游之前，拒绝有歧义或不符合 OpenRouter 限制的图片请求。
- 增加 URL 路由、请求转换、参数校验以及非 OpenRouter 行为保持不变的回归测试。

## 能力范围

### 新增能力

- `openrouter-image-relay`：定义 OpenRouter 图片端点路由、图片编辑引用转换以及供应商参数校验。

### 修改已有能力

无。

## 影响范围

- 影响 `relay/channel/openai/` 中兼容 OpenAI 协议的 OpenRouter 适配器及其测试。
- 保持现有 AIGC 图片编辑 Relay 合同不变。
- 不增加依赖，不修改 New API 对外暴露的接口路径。
