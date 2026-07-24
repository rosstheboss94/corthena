from __future__ import annotations

import json
import time
from dataclasses import replace
from datetime import UTC, datetime
from pathlib import Path
from threading import Event, Thread
from typing import Protocol, runtime_checkable

import pytest

from corthena.ui.data_experiments.actions import (
    ConfirmIngestion,
    FileBrowserCompleted,
    FilePreviewCompleted,
    IngestionCompleted,
    IngestionOperationCancelled,
    IngestionProgressed,
    Phase7Completed,
    ReconciliationCompleted,
    RequestFileBrowser,
    RequestFilePreview,
    RequestIngestionCancellation,
    RequestMassivePull,
    RequestPhase7,
    RequestReconciliation,
    RequestScheduleCommand,
    RequestSymbolDiscovery,
    ScrollFileBrowser,
    SelectDatasetSource,
    SelectFileBrowserEntry,
    SetDataIngestionView,
    SetDatasetWizardStep,
    SetSelectedSymbols,
    SymbolDiscoveryCompleted,
    UpdateFileMapping,
    UpdateIngestionForm,
)
from corthena.ui.data_experiments.deterministic import DataExperimentsDemo
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    ColumnMapping,
    CredentialRequest,
    CredentialSecretRequest,
    DataExperimentsState,
    DataIngestionView,
    DataSchedule,
    DatasetWizardStep,
    FileBrowserEntryKind,
    FileBrowserRequest,
    FilePreviewRequest,
    ImportMode,
    IngestionPlan,
    IngestionProgress,
    IngestionScenario,
    IngestionStatus,
    Phase7Request,
    Phase7Scenario,
    Phase7Workspace,
    ReconciliationRequest,
    ScheduleCadence,
    ScheduleCommand,
    ScheduleCommandKind,
    SessionPolicy,
    SourceKind,
    SymbolDiscoveryRequest,
)
from corthena.ui.data_experiments.reducer import reduce_data_experiments
from corthena.ui.effects import EffectsRuntime, RuntimeConfig
from corthena.ui.native import RaylibUIAdapter
from corthena.ui.secret_buffer import SecretEntryBuffer
from corthena.ui.shell import ShellView, project_shell
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig
from corthena.ui.state import ActivateDockPanel, AppState, reduce

FIXED_CLOCK = datetime(2026, 7, 18, 12, tzinfo=UTC)


@runtime_checkable
class _Phase7PanelRenderer(Protocol):
    def _draw_phase7_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None: ...


def _loaded() -> tuple[DataExperimentsDemo, DataExperimentsState]:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    request = Phase7Request("phase7-data-1", 1, Phase7Workspace.DATA)
    state, _ = reduce_data_experiments(DataExperimentsState(), RequestPhase7(request))
    state, _ = reduce_data_experiments(state, Phase7Completed(demo.load(request, Event())))
    return demo, state


def _browser_points(
    view: ShellView, state: DataExperimentsState, entry_kind: FileBrowserEntryKind
) -> tuple[tuple[float, float], tuple[float, float]]:
    stack = next(item for item in view.dock_stacks if item.active_panel_id == "data-catalog")
    scale = view.scale
    bounds_x = stack.body_bounds.x + 14 * scale
    bounds_y = stack.body_bounds.y + 58 * scale
    dense = (stack.body_bounds.height - 72 * scale) / scale < 260
    action_y = bounds_y + (0 if dense else 48) * scale
    content_y = action_y + 30 * scale + (8 if dense else 12) * scale
    entry_index = next(
        index
        for index, entry in enumerate(state.data.file_browser_entries)
        if entry.kind is entry_kind
    )
    row_y = content_y + 36 * scale + entry_index * 34 * scale
    return (bounds_x + 4 * scale, action_y + 4 * scale), (
        bounds_x + 4 * scale,
        row_y + 4 * scale,
    )


def _plan(demo: DataExperimentsDemo, scenario: IngestionScenario) -> IngestionPlan:
    entry = demo.load(Phase7Request("plan-load", 1, Phase7Workspace.DATA), Event()).catalog[0]
    return IngestionPlan(
        f"command-{scenario.value}",
        f"request-{scenario.value}",
        1,
        entry.dataset_id,
        entry.revision,
        entry.symbols[:2],
        "1d",
        entry.coverage,
        SessionPolicy.REGULAR,
        AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED,
        ImportMode.APPEND,
        scenario,
    )


def test_visible_catalog_and_file_import_buttons_emit_typed_actions() -> None:
    _, state = _loaded()
    adapter = RaylibUIAdapter()
    catalog = project_shell(AppState(data_experiments=state))
    stack = next(item for item in catalog.dock_stacks if item.active_panel_id == "data-catalog")
    x = stack.body_bounds.x + 24 * catalog.scale
    y = stack.body_bounds.y + 91 * catalog.scale
    assert adapter.phase7_click_actions(catalog, x, y) == [
        SetDataIngestionView(DataIngestionView.NEW_DATASET)
    ]
    assert adapter.phase7_click_actions(catalog, x + 170, y) == []

    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    file_import = project_shell(AppState(data_experiments=state))
    stack = next(item for item in file_import.dock_stacks if item.active_panel_id == "data-catalog")
    bounds_x = stack.body_bounds.x + 14 * file_import.scale
    bounds_y = stack.body_bounds.y + 58 * file_import.scale
    choose_y = bounds_y + 55 * file_import.scale
    actions = adapter.phase7_click_actions(file_import, bounds_x + 8, choose_y)
    assert len(actions) == 1
    assert isinstance(actions[0], RequestFileBrowser)
    assert actions[0].request.source_kind is None
    source_actions = adapter.phase7_click_actions(
        file_import,
        bounds_x + 8 * file_import.scale,
        bounds_y + 180 * file_import.scale,
    )
    assert source_actions == [SelectDatasetSource(state.data.sources[1].source_id)]

    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.MASSIVE_PULL))
    massive = project_shell(AppState(data_experiments=state))
    back_actions = adapter.phase7_click_actions(
        massive,
        bounds_x + 330 * massive.scale,
        bounds_y + 55 * massive.scale,
    )
    assert back_actions == [SetDataIngestionView(DataIngestionView.CATALOG)]


def test_empty_catalog_keeps_new_dataset_action_and_flow_visible(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    request = Phase7Request("phase7-data-empty", 1, Phase7Workspace.DATA, Phase7Scenario.EMPTY)
    state, _ = reduce_data_experiments(DataExperimentsState(), RequestPhase7(request))
    state, _ = reduce_data_experiments(state, Phase7Completed(demo.load(request, Event())))
    catalog = project_shell(AppState(data_experiments=state))
    stack = next(item for item in catalog.dock_stacks if item.active_panel_id == "data-catalog")
    adapter = RaylibUIAdapter()
    rendered_text: list[str] = []

    def ignore_draw(*args: object) -> None:
        return None

    def record_text(*args: object) -> None:
        if len(args) > 2 and isinstance(args[2], str):
            rendered_text.append(args[2])

    monkeypatch.setattr(adapter, "_text", record_text)
    monkeypatch.setattr(adapter, "_rect", ignore_draw)
    monkeypatch.setattr(adapter, "_outline", ignore_draw)
    monkeypatch.setattr(adapter, "_line", ignore_draw)
    assert isinstance(adapter, _Phase7PanelRenderer)
    adapter._draw_phase7_panel(  # pyright: ignore[reportPrivateUsage]
        object(),
        catalog,
        "data-catalog",
        stack.body_bounds.x,
        stack.body_bounds.y,
        stack.body_bounds.width,
        stack.body_bounds.height,
        catalog.scale,
    )
    assert "+ New Dataset" in rendered_text
    assert "No datasets are available. Create a new dataset to begin." in rendered_text

    x = stack.body_bounds.x + 24 * catalog.scale
    y = stack.body_bounds.y + 91 * catalog.scale
    actions = adapter.phase7_click_actions(catalog, x, y)
    assert actions == [SetDataIngestionView(DataIngestionView.NEW_DATASET)]

    action = actions[0]
    assert isinstance(action, SetDataIngestionView)
    state, _ = reduce_data_experiments(state, action)
    new_dataset = project_shell(AppState(data_experiments=state))
    rendered_flow: list[str] = []

    def record_new_dataset(*args: object) -> None:
        rendered_flow.append("new-dataset")

    monkeypatch.setattr(adapter, "_draw_ingestion_flow", record_new_dataset)
    adapter._draw_phase7_panel(  # pyright: ignore[reportPrivateUsage]
        object(),
        new_dataset,
        "data-catalog",
        stack.body_bounds.x,
        stack.body_bounds.y,
        stack.body_bounds.width,
        stack.body_bounds.height,
        new_dataset.scale,
    )
    assert rendered_flow == ["new-dataset"]


def test_file_browser_lists_csv_and_parquet_and_previews_only_after_selection() -> None:
    _, state = _loaded()
    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    request = FileBrowserRequest("browser-1", 1, None)
    browsing, effects = reduce_data_experiments(state, RequestFileBrowser(request))
    assert browsing.data.ingestion_view is DataIngestionView.FILE_BROWSER
    assert browsing.data.ingestion_status is IngestionStatus.LOADING
    assert len(effects) == 1

    simulator = DeterministicSimulator(SimulatorConfig(401, FIXED_CLOCK, 0.01))
    with EffectsRuntime(simulator, RuntimeConfig(worker_count=1)) as runtime:
        runtime.enqueue(effects[0])
        deadline = time.monotonic() + 2
        actions: tuple[object, ...] = ()
        while not actions and time.monotonic() < deadline:
            actions = runtime.drain()
            time.sleep(0.001)
    assert len(actions) == 1
    assert isinstance(actions[0], FileBrowserCompleted)
    listing = actions[0].listing
    assert listing.entries
    assert listing.entries[0].kind is FileBrowserEntryKind.PARENT
    assert any(entry.kind is FileBrowserEntryKind.FOLDER for entry in listing.entries)
    files = tuple(entry for entry in listing.entries if entry.kind is FileBrowserEntryKind.FILE)
    assert files
    assert {entry.source_kind for entry in files} == {SourceKind.CSV, SourceKind.PARQUET}
    assert {entry.source_name.rsplit(".", 1)[-1] for entry in files} == {"csv", "parquet"}
    ready, _ = reduce_data_experiments(browsing, actions[0])
    view = project_shell(AppState(data_experiments=ready))
    adapter = RaylibUIAdapter()
    select_point, file_point = _browser_points(view, ready, FileBrowserEntryKind.FILE)
    file_entry = next(
        entry
        for entry in ready.data.file_browser_entries
        if entry.kind is FileBrowserEntryKind.FILE
    )
    selection = adapter.phase7_click_actions(
        view,
        *file_point,
        event_time=1.0,
    )
    assert len(selection) == 1
    assert selection == [SelectFileBrowserEntry(file_entry.source_name)]
    assert isinstance(selection[0], SelectFileBrowserEntry)
    selected, _ = reduce_data_experiments(ready, selection[0])
    selected_view = project_shell(AppState(data_experiments=selected))
    submit = adapter.phase7_click_actions(
        selected_view,
        *select_point,
        event_time=1.1,
    )
    assert len(submit) == 1
    assert isinstance(submit[0], RequestFilePreview)
    assert submit[0].request.source_name == file_entry.source_name
    assert submit[0].request.source_kind is file_entry.source_kind
    previewing, preview_effects = reduce_data_experiments(selected, submit[0])
    assert previewing.data.ingestion_view is DataIngestionView.NEW_DATASET
    assert previewing.data.file_browser is None
    assert len(preview_effects) == 1
    preview = simulator.preview_file(submit[0].request, Event())
    previewed, _ = reduce_data_experiments(previewing, FilePreviewCompleted(preview))
    assert previewed.data.form_source_kind is file_entry.source_kind


def test_dataset_wizard_projects_file_schema_and_paints_editable_selection_controls(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    demo, state = _loaded()
    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    request = FilePreviewRequest(
        "selected-file-preview",
        state.data.generation,
        "C:\\Demo Data\\selected-bars.parquet",
        SourceKind.PARQUET,
    )
    loading, _ = reduce_data_experiments(state, RequestFilePreview(request))
    previewed, _ = reduce_data_experiments(
        loading, FilePreviewCompleted(demo.preview_file(request, Event()))
    )
    assert previewed.data.selected_source_id is None
    previewed, _ = reduce_data_experiments(
        previewed, SetDatasetWizardStep(DatasetWizardStep.MAP_SCHEMA)
    )
    view = project_shell(AppState(data_experiments=previewed, ui_scale_percent=100))
    stack = next(item for item in view.dock_stacks if item.active_panel_id == "data-catalog")
    adapter = RaylibUIAdapter()
    rendered_text: list[str] = []

    def ignore_draw(*args: object) -> None:
        return None

    def record_text(*args: object) -> None:
        if len(args) > 2 and isinstance(args[2], str):
            rendered_text.append(args[2])

    monkeypatch.setattr(adapter, "_text", record_text)
    monkeypatch.setattr(adapter, "_rect", ignore_draw)
    monkeypatch.setattr(adapter, "_outline", ignore_draw)
    monkeypatch.setattr(adapter, "_line", ignore_draw)
    assert isinstance(adapter, _Phase7PanelRenderer)
    adapter._draw_phase7_panel(  # pyright: ignore[reportPrivateUsage]
        object(),
        view,
        "data-catalog",
        stack.body_bounds.x,
        stack.body_bounds.y,
        stack.body_bounds.width,
        stack.body_bounds.height,
        view.scale,
    )
    assert any("Detected from selected-bars.parquet (PARQUET)" in text for text in rendered_text)
    assert {"ticker", "string", "symbol  v"} <= set(rendered_text)

    bounds_x = stack.body_bounds.x + 14 * view.scale
    bounds_y = stack.body_bounds.y + 58 * view.scale
    bounds_width = stack.body_bounds.width - 28 * view.scale
    bounds_height = stack.body_bounds.height - 72 * view.scale
    dense = bounds_height / view.scale < 260
    action_y = bounds_y + (0 if dense else 48) * view.scale
    content_y = action_y + 30 * view.scale + (8 if dense else 12) * view.scale
    mapping_y = content_y + 104 * view.scale
    role_width = min(150 * view.scale, bounds_width * 0.22)
    role_x = bounds_x + bounds_width * 0.55
    opened_actions = adapter.phase7_click_actions(
        view,
        role_x + role_width / 2,
        mapping_y + 14 * view.scale,
    )
    assert previewed.data.file_preview is not None
    assert previewed.data.file_preview.columns[0].role == "timestamp"
    assert opened_actions == []
    rendered_text.clear()
    adapter._draw_phase7_panel(  # pyright: ignore[reportPrivateUsage]
        object(),
        view,
        "data-catalog",
        stack.body_bounds.x,
        stack.body_bounds.y,
        stack.body_bounds.width,
        stack.body_bounds.height,
        view.scale,
    )
    assert {"timestamp  ^", "[x] timestamp", "[ ] ignore"} <= set(rendered_text)
    mapping_actions = adapter.phase7_click_actions(
        view,
        role_x + role_width / 2,
        mapping_y + 68 * view.scale,
    )
    assert len(mapping_actions) == 2
    for action in mapping_actions:
        assert isinstance(action, UpdateFileMapping)
        previewed, _ = reduce_data_experiments(previewed, action)
    assert previewed.data.file_preview is not None
    assert previewed.data.file_preview.columns[0].role == "symbol"
    assert previewed.data.file_preview.columns[1].role == "ignore"

    previewed, _ = reduce_data_experiments(
        previewed, SetDatasetWizardStep(DatasetWizardStep.SOURCE_SELECTION)
    )
    selection_view = project_shell(AppState(data_experiments=previewed, ui_scale_percent=100))
    rendered_text.clear()
    adapter._draw_phase7_panel(  # pyright: ignore[reportPrivateUsage]
        object(),
        selection_view,
        "data-catalog",
        stack.body_bounds.x,
        stack.body_bounds.y,
        stack.body_bounds.width,
        stack.body_bounds.height,
        selection_view.scale,
    )
    assert {
        "Choose which records the dataset recipe consumes from this source.",
        "Interval",
        "Session",
        "Adjustment",
        "Source timezone",
        "1d",
        "Regular",
        "Raw",
        "UTC",
    } <= set(rendered_text)
    selection_y = content_y + 112 * selection_view.scale
    label_width = (80 if bounds_height / selection_view.scale < 220 else 112) * view.scale
    first_interval_width = (len("1m") * 6 + 24) * view.scale
    interval_x = bounds_x + label_width + first_interval_width + 6 * view.scale
    interval_width = (len("5m") * 6 + 24) * view.scale
    interval_height = (20 if bounds_height / view.scale < 220 else 28) * view.scale
    selection_actions = adapter.phase7_click_actions(
        selection_view,
        interval_x + interval_width / 2,
        selection_y + interval_height / 2,
    )
    assert len(selection_actions) == 1
    assert isinstance(selection_actions[0], UpdateIngestionForm)
    assert selection_actions[0].interval == "5m"


def test_file_browser_double_click_opens_folder_and_exposes_parent_row() -> None:
    _, state = _loaded()
    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    simulator = DeterministicSimulator(SimulatorConfig(401, FIXED_CLOCK, 0.01))
    request = FileBrowserRequest("browser-root", 1, SourceKind.CSV)
    browsing, _ = reduce_data_experiments(state, RequestFileBrowser(request))
    ready, _ = reduce_data_experiments(
        browsing, FileBrowserCompleted(simulator.browse_files(request, Event()))
    )
    view = project_shell(AppState(data_experiments=ready))
    adapter = RaylibUIAdapter()
    _, folder_point = _browser_points(view, ready, FileBrowserEntryKind.FOLDER)
    folder = next(
        entry
        for entry in ready.data.file_browser_entries
        if entry.kind is FileBrowserEntryKind.FOLDER
    )
    first = adapter.phase7_click_actions(view, *folder_point, event_time=10.0)
    assert first == [SelectFileBrowserEntry(folder.source_name)]
    assert isinstance(first[0], SelectFileBrowserEntry)
    selected, _ = reduce_data_experiments(ready, first[0])
    second = adapter.phase7_click_actions(
        project_shell(AppState(data_experiments=selected)),
        *folder_point,
        event_time=10.25,
    )
    assert len(second) == 1 and isinstance(second[0], RequestFileBrowser)
    assert second[0].request.location == folder.source_name
    assert second[0].request.navigation_revision == 2

    opening, _ = reduce_data_experiments(selected, second[0])
    child_listing = simulator.browse_files(second[0].request, Event())
    opened, _ = reduce_data_experiments(opening, FileBrowserCompleted(child_listing))
    assert opened.data.file_browser_entries[0].kind is FileBrowserEntryKind.PARENT
    assert opened.data.file_browser_entries[0].source_name == "C:\\Demo Data"
    assert opened.data.selected_file_browser_path is None
    assert opened.data.file_browser_scroll_row == 0


def test_file_browser_merges_bounded_pages_and_clamps_scroll() -> None:
    _, state = _loaded()
    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    simulator = DeterministicSimulator(SimulatorConfig(401, FIXED_CLOCK, 0.01))
    first_request = FileBrowserRequest("browser-first", 1, SourceKind.CSV, page_size=3)
    browsing, _ = reduce_data_experiments(state, RequestFileBrowser(first_request))
    first_listing = simulator.browse_files(first_request, Event())
    assert first_listing.next_cursor == "3"
    first_ready, _ = reduce_data_experiments(browsing, FileBrowserCompleted(first_listing))
    first_paths = tuple(entry.source_name for entry in first_ready.data.file_browser_entries)

    second_request = FileBrowserRequest(
        "browser-second",
        1,
        SourceKind.CSV,
        location=first_listing.location,
        navigation_revision=1,
        cursor=first_listing.next_cursor,
        page_size=3,
    )
    loading_more, effects = reduce_data_experiments(first_ready, RequestFileBrowser(second_request))
    assert loading_more.data.file_browser_entries == first_ready.data.file_browser_entries
    assert loading_more.data.file_browser_loading_page
    assert len(effects) == 1
    merged, _ = reduce_data_experiments(
        loading_more,
        FileBrowserCompleted(simulator.browse_files(second_request, Event())),
    )
    assert len(merged.data.file_browser_entries) == 6
    assert tuple(entry.source_name for entry in merged.data.file_browser_entries[:3]) == first_paths
    assert len({entry.source_name for entry in merged.data.file_browser_entries}) == 6
    scrolled, _ = reduce_data_experiments(merged, ScrollFileBrowser(100))
    assert scrolled.data.file_browser_scroll_row == 5
    reset, _ = reduce_data_experiments(scrolled, ScrollFileBrowser(-100))
    assert reset.data.file_browser_scroll_row == 0


def test_schedules_are_a_data_panel_with_a_visible_new_schedule_button() -> None:
    _, state = _loaded()
    app_state = AppState(data_experiments=state)
    app_state, _ = reduce(app_state, ActivateDockPanel("data-schedules", 0))
    view = project_shell(app_state)
    stack = next(item for item in view.dock_stacks if item.active_panel_id == "data-schedules")
    actions = RaylibUIAdapter().phase7_click_actions(
        view,
        stack.body_bounds.x + 24 * view.scale,
        stack.body_bounds.y + 94 * view.scale,
    )
    assert len(actions) == 1
    assert isinstance(actions[0], RequestScheduleCommand)
    assert actions[0].command.kind is ScheduleCommandKind.CREATE


def test_token_actions_consume_and_discard_secret_without_published_or_repr_leakage() -> None:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    identity = CredentialRequest("credential-1", "credential-command-1", 1)
    secret = "milestone-token-should-never-escape"
    request = CredentialSecretRequest(identity, secret)
    result = demo.save_credential(request, Event())
    assert result.status.saved
    assert secret not in repr(request)
    assert secret not in repr(result)
    assert secret not in repr(demo.load(Phase7Request("load", 1, Phase7Workspace.DATA), Event()))
    invalid = demo.test_credential(CredentialSecretRequest(identity, "invalid"), Event())
    assert invalid.outcome is IngestionStatus.AUTHENTICATION_FAILED
    deleted = demo.delete_credential(identity, Event())
    assert not deleted.status.saved


def test_secret_entry_buffer_is_ui_thread_owned_bounded_and_cleared_on_take() -> None:
    buffer = SecretEntryBuffer(4)
    for character in "abcd":
        buffer.append(character)
    assert buffer.length == 4
    with pytest.raises(ValueError, match="capacity"):
        buffer.append("e")
    assert buffer.take() == "abcd"
    assert buffer.length == 0
    failures: list[BaseException] = []

    def use_from_wrong_thread() -> None:
        try:
            buffer.append("x")
        except BaseException as error:
            failures.append(error)

    thread = Thread(target=use_from_wrong_thread)
    thread.start()
    thread.join()
    assert len(failures) == 1
    assert isinstance(failures[0], RuntimeError)


def test_file_preview_mapping_and_symbol_discovery_are_bounded_and_deterministic() -> None:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    preview_request = FilePreviewRequest(
        "preview-1", 1, "bars.csv", SourceKind.CSV, IngestionScenario.SUCCESS
    )
    first = demo.preview_file(preview_request, Event())
    second = DataExperimentsDemo(401, FIXED_CLOCK).preview_file(preview_request, Event())
    assert first == second
    assert tuple(item.role for item in first.columns) == (
        "timestamp",
        "symbol",
        "open",
        "high",
        "low",
        "close",
        "volume",
    )
    invalid = demo.preview_file(
        replace(
            preview_request,
            request_id="preview-invalid",
            scenario=IngestionScenario.VALIDATION_FAILURE,
        ),
        Event(),
    )
    assert invalid.diagnostics[0].field == "high"
    discovery = demo.discover_symbols(SymbolDiscoveryRequest("symbols", 1, "m"), Event())
    assert discovery.symbols == tuple(sorted(discovery.symbols))
    assert {item.symbol for item in discovery.symbols} == {"AMD", "MSFT"}


@pytest.mark.parametrize(
    ("scenario", "status"),
    (
        (IngestionScenario.SUCCESS, IngestionStatus.COMPLETE),
        (IngestionScenario.VALIDATION_FAILURE, IngestionStatus.VALIDATION_FAILED),
        (IngestionScenario.AUTHENTICATION_FAILURE, IngestionStatus.AUTHENTICATION_FAILED),
        (IngestionScenario.ENTITLEMENT_FAILURE, IngestionStatus.ENTITLEMENT_FAILED),
        (IngestionScenario.RATE_LIMIT, IngestionStatus.RATE_LIMITED),
        (IngestionScenario.FAILURE, IngestionStatus.FAILED),
        (IngestionScenario.EMPTY, IngestionStatus.EMPTY),
        (IngestionScenario.RECOVERY, IngestionStatus.RECOVERED),
    ),
)
def test_massive_pull_models_every_required_terminal_outcome(
    scenario: IngestionScenario, status: IngestionStatus
) -> None:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    before = demo.load(Phase7Request("before", 1, Phase7Workspace.DATA), Event()).catalog
    result = demo.submit_ingestion(_plan(demo, scenario), Event(), provider=True)
    assert result.progress.status is status
    assert result.progress.provider_request_id is not None
    after = demo.load(Phase7Request("after", 1, Phase7Workspace.DATA), Event()).catalog
    if status in {IngestionStatus.COMPLETE, IngestionStatus.RECOVERED}:
        assert after[0].revision == before[0].revision + 1
    else:
        assert after == before


def test_reducer_forms_stale_results_progress_and_cancellation_are_generation_safe() -> None:
    demo, state = _loaded()
    plan = _plan(demo, IngestionScenario.SUCCESS)
    state, effects = reduce_data_experiments(
        state, SetDataIngestionView(DataIngestionView.MASSIVE_PULL)
    )
    state, effects = reduce_data_experiments(state, ConfirmIngestion(plan))
    assert state.data.ingestion_status is IngestionStatus.CONFIRMING
    assert effects == ()
    state, effects = reduce_data_experiments(state, RequestMassivePull(plan))
    assert state.data.ingestion_status is IngestionStatus.QUEUED
    assert len(effects) == 1
    running = IngestionProgress(
        plan.request_id, 1, IngestionStatus.RUNNING, 40, 100, "Pulling bars"
    )
    state, _ = reduce_data_experiments(state, IngestionProgressed(running))
    assert state.data.progress == running
    stale_result = demo.submit_ingestion(replace(plan, generation=2), Event(), provider=True)
    unchanged, _ = reduce_data_experiments(state, IngestionCompleted(stale_result))
    assert unchanged == state
    cancelling, effects = reduce_data_experiments(
        state, RequestIngestionCancellation(plan.request_id)
    )
    assert cancelling.data.ingestion_status is IngestionStatus.CANCELLING
    assert len(effects) == 1


def test_varied_same_generation_completion_order_accepts_only_latest_request() -> None:
    demo, state = _loaded()
    first = _plan(demo, IngestionScenario.AUTHENTICATION_FAILURE)
    second = replace(
        first,
        command_id="command-newer",
        correlation_id="request-newer",
        scenario=IngestionScenario.ENTITLEMENT_FAILURE,
    )
    state, _ = reduce_data_experiments(state, RequestMassivePull(first))
    state, effects = reduce_data_experiments(state, RequestMassivePull(second))
    assert len(effects) == 2
    first_result = demo.submit_ingestion(first, Event(), provider=True)
    second_result = demo.submit_ingestion(second, Event(), provider=True)
    unchanged, _ = reduce_data_experiments(state, IngestionCompleted(first_result))
    assert unchanged == state
    accepted, _ = reduce_data_experiments(state, IngestionCompleted(second_result))
    assert accepted.data.ingestion_status is IngestionStatus.ENTITLEMENT_FAILED
    assert accepted.data.progress == second_result.progress


def test_forms_preview_discovery_selection_and_reconciliation_mutate_only_typed_state() -> None:
    demo, state = _loaded()
    preview_request = FilePreviewRequest("preview", 1, "bars.parquet", SourceKind.PARQUET)
    loading, effects = reduce_data_experiments(state, RequestFilePreview(preview_request))
    assert loading.data.ingestion_status is IngestionStatus.LOADING
    assert len(effects) == 1
    preview = demo.preview_file(preview_request, Event())
    previewed, _ = reduce_data_experiments(loading, FilePreviewCompleted(preview))
    assert previewed.data.ingestion_status is IngestionStatus.PREVIEW
    edited, _ = reduce_data_experiments(
        previewed, UpdateFileMapping(ColumnMapping("ticker", "ignore", "string", False))
    )
    assert edited.data.file_preview is not None
    assert (
        next(
            item for item in edited.data.file_preview.columns if item.source_column == "ticker"
        ).role
        == "ignore"
    )
    edited, _ = reduce_data_experiments(
        edited,
        UpdateIngestionForm(
            "15m",
            SessionPolicy.ALL,
            AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED,
            ImportMode.REPLACE_RANGE,
            "America/New_York",
        ),
    )
    assert (
        edited.data.form_interval,
        edited.data.form_session,
        edited.data.form_adjustment,
        edited.data.form_mode,
        edited.data.form_source_timezone,
    ) == (
        "15m",
        SessionPolicy.ALL,
        AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED,
        ImportMode.REPLACE_RANGE,
        "America/New_York",
    )
    discovery_request = SymbolDiscoveryRequest("discover", 1, "")
    discovering, _ = reduce_data_experiments(edited, RequestSymbolDiscovery(discovery_request))
    discovered = demo.discover_symbols(discovery_request, Event())
    selected, _ = reduce_data_experiments(
        discovering,
        SymbolDiscoveryCompleted(discovered),
    )
    symbols = tuple(item.symbol for item in discovered.symbols[:2])
    selected, _ = reduce_data_experiments(selected, SetSelectedSymbols(symbols))
    assert selected.data.selected_symbols == symbols
    request = ReconciliationRequest("reconcile", 1)
    reconciling, _ = reduce_data_experiments(selected, RequestReconciliation(request))
    result = demo.reconcile(request, Event())
    reconciled, _ = reduce_data_experiments(
        reconciling,
        ReconciliationCompleted(result),
    )
    assert reconciled.data.ingestion_status is IngestionStatus.RECOVERED


def test_effect_runtime_publishes_terminal_ingestion_cancellation() -> None:
    demo, state = _loaded()
    plan = _plan(demo, IngestionScenario.SUCCESS)
    state, effects = reduce_data_experiments(state, RequestMassivePull(plan))
    simulator = DeterministicSimulator(SimulatorConfig(401, FIXED_CLOCK, 0.2))
    with EffectsRuntime(simulator, RuntimeConfig(worker_count=1)) as runtime:
        assert len(effects) == 1
        runtime.enqueue(effects[0])
        state, cancel_effects = reduce_data_experiments(
            state, RequestIngestionCancellation(plan.request_id)
        )
        runtime.enqueue(cancel_effects[0])
        deadline = time.monotonic() + 2
        actions: tuple[object, ...] = ()
        while not actions and time.monotonic() < deadline:
            actions = runtime.drain()
            time.sleep(0.001)
    assert len(actions) == 1
    assert isinstance(actions[0], IngestionOperationCancelled)
    state, _ = reduce_data_experiments(state, actions[0])
    assert state.data.ingestion_status is IngestionStatus.CANCELLED


def test_schedule_crud_enable_manual_run_and_delete_preserve_revision_identity() -> None:
    demo = DataExperimentsDemo(401, FIXED_CLOCK)
    schedule = DataSchedule(
        "schedule-test",
        1,
        "Test hourly",
        "dataset-us-equities",
        ("AAPL",),
        "1h",
        SessionPolicy.REGULAR,
        AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED,
        ScheduleCadence.HOURLY,
        True,
    )
    create = ScheduleCommand("create", "create-command", 1, ScheduleCommandKind.CREATE, schedule, 0)
    created = demo.mutate_schedule(create, Event()).schedule
    assert created == schedule
    assert created is not None
    disable = ScheduleCommand(
        "disable",
        "disable-command",
        1,
        ScheduleCommandKind.SET_ENABLED,
        replace(created, enabled=False),
        created.revision,
    )
    disabled = demo.mutate_schedule(disable, Event()).schedule
    assert disabled is not None and not disabled.enabled and disabled.revision == 2
    run = ScheduleCommand(
        "run", "run-command", 1, ScheduleCommandKind.RUN_NOW, disabled, disabled.revision
    )
    ran = demo.mutate_schedule(run, Event())
    assert ran.progress is not None and ran.progress.status is IngestionStatus.COMPLETE
    assert ran.schedule is not None
    delete = ScheduleCommand(
        "delete",
        "delete-command",
        1,
        ScheduleCommandKind.DELETE,
        ran.schedule,
        ran.schedule.revision,
    )
    deleted = demo.mutate_schedule(delete, Event())
    assert deleted.schedule is None
    assert all(item.schedule_id != schedule.schedule_id for item in deleted.schedules)


def test_canonical_ingestion_manifest_owns_complete_token_free_54_case_matrix() -> None:
    directory = Path(__file__).parent / "goldens" / "milestone1a-ingestion-golden"
    manifest = json.loads((directory / "manifest.json").read_text(encoding="utf-8"))
    entries = manifest["entries"]
    assert len(entries) == 54
    assert {entry["golden"]["metadata"]["scenario"] for entry in entries} == {
        "api_tokens",
        "file_browser",
        "file_mapping",
        "massive_pull",
        "schedule_editing",
        "active_progress",
        "validation_error",
        "authentication_error",
        "recovered",
    }
    assert {
        (
            entry["golden"]["metadata"]["width"],
            entry["golden"]["metadata"]["height"],
        )
        for entry in entries
    } == {(1280, 720), (1920, 1080)}
    assert {entry["golden"]["metadata"]["scale_percent"] for entry in entries} == {
        100,
        150,
        200,
    }
    assert all((directory / entry["golden"]["file"]).is_file() for entry in entries)
    assert all(entry["golden"]["channel_tolerance"] == 3 for entry in entries)
    assert all(entry["golden"]["max_different_ratio"] == 0.002 for entry in entries)
    assert "token" not in (directory / "manifest.json").read_text(encoding="utf-8").lower().replace(
        "api_tokens", ""
    )
