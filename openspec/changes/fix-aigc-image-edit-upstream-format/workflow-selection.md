# Workflow Selection

## Selection

- Tier: lightweight
- Requirements mode: fast
- Planning mode: serial
- Execution path: main-agent-serial
- TDD: enabled
- Compatibility: fully compatible
- Minimal implementation gate: skipped
- Final review: code

## Rationale

The failure is confined to the New API synchronous image-edit relay path. The
generic `/v1/aigc/generations` media contract remains unchanged, video adapters
remain unchanged, and the fix requires no persistence, billing, authorization,
deployment, or cross-repository coordination. Deterministic request-conversion
tests can protect the affected boundary.
