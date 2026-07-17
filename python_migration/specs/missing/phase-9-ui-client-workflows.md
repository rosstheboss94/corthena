# Phase 9 UI client workflows

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-16
**Affected:** `src/corthena/ui/models_inference/`, `src/corthena/ui/client/protocol.py`, Phase 9 effects, simulator, shell, and capture paths

## What changed

The simulator-facing `UIClient` now includes typed Phase 9 registry loading,
confirmed alias assignment, historical/latest inference scoring, and export
preparation operations. Phase 9 publishes immutable registry, tree, alias,
compatibility, prediction, ranking, distribution, history, and export values
through generation-safe UI effects and reducers.

## Why

Phase 9 requires the deterministic Models-to-Inference workflow to be exercised
behind the same bounded client boundary that will later receive a real
coordinator-backed adapter.
