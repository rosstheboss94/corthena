"""Phase 1 workstation startup and deterministic lifecycle."""

from __future__ import annotations

import threading
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

from corthena.frontend.assets import AssetLease
from corthena.frontend.effects import EffectsRuntime, EnqueueState, RuntimeConfig
from corthena.frontend.golden import encode_rgba_png
from corthena.frontend.native import CapturedFrame, NativeFrontend, RaylibFrontendAdapter
from corthena.frontend.shell import project_shell
from corthena.frontend.simulator import DeterministicSimulator, SimulatorConfig
from corthena.frontend.state import AppState, RequestSnapshot, reduce


@dataclass(frozen=True, slots=True)
class LaunchConfig:
    """Validated bounds for one workstation launch."""

    hidden: bool = False
    max_frames: int | None = None
    capture_path: Path | None = None

    def __post_init__(self) -> None:
        if self.max_frames is not None and self.max_frames < 1:
            raise ValueError("max_frames must be at least one")


@dataclass(frozen=True, slots=True)
class LaunchEvidence:
    """Native-free evidence from a completed workstation lifecycle."""

    owner_thread_id: int
    frames_rendered: int
    asset_sha256: tuple[str, str, str]
    final_state: AppState
    max_actions_drained: int


def launch(
    config: LaunchConfig | None = None,
    *,
    adapter_factory: Callable[[], NativeFrontend] = RaylibFrontendAdapter,
    runtime_config: RuntimeConfig | None = None,
) -> LaunchEvidence:
    """Validate assets, run the bounded frame loop, and clean up deterministically."""
    if config is None:
        config = LaunchConfig()
    with AssetLease() as assets:
        adapter = adapter_factory()
        primary_error: BaseException | None = None
        frames = 0
        captured: CapturedFrame | None = None
        state, startup_effects = reduce(AppState(), RequestSnapshot("phase3-startup", 0))
        simulator = DeterministicSimulator(
            SimulatorConfig(42, datetime(2026, 7, 10, 12, tzinfo=UTC))
        )
        effective_runtime_config = RuntimeConfig() if runtime_config is None else runtime_config
        runtime = EffectsRuntime(simulator, effective_runtime_config)
        max_drained = 0
        try:
            adapter.initialize(assets, hidden=config.hidden)
            for effect in startup_effects:
                outcome = runtime.enqueue(effect)
                if outcome.state is EnqueueState.BUSY and outcome.action is not None:
                    state, _ = reduce(state, outcome.action)
            while not adapter.should_close():
                metrics = adapter.frame_metrics()
                drained = runtime.drain()
                max_drained = max(max_drained, len(drained))
                for action in drained:
                    state, effects = reduce(state, action)
                    for effect in effects:
                        runtime.enqueue(effect)
                view = project_shell(
                    state,
                    width=metrics.width,
                    height=metrics.height,
                    dpi_scale=metrics.dpi_scale,
                    fps=metrics.fps,
                )
                for action in adapter.render_shell(view):
                    state, effects = reduce(state, action)
                    for effect in effects:
                        runtime.enqueue(effect)
                frames += 1
                if config.max_frames is not None and frames >= config.max_frames:
                    if config.capture_path is not None:
                        captured = adapter.capture_rgba()
                    break
        except BaseException as exc:
            primary_error = exc
            raise
        finally:
            try:
                runtime.close()
            except BaseException as cleanup_error:
                if primary_error is None:
                    primary_error = cleanup_error
                else:
                    primary_error.add_note(f"runtime cleanup also failed: {cleanup_error!r}")
            try:
                adapter.close()
            except BaseException as cleanup_error:
                if primary_error is None:
                    raise
                primary_error.add_note(f"cleanup also failed: {cleanup_error!r}")
        if primary_error is not None:
            raise primary_error
        if config.capture_path is not None:
            if captured is None:
                raise RuntimeError("capture was requested but no frame was captured")
            _encode_capture(
                config.capture_path, captured, effective_runtime_config.shutdown_timeout_seconds
            )
        return LaunchEvidence(adapter.owner_thread_id, frames, assets.sha256, state, max_drained)


def _encode_capture(path: Path, captured: CapturedFrame, timeout: float) -> None:
    errors: list[BaseException] = []

    def encode() -> None:
        try:
            encode_rgba_png(path, captured.width, captured.height, captured.rgba)
        except BaseException as error:
            errors.append(error)

    worker = threading.Thread(target=encode, name="corthena-png-encoder", daemon=False)
    worker.start()
    worker.join(timeout)
    if worker.is_alive():
        raise RuntimeError("PNG encoder did not terminate within its bounded deadline")
    if errors:
        raise errors[0]


__all__ = ["LaunchConfig", "LaunchEvidence", "launch"]
