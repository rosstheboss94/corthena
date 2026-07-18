# Phase 3 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

## Read first

- Required: `AGENTS.md`, `design-pattern.md`, Phase 3 in `roadmap.md`,
  `ui/foundation.md`, `ui/foundation-shell-state.md`,
  `ui/foundation-async-effects.md`, `migration-baseline.md`,
  `quality-common.md`, `quality-concurrency.md`, and
  `quality-visualization.md`.
- Conditional: `technology-stack.md` for dependency or tooling changes;
  `api.md` for deliberate public or process-boundary changes; and the owning
  persistence specification only if persistence behavior changes.

## Scope

- Compose the deterministic Phase 3 shell over the existing Phase 1 lifecycle
  and Phase 2 immutable state, reducer, client, effects, and simulator
  interfaces.
- Render workspace tabs, global context and component status, a central
  non-docking content host, status bar, and inert modal and toast overlays in a
  documented stable order.
- Project immutable `AppState` into typed render-neutral geometry and view
  models. Emit only closed typed `UIAction` values and handle them exhaustively.
- Keep Raylib/Raygui and other native values in the adapter. Keep domain,
  simulator, persistence, I/O, decoding, and blocking work out of render
  functions and off the UI thread.
- Drive simulator/effects startup during the hidden launch, drain results
  within the Phase 2 per-frame bound, capture the named scenario, and perform
  bounded idempotent cleanup on normal and injected-failure paths.

## Exclusions

The complete cutover shell surface includes typed
Settings, command-palette, context, scale, and panel-selection actions. Exclude
Phase 4 docking mutation algorithms, reusable controls, persisted preferences,
responsive layout policy, and layout persistence, recovery, or migration.
Also exclude Phase 5+ charts, tables, workspace workflows, and real
coordinator, network, repository, or domain behavior. Do not add a public API,
dependency, serialized schema, docking model, or persistence format for this
route unless its owning specification is deliberately revised.

## Required skill order

1. `$build-corthena-application-shell`
2. `$python-best-practices`
3. `$verify-corthena-application-shell`
4. `$python-windows-compat-gate`
5. `$review-corthena-code`

## Completion evidence

- Focused tests cover shell-region composition, typed state-to-view
  projection, exhaustive workspace-navigation actions, stable render order,
  and identical replay output for the recorded seed, clock, and action stream.
- Instrumented tests enforce the locked UI OS thread, nonblocking frames,
  bounded effect-result draining, adapter-local native values, and rejection
  of wrong-thread native calls.
- Hidden launches cover normal startup plus injected launch and render
  failures, bounded idempotent cleanup, and repeated-run resource baselines
  without task, thread, queue, handle, or native-resource growth.
- The Python lossless capture passes the first-party RGBA comparison against
  the retained `phase3_application_shell.png` using its recorded seed,
  fixed clock, viewport, scale, frame count, asset fingerprint, backend
  identity, and tolerances. Missing, skipped, JPEG, or manual evidence fails
  acceptance.
- Every applicable configured common, concurrency, visualization, Windows
  compatibility, formatting, linting, typing, test, vulnerability, and native
  build/import gate formed the acceptance evidence. Phase 3 is complete.
