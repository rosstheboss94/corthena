# Hide the compatibility loopback application factory

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-13
**Affected:** `src/corthena/compatibility/loopback`, `specs/api.md`

## What changed

The Phase 0 compatibility package no longer exposes `create_app()`. Consumers
use the typed loopback probe contract or the preserved `run_loopback_probe()`
facade instead of receiving a FastAPI implementation object.

## Why

The new agent-facing contract and modular-boundary rules require framework
implementation values to remain inside their concrete adapter.
