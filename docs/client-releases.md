# Client Releases

客户端安装包管理用于给桌面端提供版本列表、更新检查、`latest.yml` / `latest-mac.yml` 和安装包下载入口。安装包保存在私有 OSS，客户端只访问 New API；下载时 New API 生成短时签名 URL 后 302 跳转。

平台统一使用 `windows`、`macos`、`linux`。历史 mac 平台值会在数据库迁移时统一为 `macos`；新接口、管理端和文档都使用 `macos`。

## OSS 配置

```env
CLIENT_RELEASE_OSS_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
CLIENT_RELEASE_OSS_BUCKET=your-private-bucket
CLIENT_RELEASE_OSS_ACCESS_KEY_ID=xxx
CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET=xxx
CLIENT_RELEASE_OSS_PREFIX=client-releases
CLIENT_RELEASE_OSS_SIGNED_URL_EXPIRES_SECONDS=600
CLIENT_RELEASE_OSS_UPLOAD_URL_EXPIRES_SECONDS=3600
CLIENT_RELEASE_OSS_UPLOAD_TICKET_SECRET=optional-random-secret
CLIENT_RELEASE_OSS_MAX_BYTES=524288000
```

- `CLIENT_RELEASE_OSS_PREFIX` 为空时默认 `client-releases`。
- 直传对象会先写入 `CLIENT_RELEASE_OSS_PREFIX/_tmp/`；保存版本记录时后端再通过 OSS `CopyObject` 转入正式目录。
- `CLIENT_RELEASE_OSS_SIGNED_URL_EXPIRES_SECONDS` 为空时默认 `600` 秒，最大不超过 `86400` 秒。
- `CLIENT_RELEASE_OSS_UPLOAD_URL_EXPIRES_SECONDS` 为空时默认 `3600` 秒，最大不超过 `86400` 秒；这是后台直传 OSS 的 PUT signed URL 与上传票据有效期。
- `CLIENT_RELEASE_OSS_UPLOAD_TICKET_SECRET` 可选；为空时使用 OSS AccessKeySecret 对上传票据签名。
- `CLIENT_RELEASE_OSS_MAX_BYTES` 为空时默认 `500MB`。
- 生产环境必须把系统 `ServerAddress` 配成固定的 HTTPS 外部地址，管理端返回的下载、复制链接和更新清单链接都以它为基准。未配置时才使用当前请求的 `Host`；服务端忽略 `X-Forwarded-Host`，并且只接受 `http/https` 的 `X-Forwarded-Proto`。

## 上传与发布

- 管理后台使用 OSS 直传：先调用 New API 初始化上传，拿到短时 PUT signed URL 后由浏览器直接上传到 `CLIENT_RELEASE_OSS_PREFIX/_tmp/`，再调用 New API 完成确认。
- 初始化接口会先校验 `version / platform / arch / channel`、文件扩展名和文件大小，非法目标不会生成 signed URL。
- 文件名由后端强制生成：`Z-UP-Setup-{version}-{platform}-{arch}-{channel}.{ext}`，前端不允许编辑上传名。
- 文件类型按平台隔离：Windows 安装包支持 `exe/msi/zip`，macos 人工下载只支持 `dmg`、自动更新只支持 `zip`，Linux 支持 `AppImage/deb/rpm/zip`。不再接受与目标平台无关的扩展名。
- 管理端必须先选定平台工作区，再填写版本号并选择架构、通道，上传控件才会启用。版本、架构或通道变化会清空尚未保存的上传结果，避免把旧目标的对象误用到新目标。
- macos 的一条版本记录包含两个独立资产：`fileName/objectKey/size/sha*` 保存供用户手动下载的 DMG，`updaterFileName/updaterObjectKey/updaterSize/updaterSha*` 保存 `electron-updater` 使用的 ZIP。两者必须来自同一份已签名、公证的构建产物。
- macos 草稿允许暂时只保存 DMG；发布前必须同时存在 DMG 和 ZIP，且 ZIP 必须有 `updaterSha512`。其他平台不接受 updater 资产字段。
- macos 的 DMG 与 ZIP 分别执行一次“初始化 -> PUT -> 完成确认”，再把两组完成确认结果一起提交到创建或更新接口。
- 上传完成确认时，后端会从 OSS 校验对象大小并重新计算 `sha256` 和 `sha512`；保存转正时还会按 OSS ETag 条件重新读取并计算哈希，再用同一 ETag 条件复制，避免完成确认后临时对象被覆盖造成“文件与哈希不一致”。已发布版本必须带对应 feed 资产的 `sha512`，否则 `electron-updater` 不能校验更新包。
- 初始化、PUT 和完成确认不会写数据库；只有保存客户端版本记录后，后端才会把 `_tmp/` 对象复制到正式目录并写入数据库。
- 如果上传后没有保存，前端会在替换上传、切换记录、新建草稿或离开页面时 best-effort 调用 `POST /api/admin/client-releases/direct-upload/discard` 删除刚上传的 OSS 对象。
- 上传票据会签入当前管理员用户 ID；`complete` 和 `discard` 只接受原始发起人使用该票据，避免同一后台内的其他账号复用或删除上传对象。
- 如果编辑已有版本并上传新的安装包，保存成功后后端会 best-effort 删除 `_tmp/` 对象和被替换的旧安装包 OSS 对象；macos 的 DMG 与 ZIP 分别处理。
- 删除客户端版本记录成功后，后端会 best-effort 删除关联的全部安装包 OSS 对象。
- 浏览器崩溃、断网或直接关闭标签页时，前端无法保证一定发出 discard。生产环境应给 `CLIENT_RELEASE_OSS_PREFIX/_tmp/` 配置 OSS 生命周期规则，例如最后修改时间 3 天后删除；正式对象不在 `_tmp/` 下，不会被该规则清理。
- OSS Bucket 需要配置 CORS，允许管理后台域名执行 `PUT` 和 `OPTIONS`，允许 `Content-Type` 请求头，并暴露 `ETag`。

## 版本选择

- 后台列表按 `id DESC` 返回。
- 公开列表、`latest`、`latest.yml` 和 `latest-mac.yml` 只读取已发布版本。
- 同一 `platform / arch / channel` 范围内，`latest` 和 feed 按 `id` 最大的已发布记录作为最新记录。
- Windows/Linux 使用 `latest.yml`，其 `path/sha512/size` 来自主安装包字段；macos 使用 `latest-mac.yml`，其 `path/sha512/size` 只来自 updater ZIP 字段。DMG 不会写入 `latest-mac.yml`。
- 客户端仍会比较 `latest.version` 和自己的当前版本；如果误发布了版本号更低但 `id` 更大的记录，客户端不会把它当作升级版本。

## 升级与历史数据

- 服务启动迁移会把历史 mac 平台值统一为 `macos`。如果迁移发现同一 `version/arch/channel` 同时存在两条会归一化到同一目标的记录，会拒绝启动并报告冲突 ID，不会自动删除或覆盖版本。
- 历史 macos 已发布记录可能只有一个资产。升级服务后应先取消发布该记录，上传同一签名、公证构建的 DMG 与 ZIP，保存后再重新发布；资产不完整时 `latest-mac.yml` 会明确返回错误，不会把 DMG 冒充自动更新 ZIP。
- 数据库新增的 updater 字段由自动迁移创建；部署前仍应按项目惯例备份数据库，并先在与生产相同数据库类型的环境验证迁移。

## 桌面端更新流程

- 客户端先请求 `GET /api/client-releases/latest` 判断是否有新版本、是否强制更新，以及弹窗展示的版本信息。
- 用户点击“下载更新”后，客户端不会打开浏览器外链，而是交给 `electron-updater` 读取对应 feed。Windows/Linux 读取 `latest.yml`；macos 读取 `latest-mac.yml` 并下载 ZIP。
- macos 页面和公开 `downloadUrl` 继续指向 DMG，便于用户从网页手动下载；自动更新路径与人工下载路径互不混用。
- 下载过程中客户端显示进度；下载完成后按钮切换为“更新并重启程序”。
- 用户点击“更新并重启程序”后，客户端隐藏旧窗口并调用安装器静默安装；安装完成后自动启动新版本。
- 强制更新不会提供“稍后”，但仍然按“下载更新 -> 更新并重启程序”的两段式流程执行，用户也可以直接退出程序。

## 安全边界

- 管理接口都挂在 `/api/admin/client-releases` 下，并要求登录。管理员/超级管理员隐式拥有全部能力；普通用户需要细粒度权限：列表和详情接受 `client_releases.manage` 或 `client_releases.publish`，创建、编辑、上传和删除要求 `client_releases.manage`，发布和取消发布要求 `client_releases.publish`。
- 管理端查询和按 ID 读取、编辑、删除、发布、取消发布都要求显式携带 `platform=windows|macos|linux`。后端会把 ID 与平台一起校验；跨平台 ID 会按不存在/不属于当前平台处理，不能借由已知 ID 操作其他平台工作区。
- 创建和编辑接口不会接受请求体中的发布状态，新增记录始终为草稿，编辑时在行锁内保留数据库中的真实状态。发布状态只能通过独立的 `publish` / `unpublish` 接口修改，避免编辑权限绕过发布权限。
- 仅有 `client_releases.manage` 的用户可以维护草稿。修改或删除已发布记录还需要 `client_releases.publish`，检查在锁定记录后执行，避免与并发发布请求竞态。
- OSS Bucket 应保持私有。客户端只访问 New API，New API 再为已发布版本生成短时 signed URL；不要把 OSS Bucket 改成公开读。
- 直传初始化会限制文件大小，默认最大 `500MB`；文件扩展名、`version / platform / arch / channel` 会在生成 signed URL 前校验。
- signed URL 只允许上传到后端生成的单个 `_tmp/` OSS Object，前端不会拿到 OSS AccessKey；完成确认必须携带后端签发的短时上传票据。
- 上传后的文件名由后端强制生成：`Z-UP-Setup-{version}-{platform}-{arch}-{channel}.{ext}`，保存时还会校验临时上传票据的目标与当前 `version/platform/arch/channel` 一致，避免把另一个目标的上传复用到当前记录。
- 创建或替换任何资产时都必须使用刚完成直传的 `_tmp/` 对象；仅提供一个看似位于受管前缀下的正式 Object Key 不会被接受，避免管理员请求将版本重定向到任意 OSS 对象。
- `_tmp/` Object Key 的文件名必须与当前 `version/platform/arch/channel` 对应的后端生成文件名一致；转正时后端忽略请求中的大小和哈希，以 OSS 实际内容重新计算并写入数据库，同时用 ETag 条件复制防止校验与复制之间发生替换。
- 公开响应不会返回 `objectKey / status / published` 等后台字段；公开下载会再次检查版本状态，草稿版本不会签名下载。
- feed 下载路由会同时校验 URL 中的 `platform/arch/channel/id/filename`；`filename` 必须精确等于该发布记录的 DMG/主安装包名或 updater ZIP 名，不能借路由签名同前缀下的其他对象。
- 发布版本必须带 feed 对应资产的 `sha512`；macos 还必须满足主资产为 DMG、updater 资产为 ZIP。客户端安装阶段由 `electron-updater` 按 `sha512` 校验更新包。
- 生产环境必须配置系统 `ServerAddress` 为固定 HTTPS 外部地址。服务端拒绝带用户信息或非 `http/https` 协议的配置，忽略可伪造的 `X-Forwarded-Host`，防止管理端复制出被 Host Header 污染的公开下载链接。
- OSS AccessKey 只放服务端环境变量，不写入前端配置、打包产物或 Apifox 示例环境。

## 并发行为

- 同一 `version / platform / arch / channel` 由数据库唯一索引兜底；前端的“同版本覆盖”确认只是交互提示，最终仍以后端唯一约束为准。
- 两个管理员同时创建同一目标版本时，只会有一个写入成功；另一个请求会收到唯一索引冲突，需要刷新列表后决定是否覆盖。
- 上传到 OSS 的临时 object key 带随机 ID，避免并发上传同名文件时直接覆盖 OSS 对象。
- 保存版本记录时，后端在写数据库前把 `_tmp/` 对象复制到带时间戳的正式 object key；数据库保存失败会 best-effort 删除刚复制出的正式对象。
- 更新已有版本时，后端会在数据库事务中锁定当前记录并读取真正被本次更新覆盖的旧 DMG/主安装包与 updater ZIP object key，再执行旧对象清理，降低并发保存留下正式目录孤儿对象的风险。
- 请求进入事务前会记录它读到的资产键。若另一个管理员已经替换了 DMG 或 ZIP，较晚到达的旧元数据请求会保留锁内最新资产，不会把旧键写回，也不会误删刚上传的新资产。
- 每条记录包含从 `1` 开始的 `revision`。更新请求必须回传读取到的 `revision`；后端在行锁内比较并递增它。若另一位管理员已经保存或切换发布状态，旧请求会返回并发冲突，不覆盖较新的版本；管理端提示刷新后重试。
- 初始化上传和完成确认没有写数据库；管理员仍需保存版本记录才会创建或更新发布元数据。
- 更新记录、删除记录、发布和取消发布都会在数据库事务中重新校验操作者权限，并依次锁定操作者用户行与当前版本行。元数据更新保留锁内读到的发布状态，发布状态更新也只基于锁内最新记录，避免撤权竞争或并发请求使用旧对象覆盖发布状态及其他字段。
- `latest` 和 `latest.yml` 总是读取同一目标下 `id` 最大的已发布记录；如果下载过程中又发布了新记录，已开始的下载仍按当次 `latest.yml` 中的 `id` 获取安装包。
- 客户端更新检查本身做进程内去重；同一客户端同时触发自动检查和手动检查时，会复用同一次后端请求结果。

## 管理端双前端同步

- 客户端管理页同时存在 default 和 classic 两套实现：`web/default/src/features/client-releases` 与 `web/classic/src/pages/ClientReleases`。
- 修改字段、校验、按钮文案、列表标签、上传逻辑或接口 payload 时，必须同时检查两套实现。
- 两套页面都把平台显示为 `macos`，使用互相隔离的 Windows/macos/Linux 工作区；切换平台时会清空列表、筛选、编辑草稿和未保存上传，不展示上一个平台的数据。
- 两套页面都要求先填写版本号并选择架构、通道再上传，保存前检查服务端规范文件名是否与 `platform/arch/channel` 完全一致。
- 已发布记录返回外部可访问的 `downloadUrl`。页面同时提供“下载”和“复制下载链接”按钮，不直接展示长 URL；macos 的 updater ZIP 另有 `updaterDownloadUrl`，更新清单使用 `updateManifestUrl`。草稿不返回这些公开链接。
- 例如强制更新标签应两边一致：有 `minVersion` 时显示 `≥x.x.x`，没有最低版本时才兜底显示“强更”或本地化的 Forced 文案。

## 接口入口

```txt
GET  /api/client-releases
GET  /api/client-releases/latest
GET  /api/client-releases/download/{id}
GET  /api/client-releases/updates/{platform}/{arch}/{channel}/latest.yml
GET  /api/client-releases/updates/{platform}/{arch}/{channel}/latest-mac.yml
GET  /api/client-releases/updates/{platform}/{arch}/{channel}/download/{id}/{filename}

GET    /api/admin/client-releases?platform={windows|macos|linux}
POST   /api/admin/client-releases/direct-upload/init
POST   /api/admin/client-releases/direct-upload/complete
POST   /api/admin/client-releases/direct-upload/discard
POST   /api/admin/client-releases
GET    /api/admin/client-releases/{id}?platform={windows|macos|linux}
PUT    /api/admin/client-releases/{id}?platform={windows|macos|linux}
POST   /api/admin/client-releases/{id}/publish?platform={windows|macos|linux}
POST   /api/admin/client-releases/{id}/unpublish?platform={windows|macos|linux}
DELETE /api/admin/client-releases/{id}?platform={windows|macos|linux}
```

创建请求的 `revision` 为 `0`；成功后响应为 `1`。更新时必须原样回传最近一次响应中的 `revision`。所有管理端响应中的平台值都规范为 `macos`，不输出其他 mac 平台别名。
