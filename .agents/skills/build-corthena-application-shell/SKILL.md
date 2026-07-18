---
name: build-corthena-application-shell
description: Build Corthena Phase 3's deterministic visual application shell over the completed Phase 1 lifecycle and Phase 2 ui state, effects, client, and simulator architecture. Use for implementing or changing Phase 3 shell composition, workspace-tab navigation, global context/status presentation, the central non-docking host, status bar, inert overlays, or the Phase 3 hidden-launch capture lifecycle; do not use for Phase 1 scaffolding, Phase 2 state architecture, or Phase 4 docking and persistence.
---

# Build the Corthena Application Shell

## Ground the implementation

1. Read `AGENTS.md` and every required document in
   `specs/routing/phase-3.md` before editing.
2. Treat the living specs and manifest-owned Go PNG as canonical. Update the
   owning spec first if behavior or a public contract changes.
3. Preserve the existing Phase 1 lifecycle and Phase 2 typed state, reducer,
   `UIClient`, effects, simulator, generation, cancellation, and drain
   boundaries. Do not duplicate or bypass them.

## Compose the fixed shell

- Project immutable `AppState` into immutable typed render-neutral geometry and
  view models. Keep native Raylib/Raygui values inside the adapter.
- Render workspace tabs, global context and component status, central
  non-docking content, status bar, inert modal layer, and inert toast layer in
  a stable explicit order.
- Emit closed typed `UIAction` values for navigation and shell interaction.
  Handle every action variant exhaustively and fail on invariant violations.
- Keep domain decisions, simulator branching, persistence, filesystem,
  network, decoding, and blocking work outside render functions. Never wait on
  effects, sends, receives, futures, locks, joins, or cleanup in a frame.
- Keep all Raylib/Raygui initialization, polling, rendering, capture, and
  shutdown on the locked UI OS thread. Make wrong-thread calls fail fast.

## Integrate lifecycle and failure behavior

- Drive Phase 2 simulator/effects startup through the existing client boundary.
  Drain no more than the configured result count per frame and reduce results
  before rendering the latest immutable state.
- Make launch and shutdown bounded and idempotent. Preserve the initiating
  failure while reporting cleanup failures, and release every owned native and
  background resource on launch, render, capture, and shutdown failures.
- Keep replay inputs and render order independent of scheduling, completion
  order, process/thread identity, dictionary order, and wall clock.

## Respect phase boundaries

Do not implement docking mutations, reusable controls, Settings behavior,
preferences, responsive scaling policy, or layout persistence/recovery; those
belong to Phase 4. Do not add Phase 5+ workspace, chart, table, or real-backend
behavior.

## Test and hand off

- Add focused tests for region composition, state-to-view projection,
  navigation actions, stable render order, bounded draining, wrong-thread
  rejection, deterministic replay, injected launch/render failures, cleanup,
  and repeated hidden-launch leak checks.
- Produce the named lossless Phase 3 capture using the manifest inputs. Never
  replace the Go PNG with a migration JPEG or loosen its tolerances.
- Run applicable configured checks honestly, then hand the completed change to
  `$verify-corthena-application-shell` before Windows compatibility and final
  review gates.
