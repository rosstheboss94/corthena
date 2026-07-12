# Frontend Shell and State

The shell has top workspace tabs, global context, command palette,
connection/component state, central dock host, status bar, modal layer, global
Settings, and toast layer. The initial UI thread locks the OS thread before
Raylib initialization; it exclusively owns Raylib/Raygui. Adapters fail fast
in development when called elsewhere.

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
The pre-coordinator demo uses the same narrow `FrontendClient` and effects
runtime as the shell; accepted experiment definitions remain immutable when
the catalog changes.

Wrap Raygui for basic controls and implement first-party docking, virtualized
tables, trees, splitters, charts, menus, palette, tooltips, and notifications.
Use hierarchical IDs and explicit hot/active/focused/capture state; color is
never the only status signal. Preserve the documented dark design tokens,
fonts, icon atlas, spacing grid, and minimum readable text roles.
