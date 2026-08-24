## Context

AIGC materializes private image objects into signed URLs and submits them to
New API through `/v1/aigc/generations` as `inputs.images[] = {role, url}`. New
API resolves requests with source images to `image_edit`, builds an internal
`dto.ImageRequest`, and relays it as JSON to `/v1/images/edits`.

The current AIGC compatibility builder serializes `ImageRequest.Images` as an
array of URL strings. The canonical New API JSON image-edit representation
requires each array item to expose `image_url`; the malformed request currently
passes through generic relay unchanged and is rejected upstream. Video requests
do not use this synchronous image builder; they already pass through provider
task adapters that map generic roles and URLs into provider-specific payloads.

## Goals / Non-Goals

**Goals:**

- Preserve the generic AIGC `{role, url}` media contract.
- Emit structured `images[].image_url` objects for AIGC JSON image edits.
- Preserve input order and support multiple source images.
- Protect text-to-image routing and video conversion from regressions.

**Non-Goals:**

- Changing AIGC request fields or object-storage materialization.
- Reworking direct client multipart image-edit requests.
- Changing video, audio, or music provider adapters.
- Adding a general per-channel image input format setting.

## Decisions

### Convert inside the New API AIGC compatibility layer

After the AIGC resolver identifies the public model as a synchronous image model
and selects `image_edit`, `BuildSyncRequest` will transform each generic image
URL into a small structured JSON item with an `image_url` field before assigning
`ImageRequest.Images`. The resulting `dto.ImageRequest` is New API's canonical
image-edit request and then enters the existing generic relay workflow.

Converting in AIGC was rejected because it would expose upstream channel syntax
through the cross-service contract. Converting in the generic image relay or a
provider adapter was also rejected because those layers must not depend on the
request having originated from AIGC.

### Preserve generic relay behavior

The compatibility layer will hand generic relay a fully normalized
`dto.ImageRequest`. No AIGC-specific branches, context flags, or transformations
will be added to `relay/image_handler.go` or provider adapters. Downloading every
signed URL and rebuilding multipart was rejected because it adds network I/O,
file limits, MIME handling, and expiry failure modes while not matching the
canonical JSON request used by this AIGC execution path.

### Test the observable request contract

Builder tests will assert that two input URLs become two ordered objects with
`image_url` fields. Executor-level tests will capture the actual request passed
to generic relay and assert its serialized body and `/v1/images/edits` route.
Text-to-image tests will explicitly assert `/v1/images/generations` and omission
of `images`; existing video tests remain the regression boundary for that path.

## Risks / Trade-offs

- [A downstream channel needs a provider-specific representation] -> Keep the
  AIGC compatibility output canonical and handle true provider differences in
  the existing channel adapter, without adding AIGC awareness there.
- [Signed URLs expire before the upstream fetches them] -> Existing AIGC
  materialization and submission timing remain unchanged; this change neither
  shortens nor extends URL validity.
