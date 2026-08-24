## ADDED Requirements

### Requirement: Classic raw input image price configuration
The classic administration frontend SHALL expose the backend `ImageInputPrice` JSON option and SHALL submit valid edits through the existing option API.

#### Scenario: Administrator edits raw configuration
- **WHEN** an administrator updates valid `ImageInputPrice` JSON in classic ratio settings
- **THEN** classic submits the new value under the `ImageInputPrice` option key

### Requirement: Classic per-model input image surcharge editor
The classic visual model pricing editor SHALL load and save per-model input image prices for the default, 1K, 2K, 4K, and 8K tiers and the free input image count. The controls SHALL be displayed only for per-request and video-seconds billing modes and SHALL be absent from per-token and tiered-expression modes.

#### Scenario: Administrator configures a model surcharge
- **WHEN** an administrator saves input image tier prices or a free-image count for one model
- **THEN** classic updates that model in `ImageInputPrice` while preserving configurations for all other models

#### Scenario: Administrator clears a model surcharge
- **WHEN** an administrator clears every input image price and the free-image count for one model
- **THEN** classic removes only that model's `ImageInputPrice` entry

#### Scenario: Administrator switches billing mode
- **WHEN** an administrator selects per-token or tiered-expression billing
- **THEN** classic hides the input image surcharge controls without changing the stored configuration until the model is saved

### Requirement: Classic upstream price synchronization
The classic upstream ratio synchronization interface SHALL include `image_input_price` data in comparison and selected updates. It SHALL treat the field as an ancillary surcharge and SHALL preserve the model's base price, video-seconds price, ratios, billing mode, and billing expression.

#### Scenario: Upstream provides input image prices
- **WHEN** synchronized pricing contains `image_input_price` for a selected model
- **THEN** classic displays the difference and can save the selected value as `ImageInputPrice`

#### Scenario: Surcharge is synchronized for a priced model
- **WHEN** an administrator selects only `image_input_price` for a per-request or video-seconds model
- **THEN** classic updates only that model's surcharge and preserves its existing base-pricing configuration and every unselected model

### Requirement: Default frontend omits input image surcharge settings
The default administration frontend SHALL NOT expose or modify `ImageInputPrice` through its model pricing settings.

#### Scenario: Administrator uses default model pricing settings
- **WHEN** an administrator opens or saves model pricing in the default frontend
- **THEN** no input-image-surcharge control is displayed and the save operation does not write `ImageInputPrice`
