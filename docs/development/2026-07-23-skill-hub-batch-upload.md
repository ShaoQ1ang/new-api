# Skill Hub 后台批量上传

## 变更范围

- default 与 classic 技能管理页新增基于本地文件夹的批量上传入口。
- 文件夹格式复用 `scripts/skill-hub-batch-upload` 的 `manifest.json` / `manifest.jsonl`、`packages/`、`icons/`、`testcases/` 约定。
- 新增批量初始化、批量提交和批量丢弃三个管理接口，避免逐文件请求产生大量使用日志。
- 新增统一草稿/发布、推荐、强制排序，以及冲突、并发、验证、标签、来源和缺失资源策略。

## 请求流程

1. 浏览器在本地解析目录并完成路径、格式、大小和引用完整性校验。
2. `POST /api/admin/skill-hub/batch-upload/init` 先对整批冲突、统一配置和 Skill 元数据完成服务端预检；全部预检结束后，一次返回所有可处理项的临时 OSS PUT signed URL。
3. 浏览器以 1 至 10 的有限并发直接上传 Zip 和图标到 OSS。
4. `POST /api/admin/skill-hub/batch-upload/commit` 批量完成 OSS 校验、对象转正和数据库保存；前端按约 8 MB 对提交项分片。
5. 未进入提交或确定失败的临时上传通过 `POST /api/admin/skill-hub/batch-upload/discard` 批量清理。

批量提交返回逐项状态，允许部分成功。提交响应不明确时，前端使用 `unknown` 本地状态，不自动重试或清理票据，避免服务端已提交后发生重复操作。

## 安全与并发

- 三个接口均沿用 `UserAuth` 和 `skill_hub.content.manage` 权限校验。
- 初始化与提交请求体限制为 32 MB，丢弃请求体限制为 2 MB；单批最多 200 个技能、400 张清理票据。
- 客户端拒绝不安全路径、URL 引用、重复路径和重复 Skill ID；服务端再次校验 Skill ID、索引、上传票据唯一性、票据与 Skill ID/资源类型绑定关系。
- 初始化接口在签发任何上传地址前先完成全部条目的元数据、标签、评测、案例、冲突和统一覆盖配置预检；提交接口会在读取并哈希 OSS 对象前再次预检，避免明显无效的条目消耗大文件 I/O。
- OSS 完成校验固定使用 2 个 worker；每个 worker 只写独立结果槽。对象校验结束后，数据库保存与旧对象清理按请求项串行执行。
- 两个前端除禁用按钮外还使用同步执行锁，阻止快速双击在 React 状态刷新前并发启动重复批次。
- OSS 转正使用禁止覆盖的复制操作，数据库使用 Skill ID 唯一约束；并发请求不能静默覆盖同一正式对象或创建重复 Skill。
- Zip 与图标仍执行服务端大小、哈希、文件头和受管前缀校验；Zip 内 `SKILL.md` 继续执行路径穿越、符号链接、层级、编码和大小限制。
- 临时对象清理是 best-effort，部署端仍需为 Zip 与图标 `_tmp/` 前缀配置生命周期规则。

## 前端一致性

两个前端实现相同的目录格式、默认值、覆盖策略、有限并发、取消、失败重试和 JSON 报告下载。选择目录后会明确显示本地校验结果，开始上传后先显示服务端“校验中”状态。default 前端使用共享解析模块的六语言 i18n；classic 沿用该页面当前的中文管理界面。错误处理统一使用下拉框选择“继续处理其他 Skill”或“首个上传错误后停止”。Web 端不再显示统一来源配置，并在初始化和提交参数中固定关闭来源覆盖，使每个 Skill 保留 manifest 自己的来源；服务端仍保留原请求字段以兼容已有调用方。

## 验证

- Docker 的 default/classic 前端构建阶段保留 `web/{default,classic}` 与 `web/shared` 的相对目录结构，确保两个前端都能解析共享的 ZIP 解析模块。
- 本地 Compose 完整构建与启动：`docker compose -f docker-compose.local.yml up -d --build`

- Go：`GOENV=off go test ./controller ./model ./service`
- 共享目录解析：`node --test web/shared/skill-hub-batch-import.test.mjs`
- 脚本兼容：`node --test scripts/skill-hub-batch-upload/upload.test.js`
- default：`bun run build`、`bun run i18n:sync`
- classic：`bun run build`
