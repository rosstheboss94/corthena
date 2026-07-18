"""Deterministic leakage-safe Research preparation for the demo client."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.phase5b import (
    ContinuousLayer,
    OHLCVLayer,
    Rect,
    Region,
    RegionKind,
    RegionLayer,
    StyleRole,
    prepare_frame,
)
from corthena.ui.research.models import (
    ResearchBar,
    ResearchBin,
    ResearchDistribution,
    ResearchFeatureDescriptor,
    ResearchPage,
    ResearchQuery,
    ResearchScenario,
    ResearchSeries,
    ResearchSnapshot,
    ResearchSort,
    ResearchTargetPreview,
    ResearchValue,
)
from corthena.ui.table import Cell, CellKind, Column, Dataset, Row
from corthena.ui.visualization import Candle, LayerKind, Sample

_MASK = (1 << 64) - 1
_DATASET_START = datetime(2020, 7, 9, tzinfo=UTC)
_DATASET_END = datetime(2026, 7, 9, tzinfo=UTC)
_MAXIMUM_BARS = 100_000
FEATURES = (
    ResearchFeatureDescriptor("ret_5", "1.0.0", 5, "five-bar close return", "demo-ret5-v1"),
    ResearchFeatureDescriptor(
        "volatility_20",
        "1.0.0",
        20,
        "rolling return volatility",
        "demo-vol20-v1",
    ),
    ResearchFeatureDescriptor(
        "volume_z_30", "1.0.0", 30, "rolling volume z-score", "demo-volz30-v1"
    ),
)


@dataclass(frozen=True, slots=True)
class _BaseRow:
    row_id: str
    timestamp: datetime
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: float
    source_index: int


@dataclass(frozen=True, slots=True)
class _DemoRow:
    base: _BaseRow
    features: tuple[ResearchValue, ...]
    target: ResearchValue


def build_research_snapshot(
    seed: int,
    fixed_clock: datetime,
    query: ResearchQuery,
    cancellation: CancellationSignalProtocol,
) -> ResearchSnapshot:
    """Prepare one immutable Research response from explicit replay inputs."""
    _check_cancelled(cancellation, query.request_id)
    if query.scenario is ResearchScenario.LOADING:
        cancellation.wait()
        raise RequestCancelledError(query.request_id)
    if query.scenario is ResearchScenario.CANCELLED:
        raise RequestCancelledError(query.request_id)
    if query.scenario is ResearchScenario.FAILURE:
        raise RuntimeError("deterministic Research request failed")
    unknown = tuple(name for name in query.selected_features if name not in _feature_names())
    if unknown:
        raise ValueError(f"unknown Research features: {unknown!r}")
    if query.dataset_id != "dataset-us-equities":
        raise ValueError(f"unknown Research dataset {query.dataset_id!r}")
    available = {"AAPL", "MSFT", "NVDA", "AMD"}
    if not set(query.symbols) <= available:
        raise ValueError("Research query contains a symbol outside the dataset")
    prepared_at = fixed_clock.astimezone(UTC) + _generation_offset(query.generation)
    if query.scenario is ResearchScenario.EMPTY:
        return _empty_snapshot(query, seed, fixed_clock, prepared_at)
    step = query.interval.duration
    visible_start = max(query.time_range.start, _DATASET_START)
    visible_end = min(query.time_range.end, _DATASET_END)
    if visible_start >= visible_end:
        return _empty_snapshot(query, seed, fixed_clock, prepared_at)
    start = max(_DATASET_START, visible_start - step * 30)
    end = min(_DATASET_END, visible_end + step * (query.target.horizon_bars + 1))
    count = int((end - start) / step) + 1
    if count > _MAXIMUM_BARS:
        raise ValueError(f"Research query expands beyond {_MAXIMUM_BARS} bars")
    symbols = tuple(sorted(query.symbols))
    by_symbol: list[tuple[_DemoRow, ...]] = []
    for symbol_index, symbol in enumerate(symbols):
        _check_cancelled(cancellation, query.request_id)
        all_rows = _build_symbol_rows(
            seed,
            query,
            symbol,
            start,
            count,
            symbol_index,
            len(symbols),
        )
        by_symbol.append(
            tuple(row for row in all_rows if visible_start <= row.base.timestamp <= visible_end)
        )
    if not by_symbol or not by_symbol[0]:
        return _empty_snapshot(query, seed, fixed_clock, prepared_at)
    interleaved = tuple(
        by_symbol[symbol_index][row_index]
        for row_index in range(len(by_symbol[0]))
        for symbol_index in range(len(by_symbol))
    )
    primary = by_symbol[0]
    bars = tuple(_bar(row) for row in primary)
    features = _series(primary)
    target = _target(primary, query)
    return ResearchSnapshot(
        query=query,
        frame=_prepare_frame(query, primary),
        bars=bars,
        features=features,
        target=target,
        distributions=_distributions(features, target),
        rows=_page(query, interleaved),
        replay_seed=seed,
        replay_clock=fixed_clock,
        prepared_at=prepared_at,
        degraded=query.scenario in {ResearchScenario.DEGRADED, ResearchScenario.RECONNECTING},
    )


def _build_symbol_rows(
    seed: int,
    query: ResearchQuery,
    symbol: str,
    start: datetime,
    count: int,
    symbol_index: int,
    symbol_count: int,
) -> tuple[_DemoRow, ...]:
    step = query.interval.duration
    step_seconds = int(step.total_seconds())
    base_price = 70 + _stable_text_hash(f"{query.dataset_id}|{symbol}") % 180
    bases: list[_BaseRow] = []
    for index in range(count):
        timestamp = start + step * index
        absolute = int(timestamp.timestamp()) // step_seconds
        ordinal = int((timestamp - _DATASET_START) / step)
        phase = ordinal + symbol_index * 17
        trend = ordinal * 0.002
        noise = _stable_unit(seed, absolute, symbol_index + 1) - 0.5
        close = (
            base_price
            + trend
            + 5 * math.sin(phase * 0.043)
            + 2.2 * math.sin(phase * 0.013)
            + noise * 1.4
        )
        open_value = (
            base_price
            + trend
            + 5 * math.sin((phase - 0.65) * 0.043)
            + 2.2 * math.sin((phase - 0.65) * 0.013)
            + (_stable_unit(seed, absolute, symbol_index + 11) - 0.5) * 1.2
        )
        spread = 0.4 + _stable_unit(seed, absolute, symbol_index + 21) * 1.8
        high = max(open_value, close) + spread
        low = min(open_value, close) - spread * (
            0.7 + _stable_unit(seed, absolute, symbol_index + 31) * 0.4
        )
        volume = (
            650_000
            + _stable_unit(seed, absolute, symbol_index + 41) * 4_200_000
            + abs(math.sin(phase * 0.09)) * 900_000
        )
        bases.append(
            _BaseRow(
                f"{query.dataset_id}|{symbol}|{int(timestamp.timestamp() * 1_000_000_000)}",
                timestamp,
                symbol,
                open_value,
                high,
                low,
                close,
                volume,
                ordinal * symbol_count + symbol_index + 1,
            )
        )
    rows: list[_DemoRow] = []
    for index, base in enumerate(bases):
        values = tuple(
            _feature_value(tuple(bases), index, feature_index, descriptor)
            for feature_index, descriptor in enumerate(FEATURES)
        )
        target_index = index + 1 + query.target.horizon_bars
        if target_index >= len(bases):
            target = ResearchValue(base.timestamp, None)
        else:
            start_open = bases[index + 1].open
            end_open = bases[target_index].open
            value = (
                math.log(end_open / start_open)
                if query.target.log_return
                else end_open / start_open - 1
            )
            target = ResearchValue(base.timestamp, value, bases[target_index].timestamp)
        rows.append(_DemoRow(base, values, target))
    return tuple(rows)


def _feature_value(
    rows: tuple[_BaseRow, ...],
    index: int,
    feature_index: int,
    descriptor: ResearchFeatureDescriptor,
) -> ResearchValue:
    if index < descriptor.lookback:
        return ResearchValue(rows[index].timestamp, None)
    if feature_index == 0:
        value = rows[index].close / rows[index - descriptor.lookback].close - 1
    elif feature_index == 1:
        returns = tuple(
            rows[cursor].close / rows[cursor - 1].close - 1
            for cursor in range(index - descriptor.lookback + 1, index + 1)
        )
        value = _deviation(returns)
    else:
        window = tuple(rows[cursor].volume for cursor in range(index - descriptor.lookback, index))
        mean = sum(window) / len(window)
        deviation = _deviation(window)
        value = 0.0 if deviation == 0 else (rows[index].volume - mean) / deviation
    return ResearchValue(rows[index].timestamp, value)


def _prepare_frame(query: ResearchQuery, rows: tuple[_DemoRow, ...]):
    candles = tuple(
        Candle(
            index + 1,
            row.base.timestamp.timestamp(),
            row.base.open,
            row.base.high,
            row.base.low,
            row.base.close,
            row.base.volume,
        )
        for index, row in enumerate(rows)
    )
    selected = _feature_names().index(query.selected_features[0])
    feature = tuple(
        Sample(
            index + 1,
            row.base.timestamp.timestamp(),
            row.base.close * (1 + max(-4.0, min(4.0, row.features[selected].value or 0.0)) * 0.006),
        )
        for index, row in enumerate(rows)
        if not row.features[selected].missing
    )
    target = tuple(
        Sample(
            index + 1,
            row.base.timestamp.timestamp(),
            row.base.close * (1 + (row.target.value or 0.0)),
        )
        for index, row in enumerate(rows)
        if not row.target.missing
    )
    minimum_y = min(row.base.low for row in rows)
    maximum_y = max(row.base.high for row in rows)
    padding = max(1.0, (maximum_y - minimum_y) * 0.06)
    dataset_minimum = _DATASET_START.timestamp()
    dataset_maximum = _DATASET_END.timestamp()
    span = dataset_maximum - dataset_minimum
    layers = (
        RegionLayer(
            "splits",
            (
                Region(dataset_minimum, dataset_minimum + span * 0.60, RegionKind.TRAIN),
                Region(
                    dataset_minimum + span * 0.60,
                    dataset_minimum + span * 0.80,
                    RegionKind.VALIDATION,
                ),
                Region(dataset_minimum + span * 0.80, dataset_maximum, RegionKind.TEST),
            ),
        ),
        OHLCVLayer("ohlcv", candles),
        ContinuousLayer("feature", LayerKind.LINE, feature, StyleRole.WARNING),
        ContinuousLayer("target", LayerKind.SCATTER, target, StyleRole.SECONDARY),
    )
    data = Rect(
        rows[0].base.timestamp.timestamp(),
        minimum_y - padding,
        rows[-1].base.timestamp.timestamp(),
        maximum_y + padding,
    )
    viewport = Rect(0, 0, query.resolution, 600)
    return prepare_frame(query.generation, data, viewport, query.resolution, layers)


def _series(rows: tuple[_DemoRow, ...]) -> tuple[ResearchSeries, ...]:
    result: list[ResearchSeries] = []
    for feature_index, descriptor in enumerate(FEATURES):
        values = tuple(row.features[feature_index] for row in rows)
        finite = tuple(value.value for value in values if value.value is not None)
        result.append(
            ResearchSeries(
                descriptor,
                values,
                min(finite, default=0.0),
                max(finite, default=0.0),
                sum(value.missing for value in values),
            )
        )
    return tuple(result)


def _target(rows: tuple[_DemoRow, ...], query: ResearchQuery) -> ResearchTargetPreview:
    values = tuple(row.target for row in rows)
    excluded = sum(value.missing for value in values)
    return ResearchTargetPreview(query.target, values, len(values) - excluded, excluded)


def _distributions(
    series: tuple[ResearchSeries, ...], target: ResearchTargetPreview
) -> tuple[ResearchDistribution, ...]:
    return (
        *(ResearchDistribution(item.descriptor.name, _histogram(item.values)) for item in series),
        ResearchDistribution("target", _histogram(target.values)),
    )


def _histogram(values: tuple[ResearchValue, ...], count: int = 16) -> tuple[ResearchBin, ...]:
    finite = tuple(value.value for value in values if value.value is not None)
    if not finite:
        return ()
    minimum, maximum = min(finite), max(finite)
    if maximum <= minimum:
        maximum = minimum + 1
    width = (maximum - minimum) / count
    counts = [0] * count
    for value in finite:
        index = min(count - 1, max(0, int((value - minimum) / (maximum - minimum) * count)))
        counts[index] += 1
    return tuple(
        ResearchBin(minimum + index * width, minimum + (index + 1) * width, amount)
        for index, amount in enumerate(counts)
    )


def _page(query: ResearchQuery, rows: tuple[_DemoRow, ...]) -> ResearchPage:
    filtered = [
        row
        for row in rows
        if not row.target.missing and (not query.filter or query.filter in row.base.symbol)
    ]
    if query.sort is ResearchSort.TIME_DESCENDING:
        filtered.sort(key=lambda row: (-row.base.timestamp.timestamp(), row.base.source_index))
    elif query.sort is ResearchSort.TARGET_DESCENDING:
        filtered.sort(key=lambda row: (-(row.target.value or 0.0), row.base.source_index))
    offset = int(query.cursor or 0)
    page_rows = filtered[offset : offset + query.page_size]
    end = offset + len(page_rows)
    dataset = Dataset(_columns(), tuple(_table_row(row) for row in page_rows))
    return ResearchPage(dataset, str(end) if end < len(filtered) else "", len(filtered))


def _columns() -> tuple[Column, ...]:
    columns = [
        Column("row_id", "Row ID", CellKind.STRING, 210, 100, True),
        Column("timestamp", "Timestamp (UTC)", CellKind.TIME, 170, 120),
        Column("symbol", "Symbol", CellKind.STRING, 80, 60),
        Column("open", "Open", CellKind.FLOAT, 88, 64),
        Column("high", "High", CellKind.FLOAT, 88, 64),
        Column("low", "Low", CellKind.FLOAT, 88, 64),
        Column("close", "Close", CellKind.FLOAT, 88, 64),
        Column("volume", "Volume", CellKind.FLOAT, 110, 80),
    ]
    columns.extend(Column(item.name, item.name, CellKind.FLOAT, 112, 80) for item in FEATURES)
    columns.append(Column("target", "Forward target", CellKind.FLOAT, 124, 90))
    return tuple(columns)


def _table_row(row: _DemoRow) -> Row:
    base = row.base
    cells = [
        Cell(CellKind.STRING, base.row_id),
        Cell(CellKind.TIME, base.timestamp),
        Cell(CellKind.STRING, base.symbol),
        Cell(CellKind.FLOAT, base.open),
        Cell(CellKind.FLOAT, base.high),
        Cell(CellKind.FLOAT, base.low),
        Cell(CellKind.FLOAT, base.close),
        Cell(CellKind.FLOAT, base.volume),
    ]
    cells.extend(Cell(CellKind.FLOAT, value.value) for value in row.features)
    cells.append(Cell(CellKind.FLOAT, row.target.value))
    return Row(base.row_id, base.source_index, tuple(cells))


def _empty_snapshot(
    query: ResearchQuery,
    seed: int,
    fixed_clock: datetime,
    prepared_at: datetime,
) -> ResearchSnapshot:
    values = ResearchTargetPreview(query.target, (), 0, 0)
    return ResearchSnapshot(
        query,
        None,
        (),
        (),
        values,
        (),
        ResearchPage(Dataset(_columns(), ()), "", 0),
        seed,
        fixed_clock,
        prepared_at,
    )


def _bar(row: _DemoRow) -> ResearchBar:
    value = row.base
    return ResearchBar(
        value.row_id,
        value.timestamp,
        value.symbol,
        value.open,
        value.high,
        value.low,
        value.close,
        value.volume,
    )


def _feature_names() -> tuple[str, ...]:
    return tuple(feature.name for feature in FEATURES)


def _deviation(values: tuple[float, ...]) -> float:
    mean = sum(values) / len(values)
    return math.sqrt(sum((value - mean) ** 2 for value in values) / len(values))


def _stable_text_hash(value: str) -> int:
    result = 1_469_598_103_934_665_603
    for item in value.encode():
        result ^= item
        result = result * 1_099_511_628_211 & _MASK
    return result


def _stable_unit(seed: int, index: int, stream: int) -> float:
    value = (seed ^ (index * 0x9E3779B97F4A7C15) ^ (stream * 0xBF58476D1CE4E5B9)) & _MASK
    value ^= value >> 30
    value = value * 0xBF58476D1CE4E5B9 & _MASK
    value ^= value >> 27
    value = value * 0x94D049BB133111EB & _MASK
    value ^= value >> 31
    return (value >> 11) / float(1 << 53)


def _generation_offset(generation: int) -> timedelta:
    return timedelta(milliseconds=generation)


def _check_cancelled(cancellation: CancellationSignalProtocol, request_id: str) -> None:
    if cancellation.is_set():
        raise RequestCancelledError(request_id)
