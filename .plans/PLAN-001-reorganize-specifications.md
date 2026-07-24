# PLAN-001: Reorganize Specifications by General and Page Ownership

## Goal and Success Criteria

Reorganize and consolidate Corthena's specifications so every living,
authoritative rule has one discoverable owner:

- cross-cutting product, architecture, runtime, API, UI-system, and quality
  rules live under `specs/general/`;
- end-to-end workflow rules live under `specs/pages/<page>/` for Data,
  Research, Experiments, Jobs, Results, Models, and Inference; and
- ADRs, missing-change records, examples, and accepted historical evidence are
  visibly separated from living specifications.

The migration succeeds when:

- `specs/README.md` is the single root entry point and indexes every
  authoritative general and page specification;
- a task can start with `specs/general/contract.md`, follow `AGENTS.md`, and
  reach the minimum general and page-owned context without consulting retired
  phase routes;
- each normative rule occurs in one owning document and every consumer uses a
  link instead of restating that rule;
- the current working-tree versions of all specifications, including
  uncommitted Data-ingestion additions, are preserved or intentionally
  consolidated with traceable ownership;
- all validated behavior in the three current `specs/missing/` notes is
  incorporated into authoritative Data, Research, or Experiments documents,
  after which those three notes are removed;
- all repository-local links and literal specification paths resolve, all
  authoritative documents have consistent metadata, and no active route points
  at an old path; and
- no runtime behavior, Python/Cython public interface, dependency, persisted
  data format, or test baseline changes as a side effect of the documentation
  migration.

## Referenced Specifications

- `AGENTS.md` governs mandatory reading order, specification routing,
  missing-change handling, and required quality gates. Its routing table and
  paths must be migrated atomically with the specifications.
- `specs/contract.md` governs the mandatory agent-facing contract format and
  minimum-context routing. Its active requirements become
  `specs/general/contract.md`.
- `specs/design-pattern.md` governs modular-monolith boundaries, dependency
  direction, and single-owner specification maintenance. Its active
  requirements become `specs/general/design-pattern.md`.
- `specs/README.md` is the current ownership index and authority policy. It
  becomes the root taxonomy and navigation entry point.
- `specs/product.md` and `specs/roadmap.md` govern product scope, terminology,
  delivery sequence, and the current Data-ingestion milestones.
- `specs/system-architecture.md`, `specs/concurrency-and-parallelism.md`,
  `specs/technology-stack.md`, and `specs/api.md` govern cross-cutting process,
  storage, runtime, dependency, transport, client, worker-protocol, and health
  rules.
- `specs/quality.md`, `specs/quality-common.md`,
  `specs/quality-concurrency.md`, and `specs/quality-visualization.md` govern
  common and specialist verification.
- `specs/ui/foundation*.md`, `specs/ui/workspaces*.md`,
  `specs/ui/raylib-visual-system.md`, and `specs/ui/visualization.md` govern
  shared shell, state, effects, persistence, navigation, workspace, rendering,
  visualization, and responsive behavior.
- `specs/data-and-features.md` and `specs/ui/workspace-data.md` govern current
  ingestion, catalog, dataset, feature, target, materialization, scheduling,
  provenance, and Data-workspace behavior.
- `specs/ui/workspace-research.md` and
  `specs/ui/workspace-experiments.md` govern Research and Experiments workflow
  behavior and their typed `UIClientProtocol` boundaries.
- `specs/training-runtime.md` and `specs/ui/workspace-jobs.md` govern
  experiment/sweep execution, job lifecycle, scheduling, CPU leases,
  checkpoints, and Jobs-workspace behavior.
- `specs/evaluation.md`, `specs/evaluation-and-inference.md`, and
  `specs/ui/workspace-results.md` govern evaluation, reference backtests, and
  Results-workspace behavior.
- `specs/models.md`, `specs/model-estimators.md`,
  `specs/model-artifacts.md`, `specs/inference.md`,
  `specs/ui/workspace-models.md`, and `specs/ui/workspace-inference.md` govern
  estimators, artifacts, registry and aliases, scoring, export, and the Models
  and Inference workspaces.
- `specs/missing/declarative-dataset-feature-workflow.md`,
  `specs/missing/milestone-1a-simulator-ui-contract.md`, and
  `specs/missing/milestone-1b-real-data-ingestion.md` are user-approved sources
  to incorporate and close. Their requirements include source/dataset
  separation, immutable versions and bindings, feature recipes, the complete
  Data interaction model, transient-secret safety, coordinator-backed
  ingestion, durable storage, schedules, and reconciliation.
- `specs/python-migration.md`, `specs/migration-baseline.md`, and
  `specs/routing/phase-*.md` preserve accepted migration and phase evidence.
  Still-active rules must be extracted into living general/page specs before
  these files become historical records.
- `specs/decisions/` explains accepted architectural decisions and remains
  supporting rationale rather than a substitute for living specifications.
- The user selected end-to-end page ownership and explicitly authorized
  incorporation and closure of the three validated missing-change notes.

## Suggested Specifications

Create the following living-spec taxonomy during implementation. Index files
describe ownership and reading routes; they must not duplicate normative
content from their children.

```text
specs/
├── README.md
├── general/
│   ├── README.md
│   ├── product.md
│   ├── roadmap.md
│   ├── contract.md
│   ├── design-pattern.md
│   ├── system-architecture.md
│   ├── concurrency-and-parallelism.md
│   ├── technology-stack.md
│   ├── api.md
│   ├── quality/
│   │   ├── README.md
│   │   ├── common.md
│   │   ├── concurrency.md
│   │   └── visualization.md
│   └── ui/
│       ├── README.md
│       ├── shell-and-state.md
│       ├── async-effects.md
│       ├── persistence-and-responsive.md
│       ├── visual-system.md
│       ├── visualization.md
│       └── workspaces.md
├── pages/
│   ├── README.md
│   ├── data/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   ├── ingestion.md
│   │   ├── datasets.md
│   │   ├── api.md
│   │   └── roadmap.md
│   ├── research/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   └── features-and-targets.md
│   ├── experiments/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   └── definitions-and-materialization.md
│   ├── jobs/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   └── runtime.md
│   ├── results/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   └── evaluation-and-backtesting.md
│   ├── models/
│   │   ├── README.md
│   │   ├── workspace.md
│   │   ├── estimators.md
│   │   └── artifacts-and-registry.md
│   └── inference/
│       ├── README.md
│       ├── workspace.md
│       └── scoring-and-export.md
├── decisions/
├── examples/
├── missing/
│   └── README.md
└── history/
    ├── README.md
    ├── migration/
    │   ├── python-migration.md
    │   └── migration-baseline.md
    └── routing/
        └── phase-*.md
```

No additional page folder is needed until the product gains another
top-level workspace. Page folders may add focused documents later, but may not
create a second owner for a general or another page's rule.

## Decisions and Constraints

- `specs/README.md` remains at the root because it is the stable entry point,
  not a general specification. The only living normative namespaces below it
  are `general/` and `pages/`.
- `decisions/`, `examples/`, `missing/`, and `history/` remain support
  namespaces. ADRs explain decisions, examples are non-authoritative,
  `missing/` records future divergence, and `history/` preserves accepted
  evidence without participating in normal task routing.
- A rule is general only when at least two pages or a process-wide boundary
  depends on it. General documents own shared transport envelopes,
  concurrency, architecture, dependency policy, agent contracts, UI
  primitives, and quality gates.
- A page owns a value or workflow when that page creates or manages its
  lifecycle. Consumer pages link to the owner and specify only their own
  selection, presentation, pinning, or command behavior.
- Shared API transport, versioning, event envelopes, Python-client behavior,
  worker protocol, health, and error shape remain in `general/api.md`.
  Concrete Data resources, credential routes, Data DTO semantics, and Data
  error mapping move to `pages/data/api.md`. Future page-specific endpoints
  belong to that page when specified.
- Shared UI shell, navigation, effects, docking, persistence, responsive
  behavior, visual tokens, charts, tables, and workspace state rules live in
  `general/ui/`. Page-specific panel composition, forms, states, interactions,
  and golden scenarios live in each page's `workspace.md`.
- Data owns acquisition, credentials, provider/calendar adapters, canonical
  bars, source definitions/snapshots, dataset definitions/versions, catalog
  revisions, feature-recipe composition, schedules, provenance, and
  publication. Research owns feature and target computation/preview semantics.
  Experiments owns pinned dataset bindings, definitions, drafts, validation,
  estimates, sweeps, and run materialization.
- Jobs owns execution lifecycle, queueing, worker resources, pause/resume,
  cancellation, and checkpoints. Results owns walk-forward evaluation,
  metrics, comparisons, and reference backtests. Models owns estimators,
  immutable artifacts, registry, aliases, and promotion history. Inference owns
  compatibility checks for scoring, ranked outputs, prediction history, and
  export.
- Current milestone detail moves from the root roadmap into
  `pages/data/roadmap.md`. `general/roadmap.md` becomes the cross-page delivery
  index and records that later backend page milestones remain deferred.
- The migration is atomic within one implementation effort. Do not leave
  duplicate authoritative documents or redirect stubs at removed paths.
  Repository references are updated in the same change. Git history provides
  old-path provenance.
- Historical phase routes may still be linked by phase-specific verification
  skills as evidence maps, but normal `AGENTS.md` routing must use current
  general/page owners. Historical files must carry an explicit
  non-authoritative/historical status and link back to current owners.
- Extract all still-current compatibility, package, Cython, golden, and
  acceptance rules from migration records into their living owners before
  reclassifying those records as history.
- Preserve the current dirty working tree. Implementation must read and migrate
  the working-tree version of every touched specification, never restore its
  `HEAD` version, and avoid overwriting unrelated code, tests, golden images,
  lockfile, or user changes.
- This plan changes documentation organization and authority only. Any newly
  discovered behavior beyond the three user-approved missing notes remains in
  `specs/missing/` unless separately authorized.

## Interfaces and Data Changes

No runtime API, DTO, Python/Cython interface, database schema, artifact format,
or user-visible behavior changes.

Documentation paths are an internal agent/developer interface and change as
follows:

| Current source | New owner |
|---|---|
| `specs/contract.md`, `design-pattern.md`, `product.md`, `system-architecture.md`, `concurrency-and-parallelism.md`, `technology-stack.md` | Same filenames under `specs/general/` |
| Shared portions of `specs/api.md` | `specs/general/api.md` |
| `specs/quality*.md` | `specs/general/quality/` |
| Shared `specs/ui/foundation*.md`, `workspaces*.md`, `raylib-visual-system.md`, and `visualization.md` content | `specs/general/ui/` |
| Data-specific `specs/api.md`, `data-and-features.md`, `ui/workspace-data.md`, and current roadmap content | `specs/pages/data/` |
| Feature/target semantics and `ui/workspace-research.md` | `specs/pages/research/` |
| Experiment/sweep/materialization semantics and `ui/workspace-experiments.md` | `specs/pages/experiments/` |
| Job-specific `training-runtime.md` content and `ui/workspace-jobs.md` | `specs/pages/jobs/` |
| `evaluation.md`, the evaluation index, and `ui/workspace-results.md` | `specs/pages/results/` |
| `models.md`, `model-estimators.md`, `model-artifacts.md`, registry/alias rules from `inference.md`, and `ui/workspace-models.md` | `specs/pages/models/` |
| Scoring/export rules from `inference.md`, the inference index, and `ui/workspace-inference.md` | `specs/pages/inference/` |
| `python-migration.md`, `migration-baseline.md`, `routing/phase-*.md` after active-rule extraction | `specs/history/` |

Every living document must contain a title, `Status`, `Owner`, `Last updated`,
and `Related` links where relationships exist. Folder `README.md` files list
their child documents, state the folder's ownership boundary, and provide
focused reading routes.

## Implementation Plan

- [ ] Capture a pre-migration inventory from the dirty working tree: every
  specification file, metadata block, heading, local link, literal
  `specs/...` reference, and uncommitted diff. Use this as a content ledger so
  moves and splits are based on current files rather than `HEAD`.
- [ ] Create `specs/general/`, its `quality/` and `ui/` children, and
  `specs/pages/` with the seven approved page folders and index files shown in
  the target tree.
- [ ] Move and normalize the cross-cutting product, contract, architecture,
  concurrency, technology, shared API, shared UI, and quality content into
  `specs/general/`. Merge the small current index documents into the relevant
  new folder indexes where they add no independent normative behavior.
- [ ] Split shared versus page-specific API rules: keep protocol/versioning,
  event envelope, supported client, worker protocol, health, and common
  type/error rules in `general/api.md`; move the concrete Data endpoints, DTOs,
  secret restrictions, reconciliation identities, and provider error behavior
  to `pages/data/api.md`.
- [ ] Build the Data page specification set from the current working-tree
  versions of `data-and-features.md`, `workspace-data.md`, Data-specific API
  and architecture sections, and the two ingestion milestone notes. Separate
  acquisition/publication rules into `ingestion.md`, source/dataset lifecycle
  and recipe rules into `datasets.md`, interaction behavior into
  `workspace.md`, and current milestone sequencing/acceptance into
  `roadmap.md`.
- [ ] Incorporate every requirement from
  `declarative-dataset-feature-workflow.md`: typed source definitions and
  immutable snapshots; reusable dataset definitions and immutable
  versions/builds; ordered closed-union feature recipes; pinned Research and
  Experiment bindings; stale-latest behavior; bounded schema projection;
  canonical-role selection; and visible interval/session/adjustment/timezone
  controls. Assign lifecycle rules to Data, preview/query consumption to
  Research, and binding/submission rules to Experiments.
- [ ] Incorporate every requirement from
  `milestone-1a-simulator-ui-contract.md` into Data workspace/API/roadmap
  owners, including pointer-accessible actions, catalog empty-state behavior,
  standalone Schedules, bounded paginated file browsing, file selection
  semantics, flow-specific back/cancel behavior, simulator boundaries, and
  transient-secret handling.
- [ ] Incorporate every requirement from
  `milestone-1b-real-data-ingestion.md` into Data ingestion/API/roadmap and the
  relevant general architecture/API owners, including coordinator selection,
  real loopback behavior, bounded work, SQLite/Parquet durability, credentials,
  admitted calendar behavior, schedules, reconciliation, and explicit
  simulator injection for deterministic capture.
- [ ] Build Research and Experiments page sets. Move feature/target
  computation, timing, missingness, leakage, linked query, and preview behavior
  to Research. Move drafts, validation, resource estimates, sweeps, pinned
  bindings, materialization, autosave, and immutable submission to
  Experiments. Replace copied Data lifecycle rules with links to Data owners.
- [ ] Build Jobs and Results page sets. Split experiment/sweep definition rules
  out of the current training runtime before moving job lifecycle, CPU
  scheduling, worker ownership, cancellation, and checkpointing to Jobs. Move
  walk-forward folds, metric partitioning, comparisons, reference backtests,
  and Results presentation to Results.
- [ ] Build Models and Inference page sets. Move estimator behavior, artifact
  validation, registry, aliases, and promotion history to Models. Move
  historical/latest scoring, compatibility gating, ranked output, prediction
  publication/history, cancellation, and export to Inference. Ensure registry
  rules have only the Models owner and Inference links to them.
- [ ] Rewrite `specs/general/roadmap.md` as a concise cross-page sequencing
  index and move detailed Milestone 1a/1b scope and acceptance to
  `specs/pages/data/roadmap.md`, preserving the current statement that later
  backend milestones are deferred until real Data ingestion is accepted.
- [ ] Extract still-active rules from `python-migration.md`,
  `migration-baseline.md`, and phase routes into current general/page owners.
  Move the records to `specs/history/migration/` and
  `specs/history/routing/`, mark them historical/non-authoritative without
  rewriting their accepted evidence, and add `specs/history/README.md` that
  directs current work to living specs.
- [ ] Verify a requirement-by-requirement trace from each of the three
  user-approved missing notes to an authoritative destination. Remove those
  notes only after the trace is complete; retain `specs/missing/README.md` and
  any future unrelated notes.
- [ ] Rewrite `specs/README.md` around the two living namespaces, support
  records, authority rules, page ownership, and minimum-reading policy. Update
  `AGENTS.md` so `specs/general/contract.md` remains first, general design and
  quality routes point to their new owners, workflow routes select one page
  index plus only applicable general specs, and historical phase routes are
  excluded from normal work.
- [ ] Update every repository consumer in the same change: root `README.md`,
  all `.agents/skills/**/SKILL.md` and referenced skill resources,
  `scripts/docsize.go`, ADR affected-specification links, historical records,
  examples, and all cross-links in living specs. Change `docsize` route names
  from retired phase labels to representative page workflows and remove its
  already-stale `specs/frontend/` references.
- [ ] Remove superseded root and `specs/ui/` living-spec files only after their
  content ledger entries have destinations and repository-wide searches show
  no active references. Do not leave redirects or duplicate authority.
- [ ] Add a dependency-free `tests/test_specifications.py` under the existing
  pytest gate. Validate the expected taxonomy, authoritative metadata, index
  coverage, local Markdown file/anchor links, and literal repository-local
  `specs/...` references in tracked text files. Exclude external URLs and
  generated/binary files explicitly.
- [ ] Run focused and full validation, review the final diff for unintended
  runtime changes, and compare the post-migration content ledger against the
  pre-migration inventory. Any omitted normative paragraph must be restored,
  intentionally consolidated with a named owner, or reported as an unresolved
  conflict rather than silently dropped.

## Tests and Acceptance Criteria

- [ ] `uv run pytest tests/test_specifications.py` passes and proves the target
  tree, required metadata, index completeness, local file/anchor links, and
  literal spec paths are valid.
- [ ] `go run ./scripts/docsize.go` completes with page-oriented routes and no
  missing-file panic.
- [ ] Repository-wide `rg` checks find no active references to removed
  root-level living specs, `specs/ui/`, `specs/frontend/`, or normal-work
  `specs/routing/` paths. References inside historical records may point only
  to existing historical paths or current living owners.
- [ ] Each general and page index lists every authoritative child exactly once,
  and every authoritative document is reachable from `specs/README.md`.
- [ ] Each normative topic has one owner. Targeted duplicate-content review
  covers API transport versus page resources, shared UI versus page
  interaction, Data recipes versus Research calculation, Experiments
  definitions versus Jobs execution, Models registry versus Inference scoring,
  and general quality gates versus page acceptance.
- [ ] A route audit demonstrates focused minimum context for at least Data,
  Research, Experiments, Jobs, Results, Models, Inference, shared UI visual,
  concurrency, dependency, and public API tasks.
- [ ] Every statement in the three former missing notes is traceable to a
  current authoritative heading, and none of the three notes remains after
  successful incorporation.
- [ ] Active compatibility and acceptance requirements formerly found only in
  migration/phase records are present in living general or page owners before
  those records are marked historical.
- [ ] The current uncommitted Data/API/UI/architecture/technology/roadmap
  specification changes remain represented in the rewritten documents; no
  user-authored working-tree content is replaced by its `HEAD` version.
- [ ] `uv run ruff format --check`, `uv run ruff check`, `uv run pyright`, and
  `uv run pytest` pass. Documentation-only relocation does not authorize
  updating runtime code or golden images to make unrelated failures pass.
- [ ] `git diff --check` passes, and the final diff contains only the planned
  specification reorganization, path-consumer updates, documentation
  validation test, and `docsize` route update.

## Rollout, Compatibility, and Risks

Implement as one atomic documentation migration because old and new routing
cannot safely coexist with duplicate authority. There is no runtime rollout or
data migration. Internal agents, skills, scripts, and documentation links move
in the same change. External bookmarks to old paths will break; this is
accepted in favor of unambiguous ownership, with Git history and
`specs/history/` preserving provenance.

The largest risk is the dirty working tree: several source specifications and
their related implementation are already modified or untracked. The
implementation must preserve those files, migrate their working-tree content,
and avoid cleanup/reset commands. If a concurrent edit changes a source during
the migration, stop and reconcile that document from the latest working-tree
version before deleting its old path.

Splitting documents can accidentally duplicate or weaken requirements. Mitigate
this with the content ledger, explicit ownership rules, missing-note trace,
link/index tests, and targeted review of cross-page boundaries. Historical
records can also be mistaken for current requirements; their status banners,
history index, and exclusion from normal `AGENTS.md` routes must be explicit.

No compatibility stub files are retained because they would remain discoverable
as apparent specifications and could drift. If an out-of-repository consumer
later proves to require stable documentation URLs, handle that as a separate
redirect/publication concern rather than restoring duplicate source files.

## Assumptions

- The seven current top-level UI tabs are the complete page taxonomy: Data,
  Research, Experiments, Jobs, Results, Models, and Inference.
- Settings, shell navigation, command palette, status, layouts, charts, tables,
  and visual tokens are shared UI concerns, not additional pages.
- ADRs, examples, missing records, and historical evidence are useful support
  artifacts but are not living general/page specifications.
- The user-approved rewrite authorizes authoritative incorporation of the three
  named missing notes, but not unrelated behavior changes.
- The repository has no supported external documentation-path compatibility
  contract; atomic internal path updates and Git history are sufficient.
- Existing project gates and dependencies are sufficient. The link validator
  uses the standard library and pytest, with no new package or tool admission.
