# Task Briefing

- Objective: complete tasks 1.1 through 2.4 from the accepted task artifact.
- Project: `/Users/niuyouguo/go/src/new-api`
- Standards: `implementation/standards.md`
- Allowed scope: `web/default/src/features/system-settings/`, `web/default/src/features/pricing/types.ts`, `web/classic/src/pages/Setting/Ratio/`, and classic locale catalogs via scripted writes.
- Forbidden scope: backend billing, APIs, databases, unrelated frontend features, and the pre-existing `materialize-local-aigc-image-edits` change.
- TDD: required for map preservation and ancillary sync category behavior.
- Compatibility: fully compatible; retain backend `ImageInputPrice` contract.
- Acceptance: default UI no longer reads/writes the option; classic raw and visual editors support it only in per-request/video-seconds panels; sync preserves base pricing.
- Base commit: `c2db55d67bd435ca2b21137b352f16b8dbc0cb43`.
