"""Phase 1 workstation startup and deterministic lifecycle."""

from __future__ import annotations

import threading
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

from corthena.ui.assets import AssetLease
from corthena.ui.data_experiments.actions import RequestPhase7
from corthena.ui.data_experiments.models import (
    Phase7Request,
    Phase7Scenario,
    Phase7Workspace,
)
from corthena.ui.effects import EffectsRuntime, EnqueueState, RuntimeConfig
from corthena.ui.golden import encode_rgba_png
from corthena.ui.jobs_results.actions import RequestComparison, RequestPhase8
from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    Phase8LoadState,
    Phase8Request,
    Phase8Scenario,
    Phase8Workspace,
)
from corthena.ui.models_inference.actions import RequestInference, RequestPhase9
from corthena.ui.models_inference.models import (
    InferenceMode,
    InferenceQuery,
    Phase9LoadState,
    Phase9Request,
    Phase9Scenario,
    Phase9Workspace,
)
from corthena.ui.native.models import CapturedFrame, WindowSize
from corthena.ui.native.protocol import NativeUIProtocol
from corthena.ui.native.raylib import RaylibUIAdapter
from corthena.ui.persistence import (
    DocumentStore,
    LayoutCollection,
    NamedLayout,
    PersistenceWorker,
    Preferences,
)
from corthena.ui.phase5b import (
    ChartInteractionState,
    project_visualization_fixture,
    reduce_chart,
)
from corthena.ui.phase5b import (
    Rect as VisualizationRect,
)
from corthena.ui.research.actions import RequestResearch, SetResearchRange
from corthena.ui.research.models import (
    ResearchLoadState,
    ResearchScenario,
    default_research_query,
    select_range,
)
from corthena.ui.shell import project_shell
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig
from corthena.ui.state import (
    ApplyWorkspaceLayout,
    AppState,
    RequestSnapshot,
    SelectWorkspace,
    SetUIScale,
    Workspace,
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
    research_scenario: ResearchScenario | None = None
    research_linked_selection: bool = False
    phase7_workspace: Phase7Workspace | None = None
    phase7_scenario: Phase7Scenario = Phase7Scenario.NORMAL
    phase8_workspace: Phase8Workspace | None = None
    phase8_scenario: Phase8Scenario = Phase8Scenario.JOBS_SUCCESS
    phase9_workspace: Phase9Workspace | None = None
    phase9_scenario: Phase9Scenario = Phase9Scenario.MODELS_NORMAL
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
    adapter_factory: Callable[[], NativeUIProtocol] = RaylibUIAdapter,
    runtime_config: RuntimeConfig | None = None,
) -> LaunchEvidence:
    """Validate assets, run the bounded frame loop, and clean up deterministically."""
    if config is None:
        config = LaunchConfig()
    with AssetLease() as assets:
        adapter = (
            RaylibUIAdapter(WindowSize(config.width, config.height))
            if adapter_factory is RaylibUIAdapter
            else adapter_factory()
        )
        primary_error: BaseException | None = None
        frames = 0
        captured: CapturedFrame | None = None
        state, startup_effects = reduce(AppState(), RequestSnapshot("phase3-startup", 0))
        state, _ = reduce(state, SetUIScale(config.ui_scale_percent))
        if config.research_scenario is not None:
            state, _ = reduce(state, SelectWorkspace(Workspace.RESEARCH))
            state, research_effects = reduce(
                state,
                RequestResearch(default_research_query(scenario=config.research_scenario)),
            )
            startup_effects = (*startup_effects, *research_effects)
        if config.phase7_workspace is not None:
            shell_workspace = (
                Workspace.DATA
                if config.phase7_workspace is Phase7Workspace.DATA
                else Workspace.EXPERIMENTS
            )
            state, _ = reduce(state, SelectWorkspace(shell_workspace))
            state, phase7_effects = reduce(
                state,
                RequestPhase7(
                    Phase7Request(
                        f"phase7-{config.phase7_workspace.value}-00000000000000000001",
                        1,
                        config.phase7_workspace,
                        config.phase7_scenario,
                    )
                ),
            )
            startup_effects = (*startup_effects, *phase7_effects)
        if config.phase8_workspace is not None:
            shell_workspace = (
                Workspace.JOBS
                if config.phase8_workspace is Phase8Workspace.JOBS
                else Workspace.RESULTS
            )
            state, _ = reduce(state, SelectWorkspace(shell_workspace))
            state, phase8_effects = reduce(
                state,
                RequestPhase8(
                    Phase8Request(
                        f"phase8-{config.phase8_workspace.value}-00000000000000000001",
                        1,
                        config.phase8_workspace,
                        config.phase8_scenario,
                    )
                ),
            )
            startup_effects = (*startup_effects, *phase8_effects)
        if config.phase9_workspace is not None:
            shell_workspace = (
                Workspace.MODELS
                if config.phase9_workspace is Phase9Workspace.MODELS
                else Workspace.INFERENCE
            )
            state, _ = reduce(state, SelectWorkspace(shell_workspace))
            state, phase9_effects = reduce(
                state,
                RequestPhase9(
                    Phase9Request(
                        f"phase9-{config.phase9_workspace.value}-00000000000000000001",
                        1,
                        config.phase9_workspace,
                        config.phase9_scenario,
                    )
                ),
            )
            startup_effects = (*startup_effects, *phase9_effects)
        chart_fit = VisualizationRect(0, 0, 100, 100)
        chart_state = ChartInteractionState(1, chart_fit, chart_fit)
        simulator = DeterministicSimulator(
            SimulatorConfig(
                209
                if config.phase9_workspace is not None
                else 208
                if config.phase8_workspace is not None
                else 107
                if config.phase7_workspace is not None
                else 42,
                datetime(2026, 7, 10, 12, tzinfo=UTC),
            )
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
        linked_selection_pending = config.research_linked_selection
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
                results = state.jobs_results.results
                if (
                    state.workspace is Workspace.RESULTS
                    and results.state
                    in {
                        Phase8LoadState.READY,
                        Phase8LoadState.DEGRADED,
                        Phase8LoadState.RECOVERED,
                    }
                    and results.selected_run_ids
                    and results.comparison is None
                    and results.active_comparison is None
                ):
                    comparison_generation = results.comparison_generation + 1
                    state, comparison_effects = reduce(
                        state,
                        RequestComparison(
                            ComparisonQuery(
                                f"phase8-comparison-{comparison_generation:020d}",
                                comparison_generation,
                                results.selected_run_ids,
                            )
                        ),
                    )
                    for effect in comparison_effects:
                        runtime.enqueue(effect)
                inference = state.models_inference.inference
                if (
                    state.workspace is Workspace.INFERENCE
                    and inference.state
                    in {
                        Phase9LoadState.READY,
                        Phase9LoadState.DEGRADED,
                        Phase9LoadState.RECOVERED,
                    }
                    and inference.inference is None
                    and inference.active_inference is None
                ):
                    generation = inference.generation + 1
                    mode = (
                        InferenceMode.LATEST
                        if config.phase9_scenario is Phase9Scenario.INFERENCE_LATEST
                        else InferenceMode.HISTORICAL
                    )
                    state, inference_effects = reduce(
                        state,
                        RequestInference(
                            InferenceQuery(
                                f"phase9-inference-{generation:020d}",
                                generation,
                                inference.selected_model_or_alias,
                                "dataset-us-equities",
                                "incompatible-fingerprint"
                                if config.phase9_scenario is Phase9Scenario.INFERENCE_INCOMPATIBLE
                                else "dataset-fingerprint-phase9",
                                mode,
                                None
                                if mode is InferenceMode.LATEST
                                else datetime(2026, 6, 1, tzinfo=UTC),
                                None
                                if mode is InferenceMode.LATEST
                                else datetime(2026, 7, 1, tzinfo=UTC),
                            )
                        ),
                    )
                    for effect in inference_effects:
                        runtime.enqueue(effect)
                group = state.research.group("link-default-research")
                if (
                    linked_selection_pending
                    and group is not None
                    and group.query is not None
                    and group.state
                    in {
                        ResearchLoadState.READY,
                        ResearchLoadState.DEGRADED,
                        ResearchLoadState.RECOVERED,
                    }
                ):
                    state, effects = reduce(
                        state,
                        SetResearchRange(
                            group.group_id,
                            "research-ohlcv",
                            select_range(group.query.time_range, 0.25, 0.75),
                        ),
                    )
                    for effect in effects:
                        runtime.enqueue(effect)
                    linked_selection_pending = False
                view = project_shell(
                    state,
                    width=metrics.width,
                    height=metrics.height,
                    dpi_scale=metrics.dpi_scale,
                    fps=60
                    if config.phase7_workspace is not None
                    or config.phase8_workspace is not None
                    or config.phase9_workspace is not None
                    else metrics.fps,
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
