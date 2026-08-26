## 1. 回归测试

- [x] 1.1 增加 OpenRouter 图片生成和图片编辑 URL 路由的失败测试
  - 验收：`RelayModeImagesGenerations` 和 `RelayModeImagesEdits` 最终 URL 都严格等于 OpenRouter `/api/v1/images`。
- [x] 1.2 增加 JSON 编辑引用转换、字段删除、字段冲突和数量限制的失败测试
  - 验收：AIGC 风格的多个 `images` 按原顺序转换成完整的 `input_references`，最终请求不包含 `images` 和 `response_format`。
  - 验收：显式 `n=1` 和 `n=10` 保留；`n=11` 返回 HTTP 400，并且不发送上游请求。
  - 验收：同时提供 `images` 和 `input_references` 时返回 HTTP 400，并且不发送上游请求。
- [x] 1.3 增加 multipart 图片转换和非 OpenRouter 兼容性的失败测试
  - 验收：multipart 中的每张图片都转换为 MIME 类型正确的 data URL，图片顺序保持不变，上游请求 Content-Type 为 `application/json`。
  - 验收：非 OpenRouter 的 JSON 和 multipart 图片编辑仍沿用原有 URL、字段和请求体格式。
- [x] 1.4 增加输入图片数量和输出图片数量的计费回归测试
  - 验收：协议转换前后的同一组输入图片只计费一次，不因同时统计 `images` 和 `input_references` 产生重复费用。
  - 验收：预扣费使用请求的 `n`，成功响应按实际 `data` 数量结算，上游失败执行完整退款。

## 2. 供应商适配器

- [x] 2.1 将 OpenRouter 两种图片 Relay 模式都路由到统一图片端点
  - 验收：OpenRouter 图片链路不再构造或请求 `/api/v1/images/generations` 和 `/api/v1/images/edits`。
- [x] 2.2 将 JSON 和 multipart 编辑输入转换为有序的 OpenRouter 输入引用
  - 验收：HTTP(S) URL 和 data URL 均被转换为 `type=image_url`、`image_url.url=<原值>` 的引用，且多图顺序不变。
  - 验收：调用方只提供原生 `input_references` 时，其内容和顺序保持不变。
- [x] 2.3 执行 OpenRouter 字段标准化和供应商专属参数校验
  - 验收：所有 OpenRouter 图片请求都删除 `response_format`，并清除已转换的 `images`。
  - 验收：冲突输入和 `n > 10` 使用 `invalid_request` 返回 HTTP 400，且错误不会被包装成上游 5xx。
  - 验收：OpenRouter multipart 图片编辑包含 `mask` 时返回 HTTP 400，且不发送上游请求。
  - 验收：全局 `dto.MaxImageN` 保持不变，其他供应商不受 OpenRouter 数量限制影响。
- [x] 2.4 确保必要的 OpenRouter 图片转换不会被原始请求体透传绕过
  - 验收：开启全局请求体透传或渠道请求体透传后，AIGC 生成的 OpenRouter 图片编辑请求仍执行路径和请求体转换。
  - 验收：上述透传场景具有独立回归测试，不依赖生产配置或外部服务。

## 3. 验证

- [x] 3.1 对修改文件执行格式化和 Go 诊断
  - 验收：所有修改的 Go 文件通过 `gofmt`，并且 `gopls check` 不报告新增诊断。
- [x] 3.2 运行 OpenRouter、OpenAI 图片 Relay 和 AIGC 执行链路的相关测试
  - 验收：`go test ./relay ./relay/channel/openai ./relay/helper ./aigc/execution` 全部通过。
  - 验收：测试不访问真实 OpenRouter、远程 New API 或其他外部服务。
- [x] 3.3 确认 OpenSpec apply 状态显示全部任务完成
  - 验收：`openspec validate fix-openrouter-image-edit-relay` 通过，`openspec instructions apply --change fix-openrouter-image-edit-relay --json` 显示所有任务完成且无剩余任务。
