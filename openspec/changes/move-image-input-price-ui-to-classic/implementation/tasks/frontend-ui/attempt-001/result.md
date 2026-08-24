# Result

Implemented commits `c5586e842` and `3f8c265c3` from base `c2db55d67bd435ca2b21137b352f16b8dbc0cb43`.

Changed behavior:

- Removed the default settings UI and state plumbing for `ImageInputPrice` while retaining backend/catalog types needed outside settings.
- Added classic raw JSON configuration and visual default/1K/2K/4K/8K plus `free_count` fields for per-request and video-seconds modes.
- Added ancillary upstream synchronization that preserves base pricing and unselected models.
- Added seven classic locale translations and eight focused helper tests.

Verification:

- Classic helper tests: 8 passed.
- Classic i18n sync, touched-file Prettier, touched-file ESLint, and production build: passed.
- Classic repository-wide i18n lint: blocked by 558 pre-existing hardcoded-string findings; no new finding was reported for the added translated controls.
- Default i18n sync, touched-file oxlint, typecheck, touched-file format check, and production build: passed.
- Default repository-wide lint and format checks remain blocked by unrelated pre-existing findings; touched files pass both targeted checks.
- `git diff --check`: passed.
