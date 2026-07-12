# ADR 0001: Local Process Architecture

**Status:** Accepted  
**Date:** 2026-07-04

## Context

Training can run for a long time and must not freeze or crash the desktop UI. The first release is a single-user Windows workstation, not a distributed service.

## Decision

Use a loopback coordinator as the authoritative scheduler/metadata writer, a separate Raylib process, and isolated processes for active training jobs.

## Alternatives

- Run training in the UI process.
- Coordinate components directly through SQLite.
- Build remote workers in v1.

## Consequences

The UI remains responsive, worker failures are contained, and CLI/API surfaces share one application boundary. Process messaging and lifecycle reconciliation are required.

## Affected specifications

- [System architecture](../system-architecture.md)
- [Training runtime](../training-runtime.md)
- [API](../api.md)
