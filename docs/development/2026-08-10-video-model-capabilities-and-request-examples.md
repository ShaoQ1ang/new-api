# NewAPI 视频模型支持能力与完整请求示例

> 调研日期：2026-08-10
> 判定标准：只有请求字段、渠道适配器映射和必要校验均已实现的能力，才视为 NewAPI 已支持。模型厂商的原生能力或模型目录中的能力声明，不等同于 NewAPI 已经可以调用该能力。

## 能力总览

| 模型 | 文生视频 | 首帧图生视频 | 首尾帧 | 参考图 | 参考视频/编辑 | 参考音频 | 原生音频控制 | NewAPI 状态 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Veo 3.1 | 支持 | 支持 | 支持 | 未实现 | 未实现 | 未实现 | 支持 | 已接入 Gemini/Vertex/OpenRouter |
| Seedance 2.0 | 支持 | 支持 | 依赖上游 `content` 协议 | 支持 | 支持视频输入 | 支持 | 支持 | 已接入豆包视频通道 |
| MiniMax H3 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 未接入 |
| Kling Video 3.0 | 支持 | 支持 | 支持 | 不支持 `refer` | 不支持 `base/feature` | 不支持 | 支持 | 已接入百炼通道 |
| Kling Video 3.0 Omni | 支持 | 支持 | 支持 | 支持 | 支持 | 不支持 | 部分支持 | 已接入百炼通道 |
| HappyHorse 1.1 T2V | 支持 | 不适用 | 不支持 | 不适用 | 不支持 | 不支持 | 无显式开关 | 适配器已实现 |
| HappyHorse 1.1 I2V | 不适用 | 支持 | 不支持 | 单张首帧 | 不支持 | 不支持 | 无显式开关 | 适配器已实现 |
| HappyHorse 1.1 R2V | 不适用 | 不适用 | 不支持 | 支持 1-9 张 | 不支持 | 不支持 | 无显式开关 | 适配器已实现 |

## 通用接口

推荐使用 OpenAI 兼容视频接口：

```http
POST /v1/videos HTTP/1.1
Host: your-newapi.example.com
Authorization: Bearer <NEWAPI_API_TOKEN>
Content-Type: application/json

{请求 JSON}
```

NewAPI 同时兼容以下提交接口：

```text
POST /v1/video/generations
```

任务查询和视频内容获取接口：

```http
GET /v1/videos/{task_id}
Authorization: Bearer <NEWAPI_API_TOKEN>
```

```http
GET /v1/videos/{task_id}/content
Authorization: Bearer <NEWAPI_API_TOKEN>
```

当前公共任务请求支持以下字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 调用的模型 ID |
| `prompt` | string | 文本提示词 |
| `mode` | string | 模型模式，例如 Kling 的 `std`/`pro` |
| `image` | string | 兼容单图输入 |
| `images` | string[] | 首帧、尾帧或参考图列表 |
| `videos` | string[] | 参考视频或待编辑视频列表 |
| `size` | string | 分辨率或尺寸 |
| `duration` | integer/string | 视频时长，单位为秒 |
| `seconds` | string | OpenAI 兼容的视频时长字段 |
| `input_reference` | string | 兼容单个输入素材 |
| `metadata` | object/string | 厂商扩展参数；推荐传 JSON 对象 |

公共请求中没有顶层 `audios` 字段。参考音频、多镜头、Element 等能力通过 `metadata` 承载，并由具体渠道适配器解析。

## Veo 3.1

### 支持范围

已注册模型：

- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`

当前支持：

- 文生视频。
- 单张首帧图生视频。
- 两张图片的首尾帧插值，按首帧、尾帧顺序提供。
- 时长、比例和分辨率设置。
- 负面提示词、人物生成策略和随机种子。
- 原生音频生成控制。
- Gemini、Vertex 和 OpenRouter 三种渠道。

当前未支持：

- `referenceImages` 参考图。
- 视频续写和视频编辑。

### 完整请求示例

```json
{
  "model": "veo-3.1-generate-preview",
  "prompt": "A cinematic tracking shot of a robot walking through a rainy neon street",
  "images": [
    "data:image/png;base64,<FIRST_FRAME_BASE64>",
    "data:image/png;base64,<LAST_FRAME_BASE64>"
  ],
  "duration": 8,
  "metadata": {
    "durationSeconds": 8,
    "aspectRatio": "16:9",
    "resolution": "1080p",
    "negativePrompt": "blur, distortion, subtitles",
    "personGeneration": "allow_adult",
    "seed": 12345,
    "generateAudio": true,
    "compressionQuality": "optimized",
    "resizeMode": "crop"
  }
}
```

删除 `images` 即为文生视频请求；只保留第一张图片即为单首帧图生视频。Gemini 和 Vertex 将第二张图片映射为 `lastFrame`，OpenRouter 将两张图片映射为 `frame_images[0].first_frame` 和 `frame_images[1].last_frame`。

Veo 的 JSON 图片输入目前只支持 Data URI 或原始 Base64，不支持直接下载 HTTP 图片 URL。也可以使用 multipart 的 `input_reference` 上传图片。

## Seedance 2.0

### 支持范围

已注册模型：

- `doubao-seedance-2-0-260128`
- `doubao-seedance-2-0-fast-260128`

`metadata` 会被解码到完整的 Seedance 上游请求结构，并保留未知扩展字段。当前可传：

- `content` 多模态数组。
- `text`、`image_url`、`video_url`、`audio_url`。
- `tools`。
- `generate_audio`。
- `return_last_frame`。
- `resolution`、`ratio`、`duration`、`frames`。
- `seed`、`camera_fixed`、`watermark`。
- 其他上游支持的自定义字段。

NewAPI 没有在本地完整校验 Seedance 的图片、视频和音频素材数量限制，超出上游限制时由上游返回错误。

### 完整多模态请求示例

```json
{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "Keep the same character and create a cinematic rainy-night sequence",
  "duration": 10,
  "metadata": {
    "content": [
      {
        "type": "text",
        "text": "Keep the same character and create a cinematic rainy-night sequence",
        "role": "user"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/character.jpg"
        }
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://example.com/motion-reference.mp4"
        }
      },
      {
        "type": "audio_url",
        "audio_url": {
          "url": "https://example.com/music-reference.mp3"
        }
      }
    ],
    "duration": 10,
    "resolution": "1080p",
    "ratio": "16:9",
    "generate_audio": true,
    "return_last_frame": true,
    "watermark": false,
    "seed": 12345,
    "tools": [
      {
        "type": "character_reference",
        "strength": "high"
      }
    ]
  }
}
```

## Kling Video 3.0

### 支持范围

模型 ID：

```text
kling/kling-v3-video-generation
```

当前支持：

- 文生视频。
- 单张首帧图生视频。
- 首尾帧生成。
- 多镜头和自定义分镜提示词。
- Element ID 列表。
- `std`/`pro` 模式。
- 比例、音频和水印控制。

标准版只允许以下媒体组合：

- 无媒体输入。
- 一个 `first_frame`。
- 一个 `first_frame` 加一个 `last_frame`。

标准版会拒绝 `refer`、`base` 和 `feature` 媒体类型。

### 完整首尾帧请求示例

```json
{
  "model": "kling/kling-v3-video-generation",
  "prompt": "The character walks from the station onto a sunlit platform",
  "images": [
    "https://example.com/first-frame.jpg",
    "https://example.com/last-frame.jpg"
  ],
  "duration": 10,
  "metadata": {
    "mode": "pro",
    "aspect_ratio": "16:9",
    "audio": true,
    "watermark": false,
    "multi_shot": true,
    "shot_type": "customize",
    "multi_prompt": [
      {
        "index": 1,
        "prompt": "Wide shot, the train arrives at the station",
        "duration": 5
      },
      {
        "index": 2,
        "prompt": "Medium tracking shot, the character walks onto the platform",
        "duration": 5
      }
    ],
    "element_list": [
      {
        "element_id": 101
      }
    ]
  }
}
```

输入规则：

- 不传 `images`：文生视频。
- `images` 包含一张图片：首帧图生视频。
- `images` 包含两张图片：第一张映射为首帧，第二张映射为尾帧。

## Kling Video 3.0 Omni

### 支持范围

模型 ID：

```text
kling/kling-v3-omni-video-generation
```

除 Kling Video 3.0 的能力外，Omni 还支持：

- 全部为 `refer` 的参考图。
- 单个 `feature`。
- `feature + refer`。
- `feature + first_frame`。
- 单个 `base` 视频。
- `base + refer`。
- 视频编辑和重混。

约束：

- `multi_shot=true` 时必须提供 `shot_type`。
- `shot_type=customize` 时必须提供 `multi_prompt`。
- 包含 `base` 或 `feature` 视频时，`audio` 必须为 `false`。
- `base + refer` 或 `feature + refer` 场景下，参考图与 Element 总数不能超过 4。
- 纯 `refer` 场景下，参考图与 Element 总数不能超过 7。
- 首帧/首尾帧场景最多包含 3 个 Element。

### 完整视频编辑请求示例

```json
{
  "model": "kling/kling-v3-omni-video-generation",
  "prompt": "Replace the background with a futuristic city while preserving the subject",
  "duration": 10,
  "metadata": {
    "media": [
      {
        "type": "base",
        "url": "https://example.com/source-video.mp4"
      },
      {
        "type": "refer",
        "url": "https://example.com/character-reference.jpg"
      }
    ],
    "mode": "pro",
    "aspect_ratio": "16:9",
    "audio": false,
    "watermark": false,
    "element_list": [
      {
        "element_id": 101
      },
      {
        "element_id": 102
      }
    ]
  }
}
```

也可以使用公共字段简化请求：

```json
{
  "model": "kling/kling-v3-omni-video-generation",
  "prompt": "Preserve the character and change the scene to a snowy mountain",
  "videos": [
    "https://example.com/source-video.mp4"
  ],
  "images": [
    "https://example.com/character-reference.jpg"
  ],
  "duration": 10,
  "metadata": {
    "mode": "pro",
    "audio": false
  }
}
```

在简化请求中，`videos[0]` 会转换为 `base`，`images` 会转换为 `refer`。

## HappyHorse 1.1

### 支持范围

适配器已经实现：

- `happyhorse-1.1-t2v`
- `happyhorse-1.1-i2v`
- `happyhorse-1.1-r2v`

支持的扩展参数：

- `metadata.ratio`，适用于 T2V 和 R2V。
- `metadata.watermark`。
- `metadata.seed`。
- `metadata.audio_setting`，仅用于通用 `video-edit` 路径。

当前没有用于 T2V、I2V 或 R2V 的显式原生音频开关，音频行为由上游模型和渠道默认值决定。

### 文生视频请求

```json
{
  "model": "happyhorse-1.1-t2v",
  "prompt": "A cinematic aerial shot of an ancient city at sunrise",
  "duration": 10,
  "size": "1080p",
  "metadata": {
    "ratio": "16:9",
    "watermark": false,
    "seed": 12345
  }
}
```

### 单首帧图生视频请求

```json
{
  "model": "happyhorse-1.1-i2v",
  "prompt": "The subject turns toward the camera and smiles",
  "images": [
    "https://example.com/first-frame.jpg"
  ],
  "duration": 10,
  "size": "1080p",
  "metadata": {
    "watermark": false,
    "seed": 12345
  }
}
```

I2V 必须且只能传一张首帧图，提示词可以为空。

### 参考图生视频请求

```json
{
  "model": "happyhorse-1.1-r2v",
  "prompt": "Generate a continuous action sequence while preserving character identity",
  "images": [
    "https://example.com/character-front.jpg",
    "https://example.com/character-side.jpg",
    "https://example.com/costume-reference.jpg"
  ],
  "duration": 10,
  "size": "1080p",
  "metadata": {
    "ratio": "16:9",
    "watermark": false,
    "seed": 12345
  }
}
```

R2V 必须提供提示词和 1-9 张参考图。

代码中还存在通用的 `*-video-edit` 分支，要求一个 `videos` 源视频，并允许最多五张参考图。但项目当前支持清单没有列出正式的 `happyhorse-1.1-video-edit` 模型 ID，因此不将其作为 HappyHorse 1.1 的稳定公开能力。

## MiniMax H3

当前代码中没有：

- `minimax-h3` 模型 ID。
- H3 V2 请求结构。
- H3 First/Last-Frame 映射。
- H3 Omni Reference 媒体结构。
- H3 原生音频控制。
- H3 2K 再生成适配。

因此，MiniMax H3 当前应标记为未接入，不能提供有效请求示例。

项目另有 MiniMax Hailuo 视频通道，支持 `MiniMax-Hailuo-2.3`、`MiniMax-Hailuo-2.3-Fast`、`MiniMax-Hailuo-02`、`T2V-01`、`I2V-01` 和 `S2V-01` 等模型。这些模型不等同于 MiniMax H3。

## 提交响应与任务查询

提交成功后返回公开任务 ID：

```json
{
  "id": "task_xxxxxxxxx",
  "task_id": "task_xxxxxxxxx",
  "object": "video",
  "model": "kling/kling-v3-video-generation",
  "status": "queued",
  "progress": 0,
  "created_at": 1786323511
}
```

查询请求：

```http
GET /v1/videos/task_xxxxxxxxx HTTP/1.1
Host: your-newapi.example.com
Authorization: Bearer <NEWAPI_API_TOKEN>
```

任务完成后的响应示例：

```json
{
  "id": "task_xxxxxxxxx",
  "object": "video",
  "model": "kling/kling-v3-video-generation",
  "status": "completed",
  "progress": 100,
  "created_at": 1786323511,
  "completed_at": 1786323522,
  "metadata": {
    "url": "https://example.com/generated-video.mp4"
  }
}
```

任务状态包括：

- `queued`
- `in_progress`
- `completed`
- `failed`

部分渠道会同时返回 `metadata.watermark_url`。也可以通过 `/v1/videos/{task_id}/content` 让 NewAPI 代理返回视频内容。

## 部署与配置注意事项

- Kling 3 和 HappyHorse 1.1 的阿里渠道适配逻辑已经实现，但 `relay/channel/task/ali/constants.go` 的默认 `ModelList` 当前仍只列出 Wan 系列。部署时需要在渠道中手动配置对应模型或模型映射。
- `metadata` 不是对所有渠道无条件原样透传。每个渠道仅支持其适配器明确解析或保留的字段。
- 公共请求的 `duration` 可以是整数或数字字符串，但建议使用整数。
- 公共请求最大时长防护为 3600 秒；具体模型通常有更小的上游限制。
- Seedance 多模态素材数量主要由上游校验。
- Kling 和 HappyHorse 的媒体组合由 NewAPI 在提交前进行本地校验。
- 官方通用视频 Schema 尚未完整列出 `images`、`videos` 和 `size` 等当前代码已经支持的字段，调用时应以本文和当前代码为准。

## 实现位置

- 公共任务请求：`relay/common/relay_info.go` 中的 `TaskSubmitReq`。
- 视频路由：`router/video-router.go`。
- Veo 请求结构：`relay/channel/task/gemini/dto.go`。
- Veo Gemini 映射：`relay/channel/task/gemini/adaptor.go`。
- Veo Vertex 映射：`relay/channel/task/vertex/adaptor.go`。
- Veo OpenRouter 映射：`relay/channel/task/openrouter/veo.go`。
- Seedance 映射：`relay/channel/task/doubao/adaptor.go`。
- Kling 百炼映射：`relay/channel/task/ali/kling_bailian.go`。
- HappyHorse 映射：`relay/channel/task/ali/happyhorse.go`。
- 阿里视频校验：`relay/channel/task/ali/adaptor.go`。
- MiniMax Hailuo 模型列表：`relay/channel/task/hailuo/constants.go`。

## 参考资料

- [NewAPI：创建视频生成任务](https://apifox.newapi.ai/383844576e0.md)
- [NewAPI：获取视频生成任务状态](https://apifox.newapi.ai/383844577e0.md)
- [Google Developers Blog：Veo 3.1](https://developers.googleblog.com/en/introducing-veo-3-1-and-new-creative-capabilities-in-the-gemini-api/)
- [ByteDance Seed：Seedance 2.0](https://seed.bytedance.com/en/blog/official-launch-of-seedance-2-0)
- [MiniMax：H3 视频生成指南](https://platform.minimax.io/docs/guides/video-generation)
- [Kling AI：VIDEO 3.0 Model User Guide](https://kling.ai/quickstart/klingai-video-3-model-user-guide)
- [Alibaba Cloud：视频生成与编辑模型总览](https://www.alibabacloud.com/help/en/model-studio/video-generate-edit-model/)
- [Alibaba Cloud：HappyHorse 文生视频 API](https://www.alibabacloud.com/help/en/model-studio/happyhorse-text-to-video-api-reference)
- [Alibaba Cloud：HappyHorse 首帧图生视频 API](https://www.alibabacloud.com/help/en/model-studio/happyhorse-image-to-video-api-reference)
