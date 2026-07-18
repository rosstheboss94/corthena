# Phase 6 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

- Required: `AGENTS.md`, Phase 6 in `roadmap.md`, `ui/workspace-research.md`,
  `ui/foundation-async-effects.md`, `ui/visualization.md`,
  `data-and-features.md`, `quality-common.md`, `quality-concurrency.md`, and
  `quality-visualization.md`.
- Code: `src/corthena/ui/research/`, the UI client protocol, simulator,
  effects, shell renderer, capture helper, and focused tests.
- Build with `build-corthena-research-workspace`; verify with
  `verify-corthena-research-vertical-slice` and the visual-system verifier when
  rendering changes.
- Accepted scope: deterministic generation-bound OHLCV, feature, target,
  distribution, and paginated-row queries; linked six-panel behavior;
  cancellation, replay, lifecycle, benchmarks; and the retained 36-case
  manifest under `tests/goldens/research-golden/`.
- Exclude: durable catalog/repository behavior, experiment submission, training,
  model execution, and the real coordinator-backed client.
