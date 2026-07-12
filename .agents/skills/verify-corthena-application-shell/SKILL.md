---
name: verify-corthena-application-shell
description: Verify Corthena Phase 3's deterministic application shell, hidden-launch lifecycle, UI-thread ownership, and manifest-backed visual parity. Use for Phase 3 acceptance or regression testing of shell-region composition, workspace navigation, immutable state projection, bounded effect draining, failure cleanup, repeat-launch leaks, or comparison with phase3_application_shell.png; do not use for Phase 1 scaffold acceptance, Phase 2 architecture verification, or Phase 4 docking and persistence.
---

# Verify the Corthena Application Shell

## Ground the gate

1. Read `python_migration/AGENTS.md` and every required document in
   `python_migration/specs/routing/phase-3.md`.
2. Inspect the complete Phase 3 implementation, focused tests, and named
   manifest entry. Treat missing or skipped evidence as a failure, not a pass.
3. Use only configured project commands. Preserve failure artifacts and never
   rewrite the canonical Go baseline during verification.

## Verify shell behavior

- Assert exact composition and stable render order for workspace tabs, global
  context/status, central non-docking content, status bar, modal layer, and
  toast layer.
- Test immutable `AppState` to typed view/geometry projection and exhaustive
  typed workspace-navigation actions. Reject native values or simulator/domain
  decisions outside their adapters and boundaries.
- Replay the recorded seed, fixed clock, and action stream across varied effect
  completion orders. Require identical logical state, view models, actions,
  and capture output.
- Instrument frames to reject blocking work and prove each frame drains no
  more than the Phase 2 configured result bound.

## Verify ownership and cleanup

- Prove initialization, polling, rendering, capture, and native shutdown occur
  on the locked UI OS thread. Require wrong-thread calls to fail fast.
- Inject launch, render, capture, effect, and shutdown failures. Require bounded
  idempotent cleanup, preservation of the initiating failure, and return to the
  pre-launch resource baseline.
- Repeat hidden launches and assert no monotonic growth in tasks, threads,
  queues, handles, subscriptions, or native resources.

## Verify visual parity

- Capture the named Python Phase 3 scenario losslessly and compare it with the
  manifest-owned Go `phase3_application_shell.png` using the first-party RGBA
  comparator.
- Use the entry's seed, fixed clock, workspace/scenario, viewport, scale,
  hidden-frame count, asset fingerprint, backend identity, layout revision,
  channel tolerance, and maximum different-pixel ratio exactly.
- Reject a missing capture, skipped comparison, migration JPEG, manual waiver,
  unrecorded tolerance, or replacement baseline.

## Report acceptance

Run applicable common, concurrency, visualization, formatting, linting,
typing, test, vulnerability, Windows compatibility, and native build/import
checks. Report every command, result, failure, and unavailable check accurately.
Do not mark Phase 3 complete unless all route evidence exists and no Phase 4+
behavior has been absorbed.
