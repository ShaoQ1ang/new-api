## Why

Administrators of this deployment use the classic frontend, but the per-input-image surcharge can currently be configured only from the default frontend. The configuration UI must live in classic so the supported backend billing capability is available in the frontend that operators actually use.

## What Changes

- Remove the `ImageInputPrice` and input-image-surcharge controls from the default frontend only.
- Add `ImageInputPrice` to the classic raw ratio settings.
- Add per-model input image prices for default, 1K, 2K, 4K, and 8K tiers plus a free-image count to the classic visual pricing editor.
- Include `ImageInputPrice` in classic upstream pricing synchronization.
- Preserve the backend option, billing calculations, logs, and API behavior unchanged.

## Capabilities

### New Capabilities

- `classic-image-input-pricing`: Configure and synchronize per-input-image surcharge pricing from the classic administration frontend.

### Modified Capabilities

None.

## Impact

- Affects `web/default/src/features/system-settings/models/` by removing only `ImageInputPrice` UI plumbing.
- Affects classic ratio settings, model pricing editor state/components, upstream ratio sync, and frontend tests under `web/classic/src/pages/Setting/Ratio/`.
- Does not change backend APIs, persisted option names, billing behavior, or database schema.
