# Evaluation and Backtesting

Support expanding and fixed rolling walk-forward windows with train,
validation, test, purge, and embargo periods in chronological order. Select
models from validation only; test remains untouched until configuration is
fixed.

Report MSE, MAE, per-timestamp Pearson IC, stable average-rank IC, aggregate
level, and fold stability. Float64 kernels use documented missing rules and
stable reductions; insufficient finite groups produce missing metrics, not
zero. Folds and symbol evaluations may run under CPU leases, with stable fold
and symbol ordering.

Reference backtests rank by score with stable symbol-ID ties, allocate 50%
gross long to the top quantile and 50% gross short to the bottom, equal-weight
sides, execute at the next bar open, charge configurable basis-point turnover
costs, and report return, annualized volatility, Sharpe, drawdown, turnover,
hit rate, and attribution. Disclose unmodeled borrow, impact, and survivorship
correction. Missing next opens make orders ineligible; never fill from later
information.
