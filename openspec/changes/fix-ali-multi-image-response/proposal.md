## Why

阿里云百炼的同步图片模型可能在单个 `choice.message.content` 中返回多张图片。当前转换器按 choice 只生成一个 OpenAI 图片项，后续图片会覆盖前面的图片，导致客户端请求并支付多张图片时只能收到最后一张。

## What Changes

- 将阿里同步图片响应中的每个图片内容项独立转换为 OpenAI `ImageData`。
- 保留同一 choice 中的文本修订提示，并应用到该 choice 生成的每张图片。
- 忽略不包含图片的 choice，避免生成空图片项。
- 增加覆盖单 choice 多图、多 choice、文本顺序和空内容的回归测试。

## Capabilities

### New Capabilities
- `ali-multi-image-response`: 规定阿里同步图片响应到 OpenAI 图片数组的一对一转换行为。

### Modified Capabilities

无。

## Impact

- 影响 `relay/channel/ali` 的同步图片响应转换。
- 影响通过阿里渠道调用 `qwen-image` 等同步图片模型的 `/v1/images/generations` 和 `/v1/images/edits` 响应。
- 不改变请求格式、路由、模型能力配置、计费算法或外部依赖。
