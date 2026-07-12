# Raylib Frontend Foundation

**Status:** Authoritative  
**Owner:** Frontend  
**Last updated:** 2026-07-12
**Related:** [Technology stack](../technology-stack.md), [Workspaces](workspaces.md), [Visualization](visualization.md), [API](../api.md), [ADR 0004](../decisions/0004-raylib-dockable-interface.md)

## Technology constraints

Use `github.com/gen2brain/raylib-go/raylib` and its Raygui package, `net/http` plus the approved WebSocket package on background goroutines, `os.UserConfigDir` for persisted UI state, and bundled font/icon assets. Docking, widgets, file browsing, charts, and tables are first-party Go components. Do not add another GUI framework.

## Visual direction

The screenshots define a near-black, high-density trading terminal with compact typography, thin panel chrome, restrained colors, chart-first layouts, and top workspace tabs. They do not require crypto, order-flow, liquidation, or live-market functionality.

V1 uses one Windows Raylib OS window and one dark theme.

## Application shell

- Top workspace tabs: `Data`, `Research`, `Experiments`, `Jobs`, `Results`, `Models`, and `Inference`.
- Global dataset/symbol/interval context, command palette, connection state, component status, and active-job count.
- Central dock host.
- Bottom status bar with coordinator health, selection, cache, CPU slots, worker status, FPS, and shortcut hints.
- Modal layer for confirmation, file selection, command palette, and critical errors.
- Global Settings modal for application UI sizing, opened from the top navigation or `Ctrl+,`.
- Toast layer for nonblocking notifications.

## Frame loop and OS-thread ownership

The workstation command calls `runtime.LockOSThread` on its initial goroutine before Raylib initialization. That goroutine exclusively owns every Raylib and Raygui call until shutdown. Adapters record the owner thread and fail fast in development when called elsewhere.

Background goroutines handle HTTP, Arrow decoding, WebSocket events, layout I/O, clipboard requests, and file-dialog work. They communicate with the UI through bounded typed channels and never retain Raylib values or invoke render callbacks.

Each frame:

1. Drains at most a bounded number of immutable client messages.
2. Reduces `UIAction` values into state.
3. Calculates dock geometry.
4. Routes keyboard and pointer input.
5. Renders visible panels and overlays.
6. Enqueues asynchronous `UIEffect` values without blocking.

No filesystem, database, network, Arrow decoding, or training operation runs on the render thread. Channel sends from the render thread are nonblocking; backpressure coalesces replaceable effects or surfaces a typed busy state.

## Typed state

- `AppState`: workspace, selections, connection/component state, overlays, and cache metadata.
- `WorkspaceLayout`: schema version, dock root, hidden/maximized panels, and link groups.
- `DockNode`: closed internal interface implemented only by `SplitNode` and `TabStackNode`.
- `PanelDescriptor`: type, title, icon, multiplicity, minimum size, and supported links.
- `PanelInstanceState`: stable ID, panel settings, link group, and serialized view state.
- `LinkContext`: dataset, symbols, interval, time range, experiment, run, and model.
- `UIAction`: closed internal interface for typed user/server transitions.
- `UIEffect`: closed internal interface for typed API, persistence, clipboard, or file-dialog work.
- `Preferences`: global UI-size selection with independent revision and persistence state.
- `ResearchQuery`: validated dataset, symbols, interval, UTC range, selected
  features, target, resolution, cursor, sort/filter, correlation, link-group,
  scenario, and generation identity.
- `ResearchWorkspaceState`: independently ordered link-group requests,
  immutable render-ready chart buffers, typed feature/target values with
  explicit observation and future-target timestamps, virtual
  row pages, stable selections, layer visibility, stale-result state, and
  typed loading/failure/recovery conditions.
- `DataWorkspaceState`: generation-ordered catalog snapshots, stable dataset
  selection, import queue and logs, coverage metadata, typed validation
  diagnostics, correlation identities, and loading/failure/recovery state.
- `ExperimentWorkspaceState`: immutable experiment definitions, a typed
  sectioned draft, compiled-feature metadata, validation issues, deterministic
  resource estimates, submission state, and revision-aware autosave state.
- `JobsWorkspaceState`: generation-ordered queue requests, stable selected job,
  ordered stage progress, live metric series, bounded worker and CPU-lease
  telemetry, process health, checkpoint status, structured logs, pending typed
  control state, and deterministic lifecycle scenarios.
- `ResultsWorkspaceState`: generation-ordered filtered run requests, up to four
  stable comparison identities, immutable fold and metric details, equity and
  drawdown series, distributions, prediction overlays, configuration values,
  and explicit loading/failure/degraded/recovery state.

Switches over closed variants include a default branch that reports an invariant violation. Serialized discriminators are validated before constructing internal variants. Native Raylib, Raygui, Arrow, Windows, and SQLite values stay inside adapters.

The pre-coordinator demo implements Research, Data, Experiments, Jobs, and Results through the
same narrow `FrontendClient` and effects runtime used by the shell. Superseding
and hidden workspace requests are canceled by link group or workflow;
generation checks reject stale completions. Demo preparation, feature/target
calculation, LOD, sorting, filtering, pagination, catalog/import validation,
experiment evaluation and submission, and draft persistence run on bounded
background workers. Jobs and Results queries, controls, reconciliation, and
scenario preparation use the same bounded workflow ownership and reject stale
generations. Import publication and experiment submission are atomic;
accepted experiment definitions remain immutable when the catalog changes.
This internal demo contract does not define coordinator HTTP endpoints; the
public API remains owned by `specs/api.md`.

## Dock manager

- Nested horizontal/vertical splits with minimum sizes and clamped ratios.
- Left, right, top, bottom, and center/tab drag targets.
- Tab activation, reorder, movement, and closing.
- Temporary maximize and restore.
- Reopen hidden panels through a panel menu.
- Multiple chart, table, diagnostic, and inspector instances.
- Singleton job queue, settings, and primary editors.
- Autosaved last layout plus named reset/import/export layouts.
- Colored link groups for synchronized or independent comparisons.
- No detached native or free-floating panels in v1.

Dock-tree mutation and geometry are pure Go functions testable without initializing Raylib.

## Widgets and input

Wrap Raygui for basic controls. Implement custom docking, virtualized tables, trees, splitters, charts, context menus, command palette, tooltips, and notifications.

Use hierarchical widget IDs and explicit hot, active, focused, and pointer-capture state. Apply scissor clipping to scrollable panels. Frequent actions support mouse and keyboard; color is never the only status indicator.

## Design tokens

- Background `#0B0D10`
- Panel `#11151A`
- Raised/selected `#171C22`
- Divider/grid `#252B33`
- Primary text `#D6DCE5`
- Muted text `#7E8896`
- Cyan `#3CC8C8`
- Purple `#9B7CF6`
- Positive `#4CC38A`
- Negative `#EF6B73`
- Warning `#D8B45A`

Bundle Inter for controls, JetBrains Mono for data, and a Lucide-derived icon atlas. Load fonts at a high-resolution atlas size so enlarged text remains sharp. Use a 4-pixel spacing grid, approximately 32-pixel panel headers, 36-pixel top navigation, 40-pixel context bar, and 26-pixel status bar before scaling. Before scaling, use 12-pixel captions, 13-pixel body and data text, 14-pixel controls, and 18-pixel headings as minimum readable roles.

## Responsive behavior and persistence

- Optimize for 1920×1080 and remain usable at 1280×720.
- Support Windows scale factors from 100–200%.
- Below roughly 1100 logical pixels, stack secondary panels and move controls into overflow menus.
- Default the application UI-size preset to 125%; support 100%, 125%, 150%, 175%, and 200% presets.
- Compute effective UI scale as Windows DPI scale multiplied by the selected preset and clamp it to 100-200%.
- Apply effective scale to typography, spacing, rows, controls, hit targets, navigation, docking geometry, modal bounds, clipping, and panel minimum sizes.
- Open Settings with the top-navigation control or `Ctrl+,`; use `Ctrl+Plus` and `Ctrl+Minus` to step presets and `Ctrl+0` to restore 125%.
- At constrained widths, abbreviate all seven workspace tabs and move connection and active-job detail into the status bar before allowing navigation overlap.
- Preserve the active analytical panel before reducing text below its minimum size.
- Store split ratios, not physical pixels.
- Persist layouts/preferences atomically in the user application-data directory with schema versions and migrations.
- Store global preferences separately from named layouts, coalesce rapid saves on a bounded background worker, reject stale loads, quarantine invalid documents, and fall back to 125%.
- Preserve invalid documents for diagnosis and fall back to defaults.

If the coordinator is unavailable, keep the shell/layout operational, disable mutating commands, and show reconnect/restart actions.
