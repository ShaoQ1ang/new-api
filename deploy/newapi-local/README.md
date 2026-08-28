# `deploy/newapi-local` 使用说明

这个目录同时服务两类场景：

- 本地开发 / 本地联调
- `104.xx.xx.xx` 海外生产环境运维
- `120.xx.xx.xx` 国内生产环境运维

## 0. 环境约定

当前部署约定如下：

- `104` 为海外环境
- `120` 为国内环境
- `web/classic` 首页入口使用构建时参数 `VITE_HOME_ENTRY` 固定切换

当前推荐值：

- `104` 使用 `VITE_HOME_ENTRY=en`
- `120` 使用 `VITE_HOME_ENTRY=cn`

说明：

- 这里使用的是 Docker build 阶段的 `ARG VITE_HOME_ENTRY`
- 不建议把它当作容器运行时环境变量来切换，因为 `web/classic` 是 Vite 静态构建产物，运行时改环境变量不会重新生成前端代码

这个目录现在建议分两层使用：

- [deploy/newapi-local/README.md](./README.md)
  - 日常操作入口
  - 告诉你平时优先跑什么命令
- [deploy/newapi-local/104_SERVER_DEPLOYMENT.md](./104_SERVER_DEPLOYMENT.md)
  - 104 机器完整部署手册
  - 记录线上真实结构、风险点、验收和恢复方式

## 1. 推荐入口

如果你是在发 104，优先先看：

```bash
./deploy/newapi-local/release.sh <command>
```

支持的命令：

- `build [image_tag]`
- `verify-image [image_tag]`
- `backup-env`
- `upload [image_tag]`
- `deploy [image_tag]`
- `deploy-existing <image_tag>`
- `release [image_tag]`
- `list-remote-images`
- `status`
- `rollback <image_tag>`

查看帮助：

```bash
./deploy/newapi-local/release.sh help
```

使用提醒：

- 这个脚本现在只是“操作手册入口”，不会执行真实的 `docker`、`ssh`、`scp`、发布、回滚或状态查询
- 正确用法是先运行它查看对应 SOP，再手工执行命令
- 线上环境一定先以远端机器上的真实 compose、env、容器名和目录为准，不要把本地模板文件直接当生产真相

## 2. 日常推荐流程

### 2.1 最推荐的发布方式

```bash
./deploy/newapi-local/release.sh release
```

这个命令现在只会打印推荐发布检查清单。实际发布建议按下面顺序手工执行：

1. `build`
2. `verify-image`
3. `backup-env`
4. `upload`
5. `deploy`

### 2.2 只改了后端代码

```bash
./deploy/newapi-local/release.sh build
./deploy/newapi-local/release.sh upload
./deploy/newapi-local/release.sh deploy
```

### 2.3 改了前端页面或静态资源

```bash
./deploy/newapi-local/release.sh build
./deploy/newapi-local/release.sh verify-image
./deploy/newapi-local/release.sh upload
./deploy/newapi-local/release.sh deploy
```

### 2.4 发布前先备份线上环境变量

```bash
./deploy/newapi-local/release.sh backup-env
```

### 2.5 看线上当前状态

```bash
./deploy/newapi-local/release.sh status
```

### 2.6 回滚到远端已有旧镜像

```bash
./deploy/newapi-local/release.sh rollback
```

上面的命令都只输出说明，不会实际执行。

如果你要看 104 机器当前真实状态、当前线上容器名、以及当前最安全的发布方式，优先看：

- [deploy/newapi-local/104_SERVER_DEPLOYMENT.md](./104_SERVER_DEPLOYMENT.md)
- [deploy/newapi-local/120_SERVER_DEPLOYMENT.local.md](./120_SERVER_DEPLOYMENT.local.md)

## 3. 本地开发模式

这个目录可以在本地跑一套 `New API`，包含：

- `calciumion/new-api:latest`
- SQLite 持久化到 `./data`
- 日志目录 `./logs`
- HTTP 暴露到 `http://localhost:3000`
- 本地 metadata 服务暴露到 `http://localhost:8088`

启动：

```bash
docker compose up -d
```

停止：

```bash
docker compose down
```

首次访问时，打开 `http://localhost:3000`，完成初始化页面来创建管理员账号和密码。

### 3.1 从仓库根目录构建完整本地栈

仓库根目录的 `docker-compose.local.yml` 会从当前源码构建 `new-api` 和 `seedance-compat`，并启动 PostgreSQL、Redis 与统一 Nginx 网关。启动前需要在仓库根目录的 `.env` 中设置稳定且随机的 `SESSION_SECRET` 和 `CRYPTO_SECRET`；不要把真实密钥提交到版本库。

启动：

```bash
docker compose -f docker-compose.local.yml up -d --build
```

停止：

```bash
docker compose -f docker-compose.local.yml down
```

这套配置默认只监听 `127.0.0.1:3000`。确实需要从局域网访问时，可显式设置：

```env
NEWAPI_BIND_ADDRESS=0.0.0.0
NEWAPI_PORT=3000
```

同时必须替换 `NEWAPI_POSTGRES_PASSWORD` 和 `NEWAPI_REDIS_PASSWORD` 的开发默认值。由于同一个变量既用于服务端密码，也会直接拼入 PostgreSQL/Redis 连接 URI，建议使用字母、数字、连字符和下划线组成的 URL-safe 随机值。

启动顺序由 PostgreSQL、Redis 和应用健康检查控制。网关通过 Docker 内置 DNS 动态解析 `new-api` 与 `seedance-compat`，后端容器重建并更换 IP 后不需要重启网关；网关同时支持 WebSocket 升级和无缓冲流式响应。Redis 在这套本地配置中只作为易失缓存使用，停止或重建后缓存数据可能丢失。

## 4. 本地 PostgreSQL 模式

当你希望 `New API` 把主数据库存到 PostgreSQL，而不是 `./data/one-api.db` 时，使用 `docker-compose.postgres.yml`。

先创建本地 env 文件：

```bash
cp .env.postgres.example .env.postgres
```

然后编辑 `.env.postgres` 中的 `NEWAPI_POSTGRES_PASSWORD`，再启动：

```bash
docker compose --env-file .env.postgres -f docker-compose.postgres.yml up -d --build
```

停止：

```bash
docker compose --env-file .env.postgres -f docker-compose.postgres.yml down
```

这个模式使用：

```env
SQL_DSN=postgresql://<user>:<password>@postgres:5432/<db>?sslmode=disable
```

PostgreSQL 数据保存在 Docker volume `newapi_pg_data` 中。现有的 SQLite 文件 `./data/one-api.db` 不会自动迁移；切到 PostgreSQL 模式会创建一套新的数据库，除非你另行做数据迁移。

本地几个 compose 文件目前职责如下：

- `docker-compose.yml`：本地 SQLite 开发环境
- `docker-compose.postgres.yml`：本地 PostgreSQL 开发环境
- `docker-compose.devtools.yml`：固定开发/测试工具容器
- `docker-compose.dev.mock.yml`：独立多渠道视频 mock 容器（Ali、Seedance、Gemini Veo、Vertex Veo）

## 4.1 本地 devtools 测试容器

如果你希望固定一个带 `Go + Bun` 的开发/测试容器，避免每次临时 `docker run` 重复拉镜像、重复安装依赖，可以使用 `docker-compose.devtools.yml`。

启动：
```bash
docker compose -f deploy/newapi-local/docker-compose.devtools.yml up -d --build devtools
```

进入容器：
```bash
docker compose -f deploy/newapi-local/docker-compose.devtools.yml exec devtools bash
```

容器里已经预设：
- `GOPROXY=https://goproxy.cn,direct`
- `GOSUMDB=sum.golang.google.cn`
- `npm_config_registry=https://registry.npmmirror.com`
- `bun install` 使用 `https://registry.npmmirror.com`

同时会持久化这些缓存/依赖，避免重复下载：
- Go 模块缓存：`newapi_devtools_go_pkg`
- Go build 缓存：`newapi_devtools_go_build`
- Bun 安装缓存：`newapi_devtools_bun_cache`
- `web/default/node_modules`
- `web/classic/node_modules`

首次进入容器后，常用命令示例：
```bash
cd /workspace/web/classic
bun install
bun test src/pages/Setting/Ratio/modelPricingVideoSecondsPrice.test.js src/helpers/pricingTaskConditionPrice.test.js
bun run build

cd /workspace
go test ./relay/channel/task/ali ./relay/channel/task/taskcommon ./relay/helper ./relay ./setting/ratio_setting ./model
```

## 4.2 多渠道视频本地 mock

本地开发提供一个独立的 AIGC mock 服务。为保持已有脚本和部署配置兼容，服务名与环境变量前缀仍为 `ali-video-mock` / `ALI_VIDEO_MOCK_*`，但它同时实现 Ali、Seedance、OpenRouter Video、Gemini Veo、Vertex Veo、OpenAI Images、Suno 和 SunoAPI v1 的上游协议，可直接用于零成本端到端联调。

启动：

```bash
docker compose -f docker-compose.dev.mock.yml up -d --build
```

默认宿主端口是 `18080`。端口已占用时可以覆盖：

```bash
ALI_VIDEO_MOCK_PORT=18081 docker compose -f docker-compose.dev.mock.yml up -d --build
```

健康检查：

```bash
curl http://localhost:18080/healthz
```

请求历史调试页：

```text
http://localhost:18080/history
```

mock 会在内存中保留最近 500 条上游请求，完整记录 method、path、query、header、原始 body、响应状态和处理耗时，便于检查 New API 转换后的实际请求。服务重启后记录自动清空，也可以在页面内手动清空。`/history`、`/api/mock/history`、`/healthz` 和视频资源请求不会写入历史，避免调试页面刷新、健康检查与视频下载污染记录。

给 `new-api` 里的对应渠道配置本地 mock 时，把渠道 `base_url` 改成：

```text
http://host.docker.internal:18080
```

如果你在宿主机直接调 mock，也可以使用：

```text
http://127.0.0.1:18080
```

当前 mock 实现以下原生异步接口：

- `POST /api/v1/services/aigc/video-generation/video-synthesis`
- `GET /api/v1/tasks/:id`
- `POST /api/v3/contents/generations/tasks`
- `GET /api/v3/contents/generations/tasks/:id`
- `POST /v1/videos`
- `GET /v1/videos/:id`
- `POST /{version}/models/{model}:predictLongRunning`
- `GET /{version}/{operationName}`
- `POST /v1/projects/{project}/locations/{region}/publishers/google/models/{model}:predictLongRunning`
- `POST /v1/projects/{project}/locations/{region}/publishers/google/models/{model}:fetchPredictOperation`
- `POST /api/v1/generate`
- `GET /api/v1/generate/record-info?taskId=:id`

支持的模型族：

- 当前 Ali 通道 `ModelList` 中的 Wan 视频模型，包括 `wan2.7-t2v`、`wan2.7-i2v`、`wan2.5-i2v-preview`、`wan2.2-i2v-*` 和 `wanx2.1-i2v-*`
- `happyhorse-1.0-*` 和 `happyhorse-1.1-*`
- `kling/kling-v3-video-generation`
- `kling/kling-v3-omni-video-generation`
- Seedance 1.0、1.5 与 2.0 的当前 Doubao 视频通道模型列表
- OpenRouter `bytedance/seedance-*`
- OpenRouter `google/veo-*`
- `veo-3.0-generate-001`、`veo-3.0-fast-generate-001`
- `veo-3.1-generate-preview`、`veo-3.1-fast-generate-preview`
- SunoAPI v1 `V4`、`V4_5`、`V4_5PLUS`、`V4_5ALL`、`V5`、`V5_5`

行为约定：

- 会校验 Wan 文生/图生、HappyHorse 文生/首帧/参考生/视频编辑，以及 Kling 标准版/Omni 的媒体组合
- 默认在提交约 10 秒后成功，频繁轮询不会提前完成；可通过 `ALI_VIDEO_MOCK_COMPLETION_DELAY_SECONDS` 调整
- 会返回 `usage.duration`、`usage.SR`、`usage.audio`，方便验证本地计费链路
- 成功结果 URL 返回真正可播放的 H.264 MP4，不再返回字符串占位符
- 视频端点支持 `GET`、`HEAD` 和 HTTP Range 请求，可供浏览器 `<video>`、NewAPI 视频代理和下载客户端使用
- mock 在内存中按完整 H.264 GOP 扩展内嵌视频，并按任务 `duration` 缓存实际 1–15 秒 MP4，无需 ffmpeg
- 只有请求显式开启 `watermark=true` 时才返回 `watermark_video_url`
- Seedance 成功响应返回 `content.video_url` 和 token usage，可验证视频输入与分辨率计费链路
- OpenRouter Seedance/Veo 成功响应返回 `output.video_url`、`usage.video_tokens`、`usage.total_tokens`、duration 和 provider cost
- Gemini Veo 成功响应返回 `generateVideoResponse.generatedVideos[].video.uri`
- Vertex Veo 按真实协议在 operation 结果中返回 Base64 编码的可播放 MP4
- SunoAPI v1 遵循 `PENDING` / `FIRST_SUCCESS` / `SUCCESS` 轮询状态，成功时固定返回两首可播放 WAV；`callBackUrl` 仅做必填校验，不主动发起回调

Seedance 2.0 多模态请求示例：

```bash
curl -sS http://localhost:18080/api/v3/contents/generations/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "电影感城市夜景"},
      {"type": "image_url", "image_url": {"url": "https://example.com/reference.png"}},
      {"type": "video_url", "video_url": {"url": "https://example.com/motion.mp4"}},
      {"type": "audio_url", "audio_url": {"url": "https://example.com/music.mp3"}}
    ],
    "generate_audio": true,
    "resolution": "1080p",
    "ratio": "16:9",
    "duration": 10,
    "seed": 123
  }'
```

使用返回的 `id` 轮询：

```bash
curl -sS http://localhost:18080/api/v3/contents/generations/tasks/mock-seedance-000001
```

OpenRouter `google/veo-*` 请求示例：

```bash
curl -sS http://localhost:18080/v1/videos \
  -H 'Authorization: Bearer mock-key' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "google/veo-3.1-lite",
    "prompt": "让两个关键帧之间自然过渡",
    "duration": 8,
    "resolution": "1080p",
    "aspect_ratio": "16:9",
    "generate_audio": true,
    "frame_images": [
      {"frame_type": "first_frame", "image_url": {"url": "https://example.com/first.png"}},
      {"frame_type": "last_frame", "image_url": {"url": "https://example.com/last.png"}}
    ],
    "input_references": [
      {"type": "image", "url": "https://example.com/reference.png"}
    ]
  }'
```

使用返回的 `id` 轮询：

```bash
curl -sS http://localhost:18080/v1/videos/mock-openrouter-veo-000001
```

OpenRouter Veo mock 与 `origin/feat/openrouter-video-handlers` 的当前约束一致：模型按 `google/veo-*` 前缀匹配，时长支持 `4/6/8` 秒，分辨率支持 `720p/1080p`，比例支持 `16:9/9:16`；最多接受首帧和尾帧各一张，参考素材只接受图片。

OpenRouter `bytedance/seedance-*` 完整请求示例：

```bash
curl -sS http://localhost:18080/v1/videos \
  -H 'Authorization: Bearer mock-key' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "bytedance/seedance-2.0",
    "prompt": "保持人物一致性，生成多镜头城市追逐片段",
    "duration": 15,
    "resolution": "1080p",
    "aspect_ratio": "21:9",
    "generate_audio": true,
    "frame_images": [
      {"frame_type": "first_frame", "image_url": {"url": "https://example.com/first.png"}},
      {"frame_type": "last_frame", "image_url": {"url": "https://example.com/last.png"}}
    ],
    "input_references": [
      {"type": "image", "url": "https://example.com/character.png"},
      {"type": "video", "url": "https://example.com/motion.mp4"},
      {"type": "audio", "url": "https://example.com/dialogue.mp3"}
    ],
    "watermark": false,
    "req_key": "local-e2e-test",
    "seed": 123,
    "provider": {"order": ["ByteDance"]},
    "callback_url": "https://example.com/video-callback"
  }'
```

使用返回的 `id` 轮询：

```bash
curl -sS http://localhost:18080/v1/videos/mock-openrouter-seedance-000001
```

OpenRouter Seedance mock 按 `bytedance/seedance-*` 前缀匹配，时长支持 4–15 秒，分辨率支持 `480p/720p/1080p/4k`，比例支持 `1:1`、`3:4`、`9:16`、`4:3`、`16:9`、`21:9` 和 `9:21`。最多接受 9 张图片、3 个视频、3 个音频，且媒体总数不超过 12；首尾帧计入图片数量。`watermark`、`req_key`、`seed`、`provider` 和 `callback_url` 作为透传字段被接受。

Gemini Veo 请求示例：

```bash
curl -sS http://localhost:18080/v1beta/models/veo-3.1-generate-preview:predictLongRunning \
  -H 'Content-Type: application/json' \
  -d '{
    "instances": [{
      "prompt": "让首帧中的云层缓慢移动",
      "image": {"bytesBase64Encoded": "aW1hZ2U=", "mimeType": "image/png"}
    }],
    "parameters": {
      "sampleCount": 1,
      "durationSeconds": 8,
      "aspectRatio": "16:9",
      "resolution": "1080p",
      "generateAudio": true
    }
  }'
```

使用返回的 `name` 轮询；例如：

```bash
curl -sS http://localhost:18080/v1beta/models/veo-3.1-generate-preview/operations/mock-veo-000001
```

Vertex Veo 使用与 Gemini 相同的 `instances` / `parameters` 请求体，但提交与轮询均为 POST：

```bash
curl -sS http://localhost:18080/v1/projects/demo/locations/global/publishers/google/models/veo-3.1-generate-preview:predictLongRunning \
  -H 'Content-Type: application/json' \
  -d '{"instances":[{"prompt":"海浪拍打礁石"}],"parameters":{"durationSeconds":8,"resolution":"720p"}}'

curl -sS http://localhost:18080/v1/projects/demo/locations/global/publishers/google/models/veo-3.1-generate-preview:fetchPredictOperation \
  -H 'Content-Type: application/json' \
  -d '{"operationName":"projects/demo/locations/global/publishers/google/models/veo-3.1-generate-preview/operations/mock-veo-000002"}'
```

Vertex 适配器在发送请求前仍会按生产逻辑使用服务账号向 Google OAuth 换取 access token；mock 覆盖的是 Vertex 视频提交和轮询协议，不绕过这段鉴权。完全离线联调 Veo 时使用 Gemini 渠道即可，Gemini mock 只要求请求携带适配器正常生成的 API Key 请求头，mock 不校验其值。

直接验证视频资产：

```bash
curl -I http://localhost:18080/mock-assets/videos/sample.mp4
curl -H 'Range: bytes=0-31' -o /tmp/mock-video-prefix.bin http://localhost:18080/mock-assets/videos/sample.mp4
curl -o /tmp/newapi-mock-video.mp4 http://localhost:18080/mock-assets/videos/sample.mp4
```

完成等待时间可通过 `ALI_VIDEO_MOCK_COMPLETION_DELAY_SECONDS` 调整，默认 10 秒。例如改成 5 秒：

```bash
ALI_VIDEO_MOCK_COMPLETION_DELAY_SECONDS=5 docker compose -f docker-compose.dev.mock.yml up -d --build
```

`ALI_VIDEO_MOCK_COMPLETE_AFTER_POLL` 仍可用于测试兼容；设置为正整数时会覆盖墙钟等待时间，按轮询次数进入终态。

可选失败率：

- 通过环境变量 `ALI_VIDEO_MOCK_FAIL_RATE` 控制轮询终态随机失败概率
- 取值范围 `0` 到 `1`
- `0` 表示全部成功
- `1` 表示全部失败
- 例如 `0.5` 表示大约一半任务会在第 2 次轮询时返回 `FAILED`

用于验证失败退款链路时，推荐：

```bash
ALI_VIDEO_MOCK_FAIL_RATE=0.5 docker compose -f docker-compose.dev.mock.yml up -d --build
```

默认情况下，mock 使用提交请求的 Host 构造视频 URL。需要让返回 URL 指向反向代理或宿主机地址时，可以设置：

```bash
ALI_VIDEO_MOCK_PUBLIC_BASE_URL=http://127.0.0.1:18080 docker compose -f docker-compose.dev.mock.yml up -d --build
```

如果 NewAPI 运行在 Docker 容器内，并通过 `/v1/videos/{task_id}/content` 代理视频，不要把公开地址设置为容器内不可达的 `127.0.0.1`；此时保留空值，让 mock 使用容器请求的 Host 更合适。

## 5. 本地 metadata

这个目录包含一份可维护的 NewAPI upstream metadata：

- `metadata/api/newapi/models.json`
- `metadata/api/newapi/vendors.json`

`new-api` 服务默认配置：

```yaml
SYNC_UPSTREAM_BASE: "http://metadata"
```

在这套 compose 里，NewAPI 会从下面地址同步：

```text
http://metadata/api/newapi/models.json
http://metadata/api/newapi/vendors.json
```

同时这些文件也可以通过本机对外暴露：

```text
http://<this-host>:8088/api/newapi/models.json
http://<this-host>:8088/api/newapi/vendors.json
```

如果另一套 `New API` 也要复用这一份 metadata，可以配置：

```env
SYNC_UPSTREAM_BASE=http://<this-host>:8088
```

## 6. 104 生产环境说明

104 线上环境不要直接套用上面的“本地开发”步骤。

当前 104 的标准入口已经统一为：

- 工作目录：`./deploy/newapi-local/`
- compose 文件：`./deploy/newapi-local/docker-compose.postgres.yml`
- env 文件：`./deploy/newapi-local/.env.postgres`

当前实机状态和本地 compose 使用方式有几个关键差异：

- 当前生产应用容器名是 `new-api`
- 当前生产网关容器名是 `new-api-gateway`
- 当前生产 Redis 已启用
- 当前生产 PostgreSQL 与 Redis 都保留现有容器，不在普通发版里重建
- 当前普通发布应只更新应用容器，不要顺手重建整套服务

已废弃、不要再用：

- `/root/new-api/deploy/newapi-local/docker-compose.yml`
- `/root/new-api/docker-compose.yml`
- `/root/sub2api/deploy/newapi-local/docker-compose.postgres.yml`

所以 104 上的更新、回滚、验收，应以 [deploy/newapi-local/104_SERVER_DEPLOYMENT.md](./104_SERVER_DEPLOYMENT.md) 为准，不要直接照搬本 README 的本地命令。
