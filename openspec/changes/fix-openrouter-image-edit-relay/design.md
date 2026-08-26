## 背景

New API 使用通用的 OpenAI 兼容语义表示 AIGC 图片编辑：请求路径为 `/v1/images/edits`，图片输入保存在有序的 `images` 字段中。渠道选择发生在请求标准化和预扣费之后，因此 AIGC 层无法安全地产生供应商专属请求。

OpenRouter 的协议不同：图片生成和图片编辑共用 `/v1/images`，编辑图片通过 `input_references` 表示。当前 OpenAI 兼容适配器只改写图片生成 URL；JSON 编辑请求会原样通过，multipart 编辑请求则仍会被重新编码成 multipart，这两种情况都不符合 OpenRouter 的统一图片接口。

## 目标与非目标

**目标：**

- 保持 AIGC 和 New API 对外图片接口的供应商无关性。
- 为 JSON 和 multipart 图片编辑生成有效的 OpenRouter 请求。
- 保持图片顺序以及 `n` 等受支持的显式可选值。
- 拒绝有歧义的图片引用和大于 10 的 OpenRouter 图片数量。
- 保持非 OpenRouter 渠道的现有行为。

**非目标：**

- 不修改 AIGC 模式解析或其通用图片编辑表示。
- 不增加 OpenRouter 官方图片接口之外的功能。
- 不修改其他供应商共用的全局图片数量安全上限。
- 本次不调整图片响应的流式兼容转换。

## 技术决策

### 在供应商适配器中完成转换

OpenRouter 的统一 URL 和请求结构由 OpenAI 兼容适配器负责。该层是第一个明确知道最终渠道的边界，可以避免 OpenRouter 专属语法进入 AIGC 和通用 Relay。

不选择修改 `BuildSyncRequest` 直接生成 `input_references`，因为相同 AIGC 请求可能被调度到其他渠道，这会导致其他渠道收到无效请求。

### 使用现有 ImageRequest DTO 完成标准化

对于 OpenRouter 图片请求，适配器返回标准化后的 `ImageRequest`：填充 `InputReferences`，清空 `Images` 和 `ResponseFormat`。

JSON `images` 会被解析并映射为强类型的 OpenRouter 引用。multipart 图片文件从已经解析的可复用表单中读取，编码成带 MIME 类型的 data URL，然后使用相同的引用结构。

采用小型强类型引用结构，而不是直接拼接 JSON 字符串，这样可以显式校验字段结构，并继续使用项目的 JSON 包装函数。

### 拒绝冲突的输入表示

如果 OpenRouter 图片编辑请求同时包含 `images` 和 `input_references`，返回客户端请求错误。这里不做自动合并，因为无法可靠判断重复图片，而且通用 token 元数据会同时统计两个集合。

### 在转换阶段应用供应商数量限制

通用校验继续保留 `dto.MaxImageN`，用于所有供应商的计费安全。OpenRouter 适配器额外拒绝 `n > 10`，有效的显式指针值保持不变。

该限制不能放在渠道选择之前，否则会错误限制支持更多图片的其他供应商。

### 必要协议转换优先于请求体透传

当前原始请求体透传分支会完全绕过供应商适配器。当 OpenRouter 图片请求需要通用编辑协议转换时，即使启用了请求体透传，也必须执行 OpenRouter 转换。已经使用原生 `input_references` 的请求经过标准化后仍保持有效。

## 风险与权衡

- **multipart 编码会增加内存占用**：文件需要在内存中转换为 base64 data URL。继续使用现有可复用 multipart 请求体的大小限制，每个文件只读取一次。
- **OpenRouter 图片透传语义变窄**：为保证内部生成的 AIGC 请求不会绕过修复，必要协议转换优先于原始请求体透传。
- **供应商校验发生在渠道选择和预扣费之后**：转换错误走现有工作流退款路径，不会遗留费用。
- **OpenRouter 未来可能扩展引用结构**：调用方只提供 `input_references` 时保持原内容不变，因此可以兼容供应商后续增加的原生字段。

## 部署与回滚

直接部署适配器和测试，不需要数据迁移。已有图片生成请求仍然使用相同的上游端点。回滚只需撤销适配器变更，不涉及持久化数据。

## 待确认问题

无。
