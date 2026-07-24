# ADR 0002: Python Library Estimators and Audited Artifacts

**Status:** Accepted
**Date:** 2026-07-12

## Context

The project uses a Python/Cython workstation with library-backed estimators.
The product still requires deterministic orchestration,
leakage-safe data boundaries, resumable jobs where supported, immutable
artifacts, compatibility checks, and auditable registry promotion. Rewriting
every estimator in Cython would delay the migration and duplicate mature ML
libraries.

## Decision

Use `scikit-learn` and PyTorch for estimator internals when they provide the
needed model, metric, fitting, prediction, and serialization capability.
Corthena owns the orchestration and audit boundary around those libraries.

- Corthena validates model specs, data windows, feature schemas, targets,
  compatibility metadata, seeds, manifests, and promotion rules before calling
  library code.
- `scikit-learn` owns supported classical estimators, metrics, preprocessing
  primitives where approved, and prediction behavior.
- PyTorch owns supported neural model training, prediction, optimizer state,
  scheduler state, and tensor execution.
- Corthena stores project manifests, hashes, feature and data fingerprints, run
  metadata, compatibility metadata, and immutable registry records around
  library-owned artifacts.
- Cython is permitted for measured hot paths and native adapters only; it is not
  the default module implementation strategy.
- Corthena fails closed when a library artifact, manifest, schema version,
  feature/target contract, dependency version, or compatibility rule is unknown
  or invalid.

## Alternatives

- Continue first-party tree kernels.
- Reimplement all estimators in first-party Python and Cython.
- Bind to a non-Python native ML engine for all model families.
- Store raw library artifacts without project manifests or compatibility checks.

## Consequences

The project depends on library behavior and version compatibility, so artifact
manifests and tests must record exact runtime and dependency metadata. Corthena
keeps responsibility for determinism policy, leakage prevention, immutable
promotion, auditability, cancellation boundaries, and public contracts, while
library internals remain behind typed adapters.

## Affected specifications

- [Models](../pages/models/README.md)
- [Model artifacts](../pages/models/artifacts-and-registry.md)
- [Training runtime](../pages/jobs/runtime.md)
- [Evaluation and inference](../pages/results/README.md)
- [System architecture](../general/system-architecture.md)
- [Technology stack](../general/technology-stack.md)
- [Quality](../general/quality/README.md)
