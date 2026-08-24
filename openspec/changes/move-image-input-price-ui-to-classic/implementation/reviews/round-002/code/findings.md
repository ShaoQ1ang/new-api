# Closure Review Findings

Overall verdict: **advisory**

- `CODE-001` | **resolved** | `parseCurrentPricingOptions` includes `ImageInputPrice` at `modelPricingSyncFields.js:30`. Both normal synchronization at `UpstreamRatioSync.jsx:454` and conflict confirmation at `UpstreamRatioSync.jsx:1148` now use that parser. `performSync` clones the preserved map at line 552. The parser regression test is at `modelPricingSyncFields.test.js:28`.

- `CODE-002` | **resolved** | Before saving, the editor compares active models with original `ImageInputPrice` entries and assigns `null` deletion markers for removed models at `useModelPricingEditorState.js:1556`. The builder consumes that marker by deleting only the target entry at `modelPricingImageInputPrice.js:72`, preserving unrelated entries. The null-marker builder test is at `modelPricingImageInputPrice.test.js:94`.

- `CLOSURE-001` | **advisory** | The CODE-002 test manually supplies `{deletedModel: null}`. It protects builder behavior but does not exercise the editor-state comparison that creates the deletion marker. The implementation is correct, but that wiring could regress without failing this test.

All eight focused tests passed under `node --test`; `git diff --check` was clean. No affected-behavior regression was found.
