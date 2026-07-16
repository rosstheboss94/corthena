# Add the Phase 6 Research vertical slice

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-15
**Affected:** `src/corthena/ui/research`, `src/corthena/ui/client`, `src/corthena/ui/state.py`, `src/corthena/ui/effects.py`, `src/corthena/ui/simulator.py`, `specs/roadmap.md`

## What changed

The Python UI gains typed generation-bound Research queries and immutable
responses for OHLCV, leakage-safe features and targets, distributions, and
cursor-paginated rows. The simulator, bounded effects runtime, reducer, linked
six-panel workspace, Raylib renderer, and 36-case capture matrix use the same
agent-facing `UIClientProtocol` boundary.

## Why

Phase 5b is complete and the roadmap makes the deterministic Research vertical
slice the next implementation phase before Data and Experiments.
