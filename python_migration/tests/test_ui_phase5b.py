from __future__ import annotations

import subprocess
import sys
import threading
import time
from pathlib import Path

import pytest

from corthena.ui.golden import compare_pngs
from corthena.ui.phase5b import (
    ChartAction,
    ChartInteractionState,
    ContinuousLayer,
    FeatureImportanceLayer,
    HeatCell,
    HeatmapLayer,
    ImportanceValue,
    InteractionKind,
    OHLCVLayer,
    PageKey,
    PagePayload,
    PageRequest,
    PageResult,
    PaginationWorkers,
    Region,
    RegionKind,
    RegionLayer,
    ServiceRequest,
    ServiceResult,
    SharedRequestService,
    StyleRole,
    Trade,
    TradeLayer,
    TradeSide,
    prepare_frame,
    project_visualization_fixture,
    reduce_chart,
)
from corthena.ui.visualization import Candle, LayerKind, Point, Rect, Sample


def _wait_result(service: SharedRequestService, timeout: float = 1.0) -> ServiceResult:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = service.get_nowait()
        if result is not None:
            return result
        threading.Event().wait(0.005)
    raise AssertionError("timed out waiting for service result")


def test_all_generic_layers_prepare_clipped_immutable_primitives() -> None:
    samples = tuple(Sample(index, float(index), float(index % 3 + 1)) for index in range(6))
    layers = (
        OHLCVLayer("ohlcv", (Candle(0, 0, 2, 4, 1, 3, 10),)),
        ContinuousLayer("line", LayerKind.LINE, samples),
        ContinuousLayer("area", LayerKind.AREA, samples, baseline=0),
        ContinuousLayer("hist", LayerKind.HISTOGRAM, samples, baseline=0),
        ContinuousLayer("scatter", LayerKind.SCATTER, samples),
        ContinuousLayer("equity", LayerKind.EQUITY, samples),
        ContinuousLayer("drawdown", LayerKind.DRAWDOWN, samples, baseline=0),
        HeatmapLayer("heat", (HeatCell(Rect(0, 0, 1, 1), 0.5),), 0, 1),
        FeatureImportanceLayer("importance", (ImportanceValue("alpha", 4, 0),)),
        ContinuousLayer("predictions", LayerKind.PREDICTIONS, samples),
        TradeLayer(
            "trades",
            (
                Trade(1, 2, TradeSide.BUY, 0),
                Trade(2, 2, TradeSide.SELL, 1),
            ),
        ),
        RegionLayer(
            "regions",
            (
                Region(0, 2, RegionKind.TRAIN),
                Region(2, 4, RegionKind.VALIDATION),
                Region(4, 5, RegionKind.TEST),
            ),
        ),
    )
    frame = prepare_frame(1, Rect(0, 0, 5, 5), Rect(10, 20, 510, 320), 100, layers)
    assert tuple(layer.kind for layer in frame.layers) == tuple(
        layer.kind
        if isinstance(layer, ContinuousLayer)
        else LayerKind.OHLCV
        if isinstance(layer, OHLCVLayer)
        else LayerKind.HEATMAP
        if isinstance(layer, HeatmapLayer)
        else LayerKind.FEATURE_IMPORTANCE
        if isinstance(layer, FeatureImportanceLayer)
        else LayerKind.TRADES
        if isinstance(layer, TradeLayer)
        else LayerKind.REGIONS
        for layer in layers
    )
    assert frame.work_count > 0
    for layer in frame.layers:
        for rect in layer.rects:
            assert (
                frame.viewport.min_x
                <= rect.bounds.min_x
                < rect.bounds.max_x
                <= frame.viewport.max_x
            )
            assert (
                frame.viewport.min_y
                <= rect.bounds.min_y
                < rect.bounds.max_y
                <= frame.viewport.max_y
            )


def test_interaction_replay_covers_all_typed_actions() -> None:
    fit = Rect(0, 0, 100, 100)
    initial = ChartInteractionState(1, fit, fit)
    actions = (
        ChartAction(InteractionKind.PAN, delta=Point(2, 3)),
        ChartAction(InteractionKind.ZOOM, anchor=Point(50, 50), factor=2),
        ChartAction(InteractionKind.SELECT, selection=Rect(10, 10, 20, 20)),
        ChartAction(InteractionKind.CROSSHAIR, anchor=Point(15, 15)),
        ChartAction(InteractionKind.TOGGLE_VISIBILITY, series_id="prediction"),
        ChartAction(InteractionKind.KEYBOARD_PAN, delta=Point(-1, 0)),
        ChartAction(InteractionKind.LINK_AXIS, linked_axis=Rect(5, 0, 25, 100)),
        ChartAction(InteractionKind.RESET),
    )

    def replay() -> ChartInteractionState:
        state = initial
        for action in actions:
            state = reduce_chart(state, action)
        return state

    assert replay() == replay()
    assert replay().view == fit
    assert replay().hidden_series == ("prediction",)
    assert replay().generation == 9

    decorated = reduce_chart(initial, ChartAction(InteractionKind.CROSSHAIR, anchor=Point(25, 75)))
    decorated = reduce_chart(
        decorated,
        ChartAction(InteractionKind.SELECT, selection=Rect(10, 20, 30, 40)),
    )
    view = project_visualization_fixture(1280, 720, 100, decorated)
    assert view.crosshair is not None
    assert view.selection is not None
    assert tuple(value.label for value in view.tooltip) == ("x", "y")


def test_shared_request_deduplicates_watchers_and_cancels_independently() -> None:
    started = threading.Event()
    release = threading.Event()
    calls = 0

    def loader(key: str, cancelled: threading.Event) -> bytes:
        nonlocal calls
        calls += 1
        started.set()
        release.wait(0.5)
        assert not cancelled.is_set()
        return key.encode()

    service = SharedRequestService(loader, workers=1, capacity=8)
    assert service.submit(ServiceRequest("left", "same", 1))
    assert started.wait(0.5)
    assert service.submit(ServiceRequest("right", "same", 1))
    assert service.cancel("left")
    release.set()
    result = _wait_result(service)
    assert (result.scope, result.payload, calls) == ("right", b"same", 1)
    assert service.get_nowait() is None
    service.close()
    assert not any(
        thread.name.startswith("corthena-shared-viz") for thread in threading.enumerate()
    )


def test_shared_request_rejects_stale_generations_and_orders_watchers() -> None:
    service = SharedRequestService(lambda key, _: key.encode(), workers=1, capacity=8)
    assert service.submit(ServiceRequest("z", "same", 2))
    assert service.submit(ServiceRequest("a", "same", 1))
    assert service.submit(ServiceRequest("z", "stale", 1))
    results = (_wait_result(service), _wait_result(service))
    assert tuple(result.scope for result in results) == ("a", "z")
    assert all(result.key == "same" for result in results)
    service.close()


def test_pagination_deduplicates_stable_keys_and_rejects_stale_generation() -> None:
    calls = 0

    def loader(key: PageKey, _: threading.Event) -> PagePayload:
        nonlocal calls
        calls += 1
        return PagePayload((f"{key.cursor}-1", f"{key.cursor}-2"), "next")

    pager = PaginationWorkers(loader, workers=1, capacity=8)
    key = PageKey("cursor", "price:ascending", "symbol=AAPL", 2)
    assert pager.submit(PageRequest("left", 1, key))
    assert pager.submit(PageRequest("right", 1, key))
    deadline = time.monotonic() + 1
    results: list[PageResult] = []
    while len(results) < 2 and time.monotonic() < deadline:
        result = pager.get_nowait()
        if result is not None:
            results.append(result)
        threading.Event().wait(0.005)
    assert tuple(result.scope for result in results) == ("left", "right")
    assert all(result.page == PagePayload(("cursor-1", "cursor-2"), "next") for result in results)
    assert calls == 1
    assert not pager.submit(PageRequest("left", 1, key))
    threading.Event().wait(0.05)
    assert pager.get_nowait() is None
    pager.close()


def test_semantic_styles_are_not_raw_native_colors() -> None:
    assert set(StyleRole) == {
        StyleRole.PRIMARY,
        StyleRole.SECONDARY,
        StyleRole.POSITIVE,
        StyleRole.NEGATIVE,
        StyleRole.WARNING,
        StyleRole.MUTED,
        StyleRole.TRAIN,
        StyleRole.VALIDATION,
        StyleRole.TEST,
    }


@pytest.mark.parametrize("width,height", ((1280, 720), (1920, 1080)))
@pytest.mark.parametrize("scale", (100, 150, 200))
def test_python_capture_matches_go_phase5_matrix(
    tmp_path: Path, width: int, height: int, scale: int
) -> None:
    name = f"phase5_generic_{width}x{height}_{scale}.png"
    baseline = (
        Path(__file__).parents[2]
        / "internal"
        / "app"
        / "workstation"
        / "testdata"
        / "phase5-golden"
        / name
    )
    capture = tmp_path / name
    completed = subprocess.run(
        (
            sys.executable,
            "-m",
            "corthena.ui.phase5b_capture",
            "--output",
            str(capture),
            "--width",
            str(width),
            "--height",
            str(height),
            "--scale",
            str(scale),
        ),
        check=False,
        capture_output=True,
        text=True,
        timeout=15,
    )
    comparison = compare_pngs(baseline, capture, channel_tolerance=3, max_different_ratio=0.002)
    assert completed.returncode == 0, completed.stderr
    assert comparison.passed, comparison
