# Phase 9 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

- Required: `AGENTS.md`, Phase 9 in `roadmap.md`,
  `ui/workspace-models.md`, `ui/workspace-inference.md`,
  `ui/foundation-async-effects.md`, `model-estimators.md`,
  `model-artifacts.md`, `inference.md`, `quality-common.md`,
  `quality-concurrency.md`.
- Conditional: `api.md` for public/process DTOs; `ui/visualization.md`
  and `quality-visualization.md` for chart/table/tree/golden work;
  `technology-stack.md` for dependencies.
- Code: typed ui client, state/actions/effects, simulator, Models and
  Inference panels, tree buffers, registry/alias paths, and tests.
- Build with `build-corthena-models-and-inference`; verify with
  `verify-corthena-models-and-inference`.
- Exclude: estimator fitting, artifact filesystem persistence, coordinator
  repositories, real HTTP endpoints, and unrelated workspaces.
