# Code Review Findings

Overall verdict: **blocking**

- `CODE-001` | **blocking** | Conflict-confirmed upstream sync can erase the entire existing `ImageInputPrice` map. The normal sync path loads `ImageInputPrice`, but the confirmation callback rebuilds `curRatios` without it at `UpstreamRatioSync.jsx:1168`. `performSync` then clones `currentRatios.ImageInputPrice` into an empty object at `UpstreamRatioSync.jsx:570` and writes every final option. Confirming any base-pricing conflict therefore deletes surcharge entries for all unselected models, even when `image_input_price` was not selected. This violates the ancillary-field and unselected-model preservation requirements. The helper-only tests do not exercise this request-building path.

- `CODE-002` | **important** | Deleting a model from the visual editor does not remove its `ImageInputPrice` entry. `deleteModel` removes the model from editor state at `useModelPricingEditorState.js:1287`, so it is absent from the update map. However, `modelPricingImageInputPrice.js:72` only updates models present in that map and preserves every absent original entry. After save and refresh, a model represented solely by `ImageInputPrice` reappears. The clearing test passes because it explicitly supplies the target with null fields, so it does not cover the editor deletion workflow.

The six focused helper tests pass under `node --test`. `bun` was unavailable in the review environment, so the reviewer could not rerun the project Bun commands.
