## 1. Default UI rollback

- [x] 1.1 Remove `ImageInputPrice` fields, snapshots, synchronization, and controls from default model pricing settings without changing backend support or unrelated image pricing.

## 2. Classic UI support

- [x] 2.1 Add a tested classic helper that parses and updates per-model `ImageInputPrice` maps while preserving unrelated models.
- [x] 2.2 Add `ImageInputPrice` to classic raw ratio settings and option submission.
- [x] 2.3 Add default/1K/2K/4K/8K prices and `free_count` to the classic visual model pricing editor load, validation, conflict, reset, and save paths; render them only for per-request and video-seconds modes, never per-token or tiered-expression modes.
- [x] 2.4 Include `image_input_price` in classic upstream ratio synchronization as an ancillary field that preserves `ModelPrice`, `VideoSecondsPrice`, ratios, billing mode/expression, and all unselected models.

## 3. Verification

- [x] 3.1 Run `bun test` with explicit classic helper test files covering map preservation, clearing, editor serialization, and surcharge synchronization alongside both per-request and video-seconds base pricing.
- [x] 3.2 Run classic i18n sync/lint, touched-file Prettier and ESLint checks, and the classic production build.
- [x] 3.3 Run default i18n sync, lint, typecheck, format check for touched files, and the default production build.
