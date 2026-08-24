# Verdict: revise

The direction is feasible, but the approved plan does not encode one explicit placement decision and does not protect the upstream-sync path from treating `image_input_price` as an exclusive base-pricing category. Those gaps can produce an implementation that satisfies the task wording while violating the approved behavior or deleting an existing model's primary pricing configuration.

## Evidence inspected

- Review manifest and all four hash-verified artifact snapshots: `design.md`, `proposal.md`, the capability spec, and `tasks.md`.
- `plan-user-review/round-001/decision.md`, including its placement clarification.
- `web/default/AGENTS.md` and the `web/classic` / `web/default` package scripts.
- Existing classic editor state/component, raw ratio form, map helpers/tests, and upstream synchronization implementation under `web/classic/src/pages/Setting/Ratio/`.
- Existing default `ImageInputPrice` editor and synchronization plumbing under `web/default/src/features/system-settings/models/`.
- Backend references were searched only to confirm the persisted field shape; backend billing behavior was not reviewed because it is an explicit non-goal.

## Findings

### PLAN-001 - High - The approved mode-placement decision is absent from the plan and specification

**Conclusion:** Task 2.3 must state that the visual controls are rendered in both `per-request` and `video-seconds` panels and are absent from `per-token` and `tiered_expr` panels; the capability scenario or its acceptance criteria should encode the same behavior. The approved decision explicitly requires the primary controls in per-request and per-second billing panels and forbids them in token-ratio mode, while task 2.3 merely says to add fields to the visual editor. `web/classic/src/pages/Setting/Ratio/components/ModelPricingEditor.jsx` has four mutually exclusive billing-mode branches, so placement is an implementation choice that cannot be inferred safely from the current task. Without an explicit done criterion, a reviewer cannot distinguish a compliant placement from a control added to the wrong branch.

**Cited evidence:** `plan-user-review/round-001/decision.md` (Placement Clarification); task 2.3 in `tasks.md`; requirement "Classic per-model input image surcharge editor" in `specs/classic-image-input-pricing/spec.md`; `web/classic/src/pages/Setting/Ratio/components/ModelPricingEditor.jsx`.

### PLAN-002 - High - Upstream synchronization lacks an ancillary-field safety contract

**Conclusion:** Task 2.4 and task 3.1 must require `image_input_price` to be synchronized independently of the mutually exclusive base-pricing categories, preserving the selected model's `ModelPrice`, `VideoSecondsPrice`, ratios, billing mode/expression, and all unselected models. Add deterministic cases for synchronizing the surcharge alongside a per-request model and alongside a video-seconds model. Classic synchronization currently classifies fields as `price`, `video`, `tiered`, or `ratio` and deletes fields from other categories in `selectValue` / `performSync`. A naive addition to `ratioSyncFields` or `getBillingCategory` can therefore erase the base price that the surcharge is meant to supplement. The current phrase "comparison and selected update paths" and generic "synchronization helpers" test criterion do not guard this risk.

**Cited evidence:** task 2.4 and task 3.1 in `tasks.md`; requirement "Classic upstream price synchronization" in `specs/classic-image-input-pricing/spec.md`; `getBillingCategory`, `selectValue`, and `performSync` in `web/classic/src/pages/Setting/Ratio/UpstreamRatioSync.jsx`; the design goal to preserve unrelated pricing behavior.

### PLAN-003 - Medium - Verification omits required static checks and an actionable test command

**Conclusion:** Task 3.2 should include lint/static checks for every touched frontend and default TypeScript typechecking (prefer `bun run build:check`, or explicitly `bun run typecheck` plus `bun run build`). It should also name the focused classic test command/file set and include classic i18n validation for newly introduced labels. `web/default/AGENTS.md` requires typecheck and lint after TS/TSX changes, but `bun run build` is only `rsbuild build` and does not run `tsgo`; classic has no package-level `test` script despite task 3.1 saying to run focused tests. As written, verification can be marked complete while required checks were never run or while the test invocation remains ambiguous.

**Cited evidence:** task 3.1 and task 3.2 in `tasks.md`; `web/default/AGENTS.md` sections 3.1, 3.2, and 3.16; scripts in `web/default/package.json` and `web/classic/package.json`.

## Unassessed material areas

- No implementation exists yet, so no browser interaction, visual placement, save request, or synchronization request was exercised.
- Locale catalog completeness and exact new copy were not audited.
- Backend billing calculations, API contracts, and database behavior were not reviewed because the proposal declares them unchanged.

## Needs confirmation

- Confirm whether `tiered_expr` should also omit the visual surcharge controls. The approved note explicitly forbids token-ratio mode and says controls belong in per-request and per-second panels, which implies omission from `tiered_expr`; the revised plan should make that implication explicit.
- Confirm the intended focused test runner syntax for classic (for example, `bun test` with explicit helper test files), since `web/classic/package.json` has no `test` script.

## Commands and limits

- Exploration: 8 command batches (artifact/decision reads, convention/script inspection, and bounded `rg`/`sed` inspection of relevant classic/default code). Limit reached; no further exploration performed.
- Validation: 1 command. `shasum -a 256` verified all four snapshots against the manifest hashes. Limit reached; no code tests were run because this is a pre-implementation plan review.
