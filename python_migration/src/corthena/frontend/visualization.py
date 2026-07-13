"""Pure, deterministic Phase 5 chart geometry and level-of-detail kernels."""

from __future__ import annotations

import math
import struct
from dataclasses import dataclass
from enum import StrEnum


class VisualizationError(ValueError):
    """Raised when chart input cannot produce a safe immutable frame."""


@dataclass(frozen=True, slots=True)
class Point:
    """A double precision chart coordinate."""

    x: float
    y: float

    def __post_init__(self) -> None:
        _finite(self.x, "x")
        _finite(self.y, "y")


@dataclass(frozen=True, slots=True)
class Rect:
    """A finite, positive-area rectangle."""

    min_x: float
    min_y: float
    max_x: float
    max_y: float

    def __post_init__(self) -> None:
        for name, value in (
            ("min_x", self.min_x),
            ("min_y", self.min_y),
            ("max_x", self.max_x),
            ("max_y", self.max_y),
        ):
            _finite(value, name)
        if self.max_x <= self.min_x or self.max_y <= self.min_y:
            raise VisualizationError("rectangle must have positive area")

    @property
    def width(self) -> float:
        return self.max_x - self.min_x

    @property
    def height(self) -> float:
        return self.max_y - self.min_y

    def contains(self, point: Point) -> bool:
        return self.min_x <= point.x <= self.max_x and self.min_y <= point.y <= self.max_y


@dataclass(frozen=True, slots=True)
class Transform:
    """Map a data rectangle to a screen rectangle, inverting screen Y."""

    data: Rect
    screen: Rect

    def __post_init__(self) -> None:
        for value in (self.screen.min_x, self.screen.min_y, self.screen.max_x, self.screen.max_y):
            checked_float32(value)

    def forward(self, point: Point) -> Point:
        return Point(
            self.screen.min_x + (point.x - self.data.min_x) * self.screen.width / self.data.width,
            self.screen.max_y - (point.y - self.data.min_y) * self.screen.height / self.data.height,
        )

    def inverse(self, point: Point) -> Point:
        return Point(
            self.data.min_x + (point.x - self.screen.min_x) * self.data.width / self.screen.width,
            self.data.min_y + (self.screen.max_y - point.y) * self.data.height / self.screen.height,
        )


def checked_float32(value: float) -> float:
    """Validate and round a coordinate exactly as a native float32 conversion."""
    _finite(value, "coordinate")
    if abs(value) > 3.4028234663852886e38:
        raise VisualizationError("coordinate is outside float32 range")
    converted = struct.unpack("!f", struct.pack("!f", value))[0]
    if not math.isfinite(converted):
        raise VisualizationError("coordinate converted to a non-finite float32")
    return converted


def clip_segment(bounds: Rect, start: Point, end: Point) -> tuple[Point, Point] | None:
    """Clip a segment with the deterministic Liang-Barsky algorithm."""
    dx, dy = end.x - start.x, end.y - start.y
    lower, upper = 0.0, 1.0
    for p, q in (
        (-dx, start.x - bounds.min_x),
        (dx, bounds.max_x - start.x),
        (-dy, start.y - bounds.min_y),
        (dy, bounds.max_y - start.y),
    ):
        if p == 0:
            if q < 0:
                return None
            continue
        ratio = q / p
        if p < 0:
            if ratio > upper:
                return None
            lower = max(lower, ratio)
        else:
            if ratio < lower:
                return None
            upper = min(upper, ratio)
    return (
        Point(start.x + lower * dx, start.y + lower * dy),
        Point(start.x + upper * dx, start.y + upper * dy),
    )


def ticks(minimum: float, maximum: float, target: int) -> tuple[float, ...]:
    """Return deterministic 1/2/5-decade ticks within a range."""
    _finite(minimum, "minimum")
    _finite(maximum, "maximum")
    if maximum <= minimum or target < 2:
        raise VisualizationError("invalid tick range or target")
    raw = (maximum - minimum) / (target - 1)
    power = 10.0 ** math.floor(math.log10(raw))
    fraction = raw / power
    step = (
        power
        if fraction <= 1
        else 2 * power
        if fraction <= 2
        else 5 * power
        if fraction <= 5
        else 10 * power
    )
    first, last = math.ceil(minimum / step) * step, math.floor(maximum / step) * step
    count = round((last - first) / step) + 1
    if count < 0 or count > 1_000_000:
        raise VisualizationError("unreasonable tick count")
    return tuple(
        0.0 if abs(value := first + index * step) < step * 1e-12 else value
        for index in range(count)
    )


@dataclass(frozen=True, slots=True, order=True)
class Sample:
    """One ordered continuous sample with a stable source tie-breaker."""

    source_index: int
    x: float
    y: float


@dataclass(frozen=True, slots=True)
class Candle:
    """One ordered OHLCV sample."""

    source_index: int
    x: float
    open: float
    high: float
    low: float
    close: float
    volume: float


@dataclass(frozen=True, slots=True)
class CandleBucket:
    """One horizontal-pixel OHLCV aggregate."""

    first_source_index: int
    last_source_index: int
    x: float
    open: float
    high: float
    low: float
    close: float
    volume: float


@dataclass(frozen=True, slots=True)
class LODStats:
    """Instrumented source, bucket, and render output work."""

    source_values: int
    buckets: int
    output_values: int


def aggregate_continuous(
    samples: tuple[Sample, ...], minimum: float, maximum: float, pixel_width: int
) -> tuple[tuple[Sample, ...], LODStats]:
    """Preserve first/last/min/max samples per occupied pixel bucket."""
    _validate_range(minimum, maximum, pixel_width)
    _validate_samples(samples)
    buckets: list[list[Sample]] = [[] for _ in range(pixel_width)]
    for sample in samples:
        index = _bucket(sample.x, minimum, maximum, pixel_width)
        if index is not None:
            buckets[index].append(sample)
    output: list[Sample] = []
    occupied = 0
    for bucket in buckets:
        if not bucket:
            continue
        occupied += 1
        minimum_sample = min(bucket, key=lambda sample: (sample.y, sample.source_index))
        maximum_sample = min(bucket, key=lambda sample: (-sample.y, sample.source_index))
        chosen = {
            item.source_index: item
            for item in (bucket[0], minimum_sample, maximum_sample, bucket[-1])
        }
        output.extend(chosen[index] for index in sorted(chosen))
    return tuple(output), LODStats(len(samples), occupied, len(output))


def aggregate_candles(
    candles: tuple[Candle, ...], minimum: float, maximum: float, pixel_width: int
) -> tuple[tuple[CandleBucket, ...], LODStats]:
    """Aggregate ordered candles while preserving OHLCV semantics."""
    _validate_range(minimum, maximum, pixel_width)
    _validate_candles(candles)
    groups: list[list[Candle]] = [[] for _ in range(pixel_width)]
    for candle in candles:
        index = _bucket(candle.x, minimum, maximum, pixel_width)
        if index is not None:
            groups[index].append(candle)
    result = tuple(
        CandleBucket(
            group[0].source_index,
            group[-1].source_index,
            group[0].x,
            group[0].open,
            max(item.high for item in group),
            min(item.low for item in group),
            group[-1].close,
            sum(item.volume for item in group),
        )
        for group in groups
        if group
    )
    if any(not math.isfinite(item.volume) for item in result):
        raise VisualizationError("aggregated candle volume overflow")
    return result, LODStats(len(candles), len(result), len(result))


class LayerKind(StrEnum):
    """Closed generic chart layer kinds owned by Phase 5."""

    OHLCV = "ohlcv"
    LINE = "line"
    AREA = "area"
    HISTOGRAM = "histogram"
    SCATTER = "scatter"
    EQUITY = "equity"
    DRAWDOWN = "drawdown"
    HEATMAP = "heatmap"
    FEATURE_IMPORTANCE = "feature_importance"
    PREDICTIONS = "predictions"
    TRADES = "trades"
    REGIONS = "regions"


@dataclass(frozen=True, slots=True)
class ChartViewport:
    """Generation-bound generic chart interaction state."""

    generation: int
    data: Rect
    crosshair: Point | None = None
    selection: Rect | None = None

    def pan(self, delta_x: float, delta_y: float) -> ChartViewport:
        return ChartViewport(
            self.generation + 1,
            Rect(
                self.data.min_x + delta_x,
                self.data.min_y + delta_y,
                self.data.max_x + delta_x,
                self.data.max_y + delta_y,
            ),
        )

    def zoom(self, anchor: Point, factor: float) -> ChartViewport:
        if not math.isfinite(factor) or factor <= 0:
            raise VisualizationError("zoom factor must be finite and positive")

        def scaled(low: float, high: float, at: float) -> tuple[float, float]:
            return at + (low - at) / factor, at + (high - at) / factor

        min_x, max_x = scaled(self.data.min_x, self.data.max_x, anchor.x)
        min_y, max_y = scaled(self.data.min_y, self.data.max_y, anchor.y)
        return ChartViewport(self.generation + 1, Rect(min_x, min_y, max_x, max_y))


def _finite(value: float, name: str) -> None:
    if not math.isfinite(value):
        raise VisualizationError(f"{name} must be finite")


def _validate_range(minimum: float, maximum: float, width: int) -> None:
    _finite(minimum, "minimum")
    _finite(maximum, "maximum")
    if maximum <= minimum or width <= 0:
        raise VisualizationError("invalid LOD viewport")


def _bucket(x: float, minimum: float, maximum: float, width: int) -> int | None:
    if x < minimum or x > maximum:
        return None
    return (
        width - 1
        if x == maximum
        else min(width - 1, int((x - minimum) * width / (maximum - minimum)))
    )


def _validate_samples(samples: tuple[Sample, ...]) -> None:
    for index, sample in enumerate(samples):
        _finite(sample.x, "sample x")
        _finite(sample.y, "sample y")
        if sample.source_index < 0:
            raise VisualizationError("source index must be non-negative")
        if index and (
            sample.source_index <= samples[index - 1].source_index
            or sample.x < samples[index - 1].x
        ):
            raise VisualizationError("samples must have increasing indexes and nondecreasing x")


def _validate_candles(candles: tuple[Candle, ...]) -> None:
    for index, candle in enumerate(candles):
        for value in (candle.x, candle.open, candle.high, candle.low, candle.close, candle.volume):
            _finite(value, "candle value")
        if (
            candle.source_index < 0
            or candle.volume < 0
            or candle.high < max(candle.open, candle.close)
            or candle.low > min(candle.open, candle.close)
        ):
            raise VisualizationError("candle violates OHLCV bounds")
        if index and (
            candle.source_index <= candles[index - 1].source_index
            or candle.x < candles[index - 1].x
        ):
            raise VisualizationError("candles must have increasing indexes and nondecreasing x")


__all__ = [
    "Candle",
    "CandleBucket",
    "ChartViewport",
    "LODStats",
    "LayerKind",
    "Point",
    "Rect",
    "Sample",
    "Transform",
    "VisualizationError",
    "aggregate_candles",
    "aggregate_continuous",
    "checked_float32",
    "clip_segment",
    "ticks",
]
