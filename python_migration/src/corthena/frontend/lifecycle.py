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
from corthena.frontend.native import (
    CapturedFrame,
    NativeFrontend,
    RaylibFrontendAdapter,
    WindowSize,
)
from corthena.frontend.persistence import (
    DocumentStore,
    LayoutCollection,
    NamedLayout,
    PersistenceWorker,
    Preferences,
)
from corthena.frontend.phase5b import (
    ChartInteractionState,
    project_visualization_fixture,
    reduce_chart,
)
from corthena.frontend.phase5b import (
    Rect as VisualizationRect,
)
from corthena.frontend.shell import project_shell
from corthena.frontend.simulator import DeterministicSimulator, SimulatorConfig
from corthena.frontend.state import (
    ApplyWorkspaceLayout,
    AppState,
    RequestSnapshot,
    SetUIScale,
    reduce,
)


@dataclass(frozen=True, slots=True)
class LaunchConfig:
    """Validated bounds for one workstation launch."""

    hidden: bool = False
    max_frames: int | None = None
    capture_path: Path | None = None
    width: int = 1280
    height: int = 720
    ui_scale_percent: int = 100
    visualization_fixture: bool = False
    persistence_directory: Path | None = None

    def __post_init__(self) -> None:
        if self.max_frames is not None and self.max_frames < 1:
            raise ValueError("max_frames must be at least one")
        if self.width < 640 or self.height < 360:
            raise ValueError("launch viewport must be at least 640x360")
        if self.ui_scale_percent not in (100, 150, 200):
            raise ValueError("unsupported launch scale")


@dataclass(frozen=True, slots=True)
class LaunchEvidence:
    """Native-free evidence from a completed workstation lifecycle."""

    owner_thread_id: int
    frames_rendered: int
    asset_sha256: tuple[str, str, str]
    final_state: AppState
    max_actions_drained: int
    chart_state: ChartInteractionState


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
        adapter = (
            RaylibFrontendAdapter(WindowSize(config.width, config.height))
            if adapter_factory is RaylibFrontendAdapter
            else adapter_factory()
        )
        primary_error: BaseException | None = None
        frames = 0
        captured: CapturedFrame | None = None
        state, startup_effects = reduce(AppState(), RequestSnapshot("phase3-startup", 0))
        chart_fit = VisualizationRect(0, 0, 100, 100)
        chart_state = ChartInteractionState(1, chart_fit, chart_fit)
        simulator = DeterministicSimulator(
            SimulatorConfig(42, datetime(2026, 7, 10, 12, tzinfo=UTC))
        )
        effective_runtime_config = RuntimeConfig() if runtime_config is None else runtime_config
        runtime = EffectsRuntime(simulator, effective_runtime_config)
        persistence: PersistenceWorker | None = None
        if config.persistence_directory is not None:
            store = DocumentStore(config.persistence_directory)
            loaded: list[Preferences | LayoutCollection] = []

            def load_documents() -> None:
                loaded.extend((store.load_preferences(), store.load_layouts()))

            loader = threading.Thread(target=load_documents, name="corthena-persistence-load")
            loader.start()
            loader.join(effective_runtime_config.shutdown_timeout_seconds)
            if loader.is_alive():
                raise RuntimeError("persistence load did not terminate")
            preferences, layouts = loaded
            if not isinstance(preferences, Preferences) or not isinstance(
                layouts, LayoutCollection
            ):
                raise RuntimeError("persistence load returned invalid document types")
            state, _ = reduce(state, SetUIScale(preferences.ui_scale_percent))
            selected = next((item for item in layouts.layouts if item.name == "default"), None)
            if selected is not None:
                current = dict(state.workspace_layouts)[state.workspace]
                state, _ = reduce(
                    state,
                    ApplyWorkspaceLayout(
                        state.workspace, selected.layout, current.revision, selected.name
                    ),
                )
            persistence = PersistenceWorker(store)
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
                if config.visualization_fixture:
                    for chart_action in adapter.render_visualization(
                        project_visualization_fixture(
                            metrics.width,
                            metrics.height,
                            config.ui_scale_percent,
                            chart_state,
                        )
                    ):
                        chart_state = reduce_chart(chart_state, chart_action)
                else:
                    for action in adapter.render_shell(view):
                        state, effects = reduce(state, action)
                        for effect in effects:
                            runtime.enqueue(effect)
                if persistence is not None:
                    persistence.submit(
                        Preferences(state.preferences_revision, state.ui_scale_percent)
                    )
                    current_layout = dict(state.workspace_layouts)[state.workspace]
                    persistence.submit(
                        LayoutCollection(
                            current_layout.revision,
                            (NamedLayout(state.active_layout_name, current_layout),),
                        )
                    )
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
            if persistence is not None:
                try:
                    persistence.close()
                except BaseException as cleanup_error:
                    if primary_error is None:
                        primary_error = cleanup_error
                    else:
                        primary_error.add_note(
                            f"persistence cleanup also failed: {cleanup_error!r}"
                        )
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
        return LaunchEvidence(
            adapter.owner_thread_id,
            frames,
            assets.sha256,
            state,
            max_drained,
            chart_state,
        )


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
