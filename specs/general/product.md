# Product Specification

**Status:** Authoritative
**Owner:** Product
**Last updated:** 2026-07-18
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Roadmap](roadmap.md)

## Goal

Build a Windows-first, single-user Python/Cython workstation for researching
machine-learning strategies on declared raw or provider split-adjusted
US-equity OHLCV bars. It supports data
import, feature research, resumable training where supported by the estimator,
walk-forward evaluation, reference portfolio backtests, model registration, and
batch inference.

## Primary user and workflows

The initial user is a local quantitative researcher who needs to:

- Import and validate CSV or Parquet market data and pull explicitly selected
  historical symbols from Massive.
- Inspect price, feature, target, and prediction series.
- Configure pooled cross-sectional or per-symbol experiments.
- Train approved scikit-learn and PyTorch model families, including tree-based
  regressors where supported.
- Pause and recover long-running jobs.
- Compare walk-forward results and reference backtests.
- Refit selected configurations and score historical or latest data.

The supported surfaces are a Raylib desktop UI, a typed Python client package,
a versioned loopback API, and a CLI.

## V1 boundaries

- Research only; no broker integration, live orders, or capital deployment.
- Historical US-equity OHLCV comes from user-selected CSV/Parquet files or
  explicit-symbol Massive pulls. Live feeds and unrestricted full-market
  downloads are not supported.
- Raw and provider split-adjusted datasets are distinct declared policies.
  Corporate-action processing is not performed by the app, and Massive data is
  never represented as dividend-adjusted.
- Predictions are configurable forward returns or rankings, not direction classes or direct trading actions.
- Reference portfolios are analytical tools, not execution simulations.
- Windows developer installation is the delivery target. Bundled executables and other operating systems are deferred.
- Borrow availability, market impact, and survivorship-bias correction are not modeled.
- The runtime, extensions, and supported client library are implemented in
  Python, with Cython limited to measured hot paths or native adapters.

## Success criteria

- Long-running work survives clean pause/resume and unclean worker interruption.
- Time-aware feature, target, split, and execution rules prevent future-data leakage.
- Model output is deterministic for the same inputs, configuration, seed, engine version, and supported worker allocation.
- The UI remains responsive during imports, training, evaluation, and inference.
- Experiments and artifacts are auditable through immutable run metadata and fingerprints.
- The engine comfortably supports experiments approaching ten million rows using bounded-memory materialization.
- All first-party Python/Cython code passes formatting, linting, type checks,
  tests, applicable lifecycle/concurrency checks, and vulnerability scans with
  zero findings.

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
- **Compute slot:** Coordinator-leased unit of CPU parallelism consumed by a
  worker process, thread pool, or library-owned numerical pool.
- **Degraded component:** A process that remains safe and available with a disclosed loss of optional capability or performance.
