# Plan Review Decision

Decision: revise

The user clarified that image input normalization belongs to New API's AIGC
compatibility layer after the AIGC public model and generation mode resolve,
and before the request enters generic New API relay logic. Generic relay and
provider adapters must not know that the request originated from AIGC.

## Finding Dispositions

- PLAN-001: superseded. The revised design no longer describes conversion for
  a selected physical channel. `aigc/execution` produces the canonical New API
  image-edit request for every resolved AIGC `image_edit` execution; downstream
  relay retains its existing channel responsibilities.
- PLAN-002: remediate. Add an executor-level assertion over the serialized body
  and `/v1/images/edits` route received by the relay workflow.
- PLAN-003: remediate. Add explicit text-to-image assertions for
  `/v1/images/generations` and omission of `images`.
