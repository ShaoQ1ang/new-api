# Task 1.2 Briefing

- Objective: normalize resolved AIGC `image_edit` URLs into canonical
  `images[].image_url` objects before generic relay.
- Allowed scope: `aigc/execution/sync_request.go` and this attempt evidence.
- Forbidden scope: AIGC cross-service DTOs, generic relay, provider adapters,
  video, billing, and persistence.
- Compatibility: fully compatible outside the corrected image-edit shape.
- TDD: make the failing Task 1.1 test pass with the minimum production change.
- Base commit: `3d5d16485401b9a833e8e30bea324a2589e849e4`
- Validation: focused builder test and `go test ./aigc/execution -count=1`.
