# Experiment Definitions and Materialization

**Status:** Authoritative  
**Owner:** Experiments  
**Last updated:** 2026-07-23  
**Related:** [Data datasets](../data/datasets.md), [Research features and targets](../research/features-and-targets.md), [Jobs runtime](../jobs/runtime.md)

An experiment definition captures an immutable Data dataset binding, ordered
feature identities/configuration, target, split, model, portfolio, and optional
sweep. Drafts remain mutable and autosavable; accepted submissions are
command-idempotent and immutable.

Validation checks the dataset revision/fingerprint, compiled-feature identity,
forward target, chronological walk-forward split, purge/embargo horizon,
bounded model and sweep settings, finite portfolio values, and CPU limits.
Invalid drafts remain editable but cannot submit. Estimates and seeded grid or
random sweep trial lists are deterministic and persist before execution.

Draft files are schema-versioned, revision-aware, atomically replaced, and
quarantine unknown or invalid documents without exposing secrets. Evaluation,
autosave, and submission use typed asynchronous `UIClientProtocol` operations;
revision and generation checks reject late results. Submission captures the
accepted dataset revision, content fingerprint, feature descriptors, target,
full configuration, and command identity.

