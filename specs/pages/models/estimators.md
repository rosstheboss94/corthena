# Model Estimator Behavior

Corthena validates fitting, prediction, metrics, and resumable-state
boundaries, then delegates approved estimator internals to scikit-learn or
PyTorch where those libraries provide the needed capability. Typed interfaces
accept cancellable tokens, validated immutable specs, matrix views,
caller-owned buffers where useful, and explicit errors. Training, prediction,
persistence, and resumable-state boundaries expose no mutable internal arrays,
tensors, or library objects.

V1 supports approved tree-based regressors and neural models through adapters.
Loaded models are immutable at Corthena boundaries and safe for concurrent
prediction only through documented adapter rules.

Common parameters include depth, minimum split/leaf samples, feature
subsampling, histogram bins where supported, missing-value policy, seed, and
stopping criteria. Learn preprocessing, thresholds, normalization, encoders,
and model state from training data only. Reject infinities and unsupported
non-finite values before library calls.

Validate adapter-level indices, bounds, dimensions, estimator kind, fitted
state, feature names/order, target contract, dependency versions, and
compatibility metadata before exposing a model.

Tree ensembles preserve estimator count, bootstrap, learning-rate,
row-sampling, and stage-order metadata when the library exposes them. PyTorch
models preserve architecture, optimizer, scheduler, epoch, seed, and
training-state metadata.

Use versioned seed derivation with domain-separated stable identifiers. Never
derive results from scheduling, dict iteration, time, PID, process order, or
library pool arrival order. Library and adapter pools run under CPU leases,
tasks read immutable inputs, own outputs, and reducers apply results in logical
order. Nested pools are bounded.

Outputs and metrics are deterministic across supported worker counts. If a
library cannot provide byte-identical results across thread counts, constrain
execution to the deterministic supported mode and record that mode in the
manifest.

**Status:** Authoritative
**Owner:** ML
**Last updated:** 2026-07-23
**Related:** [models page index](README.md)
