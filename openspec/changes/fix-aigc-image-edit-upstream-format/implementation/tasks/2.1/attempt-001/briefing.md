# Task 2.1 Briefing

- Objective: verify the path and serialized request body handed from the AIGC
  compatibility executor to generic relay for image edit and text-to-image.
- Allowed scope: `aigc/execution/sync_executor_test.go` and this attempt evidence.
- Forbidden scope: production code, generic relay, provider adapters, video.
- Acceptance: edit body contains ordered `images[].image_url`; generation body
  omits `images`; both paths are exact.
- Base commit: `68e997359fc9bb75b9485233575bbfb383c84afa`
- Validation: focused image executor tests and full `aigc/execution` tests.
