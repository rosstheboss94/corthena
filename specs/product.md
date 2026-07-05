# Product Specification

**Status:** Authoritative  
**Owner:** Product  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Roadmap](roadmap.md)

## Goal

Build a Windows-first, single-user Go workstation for researching machine-learning strategies on adjusted US-equity OHLCV bars. It supports data import, feature research, resumable training, walk-forward evaluation, reference portfolio backtests, model registration, and batch inference.

## Primary user and workflows

The initial user is a local quantitative researcher who needs to:

- Import and validate CSV or Parquet market data.
- Inspect price, feature, target, and prediction series.
- Configure pooled cross-sectional or per-symbol experiments.
- Train regression trees, random forests, and gradient-boosted trees.
- Pause and recover long-running jobs.
- Compare walk-forward results and reference backtests.
- Refit selected configurations and score historical or latest data.

The supported surfaces are a Raylib desktop UI, a typed Go client package, a versioned loopback API, and a CLI.

## V1 boundaries

- Research only; no broker integration, live orders, or capital deployment.
- US-equity bars supplied by the user; no vendor download integration.
- Adjusted OHLCV is required. Corporate-action processing is not performed by the app.
- Predictions are configurable forward returns or rankings, not direction classes or direct trading actions.
- Reference portfolios are analytical tools, not execution simulations.
- Windows developer installation is the delivery target. Bundled executables and other operating systems are deferred.
- Borrow availability, market impact, and survivorship-bias correction are not modeled.
- The runtime, extensions, compiled feature registrations, and supported client library are implemented in Go.

## Success criteria

- Long-running work survives clean pause/resume and unclean worker interruption.
- Time-aware feature, target, split, and execution rules prevent future-data leakage.
- Model output is deterministic for the same inputs, configuration, seed, engine version, and supported worker allocation.
- The UI remains responsive during imports, training, evaluation, and inference.
- Experiments and artifacts are auditable through immutable run metadata and fingerprints.
- The engine comfortably supports experiments approaching ten million rows using bounded-memory materialization.
- All first-party Go code passes formatting, static analysis, tests, and applicable race checks with zero findings.

## Terminology

- **Dataset catalog:** Mutable logical collection of imported canonical bars.
- **Materialization:** Run-specific, disk-backed feature/target arrays.
- **Experiment definition:** Reusable configuration for data, features, target, split, model, and portfolio.
- **Run:** Immutable execution and its outputs.
- **Job:** Schedulable unit that creates or processes a run.
- **Final model:** Model refit on all eligible history through a recorded cutoff after evaluation.
- **Alias:** Human-managed name such as `candidate` or `champion` pointing to a final model.
- **Coordinator:** Loopback service that owns scheduling and durable metadata writes.
- **Worker:** Isolated process for one active training job.
- **Compute slot:** Coordinator-leased unit of CPU parallelism consumed by a worker goroutine.
- **Degraded component:** A process that remains safe and available with a disclosed loss of optional capability or performance.
