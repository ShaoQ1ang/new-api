# Task 1.2 Result

- Changed: `aigc/execution/sync_request.go`
- Base: `3d5d16485401b9a833e8e30bea324a2589e849e4`
- Head: `68e997359fc9bb75b9485233575bbfb383c84afa`
- Commit: `68e997359 fix(aigc): structure image edit inputs`
- Acceptance: resolved AIGC image edits now marshal ordered objects containing
  `image_url` before generic relay; other paths are unchanged.
- Commands:
  - `go test ./aigc/execution -run TestBuildSyncRequestPreservesResolvedImageContract -count=1`
  - `go test ./aigc/execution -count=1`
- Result: both passed.
- Deviations or unresolved risks: none.
