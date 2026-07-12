# Frontend-First Implementation Roadmap

**Status:** In progress
**Owner:** Project  
**Last updated:** 2026-07-12

Build deterministic Python/Cython frontend workflows before the real coordinator
and research engine. Behavioral requirements live in owning
specifications; `specs/routing/phase-*.md` provides compact reading routes and
`python-migration.md` owns rewrite sequencing.

| Phase | Scope | Status |
|---|---|---|
| 0 | Python/Cython Windows compatibility | Complete; the clean evidence-producing gate passed for exact regular CPython `3.14.2` and `cp314-win_amd64`. |
| 1 | Strict Raylib frontend scaffold | Complete; commands, assets, typed adapters, UI-thread enforcement, smoke launch, and gates pass. |
| 2 | Typed frontend architecture and simulator | Pending; implement typed Python state, effects, and deterministic simulator behavior. |
| 3 | Application shell | Pending; implement the Python shell and hidden-launch lifecycle. |
| 4 | Docking, controls, settings, responsive persistence | Pending; implement typed layouts, preferences, recovery, replay, and scale behavior. |
| 5 | Charts and tables | Pending; implement transforms, LOD, cache, cancellation, virtualization, benchmarks, and golden scenarios. |
| 6 | Research vertical slice | Pending; implement linked panels, deterministic scenarios, leakage checks, replay, benchmarks, and the 36-image matrix. |
| 7 | Data and Experiments | Pending; implement catalog/import, validation, estimates, autosave, immutable submission, benchmarks, and the 60-image matrix. |
| 8 | Jobs and Results | Pending; implement the virtual queue, lifecycle controls, checkpoints, immutable comparisons, charts, stale-generation behavior, and the 60-image matrix. |
| 9 | Models and Inference | Pending; implement the immutable registry, transactional aliases, artifact provenance/tree validation, compatibility-gated scoring, rankings, distributions, history, export, cancellation, and the 66-image golden matrix. |
| 10 | Backend-swap readiness | Keep the simulator behind `FrontendClient`; add contract, cancellation, reconnect, reconciliation, stale-generation, adapter, reducer, lifecycle, and golden coverage. |
| 11 | Python/Cython foundation | In progress; authoritative specs, ADRs, technology stack, quality gates, entrypoint mapping, and screenshot baseline policy define the implementation before runtime code changes. |
| 12 | Python scaffold and initial shell | Pending; create the reproducible `uv` scaffold and Phase 1--4 shell only, then accept it through named PNG baselines and parity evidence. |

## Phase 9 done condition

The dummy workflow reaches a registered immutable model and compatible
historical/latest inference output through every required panel, with stable
alias transactions, deterministic replay, stale/cancellation safety, and the
applicable quality and golden gates.

## Phase 12 exit criteria

- A reproducible `uv` environment, `pyproject.toml`, lockfile, package layout,
  and named project entry point exist.
- The Windows compatibility-spike report selects the runtime, binding pair,
  toolchain, native behavior, and locked versions only after all required
  checks pass.
- The Python shell completes a hidden smoke launch with locked UI-thread
  ownership, bundled fonts/icons, lifecycle cleanup, and Cython build/import
  evidence.
- Named Phase 1--4 baseline scenarios pass functional, deterministic-replay,
  and manifest-owned PNG pixel-comparison parity against the approved legacy
  baseline.
- Formatting, linting, typing, tests, replay/lifecycle checks, and required
  Cython checks pass using the recorded project commands.

Package scaffolding alone never marks a phase ported.

## Global acceptance

- Runtime, build, tests, and extensions use Python, with Cython only for
  measured hot paths or native adapters.
- All workspaces use typed state and keep I/O, decoding, database, network,
  training, and simulator work off the render thread.
- Shared inputs are read-only; tasks own mutable outputs; completed runs and
  artifacts are immutable.
- Repeating a seed and action sequence produces identical state and screenshots.
- Formatting, linting, type checks, tests, lifecycle/concurrency checks,
  benchmarks where applicable, Cython builds, and vulnerability checks pass.

## After the frontend

Implement the Python coordinator, health, worker protocol, repositories,
imports, catalog revisions, typed features, targets, leakage-safe splits,
library-backed models, artifacts, checkpoints, evaluation, backtests, refits,
aliases, inference, and the real Python client behind the shared contract
suite.
