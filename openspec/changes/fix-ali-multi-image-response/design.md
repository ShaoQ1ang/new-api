## Context

阿里同步图片响应使用 `output.choices[].message.content[]` 承载文本和图片。`qwen-image-2.0` 在 `n > 1` 时可以把多张图片放入同一个 choice。现有转换器在每个 choice 内复用一个 `ImageData`，因此后出现的图片会覆盖先出现的图片，而 AIGC 层只能收到一个 output。

## Goals / Non-Goals

**Goals:**

- 保证每个阿里图片内容项对应一个 OpenAI 图片数据项。
- 保持 choice 和 content 中图片的原始顺序。
- 让同一 choice 的修订提示对内容顺序不敏感，并附加到该 choice 的每张图片。
- 不返回没有 URL 或 Base64 内容的空图片项。

**Non-Goals:**

- 不修改请求侧 `n`、模型能力或计费逻辑。
- 不修改异步 `results` 响应的转换。
- 不改变图片下载或 Base64 转换失败时跳过该图片的现有行为。

## Decisions

1. 对每个 choice 先扫描文本，再按原顺序扫描图片。这样文本出现在图片之前或之后时都能作为该 choice 的 `revised_prompt`，同时避免为了处理顺序而回写已经追加的结果。
2. 每遇到一个有效图片内容项就构造并追加一个新的 `dto.ImageData`。不复用 choice 级图片对象，从根本上避免覆盖。
3. choice 中有多个文本项时沿用现有语义，使用最后一个非空文本作为修订提示。文本项本身不生成独立图片结果。
4. 测试直接构造 `AliOutput` 并验证公开响应转换结果，覆盖用户可观察的图片数量、顺序、内容和修订提示，不锁定内部循环实现。

## Risks / Trade-offs

- [同一 choice 的修订提示被复制到多张图片] → 这是 OpenAI 图片数组中表达 choice 级文本的唯一无损方式，并保持每个结果自包含。
- [URL 转 Base64 下载失败导致结果数量少于上游图片数] → 维持现有跳过失败图片行为，本变更不扩大错误处理范围。
- [阿里未来改变响应结构] → DTO 解码和回归测试将继续约束当前 `choices[].message.content[]` 契约。

## Migration Plan

无需数据迁移。发布新镜像后转换行为立即生效；回滚到旧镜像即可恢复原行为。

## Open Questions

无。
