# Migration Parity Baseline

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-12  
**Related:** [Python migration](python-migration.md), [Technology stack](technology-stack.md), [Quality](quality.md), [Phase 12 route](routing/phase-12.md)

This document defines the evidence required to accept a Python/Cython
implementation. The approved legacy implementation and its manifest-backed
tests are the canonical source of parity. Migration JPEGs are inspection aids
only and are never normative.

## Initial Phase 12 Ownership Map

Phase 12 is limited to project scaffolding and the Phase 1--4 workstation
shell. Every Python subsystem records its legacy ownership mapping, baseline
scenarios, parity tests, and any accepted deviations before it is accepted.

| Python target | Legacy reference area | Legacy tests/baselines | Required initial evidence |
|---|---|---|---|
| project packaging and entry point | workstation launch and packaging | workstation launch test | reproducible environment and a hidden shell launch |
| workstation lifecycle and UI-thread adapter | UI lifecycle and native adapter | lifecycle test and Phase 1--4 PNG manifest | one locked UI-thread lifecycle and cleanup test |
| shell state, workspace navigation, controls, and status | frontend state and shell | frontend state tests and Phase 1--4 PNG manifest | deterministic action replay and visual parity |
| simulator-backed startup shell | simulator and effects | simulator/effects tests and Phase 2 PNG scenario | deterministic seeded startup state |
| docking, preferences, and layout persistence | layouts, preferences, and drafts | persistence tests and Phase 4 PNG scenario | round-trip persistence, recovery, and visual parity |

Coordinator, worker, data catalog/import, experiment, job/result, model,
inference, and real-backend ports are outside Phase 12. Add their rows only
when their owning route begins.

## Parity Criteria

A port is accepted only when all applicable criteria below pass against the
named legacy baseline:

- Functional parity: the same supported inputs produce the same user-visible
  state transitions, validations, persisted layout behavior, and lifecycle
  outcome.
- Deterministic-replay parity: repeating the recorded seed, fixed clock,
  workspace/scenario, layout state, and action sequence produces equivalent
  state and capture evidence independent of worker completion order.
- Visual parity: the Python lossless capture passes the manifest-owned
  first-party pixel comparison against the legacy PNG baseline.

Record accepted deviations next to the implementation's evidence with the
affected legacy source/tests, scenario, reason, and acceptance owner. Update the owning
specification for every deviation; add an ADR only when it is a durable
architectural decision with meaningful alternatives.

## Canonical Golden Inputs

The canonical visual targets are lossless legacy PNGs and their versioned JSON
manifests. Phase 1--4 targets live in
`internal/app/workstation/testdata/phase1to4-golden/`. Phase 5--9 targets are
the existing manifest-backed matrices under
`internal/app/workstation/testdata/{research-golden,phase7-golden,phase8-golden,phase9-golden}/`.
The JPEGs in `python_migration/screenshots/` remain quick visual references.

Every manifest entry must own all capture inputs: seed; fixed UTC clock;
workspace and named scenario; viewport; UI scale; hidden-frame count; bundled
font and icon asset fingerprints; rendering/backend and dependency identity;
and layout name plus serialized layout/app-state revision. It must also name
the baseline PNG and record its baseline version.

## Pixel Comparison Policy

Use the first-party RGBA comparison implementation; capture work stays on the
UI thread and image encoding/comparison stays outside it. Tolerances belong to
each manifest entry, never to an unversioned test default. Carry forward the
legacy defaults of channel tolerance `3` and maximum different-pixel ratio
`0.002`. A different value requires a newly captured baseline entry that
records the reason for the exception.

Do not use JPEG, perceptual comparison, or manual inspection to waive a PNG
failure. Regenerate a baseline only for an intentional accepted visual change.
