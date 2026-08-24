# Task 1.1 Briefing

- Objective: require ordered `images[].image_url` objects in the AIGC sync
  request builder regression test.
- Task artifact: `openspec/changes/fix-aigc-image-edit-upstream-format/tasks.md`
- Standards: `openspec/changes/fix-aigc-image-edit-upstream-format/implementation/standards.md`
- Allowed scope: `aigc/execution/sync_executor_test.go` and this attempt evidence.
- Forbidden scope: production code, generic relay, provider adapters, video code.
- TDD: confirm the focused test fails for the current string-array output.
- Base commit: `8806b2def`
- Validation: `go test ./aigc/execution -run TestBuildSyncRequestPreservesResolvedImageContract -count=1`
