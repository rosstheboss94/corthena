# Evaluation and Inference Specification

**Status:** Authoritative  
**Owner:** Research  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [Data](data-and-features.md), [Models](models.md), [API](api.md)

## Technology constraints

Implement statistics, ranks, portfolio accounting, and backtests as first-party Go code over typed slices. Use Apache Arrow Go for prediction and report tables and standard-library serialization where appropriate. Do not use an external statistics, dataframe, numerical, ML, or backtesting runtime.

## Walk-forward evaluation

Support expanding and fixed-length rolling windows with configurable train, validation, and test periods. Apply purge and embargo around target horizons. All folds preserve chronological ordering.

Model selection uses validation data only. Test observations remain untouched until the fold configuration is fixed.

Report:

- MSE and MAE.
- Per-timestamp Pearson information coefficient.
- Per-timestamp rank information coefficient using stable average ranks for ties.
- Aggregate metric level and stability across folds.

Metric kernels use `float64` accumulators, documented missing-value rules, and stable reduction order. Invalid groups with insufficient finite observations produce an explicit missing metric, not a silently coerced zero.

Folds and per-symbol evaluations may run concurrently when they hold CPU leases and do not introduce nested oversubscription. Results are stored and aggregated in stable fold and symbol order rather than completion order.

## Reference backtest

- Rank predictions at each rebalance timestamp with stable symbol-ID tie-breaking after equal scores.
- Allocate 50% gross long to the top quantile and 50% gross short to the bottom quantile.
- Equal-weight symbols within each side.
- Execute using the next bar open.
- Apply configurable basis-point costs to notional turnover.
- Report return, annualized volatility, Sharpe, drawdown, turnover, hit rate, and long/short attribution.

The result must disclose that borrow availability, impact, and survivorship correction are not modeled. Missing next-open values make the affected order ineligible; they must not be filled using later information.

## Final refit and registry

After walk-forward evaluation, refit the selected configuration on all eligible history through a recorded cutoff. The final model is the inference artifact.

Runs and model artifacts are immutable. Human-managed aliases such as `candidate` and `champion` point to final models. Promotion is never automatic. Alias changes are coordinator-owned transactions with immutable history.

## Batch inference

Score either a selected historical range or the latest imported snapshot.

Validate model schema, engine version, feature registry descriptors, feature schema, target definition, data compatibility, and required lookback before scoring. Persist predictions with symbol, timestamp, model ID, run ID, data fingerprint, feature implementation fingerprints, and score.

Loaded models are immutable and may predict concurrently into caller-owned exclusive output ranges. Cancellation discards incomplete temporary outputs. The coordinator promotes and indexes only complete checksummed prediction artifacts.

## Public domain types

`SplitSpec` and `PortfolioSpec` are validated Go value structs. Evaluation and inference results use typed domain values and explicit API DTO conversion rather than unstructured maps.
