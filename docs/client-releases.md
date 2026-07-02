# Client Releases

客户端安装包管理用于给桌面端提供版本列表、更新检查、`latest.yml` 和安装包下载入口。安装包保存在私有 OSS，客户端只访问 New API；下载时 New API 生成短时签名 URL 后 302 跳转。

## OSS 配置

```env
CLIENT_RELEASE_OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
CLIENT_RELEASE_OSS_BUCKET=your-private-bucket
CLIENT_RELEASE_OSS_ACCESS_KEY_ID=xxx
CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET=xxx
CLIENT_RELEASE_OSS_PREFIX=client-releases
CLIENT_RELEASE_OSS_SIGNED_URL_EXPIRES_SECONDS=600
CLIENT_RELEASE_OSS_MAX_BYTES=524288000
```

- `CLIENT_RELEASE_OSS_PREFIX` 为空时默认 `client-releases`。
- `CLIENT_RELEASE_OSS_SIGNED_URL_EXPIRES_SECONDS` 为空时默认 `600` 秒，最大不超过 `86400` 秒。
- `CLIENT_RELEASE_OSS_MAX_BYTES` 为空时默认 `500MB`。
- 生产环境建议配置系统 `ServerAddress` 为 HTTPS 外部地址，避免公开接口根据代理 Header 推导出错误的下载 URL。

## 上传与发布

- 上传接口会先校验 `version / platform / arch / channel` 和文件扩展名，非法目标不会写入 OSS。
- 文件名由后端强制生成：`Z-UP-Setup-{version}-{platform}-{arch}-{channel}.{ext}`，前端不允许编辑上传名。
- 支持扩展名：`exe`、`msi`、`dmg`、`pkg`、`zip`、`AppImage`、`deb`、`rpm`、`yml`、`yaml`。
- 上传会返回 `sha256` 和 `sha512`；已发布版本必须带 `sha512`，否则 `electron-updater` 不能校验 `latest.yml`。

## 版本选择

- 后台列表按 `id DESC` 返回。
- 公开列表、`latest` 和 `latest.yml` 只读取已发布版本。
- 同一 `platform / arch / channel` 范围内，`latest` 和 `latest.yml` 按 `id` 最大的已发布记录作为最新记录。
- 客户端仍会比较 `latest.version` 和自己的当前版本；如果误发布了版本号更低但 `id` 更大的记录，客户端不会把它当作升级版本。

## 接口入口

```txt
GET  /api/client-releases
GET  /api/client-releases/latest
GET  /api/client-releases/download/{id}
GET  /api/client-releases/updates/{platform}/{arch}/{channel}/latest.yml
GET  /api/client-releases/updates/{platform}/{arch}/{channel}/download/{id}/{filename}

GET    /api/admin/client-releases
POST   /api/admin/client-releases/upload
POST   /api/admin/client-releases
PUT    /api/admin/client-releases/{id}
POST   /api/admin/client-releases/{id}/publish
POST   /api/admin/client-releases/{id}/unpublish
DELETE /api/admin/client-releases/{id}
```

