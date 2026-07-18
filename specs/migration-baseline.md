# Migration Parity Baseline

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-18
**Related:** [Python migration](python-migration.md), [Technology stack](technology-stack.md), [Quality](quality.md), [Phase 12 route](routing/phase-12.md)

This document records the evidence used to accept the Python/Cython root and UI
cutover. The retained manifests and PNGs under `tests/goldens/` are frozen
cutover evidence and regression references. The retired Go source is no longer
an active authority. JPEGs under `screenshots/` are inspection aids only.

## Root and Shell Ownership Map

Phase 12 completed the root scaffold and Phase 1--4 workstation shell cutover.
The current Python-owned evidence is:

| Python target | Current ownership | Retained baseline | Accepted evidence |
|---|---|---|---|
| project packaging and entry point | repository root and `src/corthena/workstation` | focused launch tests | reproducible environment and hidden shell launch |
| workstation lifecycle and UI-thread adapter | `src/corthena/ui/native` and lifecycle modules | `tests/goldens/phase1to4-golden/` | locked UI-thread lifecycle and cleanup tests |
| shell state, navigation, controls, and status | `src/corthena/ui` | Phase 1--4 manifest and PNGs | deterministic action replay and accepted visual evidence |
| simulator-backed startup shell | UI client, simulator, and effects modules | Phase 2 scenario | deterministic seeded startup state |
| docking, preferences, and layout persistence | UI docking, controls, persistence, and effects modules | Phase 4 scenario | round-trip persistence, recovery, and accepted cutover evidence |

Coordinator, worker, durable catalog/import, real model execution, and other
backend ports remain outside Phase 12 and are not implied by UI cutover.

## Phase 5 and Phase 5b Ownership Map

| Python target | Current ownership | Retained tests/baselines | Required evidence |
|---|---|---|---|
| chart and table numerical kernels, LOD, cache/workers, and virtualization foundation (Phase 5) | Python UI visualization, table, native adapter, and effects modules | chart/table unit, property, service, and benchmark evidence | deterministic numerical behavior, proportional work, bounded lifecycles, and immutable publication |
| generic rendering, interactions, cross-scope request/pagination parity, and six-case visual acceptance (Phase 5b) | Python UI visualization, table, native rendering, effects, and capture modules | interaction, race, lifecycle, and `tests/goldens/phase5-golden/` evidence | functional and replay behavior plus decoded-RGBA comparison against retained cutover PNGs |

## Phase 6--9 Simulator UI Ownership Map

| Phase | Python ownership | Retained baseline | Accepted evidence |
|---|---|---|---|
| 6 Research | `src/corthena/ui/research`, client, simulator, effects, and renderer | `tests/goldens/research-golden/` | deterministic linked queries, leakage checks, replay, lifecycle tests, benchmarks, and 36-case manifest |
| 7 Data and Experiments | `src/corthena/ui/data_experiments`, client, simulator, effects, and renderer | `tests/goldens/phase7-golden/` | typed imports/drafts/submission, replay, lifecycle tests, benchmarks, and 60-case manifest; recorded high-scale visual drift remains follow-up evidence |
| 8 Jobs and Results | `src/corthena/ui/jobs_results`, client, simulator, effects, and renderer | `tests/goldens/phase8-golden/` | deterministic virtual jobs, lifecycle commands, comparisons, replay, benchmarks, and 60-case manifest |
| 9 Models and Inference | `src/corthena/ui/models_inference`, client, simulator, effects, and renderer | `tests/goldens/phase9-golden/` | immutable simulated registry/inference workflows, replay, benchmarks, and 66-case manifest |

## Parity Criteria

A migrated surface is accepted when its applicable criteria pass or an explicit
cutover acceptance records the remaining deviation:

- Functional parity: the same supported inputs produce the same user-visible
  state transitions, validations, persisted layout behavior, and lifecycle
  outcome.
- Deterministic-replay parity: repeating the recorded seed, fixed clock,
  workspace/scenario, layout state, and action sequence produces equivalent
  state and capture evidence independent of worker completion order.
- Visual parity: the Python lossless capture passes the manifest-owned
  first-party pixel comparison against its retained cutover PNG baseline.

Record accepted deviations next to the implementation evidence with the
affected scenario, reason, and acceptance owner. Update the owning
specification for every durable behavior change; add an ADR only when it is a
durable architectural decision with meaningful alternatives.

## Canonical Golden Inputs

The canonical regression inputs are the retained lossless PNGs and versioned
JSON manifests under `tests/goldens/`. Phase 1--4 uses
`tests/goldens/phase1to4-golden/`, Phase 5b uses
`tests/goldens/phase5-golden/`, and Phase 6--9 use the `research-golden`,
`phase7-golden`, `phase8-golden`, and `phase9-golden` directories. The JPEGs
under `screenshots/` remain quick visual references.

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
