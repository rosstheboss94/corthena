# Raylib Frontend Foundation

**Status:** Authoritative  
**Owner:** Frontend  
**Last updated:** 2026-07-04  
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

Switches over closed variants include a default branch that reports an invariant violation. Serialized discriminators are validated before constructing internal variants. Native Raylib, Raygui, Arrow, Windows, and SQLite values stay inside adapters.

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

Bundle Inter for controls, JetBrains Mono for data, and a Lucide-derived icon atlas. Use a 4-pixel spacing grid, approximately 26-pixel panel headers, 32-pixel top navigation, and 22-pixel status bar at 100% scale.

## Responsive behavior and persistence

- Optimize for 1920×1080 and remain usable at 1280×720.
- Support Windows scale factors from 100–200%.
- Below roughly 1100 logical pixels, stack secondary panels and move controls into overflow menus.
- Preserve the active analytical panel before reducing text below its minimum size.
- Store split ratios, not physical pixels.
- Persist layouts/preferences atomically in the user application-data directory with schema versions and migrations.
- Preserve invalid documents for diagnosis and fall back to defaults.

If the coordinator is unavailable, keep the shell/layout operational, disable mutating commands, and show reconnect/restart actions.
