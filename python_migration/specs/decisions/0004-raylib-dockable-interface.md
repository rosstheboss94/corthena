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
- A conventional web frontend.

## Consequences

Dock geometry, focus, persistence, tables, and charts are first-party frontend infrastructure. Native windows and free-floating panels are deferred.

## Affected specifications

- [Frontend foundation](../frontend/foundation.md)
- [Frontend workspaces](../frontend/workspaces.md)
- [Frontend visualization](../frontend/visualization.md)
