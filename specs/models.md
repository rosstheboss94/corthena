# Go Model Specification

**Status:** Authoritative  
**Owner:** ML  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [Training runtime](training-runtime.md), [Evaluation](evaluation-and-inference.md), [ADR 0002](decisions/0002-go-tree-engine-and-artifacts.md)

## Technology constraints

Implement fitting, split search, prediction, required metrics, and model serialization as first-party Go code over typed contiguous slices. Apache Arrow Go may encode typed arrays but must not supply estimator algorithms. Do not use an external ML, dataframe, numerical, statistics, or serialization runtime.

## Estimator contract

Define typed interfaces for training, prediction, persistence, and resumable state. Methods accept `context.Context`, validated immutable specifications and matrix views, caller-owned output buffers where appropriate, and explicit error returns. V1 estimators are:

- Histogram regression tree.
- Random-forest regressor.
- Squared-error gradient-boosted regression trees.

Implementations expose no mutable internal slices through public interfaces. Loaded models are immutable and safe for concurrent prediction after validation.

## Tree behavior

Common parameters include maximum depth, minimum split/leaf samples, feature subsampling, histogram-bin count, missing-value routing, seed, and stopping criteria.

- Learn numeric histogram thresholds from training data only.
- Treat NaN feature values as a dedicated bin and learn their branch direction.
- Reject infinite feature values at materialization boundaries.
- Choose splits by squared-error reduction using `float64` accumulators.
- Use the deterministic tie-break: gain, feature index, threshold-bin index, then missing direction.
- Define equality using exact stored values; do not use epsilon-based tie collapsing.
- Build trees iteratively so partial state can be checkpointed.
- Store tree fields as same-length typed arrays with node indices, not pointer graphs.
- Validate child indices, leaf/split exclusivity, feature bounds, array lengths, and acyclicity when loading.

Random forests add estimator count and bootstrap settings and average predictions in estimator-index order. Gradient boosting adds learning rate, row subsampling, and stage count; stages and residual updates remain sequential.

## Deterministic random generation

Use a first-party, versioned counter-based pseudo-random generator whose algorithm is part of the engine schema. Derive independent streams from stable run, fold, estimator, stage, node, and task identifiers with domain-separated hashing. Never derive results from goroutine scheduling, map iteration, wall-clock time, PID, or process start order.

## Parallel execution

- Random-forest estimators may train concurrently under a CPU lease.
- Tree-node candidate features may be divided into indexed compute tasks.
- Each task reads immutable arrays and returns a task-owned split candidate.
- The orchestration goroutine applies the winner and solely mutates tree state.
- Gradient-boosting split evaluation may be parallel, but residual updates and stages are ordered.
- Results are reduced in logical task-index order rather than channel arrival order.
- Nested parallel sections reuse the worker lease and never create an unbounded goroutine per row, feature, or candidate.

Output artifacts and reported metrics must be byte-identical across supported goroutine counts and task completion orders for the same platform, engine version, inputs, and configuration. Cross-engine-version differences require an explicit engine-version change.

## Artifacts

A model artifact directory contains:

- a canonical versioned JSON manifest;
- one or more Arrow IPC files containing typed contiguous arrays;
- checksums for every file and the manifest payload;
- schema and engine versions;
- model kind and complete configuration;
- feature schema and target definition;
- training data fingerprint and cutoff;
- seeds, generator version, and deterministic ordering metadata;
- application build revision and feature implementation fingerprints.

Manifest JSON uses stable field names and deterministic canonical encoding defined by the artifact schema. Floating-point non-finite values are not permitted in JSON. Artifact completion writes all files to a sibling temporary directory, flushes and validates them, then atomically promotes the directory reference. The coordinator indexes only validated completed artifacts.

Loading rejects incompatible schema, engine, feature, target, array type, dimension, checksum, or model invariant values. Unknown required manifest fields or Arrow schemas fail closed. Completed artifacts are immutable.
