## Why

AIGC image-edit requests reach New API successfully but New API forwards URL
inputs to an OpenAI-compatible `/v1/images/edits` upstream in an incompatible
JSON shape. The upstream rejects the request before it can access the images.

## What Changes

- Preserve the generic `/v1/aigc/generations` media contract using
  `inputs.images[] = {role, url}`.
- After resolving an AIGC public image model to `image_edit`, convert URL inputs
  into New API's canonical image-edit request before entering generic relay.
- Preserve text-to-image behavior and existing video provider conversions.
- Add regression coverage for multiple image-edit URL inputs and outbound
  request shape.

## Capabilities

### New Capabilities

- `aigc-image-edit-relay`: Relay AIGC image-edit URL inputs through the selected
  image channel using a valid upstream request representation.

### Modified Capabilities

None.

## Impact

- Affected code: `aigc/execution` compatibility conversion and focused Go tests.
- Public AIGC request and response contracts remain unchanged.
- No database, billing, authorization, frontend, or video protocol changes.
