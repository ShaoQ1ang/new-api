# Implementation Standards

## Entry

- `AGENTS.md`

## Evidence

- Git revision: `fee625c2b03f646cbea4f28a2a9bc745c8abfe94`
- `AGENTS.md` SHA-256:
  `df586298c44bff82606f8c69178dd78cc84252840ebe442a72d83d73117491e0`

## Applicable Constraints

- Keep the change direct and within the existing AIGC execution boundary.
- Use `common.Marshal` and other project JSON wrappers; do not call the standard
  library JSON marshal/unmarshal functions directly.
- Preserve optional-field and existing relay behavior outside the declared
  AIGC image-edit compatibility contract.
- Add deterministic regression tests using `testify/require` for fatal checks
  and `testify/assert` for non-fatal assertions.
- Do not modify billing, database behavior, frontend code, provider adapters,
  project identity, or protected metadata.
