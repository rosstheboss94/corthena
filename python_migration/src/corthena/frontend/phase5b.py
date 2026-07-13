"""Typed Phase 5b render preparation, interactions, and bounded services."""

from __future__ import annotations

import math
import queue
import threading
import time
from collections.abc import Callable
from contextlib import suppress
from dataclasses import dataclass, replace
from enum import StrEnum
from itertools import pairwise

from corthena.frontend.visualization import (
    Candle,
    LayerKind,
    Point,
    Rect,
    Sample,
    Transform,
    aggregate_candles,
    aggregate_continuous,
    checked_float32,
    clip_segment,
)


class StyleRole(StrEnum):
    """Semantic styles resolved only by the native visual adapter."""

    PRIMARY = "primary"
    SECONDARY = "secondary"
    POSITIVE = "positive"
    NEGATIVE = "negative"
    WARNING = "warning"
    MUTED = "muted"
    TRAIN = "train"
    VALIDATION = "validation"
    TEST = "test"


class MarkerShape(StrEnum):
    CIRCLE = "circle"
    TRIANGLE_UP = "triangle_up"
    TRIANGLE_DOWN = "triangle_down"


@dataclass(frozen=True, slots=True)
class RenderRect:
    bounds: Rect
    style: StyleRole
    value: float = 0.0


@dataclass(frozen=True, slots=True)
class RenderSegment:
    start: Point
    end: Point
    style: StyleRole


@dataclass(frozen=True, slots=True)
class RenderMarker:
    center: Point
    size: float
    shape: MarkerShape
    style: StyleRole


@dataclass(frozen=True, slots=True)
class RenderPolygon:
    points: tuple[Point, ...]
    style: StyleRole


@dataclass(frozen=True, slots=True)
class RenderLabel:
    position: Point
    text: str
    style: StyleRole


@dataclass(frozen=True, slots=True)
class PreparedLayer:
    id: str
    kind: LayerKind
    rects: tuple[RenderRect, ...] = ()
    segments: tuple[RenderSegment, ...] = ()
    markers: tuple[RenderMarker, ...] = ()
    polygons: tuple[RenderPolygon, ...] = ()
    labels: tuple[RenderLabel, ...] = ()

    def __post_init__(self) -> None:
        if not self.id:
            raise ValueError("layer ID is required")


@dataclass(frozen=True, slots=True)
class PreparedFrame:
    generation: int
    viewport: Rect
    data: Rect
    layers: tuple[PreparedLayer, ...]
    work_count: int

    def __post_init__(self) -> None:
        if self.generation < 1 or self.work_count < 0:
            raise ValueError("invalid prepared frame")


@dataclass(frozen=True, slots=True)
class ContinuousLayer:
    id: str
    kind: LayerKind
    samples: tuple[Sample, ...]
    style: StyleRole = StyleRole.PRIMARY
    baseline: float | None = None

    def __post_init__(self) -> None:
        if self.kind not in {
            LayerKind.LINE,
            LayerKind.AREA,
            LayerKind.HISTOGRAM,
            LayerKind.SCATTER,
            LayerKind.EQUITY,
            LayerKind.DRAWDOWN,
            LayerKind.PREDICTIONS,
        }:
            raise ValueError("continuous layer kind is invalid")


@dataclass(frozen=True, slots=True)
class OHLCVLayer:
    id: str
    candles: tuple[Candle, ...]


@dataclass(frozen=True, slots=True)
class HeatCell:
    bounds: Rect
    value: float


@dataclass(frozen=True, slots=True)
class HeatmapLayer:
    id: str
    cells: tuple[HeatCell, ...]
    minimum: float
    maximum: float


@dataclass(frozen=True, slots=True)
class ImportanceValue:
    name: str
    value: float
    row: int


@dataclass(frozen=True, slots=True)
class FeatureImportanceLayer:
    id: str
    values: tuple[ImportanceValue, ...]
    style: StyleRole = StyleRole.PRIMARY


class TradeSide(StrEnum):
    BUY = "buy"
    SELL = "sell"


@dataclass(frozen=True, slots=True)
class Trade:
    x: float
    y: float
    side: TradeSide
    source_index: int


@dataclass(frozen=True, slots=True)
class TradeLayer:
    id: str
    trades: tuple[Trade, ...]


class RegionKind(StrEnum):
    TRAIN = "train"
    VALIDATION = "validation"
    TEST = "test"


@dataclass(frozen=True, slots=True)
class Region:
    minimum_x: float
    maximum_x: float
    kind: RegionKind


@dataclass(frozen=True, slots=True)
class RegionLayer:
    id: str
    regions: tuple[Region, ...]


SourceLayer = (
    ContinuousLayer | OHLCVLayer | HeatmapLayer | FeatureImportanceLayer | TradeLayer | RegionLayer
)


def prepare_frame(
    generation: int, data: Rect, viewport: Rect, pixel_width: int, layers: tuple[SourceLayer, ...]
) -> PreparedFrame:
    """Prepare every generic layer into immutable, clipped render-neutral primitives."""
    if generation < 1 or pixel_width < 1:
        raise ValueError("generation and pixel width must be positive")
    transform = Transform(data, viewport)
    prepared: list[PreparedLayer] = []
    work = 0
    for layer in layers:
        result, layer_work = _prepare_layer(transform, pixel_width, layer)
        prepared.append(result)
        work += layer_work
    return PreparedFrame(generation, viewport, data, tuple(prepared), work)


def _prepare_layer(
    transform: Transform, width: int, layer: SourceLayer
) -> tuple[PreparedLayer, int]:
    if isinstance(layer, OHLCVLayer):
        buckets, stats = aggregate_candles(
            layer.candles, transform.data.min_x, transform.data.max_x, width
        )
        max_volume = max((item.volume for item in buckets), default=0.0)
        candle_width = max(1.0, transform.screen.width / width * 0.72)
        rects: list[RenderRect] = []
        segments: list[RenderSegment] = []
        for item in buckets:
            high = transform.forward(Point(item.x, item.high))
            low = transform.forward(Point(item.x, item.low))
            clipped = clip_segment(transform.screen, high, low)
            style = StyleRole.POSITIVE if item.close >= item.open else StyleRole.NEGATIVE
            if clipped is not None:
                segments.append(RenderSegment(*clipped, style))
            opened = transform.forward(Point(item.x, item.open))
            closed = transform.forward(Point(item.x, item.close))
            top, bottom = sorted((opened.y, closed.y))
            rect = _clip_rect(
                transform.screen,
                Rect(
                    opened.x - candle_width / 2,
                    top,
                    opened.x + candle_width / 2,
                    max(bottom, top + 1),
                ),
            )
            if rect is not None:
                rects.append(RenderRect(rect, style))
            if max_volume > 0:
                height = transform.screen.height * 0.18 * item.volume / max_volume
                volume = _clip_rect(
                    transform.screen,
                    Rect(
                        opened.x - candle_width / 2,
                        transform.screen.max_y - height,
                        opened.x + candle_width / 2,
                        transform.screen.max_y,
                    ),
                )
                if volume is not None:
                    rects.append(RenderRect(volume, StyleRole.MUTED))
        return PreparedLayer(
            layer.id, LayerKind.OHLCV, tuple(rects), tuple(segments)
        ), stats.output_values * 3
    if isinstance(layer, ContinuousLayer):
        values, stats = aggregate_continuous(
            layer.samples, transform.data.min_x, transform.data.max_x, width
        )
        points = tuple(transform.forward(Point(item.x, item.y)) for item in values)
        if layer.kind is LayerKind.SCATTER:
            scatter_markers = tuple(
                RenderMarker(point, 3, MarkerShape.CIRCLE, layer.style)
                for point in points
                if transform.screen.contains(point)
            )
            return PreparedLayer(layer.id, layer.kind, markers=scatter_markers), stats.output_values
        if layer.kind is LayerKind.HISTOGRAM:
            baseline = 0.0 if layer.baseline is None else layer.baseline
            bar_width = max(1.0, transform.screen.width / width * 0.8)
            rects = []
            for point, sample in zip(points, values, strict=True):
                base = transform.forward(Point(sample.x, baseline))
                candidate = Rect(
                    point.x - bar_width / 2,
                    min(point.y, base.y),
                    point.x + bar_width / 2,
                    max(point.y, base.y),
                )
                clipped = _clip_rect(transform.screen, candidate)
                if clipped is not None:
                    rects.append(RenderRect(clipped, layer.style))
            return PreparedLayer(layer.id, layer.kind, tuple(rects)), stats.output_values
        style = (
            StyleRole.POSITIVE
            if layer.kind is LayerKind.EQUITY
            else StyleRole.NEGATIVE
            if layer.kind is LayerKind.DRAWDOWN
            else StyleRole.SECONDARY
            if layer.kind is LayerKind.PREDICTIONS
            else layer.style
        )
        segments = []
        for start, end in pairwise(points):
            clipped = clip_segment(transform.screen, start, end)
            if clipped is not None:
                segments.append(RenderSegment(*clipped, style))
        polygons: tuple[RenderPolygon, ...] = ()
        if layer.kind in {LayerKind.AREA, LayerKind.DRAWDOWN} and len(points) >= 2:
            baseline = 0.0 if layer.baseline is None else layer.baseline
            base_y = min(
                max(
                    transform.forward(Point(transform.data.min_x, baseline)).y,
                    transform.screen.min_y,
                ),
                transform.screen.max_y,
            )
            visible = tuple(
                Point(
                    min(max(point.x, transform.screen.min_x), transform.screen.max_x),
                    min(max(point.y, transform.screen.min_y), transform.screen.max_y),
                )
                for point in points
            )
            polygons = (
                RenderPolygon(
                    (Point(visible[0].x, base_y), *visible, Point(visible[-1].x, base_y)), style
                ),
            )
        return PreparedLayer(
            layer.id, layer.kind, segments=tuple(segments), polygons=polygons
        ), stats.output_values + len(segments)
    if isinstance(layer, HeatmapLayer):
        if (
            not math.isfinite(layer.minimum)
            or not math.isfinite(layer.maximum)
            or layer.maximum <= layer.minimum
        ):
            raise ValueError("invalid heatmap range")
        rects = []
        for cell in layer.cells:
            if not math.isfinite(cell.value):
                raise ValueError("heatmap values must be finite")
            top_left = transform.forward(Point(cell.bounds.min_x, cell.bounds.max_y))
            bottom_right = transform.forward(Point(cell.bounds.max_x, cell.bounds.min_y))
            clipped = _clip_rect(
                transform.screen, Rect(top_left.x, top_left.y, bottom_right.x, bottom_right.y)
            )
            if clipped is not None:
                normalized = min(
                    1.0, max(0.0, (cell.value - layer.minimum) / (layer.maximum - layer.minimum))
                )
                rects.append(RenderRect(clipped, StyleRole.PRIMARY, normalized))
        return PreparedLayer(layer.id, LayerKind.HEATMAP, tuple(rects)), len(rects)
    if isinstance(layer, FeatureImportanceLayer):
        rects: list[RenderRect] = []
        labels: list[RenderLabel] = []
        for value in layer.values:
            if not value.name or value.row < 0 or not math.isfinite(value.value):
                raise ValueError("invalid feature importance")
            start = transform.forward(Point(0, value.row))
            end = transform.forward(Point(value.value, value.row + 1))
            clipped = _clip_rect(
                transform.screen,
                Rect(
                    min(start.x, end.x),
                    min(start.y, end.y),
                    max(start.x, end.x),
                    max(start.y, end.y),
                ),
            )
            if clipped is not None:
                rects.append(RenderRect(clipped, layer.style))
                labels.append(
                    RenderLabel(
                        Point(transform.screen.min_x + 2, (start.y + end.y) / 2),
                        value.name,
                        StyleRole.MUTED,
                    )
                )
        return PreparedLayer(
            layer.id, LayerKind.FEATURE_IMPORTANCE, tuple(rects), labels=tuple(labels)
        ), len(rects) + len(labels)
    if isinstance(layer, TradeLayer):
        trade_markers: list[RenderMarker] = []
        previous = -1
        for trade in layer.trades:
            if (
                trade.source_index <= previous
                or not math.isfinite(trade.x)
                or not math.isfinite(trade.y)
            ):
                raise ValueError("invalid trade ordering")
            previous = trade.source_index
            point = transform.forward(Point(trade.x, trade.y))
            if transform.screen.contains(point):
                buy = trade.side is TradeSide.BUY
                trade_markers.append(
                    RenderMarker(
                        point,
                        5,
                        MarkerShape.TRIANGLE_UP if buy else MarkerShape.TRIANGLE_DOWN,
                        StyleRole.POSITIVE if buy else StyleRole.NEGATIVE,
                    )
                )
        return PreparedLayer(layer.id, LayerKind.TRADES, markers=tuple(trade_markers)), len(
            trade_markers
        )
    rects = []
    styles = {
        RegionKind.TRAIN: StyleRole.TRAIN,
        RegionKind.VALIDATION: StyleRole.VALIDATION,
        RegionKind.TEST: StyleRole.TEST,
    }
    for region in layer.regions:
        if (
            not math.isfinite(region.minimum_x)
            or not math.isfinite(region.maximum_x)
            or region.maximum_x <= region.minimum_x
        ):
            raise ValueError("invalid partition region")
        left = transform.forward(Point(region.minimum_x, transform.data.min_y)).x
        right = transform.forward(Point(region.maximum_x, transform.data.max_y)).x
        clipped = _clip_rect(
            transform.screen,
            Rect(
                min(left, right), transform.screen.min_y, max(left, right), transform.screen.max_y
            ),
        )
        if clipped is not None:
            rects.append(RenderRect(clipped, styles[region.kind]))
    return PreparedLayer(layer.id, LayerKind.REGIONS, tuple(rects)), len(rects)


def _clip_rect(bounds: Rect, candidate: Rect) -> Rect | None:
    values = (
        max(bounds.min_x, candidate.min_x),
        max(bounds.min_y, candidate.min_y),
        min(bounds.max_x, candidate.max_x),
        min(bounds.max_y, candidate.max_y),
    )
    if values[2] <= values[0] or values[3] <= values[1]:
        return None
    for value in values:
        checked_float32(value)
    return Rect(*values)


class InteractionKind(StrEnum):
    PAN = "pan"
    ZOOM = "zoom"
    SELECT = "select"
    CROSSHAIR = "crosshair"
    TOGGLE_VISIBILITY = "toggle_visibility"
    RESET = "reset"
    KEYBOARD_PAN = "keyboard_pan"
    LINK_AXIS = "link_axis"


@dataclass(frozen=True, slots=True)
class TooltipValue:
    label: str
    number: float | None = None
    text: str | None = None
    unix_nanoseconds: int | None = None
    missing: bool = False

    def __post_init__(self) -> None:
        if (
            not self.label
            or sum(value is not None for value in (self.number, self.text, self.unix_nanoseconds))
            > 1
        ):
            raise ValueError("tooltip value must have one typed payload")
        if self.number is not None and not math.isfinite(self.number):
            raise ValueError("tooltip number must be finite")


@dataclass(frozen=True, slots=True)
class ChartInteractionState:
    generation: int
    view: Rect
    fit: Rect
    crosshair: Point | None = None
    selection: Rect | None = None
    hidden_series: tuple[str, ...] = ()
    linked_axis: Rect | None = None


@dataclass(frozen=True, slots=True)
class ChartAction:
    kind: InteractionKind
    delta: Point | None = None
    anchor: Point | None = None
    factor: float = 1.0
    selection: Rect | None = None
    series_id: str = ""
    linked_axis: Rect | None = None


def reduce_chart(state: ChartInteractionState, action: ChartAction) -> ChartInteractionState:
    """Pure deterministic interaction reducer suitable for replay."""
    generation = state.generation + 1
    if action.kind in {InteractionKind.PAN, InteractionKind.KEYBOARD_PAN}:
        if action.delta is None:
            raise ValueError("pan requires a delta")
        view = Rect(
            state.view.min_x + action.delta.x,
            state.view.min_y + action.delta.y,
            state.view.max_x + action.delta.x,
            state.view.max_y + action.delta.y,
        )
        return replace(state, generation=generation, view=view, crosshair=None, selection=None)
    if action.kind is InteractionKind.ZOOM:
        if action.anchor is None or not math.isfinite(action.factor) or action.factor <= 0:
            raise ValueError("zoom requires a finite positive factor and anchor")
        anchor = action.anchor
        view = Rect(
            anchor.x + (state.view.min_x - anchor.x) / action.factor,
            anchor.y + (state.view.min_y - anchor.y) / action.factor,
            anchor.x + (state.view.max_x - anchor.x) / action.factor,
            anchor.y + (state.view.max_y - anchor.y) / action.factor,
        )
        return replace(state, generation=generation, view=view, crosshair=None, selection=None)
    if action.kind is InteractionKind.SELECT:
        if action.selection is None:
            raise ValueError("selection is required")
        return replace(state, generation=generation, selection=action.selection)
    if action.kind is InteractionKind.CROSSHAIR:
        return replace(state, generation=generation, crosshair=action.anchor)
    if action.kind is InteractionKind.TOGGLE_VISIBILITY:
        if not action.series_id:
            raise ValueError("series ID is required")
        hidden = set(state.hidden_series)
        hidden.remove(action.series_id) if action.series_id in hidden else hidden.add(
            action.series_id
        )
        return replace(state, generation=generation, hidden_series=tuple(sorted(hidden)))
    if action.kind is InteractionKind.RESET:
        return replace(state, generation=generation, view=state.fit, crosshair=None, selection=None)
    if action.kind is InteractionKind.LINK_AXIS:
        if action.linked_axis is None:
            raise ValueError("linked axis is required")
        return replace(
            state, generation=generation, view=action.linked_axis, linked_axis=action.linked_axis
        )
    raise AssertionError(f"unhandled chart action: {action.kind}")


@dataclass(frozen=True, slots=True)
class ServiceRequest:
    scope: str
    key: str
    generation: int

    def __post_init__(self) -> None:
        if not self.scope or not self.key or self.generation < 1:
            raise ValueError("invalid service request")


@dataclass(frozen=True, slots=True)
class ServiceResult:
    scope: str
    key: str
    generation: int
    payload: bytes | None = None
    error: str | None = None


ServiceLoader = Callable[[str, threading.Event], bytes]


@dataclass(slots=True)
class _Flight:
    key: str
    cancelled: threading.Event
    watchers: dict[str, int]


@dataclass(frozen=True, slots=True)
class _Cancel:
    scope: str


class SharedRequestService:
    """Bounded cross-scope deduplicating workers with independent watchers."""

    _STOP = object()

    def __init__(self, loader: ServiceLoader, *, workers: int, capacity: int) -> None:
        if workers < 1 or capacity < 1:
            raise ValueError("workers and capacity must be positive")
        self._loader = loader
        self._requests: queue.Queue[ServiceRequest | _Cancel | object] = queue.Queue(capacity)
        self._jobs: queue.Queue[_Flight | object] = queue.Queue(workers)
        self._completions: queue.Queue[tuple[_Flight, bytes | None, str | None]] = queue.Queue(
            workers
        )
        self._results: queue.Queue[ServiceResult] = queue.Queue(capacity)
        self._lock = threading.Lock()
        self._closed = False
        self._scopes: dict[str, tuple[int, str]] = {}
        self._flights: dict[str, _Flight] = {}
        self._workers = tuple(
            threading.Thread(target=self._worker, name=f"corthena-shared-viz-{i}")
            for i in range(workers)
        )
        self._dispatcher = threading.Thread(
            target=self._dispatch, name="corthena-shared-viz-dispatch"
        )
        for worker in self._workers:
            worker.start()
        self._dispatcher.start()

    def submit(self, request: ServiceRequest) -> bool:
        if not request.scope or not request.key or request.generation < 1:
            raise ValueError("invalid service request")
        with self._lock:
            if self._closed:
                return False
        try:
            self._requests.put_nowait(request)
            return True
        except queue.Full:
            return False

    def cancel(self, scope: str) -> bool:
        with self._lock:
            if self._closed:
                return False
        try:
            self._requests.put_nowait(_Cancel(scope))
            return True
        except queue.Full:
            return False

    def get_nowait(self) -> ServiceResult | None:
        try:
            return self._results.get_nowait()
        except queue.Empty:
            return None

    def close(self, timeout: float = 2.0) -> None:
        if timeout <= 0:
            raise ValueError("shutdown timeout must be positive")
        deadline = time.monotonic() + timeout
        with self._lock:
            if self._closed:
                return
            self._closed = True
        self._requests.put(self._STOP, timeout=max(0.0, deadline - time.monotonic()))
        self._dispatcher.join(max(0.0, deadline - time.monotonic()))
        if self._dispatcher.is_alive() or any(worker.is_alive() for worker in self._workers):
            raise TimeoutError("shared visualization service shutdown timed out")

    def _detach(self, scope: str) -> None:
        current = self._scopes.pop(scope, None)
        if current is None:
            return
        flight = self._flights.get(current[1])
        if flight is not None:
            flight.watchers.pop(scope, None)
            if not flight.watchers:
                flight.cancelled.set()
                self._flights.pop(flight.key, None)

    def _dispatch(self) -> None:
        stopping = False
        while not stopping:
            try:
                completion = self._completions.get_nowait()
                self._complete(*completion)
                continue
            except queue.Empty:
                pass
            try:
                item = self._requests.get(timeout=0.01)
            except queue.Empty:
                continue
            if item is self._STOP:
                stopping = True
                continue
            if isinstance(item, _Cancel):
                self._detach(item.scope)
                continue
            if not isinstance(item, ServiceRequest):
                continue
            current = self._scopes.get(item.scope)
            if current is not None and item.generation <= current[0]:
                continue
            self._detach(item.scope)
            self._scopes[item.scope] = (item.generation, item.key)
            flight = self._flights.get(item.key)
            if flight is not None:
                flight.watchers[item.scope] = item.generation
                continue
            flight = _Flight(item.key, threading.Event(), {item.scope: item.generation})
            try:
                self._jobs.put_nowait(flight)
                self._flights[item.key] = flight
            except queue.Full:
                self._scopes.pop(item.scope, None)
                self._publish(
                    ServiceResult(item.scope, item.key, item.generation, error="service busy")
                )
        for flight in self._flights.values():
            flight.cancelled.set()
        for _ in self._workers:
            self._jobs.put(self._STOP)
        for worker in self._workers:
            worker.join()

    def _worker(self) -> None:
        while True:
            item = self._jobs.get()
            if item is self._STOP:
                return
            assert isinstance(item, _Flight)
            try:
                payload = self._loader(item.key, item.cancelled)
                completion = (item, bytes(payload), None)
            except Exception as error:
                completion = (item, None, f"{type(error).__name__}: {error}")
            self._completions.put(completion)

    def _complete(self, flight: _Flight, payload: bytes | None, error: str | None) -> None:
        if self._flights.get(flight.key) is not flight:
            return
        self._flights.pop(flight.key, None)
        for scope, generation in sorted(flight.watchers.items()):
            if self._scopes.get(scope) == (generation, flight.key):
                self._publish(ServiceResult(scope, flight.key, generation, payload, error))

    def _publish(self, result: ServiceResult) -> None:
        with suppress(queue.Full):
            self._results.put_nowait(result)


@dataclass(frozen=True, slots=True)
class PageKey:
    cursor: str
    sort_key: str
    filter_key: str
    page_size: int

    def __post_init__(self) -> None:
        if self.page_size < 1 or self.page_size > 10_000:
            raise ValueError("page size is outside bounds")
        if any("\0" in value for value in (self.cursor, self.sort_key, self.filter_key)):
            raise ValueError("page key strings cannot contain NUL")


@dataclass(frozen=True, slots=True)
class PageRequest:
    scope: str
    generation: int
    key: PageKey

    def __post_init__(self) -> None:
        if not self.scope or self.generation < 1:
            raise ValueError("invalid page request")


@dataclass(frozen=True, slots=True)
class PagePayload:
    row_ids: tuple[str, ...]
    next_cursor: str | None

    def __post_init__(self) -> None:
        if len(self.row_ids) != len(frozenset(self.row_ids)) or any(
            not row_id or "\n" in row_id or "\0" in row_id for row_id in self.row_ids
        ):
            raise ValueError("page row IDs must be unique and encoding-safe")
        if self.next_cursor is not None and "\0" in self.next_cursor:
            raise ValueError("next cursor cannot contain NUL")


@dataclass(frozen=True, slots=True)
class PageResult:
    scope: str
    generation: int
    key: PageKey
    page: PagePayload | None = None
    error: str | None = None


PageLoader = Callable[[PageKey, threading.Event], PagePayload]


class PaginationWorkers:
    """Typed pagination parity built on shared-flight watcher semantics."""

    def __init__(self, loader: PageLoader, *, workers: int, capacity: int) -> None:
        self._keys: dict[str, PageKey] = {}
        self._references: dict[str, int] = {}
        self._scopes: dict[str, tuple[int, str]] = {}
        self._latest: dict[str, int] = {}
        self._capacity = capacity
        self._lock = threading.Lock()

        def encoded(key: str, cancelled: threading.Event) -> bytes:
            with self._lock:
                page_key = self._keys[key]
            page = loader(page_key, cancelled)
            return ("\n".join(page.row_ids) + "\0" + (page.next_cursor or "")).encode()

        self._service = SharedRequestService(encoded, workers=workers, capacity=capacity)

    @staticmethod
    def _identity(key: PageKey) -> str:
        return (
            f"{len(key.cursor)}:{key.cursor}|{len(key.sort_key)}:{key.sort_key}|"
            f"{len(key.filter_key)}:{key.filter_key}|{key.page_size}"
        )

    def submit(self, request: PageRequest) -> bool:
        identity = self._identity(request.key)
        with self._lock:
            current = self._scopes.get(request.scope)
            if request.generation <= self._latest.get(request.scope, 0):
                return False
            if request.scope not in self._latest and len(self._latest) >= self._capacity:
                return False
            if identity not in self._keys and len(self._keys) >= self._capacity:
                return False
            self._keys[identity] = request.key
            self._references[identity] = self._references.get(identity, 0) + 1
            self._scopes[request.scope] = (request.generation, identity)
            self._latest[request.scope] = request.generation
            accepted = self._service.submit(
                ServiceRequest(request.scope, identity, request.generation)
            )
            if not accepted:
                self._release(identity)
                if current is None:
                    self._scopes.pop(request.scope, None)
                else:
                    self._scopes[request.scope] = current
                if current is None:
                    self._latest.pop(request.scope, None)
                else:
                    self._latest[request.scope] = current[0]
                return False
            if current is not None:
                self._release(current[1])
        return True

    def cancel(self, scope: str) -> bool:
        if not self._service.cancel(scope):
            return False
        with self._lock:
            previous = self._scopes.pop(scope, None)
            if previous is not None:
                self._release(previous[1])
        return True

    def get_nowait(self) -> PageResult | None:
        result = self._service.get_nowait()
        if result is None:
            return None
        with self._lock:
            key = self._keys[result.key]
            current = self._scopes.get(result.scope)
            if current == (result.generation, result.key):
                self._scopes.pop(result.scope, None)
                self._release(result.key)
        if result.payload is None:
            return PageResult(result.scope, result.generation, key, error=result.error)
        rows, cursor = result.payload.decode().split("\0", 1)
        return PageResult(
            result.scope,
            result.generation,
            key,
            PagePayload(tuple(filter(None, rows.split("\n"))), cursor or None),
            result.error,
        )

    def close(self, timeout: float = 2.0) -> None:
        self._service.close(timeout)
        with self._lock:
            self._scopes.clear()
            self._latest.clear()
            self._references.clear()
            self._keys.clear()

    def _release(self, identity: str) -> None:
        remaining = self._references[identity] - 1
        if remaining == 0:
            self._references.pop(identity)
            self._keys.pop(identity)
        else:
            self._references[identity] = remaining


@dataclass(frozen=True, slots=True)
class VisualizationView:
    """Immutable generic fixture with shared paint, clip, hit, and capture geometry."""

    viewport: Rect
    chart_bounds: Rect
    table_bounds: Rect
    frame: PreparedFrame
    headers: tuple[str, ...]
    rows: tuple[tuple[str, ...], ...]
    scale: float
    crosshair: Point | None = None
    selection: Rect | None = None
    tooltip: tuple[TooltipValue, ...] = ()


def project_visualization_fixture(
    width: int,
    height: int,
    scale_percent: int,
    interaction: ChartInteractionState | None = None,
) -> VisualizationView:
    """Build the deterministic Phase 5b golden fixture without domain state."""
    if width < 640 or height < 360 or scale_percent not in (100, 150, 200):
        raise ValueError("unsupported visualization fixture metrics")
    scale = scale_percent / 100
    inset, header, gap = 16 * scale, 44 * scale, 12 * scale
    table_height = min(190 * scale, height * 0.32)
    chart = Rect(inset, header + inset, width - inset, height - inset - table_height - gap)
    table = Rect(inset, chart.max_y + gap, width - inset, height - inset)
    data = Rect(0, 0, 100, 100) if interaction is None else interaction.view
    samples = tuple(
        Sample(index, float(index), 48 + 18 * math.sin(index / 8) + index * 0.16)
        for index in range(101)
    )
    predictions = tuple(
        Sample(index, float(index), 50 + 15 * math.sin((index + 3) / 8) + index * 0.14)
        for index in range(101)
    )
    candles = tuple(
        Candle(
            index,
            float(index * 5 + 2),
            42 + index * 2,
            50 + index * 2,
            38 + index * 2,
            47 + index * 2,
            float(100 + index * 20),
        )
        for index in range(18)
    )
    layers: tuple[SourceLayer, ...] = (
        RegionLayer(
            "partitions",
            (
                Region(0, 60, RegionKind.TRAIN),
                Region(60, 80, RegionKind.VALIDATION),
                Region(80, 100, RegionKind.TEST),
            ),
        ),
        OHLCVLayer("ohlcv", candles),
        ContinuousLayer("line", LayerKind.LINE, samples, StyleRole.PRIMARY),
        ContinuousLayer("area", LayerKind.AREA, samples[::4], baseline=35),
        ContinuousLayer(
            "histogram", LayerKind.HISTOGRAM, samples[::5], StyleRole.WARNING, baseline=35
        ),
        ContinuousLayer("scatter", LayerKind.SCATTER, samples[::8], StyleRole.SECONDARY),
        ContinuousLayer("equity", LayerKind.EQUITY, samples),
        ContinuousLayer("drawdown", LayerKind.DRAWDOWN, samples[::3], baseline=0),
        HeatmapLayer(
            "heatmap",
            tuple(
                HeatCell(
                    Rect(82 + column * 4, 5 + row * 5, 86 + column * 4, 10 + row * 5),
                    (row + column) / 7,
                )
                for row in range(4)
                for column in range(4)
            ),
            0,
            1,
        ),
        FeatureImportanceLayer(
            "importance",
            (
                ImportanceValue("momentum", 18, 82),
                ImportanceValue("volatility", 14, 87),
                ImportanceValue("volume", 10, 92),
            ),
            StyleRole.SECONDARY,
        ),
        ContinuousLayer("predictions", LayerKind.PREDICTIONS, predictions, StyleRole.SECONDARY),
        TradeLayer(
            "trades",
            (
                Trade(25, 58, TradeSide.BUY, 0),
                Trade(55, 67, TradeSide.SELL, 1),
                Trade(75, 72, TradeSide.BUY, 2),
            ),
        ),
    )
    hidden: frozenset[str] = (
        frozenset() if interaction is None else frozenset(interaction.hidden_series)
    )
    visible_layers = tuple(layer for layer in layers if layer.id not in hidden)
    generation = 1 if interaction is None else interaction.generation
    frame = prepare_frame(generation, data, chart, max(1, round(chart.width)), visible_layers)
    transform = Transform(data, chart)
    crosshair = (
        None
        if interaction is None or interaction.crosshair is None
        else transform.forward(interaction.crosshair)
    )
    selection = None
    if interaction is not None and interaction.selection is not None:
        first = transform.forward(Point(interaction.selection.min_x, interaction.selection.min_y))
        second = transform.forward(Point(interaction.selection.max_x, interaction.selection.max_y))
        selection = Rect(
            min(first.x, second.x),
            min(first.y, second.y),
            max(first.x, second.x),
            max(first.y, second.y),
        )
    tooltip = (
        ()
        if interaction is None or interaction.crosshair is None
        else (
            TooltipValue("x", number=interaction.crosshair.x),
            TooltipValue("y", number=interaction.crosshair.y),
        )
    )
    return VisualizationView(
        Rect(0, 0, width, height),
        chart,
        table,
        frame,
        ("Symbol", "Last", "Prediction", "Signal", "Partition"),
        (
            ("AAPL", "228.14", "231.02", "BUY", "test"),
            ("MSFT", "512.44", "508.91", "HOLD", "validation"),
            ("NVDA", "189.27", "193.85", "BUY", "test"),
            ("AMD", "171.33", "168.22", "SELL", "train"),
        ),
        scale,
        crosshair,
        selection,
        tooltip,
    )


__all__ = [
    "ChartAction",
    "ChartInteractionState",
    "ContinuousLayer",
    "FeatureImportanceLayer",
    "HeatCell",
    "HeatmapLayer",
    "ImportanceValue",
    "InteractionKind",
    "MarkerShape",
    "OHLCVLayer",
    "PageKey",
    "PagePayload",
    "PageRequest",
    "PageResult",
    "PaginationWorkers",
    "PreparedFrame",
    "PreparedLayer",
    "Region",
    "RegionKind",
    "RegionLayer",
    "RenderLabel",
    "RenderMarker",
    "RenderPolygon",
    "RenderRect",
    "RenderSegment",
    "ServiceRequest",
    "ServiceResult",
    "SharedRequestService",
    "StyleRole",
    "TooltipValue",
    "Trade",
    "TradeLayer",
    "TradeSide",
    "VisualizationView",
    "prepare_frame",
    "project_visualization_fixture",
    "reduce_chart",
]
