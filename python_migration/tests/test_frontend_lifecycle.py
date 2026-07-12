from __future__ import annotations

import threading
from dataclasses import dataclass

import pytest

from corthena.frontend.assets import FrontendAssets
from corthena.frontend.lifecycle import LaunchConfig, launch
from corthena.frontend.native import RaylibFrontendAdapter, UiThreadViolationError


@dataclass
class FakeNative:
    owner_thread_id: int = 42
    initialized: bool = False
    closed: bool = False
    frames: int = 0
    fail_frame: bool = False
    fail_close: bool = False
    fail_initialize: bool = False

    def initialize(self, assets: FrontendAssets, *, hidden: bool) -> None:
        assert hidden
        assert len(assets.sha256) == 3
        self.initialized = True
        if self.fail_initialize:
            raise RuntimeError("initialize failed")

    def should_close(self) -> bool:
        return False

    def render_frame(self) -> None:
        if self.fail_frame:
            raise RuntimeError("frame failed")
        self.frames += 1

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
    adapter = RaylibFrontendAdapter()
    adapter.close()
    adapter.close()


def test_wrong_thread_fails_before_any_native_import() -> None:
    adapter = RaylibFrontendAdapter()
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
