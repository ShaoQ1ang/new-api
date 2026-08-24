# aigc-image-edit-relay Specification

## Purpose
Define how New API normalizes AIGC image inputs after mode resolution so image
edits enter generic relay in the canonical structured JSON format without
exposing channel-specific syntax to AIGC callers.
## Requirements
### Requirement: Preserve generic AIGC image inputs
New API SHALL accept AIGC image inputs as ordered `{role, url}` media items
without requiring callers to know the selected upstream image channel format.

#### Scenario: AIGC submits multiple source images
- **WHEN** an AIGC image generation request contains multiple source image URLs
- **THEN** New API preserves the URL order while resolving the request as an image edit

### Requirement: Build structured JSON image-edit inputs
The New API AIGC compatibility layer SHALL, after resolving an AIGC image
request to `image_edit`, represent every source URL as an object containing the
`image_url` field before the request enters generic relay.

#### Scenario: Relay two image URLs
- **WHEN** New API builds an image-edit relay request from two AIGC source images
- **THEN** the outbound `images` array contains two ordered objects whose `image_url` values match the source URLs

#### Scenario: Generic relay receives the normalized request
- **WHEN** the AIGC compatibility layer invokes generic relay for an image edit
- **THEN** generic relay receives `/v1/images/edits` with the normalized structured `images` array and requires no AIGC-specific conversion

### Requirement: Preserve image generation routing
New API SHALL continue routing AIGC requests without source images to
`/v1/images/generations` and SHALL omit image-edit inputs from those requests.

#### Scenario: Relay text-to-image request
- **WHEN** an AIGC image request contains no source images
- **THEN** New API builds an `/v1/images/generations` request without an `images` field
