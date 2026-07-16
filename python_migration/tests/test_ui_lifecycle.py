from __future__ import annotations

import threading
from dataclasses import dataclass
from pathlib import Path

import pytest

from corthena.ui.assets import UIAssets
from corthena.ui.lifecycle import LaunchConfig, launch
from corthena.ui.native import (
    CapturedFrame,
    FrameMetrics,
    RaylibUIAdapter,
    UiThreadViolationError,
)
from corthena.ui.phase5b import ChartAction, VisualizationView
from corthena.ui.shell import ShellView
from corthena.ui.state import UIAction, Workspace


@dataclass
class FakeNative:
    owner_thread_id: int = 42
    initialized: bool = False
    closed: bool = False
    frames: int = 0
    fail_frame: bool = False
    fail_close: bool = False
    fail_initialize: bool = False
    fail_capture: bool = False
    metric_sequence: tuple[FrameMetrics, ...] = (FrameMetrics(1280, 720, 1.0, 60),)
    views: list[ShellView] | None = None

    def initialize(self, assets: UIAssets, *, hidden: bool) -> None:
        assert hidden
        assert len(assets.sha256) == 3
        self.initialized = True
        if self.fail_initialize:
            raise RuntimeError("initialize failed")

    def should_close(self) -> bool:
        return False

    def frame_metrics(self) -> FrameMetrics:
        rendered = 0 if self.views is None else len(self.views)
        return self.metric_sequence[min(rendered, len(self.metric_sequence) - 1)]

    def render_frame(self) -> None:
        if self.fail_frame:
            raise RuntimeError("frame failed")
        self.frames += 1

    def render_shell(self, view: ShellView) -> tuple[UIAction, ...]:
        assert view.render_order
        if self.views is not None:
            self.views.append(view)
        self.render_frame()
        return ()

    def render_visualization(self, view: VisualizationView) -> tuple[ChartAction, ...]:
        assert view.frame.layers
        self.render_frame()
        return ()

    def capture_rgba(self) -> CapturedFrame:
        if self.fail_capture:
            raise RuntimeError("capture failed")
        return CapturedFrame(1, 1, b"\x01\x02\x03\xff")

    def close(self) -> None:
        self.closed = True
        if self.fail_close:
            raise RuntimeError("close failed")


def test_bounded_launch_renders_and_cleans_up() -> None:
    native = FakeNative()
    evidence = launch(LaunchConfig(hidden=True, max_frames=2), adapter_factory=lambda: native)
    assert native.initialized
    assert native.closed
    assert evidence.frames_rendered == 2
    assert evidence.owner_thread_id == 42
    assert evidence.max_actions_drained <= 4


def test_named_capture_occurs_after_bounded_final_frame(tmp_path: Path) -> None:
    native = FakeNative()
    capture = tmp_path / "phase3_application_shell.png"
    launch(
        LaunchConfig(hidden=True, max_frames=30, capture_path=capture),
        adapter_factory=lambda: native,
    )
    assert native.frames == 30
    assert capture.read_bytes().startswith(b"\x89PNG")


def test_capture_failure_preserves_primary_error_and_cleans_up(tmp_path: Path) -> None:
    native = FakeNative(fail_capture=True)
    with pytest.raises(RuntimeError, match="capture failed"):
        launch(
            LaunchConfig(hidden=True, max_frames=1, capture_path=tmp_path / "failed.png"),
            adapter_factory=lambda: native,
        )
    assert native.closed


def test_repeated_hidden_launches_return_effect_threads_to_baseline() -> None:
    baseline = tuple(thread.name for thread in threading.enumerate())
    for _ in range(3):
        native = FakeNative()
        launch(
            LaunchConfig(hidden=True, max_frames=2),
            adapter_factory=lambda native=native: native,
        )
        assert native.closed
    assert tuple(thread.name for thread in threading.enumerate()) == baseline


def test_live_frame_metrics_reproject_after_window_maximize() -> None:
    views: list[ShellView] = []
    native = FakeNative(
        metric_sequence=(
            FrameMetrics(1280, 720, 1.0, 60),
            FrameMetrics(2048, 826, 1.0, 60),
        ),
        views=views,
    )
    launch(LaunchConfig(hidden=True, max_frames=2), adapter_factory=lambda: native)
    assert tuple((view.viewport.width, view.viewport.height) for view in views) == (
        (1280, 720),
        (2048, 826),
    )
    assert views[1].content_bounds.width == 2048
    assert tuple(tab.label for tab in views[1].tabs) == tuple(
        workspace.value.title() for workspace in Workspace
    )


def test_frame_failure_keeps_primary_error_and_attempts_cleanup() -> None:
    native = FakeNative(fail_frame=True, fail_close=True)
    with pytest.raises(RuntimeError, match="frame failed") as raised:
        launch(LaunchConfig(hidden=True, max_frames=1), adapter_factory=lambda: native)
    assert native.closed
    assert any("cleanup also failed" in note for note in raised.value.__notes__)


def test_initialization_failure_attempts_cleanup() -> None:
    native = FakeNative(fail_initialize=True)
    with pytest.raises(RuntimeError, match="initialize failed"):
        launch(LaunchConfig(hidden=True, max_frames=1), adapter_factory=lambda: native)
    assert native.initialized
    assert native.closed


def test_close_is_idempotent_before_initialization() -> None:
    adapter = RaylibUIAdapter()
    adapter.close()
    adapter.close()


def test_wrong_thread_fails_before_any_native_import() -> None:
    adapter = RaylibUIAdapter()
    errors: list[BaseException] = []

    def call_from_wrong_thread() -> None:
        try:
            adapter.should_close()
        except BaseException as exc:
            errors.append(exc)

    thread = threading.Thread(target=call_from_wrong_thread)
    thread.start()
    thread.join(timeout=2)
    assert not thread.is_alive()
    assert len(errors) == 1
    assert isinstance(errors[0], UiThreadViolationError)


def test_capture_from_wrong_thread_fails_before_native_capture() -> None:
    adapter = RaylibUIAdapter()
    errors: list[BaseException] = []

    def call_from_wrong_thread() -> None:
        try:
            adapter.capture_rgba()
        except BaseException as exc:
            errors.append(exc)

    thread = threading.Thread(target=call_from_wrong_thread)
    thread.start()
    thread.join(timeout=2)
    assert not thread.is_alive()
    assert len(errors) == 1
    assert isinstance(errors[0], UiThreadViolationError)
