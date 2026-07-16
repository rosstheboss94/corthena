"""Immutable typed values for the Phase 6 Research workflow."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum

from corthena.ui.phase5b import PreparedFrame
from corthena.ui.table import Dataset


class ResearchScenario(StrEnum):
    NORMAL = "normal"
    LOADING = "loading"
    EMPTY = "empty"
    FAILURE = "failure"
    DEGRADED = "degraded"
    RECONNECTING = "reconnecting"
    RECOVERED = "recovered"
    CANCELLED = "cancelled"
    QUEUE_SATURATED = "queue_saturated"


class ResearchSort(StrEnum):
    TIME_ASCENDING = "time_ascending"
    TIME_DESCENDING = "time_descending"
    TARGET_DESCENDING = "target_descending"


class ResearchLoadState(StrEnum):
    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    EMPTY = "empty"
    FAILED = "error"
    DEGRADED = "degraded"
    RECOVERED = "recovered"
    CANCELLED = "cancelled"
    BUSY = "queue_saturated"


class BarInterval(StrEnum):
    DAILY = "1d"
    HOURLY = "1h"

    @property
    def duration(self) -> timedelta:
        return timedelta(days=1) if self is BarInterval.DAILY else timedelta(hours=1)


@dataclass(frozen=True, slots=True)
class TimeRange:
    start: datetime
    end: datetime

    def __post_init__(self) -> None:
        if self.start.tzinfo is None or self.end.tzinfo is None:
            raise ValueError("Research range must be timezone-aware")
        if self.start.utcoffset() != timedelta(0) or self.end.utcoffset() != timedelta(0):
            raise ValueError("Research range must be normalized to UTC")
        if self.start >= self.end:
            raise ValueError("Research range start must precede end")
        if self.end - self.start > timedelta(days=36500):
            raise ValueError("Research range is too large")


@dataclass(frozen=True, slots=True)
class TargetSpec:
    kind: str = "forward_open_return"
    horizon_bars: int = 5
    log_return: bool = False

    def __post_init__(self) -> None:
        if self.kind != "forward_open_return" or not 1 <= self.horizon_bars <= 1024:
            raise ValueError("target must be a 1-1024 bar forward open return")


@dataclass(frozen=True, slots=True)
class ResearchQuery:
    correlation_id: str
    group_id: str
    generation: int
    dataset_id: str
    symbols: tuple[str, ...]
    interval: BarInterval
    time_range: TimeRange
    selected_features: tuple[str, ...]
    target: TargetSpec
    resolution: int = 1200
    cursor: str = ""
    page_size: int = 120
    sort: ResearchSort = ResearchSort.TIME_ASCENDING
    filter: str = ""
    scenario: ResearchScenario = ResearchScenario.NORMAL

    def __post_init__(self) -> None:
        identities = (self.correlation_id, self.group_id, self.dataset_id)
        if any(not value or value.strip() != value for value in identities):
            raise ValueError("Research correlation, group, and dataset identities are required")
        if self.generation < 1:
            raise ValueError("Research generation must be positive")
        if not 1 <= len(self.symbols) <= 64 or len(set(self.symbols)) != len(self.symbols):
            raise ValueError("Research requires one to 64 unique symbols")
        if any(not value or value.strip() != value for value in self.symbols):
            raise ValueError("Research symbols must be normalized")
        if not 1 <= len(self.selected_features) <= 16 or len(set(self.selected_features)) != len(
            self.selected_features
        ):
            raise ValueError("Research requires one to 16 unique features")
        if any(not value or value.strip() != value for value in self.selected_features):
            raise ValueError("Research features must be normalized")
        if not 64 <= self.resolution <= 8192 or not 1 <= self.page_size <= 500:
            raise ValueError("Research resolution or page size is outside its bound")
        if len(self.filter) > 128:
            raise ValueError("Research filter is too long")
        if self.cursor and (not self.cursor.isdecimal() or int(self.cursor) > 10_000_000_000):
            raise ValueError("Research cursor is invalid")

    @property
    def request_id(self) -> str:
        return self.correlation_id


@dataclass(frozen=True, slots=True)
class ResearchBar:
    row_id: str
    timestamp: datetime
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: float

    def __post_init__(self) -> None:
        values = (self.open, self.high, self.low, self.close, self.volume)
        if not self.row_id or not self.symbol or not all(math.isfinite(value) for value in values):
            raise ValueError("Research bar identity and values must be valid")
        if self.timestamp.tzinfo is None or self.timestamp.utcoffset() != timedelta(0):
            raise ValueError("Research bar timestamp must be UTC")
        if (
            self.volume < 0
            or self.low > min(self.open, self.close)
            or self.high < max(self.open, self.close)
        ):
            raise ValueError("Research bar violates OHLCV invariants")


@dataclass(frozen=True, slots=True)
class ResearchFeatureDescriptor:
    name: str
    version: str
    lookback: int
    description: str
    fingerprint: str

    def __post_init__(self) -> None:
        if (
            not self.name
            or not self.version
            or self.lookback < 1
            or not self.description
            or not self.fingerprint
        ):
            raise ValueError("Research feature descriptor is invalid")


@dataclass(frozen=True, slots=True)
class ResearchValue:
    timestamp: datetime
    value: float | None
    target_timestamp: datetime | None = None

    def __post_init__(self) -> None:
        if self.timestamp.tzinfo is None or self.timestamp.utcoffset() != timedelta(0):
            raise ValueError("Research value timestamp must be UTC")
        if self.value is not None and not math.isfinite(self.value):
            raise ValueError("Research value must be finite or explicitly missing")
        if self.target_timestamp is not None and (
            self.target_timestamp.tzinfo is None
            or self.target_timestamp.utcoffset() != timedelta(0)
            or self.target_timestamp <= self.timestamp
        ):
            raise ValueError("Research target timestamp must be a later UTC timestamp")

    @property
    def missing(self) -> bool:
        return self.value is None


@dataclass(frozen=True, slots=True)
class ResearchSeries:
    descriptor: ResearchFeatureDescriptor
    values: tuple[ResearchValue, ...]
    minimum: float
    maximum: float
    missing: int

    def __post_init__(self) -> None:
        if self.missing != sum(value.missing for value in self.values):
            raise ValueError("Research series missing count is inconsistent")
        if not math.isfinite(self.minimum) or not math.isfinite(self.maximum):
            raise ValueError("Research series bounds must be finite")


@dataclass(frozen=True, slots=True)
class ResearchTargetPreview:
    spec: TargetSpec
    values: tuple[ResearchValue, ...]
    valid_rows: int
    excluded_rows: int

    def __post_init__(self) -> None:
        if self.valid_rows + self.excluded_rows != len(self.values):
            raise ValueError("Research target counts are inconsistent")


@dataclass(frozen=True, slots=True)
class ResearchBin:
    minimum: float
    maximum: float
    count: int

    def __post_init__(self) -> None:
        if (
            not math.isfinite(self.minimum)
            or not math.isfinite(self.maximum)
            or self.minimum >= self.maximum
            or self.count < 0
        ):
            raise ValueError("Research distribution bin is invalid")


@dataclass(frozen=True, slots=True)
class ResearchDistribution:
    name: str
    bins: tuple[ResearchBin, ...]

    def __post_init__(self) -> None:
        if not self.name:
            raise ValueError("Research distribution name is required")


@dataclass(frozen=True, slots=True)
class ResearchPage:
    dataset: Dataset
    next_cursor: str
    total_rows: int

    def __post_init__(self) -> None:
        if self.total_rows < len(self.dataset.rows):
            raise ValueError("Research page total cannot be smaller than its rows")
        if self.next_cursor and not self.next_cursor.isdecimal():
            raise ValueError("Research next cursor is invalid")


@dataclass(frozen=True, slots=True)
class ResearchSnapshot:
    query: ResearchQuery
    frame: PreparedFrame | None
    bars: tuple[ResearchBar, ...]
    features: tuple[ResearchSeries, ...]
    target: ResearchTargetPreview
    distributions: tuple[ResearchDistribution, ...]
    rows: ResearchPage
    replay_seed: int
    replay_clock: datetime
    prepared_at: datetime
    degraded: bool = False

    def __post_init__(self) -> None:
        timestamps = (self.replay_clock, self.prepared_at)
        if any(value.tzinfo is None or value.utcoffset() != timedelta(0) for value in timestamps):
            raise ValueError("Research replay and preparation times must be UTC")
        if tuple(sorted(self.bars, key=lambda value: (value.timestamp, value.symbol))) != self.bars:
            raise ValueError("Research bars must be chronologically ordered")


@dataclass(frozen=True, slots=True)
class ResearchGroupState:
    group_id: str
    generation: int = 0
    selected_feature: str = "ret_5"
    scenario: ResearchScenario = ResearchScenario.NORMAL
    show_ohlcv: bool = True
    show_feature: bool = True
    show_target: bool = False
    state: ResearchLoadState = ResearchLoadState.IDLE
    stale: bool = False
    query: ResearchQuery | None = None
    snapshot: ResearchSnapshot | None = None
    error: str | None = None
    selected_rows: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if not self.group_id or self.generation < 0:
            raise ValueError("Research group identity is invalid")


@dataclass(frozen=True, slots=True)
class ResearchWorkspaceState:
    selected_feature: str = "ret_5"
    target: TargetSpec = TargetSpec()
    show_ohlcv: bool = True
    show_feature: bool = True
    show_target: bool = False
    scenario: ResearchScenario = ResearchScenario.NORMAL
    groups: tuple[ResearchGroupState, ...] = ()

    def group(self, group_id: str) -> ResearchGroupState | None:
        return next((group for group in self.groups if group.group_id == group_id), None)


def default_research_query(
    generation: int = 1,
    *,
    scenario: ResearchScenario = ResearchScenario.NORMAL,
    group_id: str = "link-default-research",
) -> ResearchQuery:
    return ResearchQuery(
        correlation_id=f"research-{group_id}-{generation:020d}",
        group_id=group_id,
        generation=generation,
        dataset_id="dataset-us-equities",
        symbols=("AAPL", "MSFT", "NVDA", "AMD"),
        interval=BarInterval.DAILY,
        time_range=TimeRange(
            datetime(2020, 7, 9, tzinfo=UTC),
            datetime(2026, 7, 9, tzinfo=UTC),
        ),
        selected_features=("ret_5",),
        target=TargetSpec(),
        scenario=scenario,
    )


def zoom_range(current: TimeRange, center: float, factor: float) -> TimeRange:
    if not 0 <= center <= 1 or not 0.05 <= factor <= 20:
        raise ValueError("invalid Research zoom")
    span = current.end - current.start
    new_span = max(timedelta(minutes=1), span * factor)
    anchor = current.start + span * center
    start = anchor - new_span * center
    return TimeRange(start, start + new_span)


def select_range(current: TimeRange, first: float, second: float) -> TimeRange:
    if not 0 <= first <= 1 or not 0 <= second <= 1 or first == second:
        raise ValueError("invalid Research selection")
    minimum, maximum = sorted((first, second))
    span = current.end - current.start
    return TimeRange(current.start + span * minimum, current.start + span * maximum)


def pan_range(current: TimeRange, fraction: float) -> TimeRange:
    if not -10 <= fraction <= 10:
        raise ValueError("invalid Research pan")
    offset = (current.end - current.start) * fraction
    return TimeRange(current.start + offset, current.end + offset)
