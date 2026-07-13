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
| 1 | [Strict Raylib frontend scaffold](routing/phase-1.md) | Complete; the packaged assets, typed adapter, UI-thread checks, bounded native smoke launch, cleanup tests, and configured gates pass. |
| 2 | [Typed frontend architecture and simulator](routing/phase-2.md) | Complete; typed immutable state, closed actions/effects, deterministic simulation, bounded workers, replay, cancellation, saturation, and lifecycle evidence pass. |
| 3 | [Application shell](routing/phase-3.md) | Complete; the Go-equivalent Raylib shell, typed interactions, bounded lifecycle, raw-RGBA capture, and manifest-owned PNG parity pass. |
| 4 | [Docking, controls, settings, responsive persistence](routing/phase-4.md) | Complete by user acceptance; live immutable workspace docking, directional drop targets and previews, splitter interaction, revisioned persistence, responsive geometry, automated quality gates, and manual docking validation pass. A revised split-layout golden remains follow-up evidence. |
| 5 | [Charts and tables foundation](routing/phase-5.md) | Complete; accepted typed numerical kernels, clipping, LOD, virtualization, bounded generation-safe cache/workers, immutable publication, lifecycle checks, proportional-work evidence, and benchmarks are delivered. |
| 5b | [Visualization acceptance and visual parity](routing/phase-5b.md) | Complete; generic Raylib layers and typed interactions, bounded cross-scope request and pagination-worker parity, the reviewed six-case legacy Go golden family, exact Python decoded-RGBA comparisons, Windows/build gates, and final audits pass. |
| 6 | Research vertical slice | Pending; Phase 5b prerequisite is satisfied. Implement linked panels, deterministic scenarios, leakage checks, replay, benchmarks, and the 36-image matrix. |
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

## Phase 1 route and done condition

Use the [Phase 1 task route](routing/phase-1.md) with these required skills, in
order:

1. `$build-corthena-raylib-frontend`
2. `$python-best-practices`
3. `$python-windows-compat-gate`
4. `$review-corthena-code`

Phase 1 is complete only when the strict empty workstation scaffold has a
named project entry point and frontend package; validates bundled fonts and
icons before native initialization; contains Raylib, Raygui, and Windows
values in a typed adapter; locks and enforces the UI OS thread for every native
call; renders at least one frame in a bounded smoke launch; and shuts down
cleanly. Focused adapter tests and every applicable configured quality and
Windows compatibility gate must pass. Phase 1 does not include typed shell
state, effects, docking, workspaces, charts, simulator behavior, or domain
workflows from Phase 2 or later.

## Phase 2 route and done condition

Use the [Phase 2 task route](routing/phase-2.md) with these required skills, in
order:

1. `$build-corthena-frontend-state-and-simulator`
2. `$python-best-practices`
3. `$verify-corthena-frontend-state-and-simulator`
4. `$review-corthena-code`

Phase 2 is complete only when immutable typed frontend state, closed action and
effect variants, a pure deterministic reducer, a narrow `FrontendClient`, a
bounded effects runtime, and the seeded simulator are implemented behind typed
boundaries. Identical seeds and action sequences must replay to identical
state across completion orders; stale generations must be rejected; and
cancellation, queue saturation, bounded draining, and shutdown must have
focused leak-free evidence. The simulator-backed Phase 2 startup scenario and
every applicable configured common and concurrency gate must pass. Phase 2 is
complete with that implementation and evidence in place. It does
not include the Phase 3 visual shell, docking, persistence, workspace
workflows, charts, tables, or real coordinator behavior.

## Phase 3 route and done condition

Use the [Phase 3 task route](routing/phase-3.md) with these required skills, in
order:

1. `$build-corthena-application-shell`
2. `$python-best-practices`
3. `$verify-corthena-application-shell`
4. `$python-windows-compat-gate`
5. `$review-corthena-code`

Phase 3 is complete only when the deterministic visual application shell
composes workspace-tab navigation, global context and component status,
central non-docking content, a status bar, and inert modal and toast overlay
layers from immutable `AppState`. Rendering must emit typed `UIAction` values,
use a stable render order, remain nonblocking, and keep domain, simulator,
persistence, and native values outside render-neutral shell code. A hidden
launch must drive the Phase 2 simulator/effects lifecycle, drain results within
the per-frame bound, render the named Phase 3 scenario, and clean up without
leaks. Deterministic replay, UI-thread enforcement, fault-injected launch and
render cleanup, every applicable common/concurrency/visual and Windows gate,
and manifest-owned PNG parity against the legacy
`phase3_application_shell.png` must pass. The complete Go shell surface is
ported, including typed Settings, command-palette, scale, context, and
panel-selection actions. Docking mutation algorithms, reusable controls,
persisted preferences, responsive layout policy, and layout persistence and
recovery remain Phase 4.

## Phase 4 route and done condition

Use the [Phase 4 task route](routing/phase-4.md) with these required skills, in
order:

1. `$build-corthena-docking-and-persistence`
2. `$build-corthena-raylib-visual-system`
3. `$python-best-practices`
4. `$verify-corthena-docking-and-persistence`
5. `$verify-corthena-raylib-visual-system`
6. `$python-windows-compat-gate`
7. `$review-corthena-code`

Phase 4 is complete only when immutable typed dock trees support pure,
deterministic activation, reordering, movement, splitting/docking,
close/reopen, maximize/restore, and resizing while preserving stable IDs,
hidden-panel state, valid split collapse, ratio-based layouts, minimum extents,
and deterministic replay. Reusable controls must provide hierarchical IDs and
deterministic pointer, keyboard, focus, capture, clipping, and cancellation
behavior through shared visual primitives. Responsive evidence must cover
1280x720 and 1920x1080 at every applicable scale preset with live DPI times
preset scaling applied once.

Preferences and named layouts must remain separate versioned documents with
strict validation, atomic replacement, bounded and coalesced background saves,
stale-result rejection, corruption quarantine, fallback defaults, recovery,
migration, cancellation, and leak-free shutdown. Filesystem I/O remains off the
UI thread. Every applicable common, concurrency, visualization, Windows, and
project quality gate and manifest-owned PNG parity against
`phase4_dockable_data.png` must pass. Phase 4 remains Pending until all runtime
implementation and evidence exist, and excludes Phase 5 charts/tables and all
later domain workflows.

## Phase 5 route and completed foundation

Use the [Phase 5 task route](routing/phase-5.md) with these required skills, in
order:

1. `$build-corthena-charts-and-tables`
2. `$build-corthena-raylib-visual-system`
3. `$python-best-practices`
4. `$verify-corthena-visualization-performance`
5. `$verify-corthena-raylib-visual-system`
6. `$python-windows-compat-gate`
7. `$review-corthena-code`

Phase 5 is complete because the accepted foundation provides deterministic
typed `float64` transforms, inverse transforms, clipping, ticks, checked final
draw conversion, OHLCV-preserving and continuous-series LOD, row and column
virtualization, deterministic typed sort/filter/null behavior, stable row-ID
selection, a byte-bounded generation-safe LRU, bounded cancellable preparation
workers, immutable publication, and proportional-work instrumentation.

Hand-calculated, property, race, lifecycle, work-count, and benchmark evidence
covers the foundation's numerical boundaries, OHLCV and continuous-series
semantics, cache accounting and eviction, bounded worker saturation,
cancellation and stale generations, table windows and stable selection, and
leak-free shutdown. Full generic Raylib layer rendering and interaction wiring,
cross-scope request deduplication, pagination-worker parity, canonical golden
creation/comparison, and final acceptance audits are not Phase 5 completion
requirements; they are owned by Phase 5b.

## Phase 5b route and done condition

Use the [Phase 5b task route](routing/phase-5b.md) with its required skills in
the recorded order. Phase 5b is complete only when every generic visualization
layer is prepared and rendered through shared Raylib primitives; every required
pointer and keyboard interaction is routed through immutable typed state and
Phase 4 controls; cross-scope visualization requests and typed pagination
requests have independent watcher, deduplication, generation, cancellation,
saturation, stale-result, immutable-publication, and bounded-shutdown parity;
and paint, clip, hit-test, and capture rectangles are identical.

Legacy Go must supply the reviewed `phase5-golden` manifest and six lossless PNG
baselines at 1280x720 and 1920x1080 for 100%, 150%, and 200% scale. Python
captures must compare decoded RGBA against those exact baselines with channel
tolerance `3` and maximum differing-pixel ratio `0.002`. Every required layer,
interaction, replay, race, lifecycle, bounded-work, golden, common,
concurrency, visualization, Windows, formatting, linting, typing, test,
property, benchmark, vulnerability, Cython build/import, and finding-free final
review requirement must pass. Missing implementation, baseline, comparison, or
audit evidence keeps Phase 5b Pending and Phase 6 blocked. Phase 5b excludes
Research-specific queries, fixtures, linked-workspace behavior, and Phase 7+
domain workflows.

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
