# ADR 0002: First-Party Go Tree Engine and Arrow Artifacts

**Status:** Accepted  
**Date:** 2026-07-04

## Context

The project exists partly to implement and understand tree algorithms rather than wrap existing frameworks. Datasets may approach ten million rows on a CPU workstation. Models require typed contiguous data, deterministic parallel split search, resumable checkpoints, and auditable immutable artifacts.

## Decision

Implement histogram regression trees, random forests, squared-error gradient boosting, metrics, ranking, and portfolio calculations as first-party Go kernels over typed contiguous slices and versioned memory-mapped files.

- Keep histogram thresholds, missing routing, deterministic tie-breaking, iterative construction, and ordered ensemble aggregation.
- Use `float64` accumulators for reductions and a versioned counter-based random generator with domain-separated seed derivation.
- Store model and checkpoint metadata in canonical versioned JSON manifests.
- Store typed model and checkpoint arrays in checksummed Arrow IPC files.
- Keep Parquet for canonical tabular data and reports.
- Validate dimensions, element types, graph invariants, schemas, engine versions, and checksums before exposing loaded models.
- Keep estimator fitting, split search, metrics, ranking, and portfolio accounting in first-party Go code.

## Alternatives

- Use an external Go ML framework.
- Bind to a native ML engine.
- Define a project-specific binary array container.
- Store models as pointer graphs encoded with a Go-specific object serializer.

## Consequences

The project owns algorithm correctness, numerical stability, optimization, serialization, and recovery. Arrow IPC reuses the approved typed columnar stack, while artifact schemas and canonical JSON rules must be versioned explicitly. Determinism tests must cover goroutine counts and completion orders, and hot kernels require benchmark-driven optimization.

## Affected specifications

- [Models](../models.md)
- [Data and features](../data-and-features.md)
- [Training runtime](../training-runtime.md)
- [Evaluation and inference](../evaluation-and-inference.md)
- [System architecture](../system-architecture.md)
- [Quality](../quality.md)
