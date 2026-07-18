---
name: verify-corthena-ui-state-and-simulator
description: Verify Phase 2 typed ui state, pure reducers, bounded asynchronous effects, UIClient separation, deterministic simulator replay, generations, cancellation, and lifecycle safety. Use for Phase 2 acceptance, regression testing, or audits.
---

# Verify Corthena UI State and Simulator

Produce repeatable evidence that Phase 2 is typed, deterministic, bounded, and
safe to replace with a real backend later.

## Ground verification

1. Read `AGENTS.md` and
   `specs/routing/phase-2.md`, then every required spec in the
   route.
2. Inspect state, actions, effects, client contracts, simulator, worker/queue
   ownership, and tests before selecting checks.
3. Stay read-only for an audit. Add tests only when implementation or explicit
   acceptance-gap closure is in scope.

## Verify types and reducer behavior

- Exercise every closed action and effect variant, invalid discriminator, and
  invariant default. Assert state and published collections remain immutable.
- Use property tests for reducer purity, repeatability, stable ordering, and
  invariants under generated valid action sequences.
- Replay identical seeds, fixed clocks, and action sequences while varying
  worker counts and completion orders. Require identical state, effects,
  requests, simulator responses, and request identities.
- Fail if ui state, reducers, or future panels import simulator packages,
  expose simulator values, or branch on simulator implementation details
  instead of the narrow `UIClient` contract.

## Verify concurrency and lifecycle

- Saturate every bounded queue. Prove render-thread sends do not block,
  replaceable effects coalesce or report typed busy state, and draining is
  bounded.
- Inject stale, duplicate, unknown, and wrong-generation completions. Prove
  they cannot mutate current state or satisfy a newer request.
- Cancel before dispatch, during work, after completion, and during shutdown.
  Verify bounded waits, queue closure, idempotent cleanup, and no surviving
  tasks or threads.
- Assert ownership is explicit for each worker, task, queue, sender, receiver,
  and closer; do not accept the GIL as synchronization.

## Conclude the gate

- Run the focused state/reducer, client, simulator, replay, saturation,
  generation, cancellation, shutdown, and leak suites.
- Run the migration-baseline Phase 2 seeded startup scenario without requiring
  Phase 3 rendering. Confirm repeated and reordered runs yield identical state.
- Run every applicable configured common and concurrency gate. Report exact
  commands, completion-order coverage, skipped checks, and residual risks.
  Do not mark Phase 2 complete while any route evidence or required gate is
  missing.
