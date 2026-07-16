from __future__ import annotations

import math
import threading
import time

import pytest
from hypothesis import given
from hypothesis import strategies as st

from corthena.ui.table import (
    Cell,
    CellKind,
    Column,
    Dataset,
    NullOrder,
    Row,
    Selection,
    SortDirection,
    SortSpec,
    TableError,
    compute_window,
    copy_selection,
    sort_dataset,
)
from corthena.ui.visualization import (
    Candle,
    Point,
    Rect,
    Sample,
    Transform,
    VisualizationError,
    aggregate_candles,
    aggregate_continuous,
    checked_float32,
    clip_segment,
    ticks,
)
from corthena.ui.visualization_runtime import (
    ByteLRU,
    PrepareRequest,
    RenderFrame,
    VisualizationWorkers,
)


def test_transform_inverse_clipping_ticks_and_rejection() -> None:
    transform = Transform(Rect(0, 0, 10, 20), Rect(100, 200, 300, 600))
    assert transform.forward(Point(5, 5)) == Point(200, 500)
    assert transform.inverse(Point(200, 500)) == Point(5, 5)
    assert clip_segment(Rect(0, 0, 10, 10), Point(-5, 5), Point(15, 5)) == (
        Point(0, 5),
        Point(10, 5),
    )
    assert ticks(-0.2, 1.2, 4) == (0.0, 0.5, 1.0)
    with pytest.raises(VisualizationError):
        Rect(0, 0, 0, 1)
    with pytest.raises(VisualizationError):
        checked_float32(math.inf)


@given(
    st.floats(-1e6, 1e6, allow_nan=False, allow_infinity=False),
    st.floats(-1e6, 1e6, allow_nan=False, allow_infinity=False),
)
def test_transform_round_trip(x: float, y: float) -> None:
    transform = Transform(Rect(-1e6 - 1, -1e6 - 1, 1e6 + 1, 1e6 + 1), Rect(0, 0, 1920, 1080))
    restored = transform.inverse(transform.forward(Point(x, y)))
    assert restored.x == pytest.approx(x, abs=1e-8)
    assert restored.y == pytest.approx(y, abs=1e-8)


def test_lod_preserves_extrema_ohlcv_and_bounds_output_work() -> None:
    samples = tuple(
        Sample(index, index / 10, value) for index, value in enumerate((5, 1, 9, 4, 2, 8))
    )
    output, stats = aggregate_continuous(samples, 0, 1, 2)
    assert tuple(item.source_index for item in output) == (0, 1, 2, 4, 5)
    assert stats.output_values <= stats.buckets * 4 <= 8
    candles = (Candle(0, 0, 10, 13, 8, 12, 2), Candle(1, 0.2, 12, 14, 9, 11, 3))
    buckets, candle_stats = aggregate_candles(candles, 0, 1, 1)
    assert (
        buckets[0].open,
        buckets[0].high,
        buckets[0].low,
        buckets[0].close,
        buckets[0].volume,
    ) == (10, 14, 8, 11, 5)
    assert candle_stats.output_values == 1


def _dataset() -> Dataset:
    columns = (
        Column("id", "ID", CellKind.STRING, 80, pinned=True),
        Column("value", "Value", CellKind.FLOAT, 100),
        Column("note", "Note", CellKind.STRING, 120),
    )
    rows = (
        Row(
            "a",
            0,
            (Cell(CellKind.STRING, "a"), Cell(CellKind.FLOAT, None), Cell(CellKind.STRING, "x")),
        ),
        Row(
            "b",
            1,
            (Cell(CellKind.STRING, "b"), Cell(CellKind.FLOAT, 2.0), Cell(CellKind.STRING, "y")),
        ),
        Row(
            "c",
            2,
            (Cell(CellKind.STRING, "c"), Cell(CellKind.FLOAT, 1.0), Cell(CellKind.STRING, "z")),
        ),
    )
    return Dataset(columns, rows)


def test_table_virtualization_sort_selection_and_copy_are_bounded() -> None:
    dataset = _dataset()
    ordered = sort_dataset(dataset, (SortSpec("value", SortDirection.ASCENDING, NullOrder.LAST),))
    assert tuple(row.id for row in ordered.rows) == ("c", "b", "a")
    selection = Selection().select(dataset.rows, "b").select(dataset.rows, "c", extend=True)
    assert selection.ids == ("b", "c")
    assert (
        copy_selection(dataset, selection, ("id", "value"), include_header=True, max_bytes=100)
        == "ID\tValue\nb\t2.0\nc\t1.0\n"
    )
    with pytest.raises(TableError):
        copy_selection(dataset, selection, ("id",), include_header=False, max_bytes=1)
    window = compute_window(
        dataset.columns, 10_000_000, 220, 100, 0, 5_000_000, 20, row_overscan=2, column_overscan=1
    )
    assert window.row_end - window.row_start <= 9
    assert window.cell_work <= 27


def test_byte_lru_exact_accounting_replacement_eviction_and_oversize() -> None:
    first, second = RenderFrame("a", 1, b"123"), RenderFrame("b", 1, b"456")
    cache = ByteLRU(first.byte_size + second.byte_size)
    assert cache.put(first) and cache.put(second)
    assert cache.used_bytes == first.byte_size + second.byte_size
    replacement = RenderFrame("a", 2, b"x")
    assert cache.put(replacement)
    assert cache.used_bytes == replacement.byte_size + second.byte_size
    assert not cache.put(RenderFrame("huge", 1, b"x" * 1000))
    assert cache.get("a") == replacement


def test_workers_reject_stale_cancel_and_shutdown_without_leaks() -> None:
    started = threading.Event()
    release = threading.Event()

    def prepare(request: PrepareRequest, cancelled: threading.Event) -> bytes:
        started.set()
        release.wait(0.5)
        return request.key.encode() if not cancelled.is_set() else b"cancelled"

    baseline = {thread.ident for thread in threading.enumerate()}
    workers = VisualizationWorkers(prepare, workers=1, capacity=2, cache_bytes=100)
    assert workers.submit(PrepareRequest("chart", "old", 1))
    assert started.wait(0.5)
    assert workers.submit(PrepareRequest("chart", "new", 2))
    assert not workers.submit(PrepareRequest("chart", "stale", 1))
    release.set()
    deadline = time.monotonic() + 1
    result = None
    while result is None and time.monotonic() < deadline:
        result = workers.get_nowait()
        time.sleep(0.005)
    assert result is not None and result.generation == 2 and result.frame is not None
    workers.close()
    assert all(
        thread.ident in baseline
        for thread in threading.enumerate()
        if thread.name.startswith("corthena-viz-")
    )
