# Research Workspace

Default panels are the primary OHLCV chart, feature browser, series inspector,
target preview, feature/target distributions, and row-level data table.

Linked panels synchronize dataset, symbols, interval, and visible time range
through configurable link groups. The default layout keeps the OHLCV chart
primary, groups inspectors in a side tab stack, and places the virtualized row
table below. Constrained widths collapse the six panels into one tab stack.

The chart provides candlesticks, volume, feature/target overlays,
train/validation/test regions, crosshair, wheel zoom, shift-drag pan, box
selection, layer visibility, and reset. Feature/row selection, sorting,
filtering, and cursor pagination retain stable identities.

Feature values retain explicit missing prefixes until lookback is available.
Forward open-to-open targets exclude rows without a valid future target and do
not change membership when the visible range changes.
