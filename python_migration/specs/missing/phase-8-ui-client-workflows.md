# Phase 8 UI client workflows

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-16
**Affected:** `src/corthena/ui/client/protocol.py`, `src/corthena/ui/jobs_results/`, Phase 8 UI effects and simulator

## What changed

`UIClientProtocol` now exposes typed simulator operations for Phase 8 snapshot
loading, idempotent job lifecycle commands, and immutable run comparisons. The
UI state and effects boundary carries explicit request, command, correlation,
comparison, workspace, and generation identities for those operations.

## Why

The Phase 8 Jobs and Results workspaces require a backend-swappable typed
boundary while remaining simulator-only until the coordinator is implemented.
