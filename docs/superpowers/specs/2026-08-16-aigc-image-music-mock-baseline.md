# AIGC Image and Music Mock Baseline

## Scope

This baseline covers every image and music model currently routed through the
unified NewAPI AIGC generation API.

- Image: `gpt-image-2`, `gemini-3-pro-image-preview`,
  `gemini-3.1-flash-image`, `qwen-image-2.0`.
- Music: `chirp-v4`.
- Excluded legacy local placeholders: `rhythm-v1`, `ambient-lab`.

The public flow is:

```text
AIGC POST /api/conversations/:id/turns
  -> NewAPI POST /v1/aigc/generations
  -> provider relay
  -> local Mock
  -> generated asset download
  -> AIGC object storage
```

## Local Channels

| Channel | ID | NewAPI type | Mock protocol | Models |
|---|---:|---:|---|---|
| Mock - OpenAI Images | 8 | 1 | OpenAI Images | four image models |
| Mock - Suno Music | 9 | 36 | Suno | `chirp-v4` |

Both channels use `http://ali-video-mock:8080` and priority 2. The image Mock
therefore wins over production channels during local regression testing.

## Successful Matrix

- Run: `image-music-matrix-1786895482423`
- AIGC conversation: `conversation-4af8d16885`
- NewAPI AIGC requests: 164-173
- NewAPI Suno tasks: 261-262
- Mock records: 454-464
- Result: 10/10 completed, 16 stored assets

| Public model | Mode | Final Mock route | Required final fields | Assets | Channel |
|---|---|---|---|---:|---|
| `gpt-image-2` | `text_to_image` | `/v1/images/generations` | `n:2`, `size:1024x576` | 2 | Ch8 |
| `gpt-image-2` | `image_edit` | `/v1/images/edits` | one `images` URL, `n:1`, `size:1024x1024` | 1 | Ch8 |
| `gemini-3-pro-image-preview` | `text_to_image` | `/v1/images/generations` | `n:2`, `size:1024x576`, `response_format:url` | 2 | Ch8 |
| `gemini-3-pro-image-preview` | `image_edit` | `/v1/images/edits` | one `images` URL, `n:1`, `size:1024x1024` | 1 | Ch8 |
| `gemini-3.1-flash-image` | `text_to_image` | `/v1/images/generations` | `n:2`, `size:1024x576`, `response_format:url` | 2 | Ch8 |
| `gemini-3.1-flash-image` | `image_edit` | `/v1/images/edits` | one `images` URL, `n:1`, `size:1024x1024` | 1 | Ch8 |
| `qwen-image-2.0` | `text_to_image` | `/v1/images/generations` | `n:2`, `size:1024x576`, `response_format:url` | 2 | Ch8 |
| `qwen-image-2.0` | `image_edit` | `/v1/images/edits` | one `images` URL, `n:1`, `size:1024x1024` | 1 | Ch8 |
| `chirp-v4` | vocal | `/suno/submit/MUSIC` | `mv:chirp-v4`, `make_instrumental:false` | 2 | Ch9 |
| `chirp-v4` | instrumental | `/suno/submit/MUSIC` | `mv:chirp-v4`, `make_instrumental:true` | 2 | Ch9 |

The two music tasks are polled together with one `/suno/fetch` request containing
both upstream task IDs. Both tasks reached `SUCCESS` and each returned two playable
WAV tracks plus PNG artwork.

## Request Semantics

- Image edit begins with AIGC direct-upload grant, MinIO upload, and confirmation.
  The final `images` value is a newly signed readable URL, not the stored
  `object://` reference.
- The OpenAI channel removes `response_format` for `gpt-image-2`; the other image
  models preserve `response_format:"url"`. Both are expected adapter behavior.
- Image assets are valid PNG files at the requested dimensions. Music assets are
  valid PCM WAV files. Mock asset downloads are excluded from request history.
- Mock history is persisted under the mounted trace directory and survives
  container replacement. The pre-existing 445 video records were restored before
  this run; the final history count is 464.

## Billing Baseline

| Model | Mode | Quota |
|---|---|---:|
| `gpt-image-2` | text, 2 images | 1,000,000 |
| `gpt-image-2` | edit, 1 image | 500,000 |
| each other image model | text, 2 images | 50,000 |
| each other image model | edit, 1 image | 25,000 |
| `chirp-v4` | vocal | 50,000 |
| `chirp-v4` | instrumental | 50,000 |

All image logs record Ch8. Music tasks 261 and 262 record Ch9, fixed price 0.1,
quota 50,000, and `per_call_billing:true`.

## Regression Fixed

The unified task billing path previously assumed every task adapter stored a video
`TaskSubmitReq`. Suno stores `SunoSubmitReq`, so music generation failed before the
upstream request with `invalid task request type`. Suno has no image inputs; the
relay now skips video image-input pricing for the Suno platform while preserving the
existing validation for video tasks.

## Verification

- `go test ./aigc/... ./relay/... ./cmd/ali-video-mock`
- Every Turn has AIGC, NewAPI, and Mock trace events.
- NewAPI and Mock containers are healthy.
- Mock history records 454-464 match the matrix above.
