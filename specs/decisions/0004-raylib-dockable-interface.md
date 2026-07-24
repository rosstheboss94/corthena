# ADR 0004: Raylib Dockable Interface

**Status:** Accepted
**Date:** 2026-07-04

## Context

The desired interface resembles a dense, multi-pane trading terminal. Raylib provides immediate-mode rendering but not a complete desktop docking model.

## Decision

Use one Raylib OS window with a custom retained dock tree, top workspace tabs, typed panel state, named layouts, and configurable panel link groups.

## Alternatives

- Fixed screens.
- Fully native detachable windows.
- A conventional web ui.

## Consequences

Dock geometry, focus, persistence, tables, and charts are first-party ui infrastructure. Native windows and free-floating panels are deferred.

## Affected specifications

- [UI foundation](../general/ui/README.md)
- [UI workspaces](../general/ui/workspaces.md)
- [UI visualization](../general/ui/visualization.md)
