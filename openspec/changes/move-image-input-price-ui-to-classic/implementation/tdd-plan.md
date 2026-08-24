# TDD Plan

Strategy: strict, serial execution.

- Protect `ImageInputPrice` parsing, per-model updates, unrelated-model preservation, and clearing through the dedicated classic helper tests.
- Protect synchronization category behavior so the ancillary surcharge coexists with per-request and video-seconds base pricing while those base categories remain mutually exclusive.
- Verify UI integration through touched-file lint/format checks, type checking where applicable, i18n tooling, and both production builds.

The implementation began before the current workflow-state schema was adopted, so red/green command transcripts were not persisted retroactively.
