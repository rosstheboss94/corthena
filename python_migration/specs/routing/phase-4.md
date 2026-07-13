# Phase 4 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

## Read first

- Required: `AGENTS.md`, `design-pattern.md`, Phase 4 in `roadmap.md`,
  `frontend/foundation.md`, `frontend/foundation-shell-state.md`,
  `frontend/foundation-async-effects.md`, `frontend/foundation-persistence.md`,
  `frontend/raylib-visual-system.md`, `migration-baseline.md`,
  `quality-common.md`, `quality-concurrency.md`, and
  `quality-visualization.md`.
- Conditional: `technology-stack.md` for dependency or tooling changes;
  `api.md` for deliberate public or process-boundary changes; and
  `frontend/visualization.md` only to enforce the Phase 5 exclusion.
- Legacy parity references: `internal/frontend/docking/`,
  `internal/frontend/controls/`, `internal/frontend/layouts/`,
  `internal/frontend/preferences/`, the layout and preference workers under
  `internal/frontend/effects/`, `internal/frontend/appstate/layout_reducer_test.go`,
  `internal/frontend/appstate/layout_validation.go`,
  `internal/frontend/appstate/preferences_test.go`, and
  `internal/frontend/nativeui/dock_shell_windows.go` with its tests.

## Scope

- Implement immutable typed dock trees and pure deterministic activate,
  reorder, move, split/dock, close/reopen, maximize/restore, and resize
  mutations with stable IDs, valid split collapse, hidden-panel state, ratio
  persistence, minimum extents, and deterministic geometry.
- Implement reusable controls with hierarchical widget IDs and explicit,
  deterministic hot, active, focus, capture, clipping, keyboard, pointer, and
  cancellation behavior through typed actions and shared visual primitives.
- Implement responsive layouts at 1280x720 and 1920x1080 with live DPI times
  preset scaling applied once, constrained-width modes, and identical final
  rectangles for painting, clipping, and hit testing.
- Persist global preferences separately from named layouts in versioned,
  atomically replaced documents. Use bounded/coalesced background saves,
  revision and stale-result checks, strict validation, corruption quarantine,
  fallback defaults, recovery, migration, cancellation, and bounded shutdown.
- Preserve the Phase 1-3 lifecycle, immutable state/effects, client/simulator,
  UI-thread, and shell boundaries. Keep filesystem I/O and native values out of
  render-neutral code.

## Exclusions

Exclude Phase 5 charts, tables, transforms, LOD, caches, and virtualization;
Phase 6+ workspace and domain workflows; and real coordinator, network,
repository, or training behavior. Do not add a dependency, public API, or
serialized schema outside the owning specifications.

## Required skill order

1. `$build-corthena-docking-and-persistence`
2. `$build-corthena-raylib-visual-system`
3. `$python-best-practices`
4. `$verify-corthena-docking-and-persistence`
5. `$verify-corthena-raylib-visual-system`
6. `$python-windows-compat-gate`
7. `$review-corthena-code`

## Completion evidence

- Focused and property tests cover every dock mutation and invariant, stable
  IDs, split collapse, hidden-panel state, minimum geometry, complete input
  routing, immutable publication, and deterministic replay.
- Persistence tests cover canonical round trips, revisions, migrations,
  corruption quarantine, fallback recovery, every atomic-write failure stage,
  queue saturation, coalescing, stale completions, cancellation, shutdown, and
  repeated lifecycle leak checks.
- Responsive evidence covers 1280x720 and 1920x1080 at every applicable scale
  preset. The Python lossless capture passes first-party RGBA comparison against
  manifest-owned `phase4_dockable_data.png` with unchanged inputs and tolerances.
- Every applicable configured common, concurrency, visualization, Windows,
  formatting, linting, typing, test, vulnerability, and native build/import gate
  passes, and final review has no unresolved findings. Keep Phase 4 Pending until
  the implementation and all evidence exist.
