---
name: build-corthena-frontend-state-and-simulator
description: Build Phase 2 typed frontend state, reducers, asynchronous effects, FrontendClient boundaries, and deterministic simulator behavior. Use for Corthena Phase 2 implementation or changes to its state/effect architecture and simulator lifecycle.
---

# Build Corthena Frontend State and Simulator

Implement the deterministic frontend architecture without introducing the
visual shell or real coordinator behavior.

## Ground the change

1. Read `python_migration/AGENTS.md` and
   `python_migration/specs/routing/phase-2.md`, then every required spec in the
   route.
2. Read conditional specs only when their trigger applies. Update the owning
   living spec before changing behavior or a public contract.
3. Inspect the existing frontend adapter, state, effects, client, simulator,
   and tests. Preserve unrelated changes and Phase 1 UI-thread guarantees.

## Build typed deterministic state

- Use concrete immutable dataclasses, enums, protocols, and validated DTOs.
  Avoid `Any`, untyped payloads, unchecked casts, and native values outside
  adapters.
- Define closed `UIAction` and `UIEffect` variants with validated serialized
  discriminators and exhaustive handling with an invariant-violation default.
- Keep reducers pure. Apply results in stable logical order, never arrival
  order, and publish immutable snapshots.
- Keep panels and reducers dependent on a narrow consumer-owned
  `FrontendClient`; do not import, expose, or type-switch on simulator details.

## Build effects and simulator ownership

- Put simulator preparation and blocking work on owned workers behind bounded
  typed queues. Keep UI-thread sends nonblocking and drain a bounded result
  count per frame.
- Document queue sender, receiver, closer, capacity, saturation behavior, and
  owner. Coalesce replaceable effects or publish typed busy state.
- Carry request identity, generation, seed, and fixed clock explicitly. Reject
  stale or wrong-generation completions before state mutation.
- Derive simulator output deterministically from stable inputs. Make results
  independent of worker count, scheduling, and completion order.
- Give every task and thread a cancellation path and bounded termination.
  Make closure and cleanup idempotent and prove no task, thread, or queue leak.

## Verify before handoff

- Add reducer/property, boundary, replay, completion-order, saturation,
  generation, cancellation, shutdown, and leak tests with the implementation.
- Exercise the migration-baseline Phase 2 seeded startup scenario without
  adding the Phase 3 shell, docking, persistence, workspaces, charts, or tables.
- Run `$python-best-practices`, then hand off to
  `$verify-corthena-frontend-state-and-simulator` and
  `$review-corthena-code`. Do not mark Phase 2 complete until all route evidence
  and applicable configured gates pass.
