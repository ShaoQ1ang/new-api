## 1. Image Edit Contract

- [x] 1.1 Update the AIGC synchronous request builder test to require ordered `images[].image_url` objects for multiple source URLs.
- [x] 1.2 Change `aigc/execution/BuildSyncRequest` to build the canonical structured image-edit inputs after mode resolution and before generic relay.

## 2. Verification

- [x] 2.1 Add executor-level tests that capture the generic relay path and serialized body for both image edit and text-to-image field omission.
- [x] 2.2 Run formatting plus focused and broader affected Go package tests, including existing video task request conversion coverage.
