# UI Shell and State

The shell has top workspace tabs, global context, command palette,
connection/component state, central dock host, status bar, modal layer, global
Settings, and toast layer. The initial UI thread locks the OS thread before
Raylib initialization; it exclusively owns Raylib/Raygui. Adapters fail fast
in development when called elsewhere.

## Phase ownership

Phase 3 owns deterministic shell composition over the Phase 1 lifecycle and
Phase 2 state/effects interfaces. It renders workspace-tab navigation, global
context and component-status presentation, a central non-docking content host,
the status bar, and inert modal and toast overlay layers in a stable order.
Rendering consumes immutable `AppState`, projects it through typed
render-neutral geometry and view models, and emits typed `UIAction` values.
Render functions contain no domain, simulator, persistence, filesystem,
network, decoding, or other blocking work, and native values remain contained
inside the Raylib/Raygui adapter.

The Phase 3 hidden launch drives the Phase 2 simulator and effects lifecycle,
drains no more than the configured per-frame result bound, renders the named
shell scenario, and shuts down all owned resources cleanly on success and
failure. Its retained visual target is the manifest-owned
`phase3_application_shell.png`: seed `42`, fixed clock
`2026-07-10T12:00:00Z`, `data` workspace, `normal` scenario, `1280x720`
viewport, `100%` scale, `30` hidden frames, channel tolerance `3`, and maximum
different-pixel ratio `0.002`. Migration JPEGs are inspection aids and cannot
serve as baselines.

Phase 3 preserves the complete cutover application-shell surface so its shared
navigation, context cycling, shell-level panel activation, command palette,
Settings overlay, and UI-scale actions remain one coherent typed surface.
The accepted Python shell implementation is under `src/corthena/ui/shell.py`.
Phase 4 continues to own
docking mutation algorithms, reusable controls, revisioned preferences,
responsive layout policy, and layout persistence, recovery, and migration;
Phase 3 shell actions that require those owners remain typed and inert.

Typed state includes `AppState`, `WorkspaceLayout`, closed `DockNode` variants,
`PanelDescriptor`, stable `PanelInstanceState`, `LinkContext`, closed typed
`UIAction` and `UIEffect` variants, and revisioned `Preferences`. Serialized
discriminators are validated before internal construction; native values stay
inside adapters. Dock mutations and geometry are pure Python, with nested splits,
tabs, drag targets, maximize/restore, hidden-panel reopening, link groups,
autosaved and named layouts, and no detached native panels.

Workspace state owns immutable, generation-ordered snapshots: Research owns
link-group queries, render-ready buffers, missing-aware features/targets,
virtual rows, stable selections, and stale/loading/failure states; Data owns
catalog revisions, import queues/logs, coverage, diagnostics, and correlation;
Experiments owns definitions, typed drafts, compiled-feature metadata,
validation, estimates, submission, and autosave; Jobs owns queue, stages,
metrics, leases, health, checkpoints, logs, controls, and lifecycle scenarios;
Results owns filtered requests, up to four comparisons, immutable metrics and
series, distributions, overlays, configuration, and degradation/recovery.

Closed variants use exhaustive switches with an invariant-violation default.
The pre-coordinator demo uses the same narrow `UIClient` and effects
runtime as the shell; accepted experiment definitions remain immutable when
the catalog changes.

Wrap Raygui for basic controls and implement first-party docking, virtualized
tables, trees, splitters, charts, menus, palette, tooltips, and notifications.
Use hierarchical IDs and explicit hot/active/focused/capture state; color is
never the only status signal. Preserve the documented dark design tokens,
fonts, icon atlas, spacing grid, and minimum readable text roles.
