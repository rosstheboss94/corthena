# Phase 2 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

## Read first

- Required: `AGENTS.md`, `design-pattern.md`, Phase 2 in `roadmap.md`,
  `frontend/foundation.md`, `frontend/foundation-shell-state.md`,
  `frontend/foundation-async-effects.md`, `quality-common.md`,
  `quality-concurrency.md`, and `migration-baseline.md`.
- Conditional: `api.md` only for deliberate public or process contracts;
  `technology-stack.md` for dependency or tooling changes; and
  `quality-visualization.md` only if capture infrastructure changes.

## Scope

- Define immutable typed application state and closed, exhaustively handled
  `UIAction` and `UIEffect` variants with validated serialized discriminators.
- Keep the reducer pure and deterministic. Apply results in stable logical
  order rather than arrival order and publish immutable snapshots.
- Keep panels and future shell code behind a narrow typed `FrontendClient`;
  simulator-specific values and behavior must not cross that boundary.
- Run effects on owned workers through bounded typed queues. Render-thread
  sends remain nonblocking, replaceable work coalesces or reports typed busy
  state, and each frame drains a bounded result count.
- Provide a deterministic seeded simulator through the same client/effects
  path intended for the future coordinator. Internal demo behavior does not
  define coordinator HTTP endpoints.
- Carry request identity and generation explicitly. Reject stale and
  wrong-generation completions, support cancellation of superseded work, and
  provide bounded idempotent shutdown without task, thread, or queue leaks.
- Replay the same seed, fixed clock, and action sequence to identical state
  and emitted effects across worker counts and completion orders.

## Exclusions

Exclude the Phase 3 visual application shell, docking, layout or preference
persistence, workspace workflows, charts, tables, capture-baseline changes,
and real coordinator, network, repository, or domain behavior.

## Required skill order

1. `$build-corthena-frontend-state-and-simulator`
2. `$python-best-practices`
3. `$verify-corthena-frontend-state-and-simulator`
4. `$review-corthena-code`

## Completion evidence

- Type and property tests cover immutable state, every closed action/effect
  variant, reducer invariants, invalid discriminators, and stable ordering.
- Replay tests vary completion order and worker count while producing identical
  state, emitted effects, simulator responses, and request identities.
- Focused lifecycle tests cover queue saturation, nonblocking sends, bounded
  draining, wrong/stale-generation rejection, cancellation before and during
  work, queue closure, idempotent shutdown, and thread/task leak checks.
- Boundary tests prove frontend state and reducers depend on `FrontendClient`
  contracts and do not import, expose, or branch on simulator internals.
- The migration-baseline Phase 2 simulator-backed startup scenario passes with
  its recorded seed and fixed clock, without introducing the Phase 3 shell.
- Every applicable configured common and concurrency quality gate passes, and
  the final review reports no unresolved findings. Keep Phase 2 Pending until
  the future runtime implementation and all of this evidence exist.
