"""Pure Phase 7 reducer with generation and draft-revision safety."""

from dataclasses import replace
from typing import assert_never

from corthena.ui.data_experiments.actions import (
    AddDatasetFeatureStep,
    BrowseFiles,
    BuildDataset,
    CancelPhase7,
    CloseFileBrowser,
    ConfirmIngestion,
    CredentialCompleted,
    DataImportCompleted,
    DatasetBuildCompleted,
    DatasetSaveCompleted,
    DeleteCredential,
    DiscoverSymbols,
    DraftEvaluationCompleted,
    DraftSaveCompleted,
    EditExperimentDraft,
    EvaluateDraft,
    ExecuteScheduleCommand,
    FileBrowserCompleted,
    FilePreviewCompleted,
    IngestionCompleted,
    IngestionOperationCancelled,
    IngestionOperationFailed,
    IngestionProgressed,
    LoadCredentialStatus,
    LoadPhase7,
    MoveDatasetFeatureStep,
    Phase7Action,
    Phase7Cancelled,
    Phase7Completed,
    Phase7Effect,
    Phase7Failed,
    PreviewFile,
    ReconcileData,
    ReconciliationCompleted,
    RemoveDatasetFeatureStep,
    RequestCredentialDelete,
    RequestCredentialSave,
    RequestCredentialStatus,
    RequestCredentialTest,
    RequestDataImport,
    RequestDatasetBuild,
    RequestDatasetSave,
    RequestDraftEvaluation,
    RequestDraftSave,
    RequestFileBrowser,
    RequestFileIngestion,
    RequestFilePreview,
    RequestIngestionCancellation,
    RequestMassivePull,
    RequestPhase7,
    RequestReconciliation,
    RequestScheduleCommand,
    RequestSubmission,
    RequestSymbolDiscovery,
    RunDataImport,
    RunFileIngestion,
    RunMassivePull,
    SaveCredential,
    SaveDataset,
    SaveDraft,
    ScheduleCommandCompleted,
    ScrollFileBrowser,
    SelectDataset,
    SelectDatasetSource,
    SelectFileBrowserEntry,
    SelectSchedule,
    SetDataIngestionView,
    SetDatasetWizardStep,
    SetFileSourceKind,
    SetIngestionScenario,
    SetPhase7Scenario,
    SetSelectedSymbols,
    SubmissionCompleted,
    SubmitExperiment,
    SymbolDiscoveryCompleted,
    TestCredential,
    UpdateFileMapping,
    UpdateIngestionForm,
)
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    DataExperimentsState,
    DataIngestionView,
    FileBrowserEntryKind,
    IngestionStatus,
    Phase7LoadState,
    Phase7Request,
    Phase7Scenario,
    Phase7Workspace,
    Phase7WorkspaceState,
)


def reduce_data_experiments(
    state: DataExperimentsState, action: Phase7Action
) -> tuple[DataExperimentsState, tuple[Phase7Effect, ...]]:
    match action:
        case RequestPhase7(request=request):
            current = state.workspace(request.workspace)
            if request.generation <= current.generation:
                raise ValueError("Phase 7 generation must advance monotonically")
            effects: tuple[Phase7Effect, ...] = (LoadPhase7(request),)
            if current.active_request is not None and current.state is Phase7LoadState.LOADING:
                effects = (CancelPhase7(current.active_request.request_id), *effects)
            updated = replace(
                current,
                generation=request.generation,
                state=Phase7LoadState.LOADING,
                scenario=request.scenario,
                active_request=request,
                error=None,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, request.workspace, updated), effects
        case Phase7Completed(snapshot=snapshot):
            request = snapshot.request
            current = state.workspace(request.workspace)
            if current.active_request != request or current.generation != request.generation:
                return state, ()
            status = (
                Phase7LoadState.DEGRADED
                if snapshot.degraded
                else Phase7LoadState.RECOVERED
                if request.scenario is Phase7Scenario.RECOVERED
                else Phase7LoadState.EMPTY
                if not snapshot.catalog
                else Phase7LoadState.READY
            )
            updated = replace(
                current,
                state=status,
                active_request=None,
                snapshot=snapshot,
                draft=snapshot.draft,
                evaluation=snapshot.evaluation,
                credential=snapshot.credential,
                schedules=snapshot.schedules,
                sources=snapshot.sources,
                source_snapshots=snapshot.source_snapshots,
                dataset_definitions=snapshot.dataset_definitions,
                dataset_versions=snapshot.dataset_versions,
                dataset_builds=snapshot.dataset_builds,
                dataset_recipe_steps=(
                    current.dataset_recipe_steps
                    or (snapshot.dataset_versions[0].steps if snapshot.dataset_versions else ())
                ),
                last_successful_dataset_build=current.last_successful_dataset_build
                or next(
                    (build for build in snapshot.dataset_builds if build.binding is not None),
                    None,
                ),
                selected_source_id=current.selected_source_id
                or (snapshot.sources[0].source_id if snapshot.sources else None),
                selected_dataset_id=current.selected_dataset_id
                or (snapshot.catalog[0].dataset_id if snapshot.catalog else None),
                error=None,
                stale=False,
            )
            return _replace_workspace(state, request.workspace, updated), ()
        case Phase7Failed(workspace=workspace, generation=generation, message=message, busy=busy):
            current = state.workspace(workspace)
            if generation != current.generation:
                return state, ()
            updated = replace(
                current,
                state=Phase7LoadState.BUSY if busy else Phase7LoadState.FAILED,
                active_request=None,
                error=message,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, workspace, updated), ()
        case Phase7Cancelled(workspace=workspace, generation=generation):
            current = state.workspace(workspace)
            if generation != current.generation:
                return state, ()
            return _replace_workspace(
                state,
                workspace,
                replace(
                    current,
                    state=Phase7LoadState.CANCELLED,
                    active_request=None,
                    error="Request cancelled",
                ),
            ), ()
        case SetPhase7Scenario(workspace=workspace, scenario=scenario):
            current = state.workspace(workspace)
            generation = current.generation + 1
            return reduce_data_experiments(
                state,
                RequestPhase7(
                    Phase7Request(
                        f"phase7-{workspace.value}-{generation:020d}",
                        generation,
                        workspace,
                        scenario,
                    )
                ),
            )
        case SelectDataset(dataset_id=dataset_id):
            if not dataset_id:
                raise ValueError("dataset ID is required")
            current = state.data
            if current.snapshot is not None and dataset_id not in {
                item.dataset_id for item in current.snapshot.catalog
            }:
                raise ValueError("selected dataset is not in the catalog")
            return replace(state, data=replace(current, selected_dataset_id=dataset_id)), ()
        case SetDataIngestionView(view=view):
            current = state.data
            return replace(
                state,
                data=replace(
                    current,
                    ingestion_view=view,
                    ingestion_status=IngestionStatus.IDLE,
                    file_browser=None,
                    file_browser_entries=(),
                    file_browser_origin=None,
                    selected_file_browser_path=None,
                    file_browser_scroll_row=0,
                    file_browser_loading_page=False,
                    error=None,
                ),
            ), ()
        case SetDatasetWizardStep(step=step):
            if state.data.ingestion_view is not DataIngestionView.NEW_DATASET:
                raise ValueError("dataset wizard step requires the New Dataset workflow")
            return replace(state, data=replace(state.data, dataset_wizard_step=step)), ()
        case SelectDatasetSource(source_id=source_id):
            if source_id not in {item.source_id for item in state.data.sources}:
                raise ValueError("selected source does not exist")
            return replace(
                state,
                data=replace(
                    state.data,
                    selected_source_id=source_id,
                    file_preview=None,
                    ingestion_status=IngestionStatus.IDLE,
                    error=None,
                ),
            ), ()
        case AddDatasetFeatureStep(step=step):
            return replace(
                state,
                data=replace(
                    state.data,
                    dataset_recipe_steps=(*state.data.dataset_recipe_steps, step),
                ),
            ), ()
        case RemoveDatasetFeatureStep(index=index):
            if not 0 <= index < len(state.data.dataset_recipe_steps):
                raise ValueError("dataset feature step index is out of range")
            return replace(
                state,
                data=replace(
                    state.data,
                    dataset_recipe_steps=tuple(
                        step
                        for step_index, step in enumerate(state.data.dataset_recipe_steps)
                        if step_index != index
                    ),
                ),
            ), ()
        case MoveDatasetFeatureStep(index=index, offset=offset):
            target = index + offset
            if (
                offset not in {-1, 1}
                or not 0 <= index < len(state.data.dataset_recipe_steps)
                or not 0 <= target < len(state.data.dataset_recipe_steps)
            ):
                raise ValueError("dataset feature step move is invalid")
            steps = list(state.data.dataset_recipe_steps)
            steps[index], steps[target] = steps[target], steps[index]
            return replace(
                state,
                data=replace(state.data, dataset_recipe_steps=tuple(steps)),
            ), ()
        case SetIngestionScenario(scenario=scenario):
            return replace(
                state,
                data=replace(
                    state.data,
                    ingestion_scenario=scenario,
                    ingestion_status=IngestionStatus.IDLE,
                    error=None,
                ),
            ), ()
        case RequestCredentialStatus(request=request):
            return _start_ingestion(
                state, request.generation, request.request_id, LoadCredentialStatus(request)
            )
        case RequestCredentialSave(request=request):
            return _start_ingestion(
                state,
                request.request.generation,
                request.request.request_id,
                SaveCredential(request),
            )
        case RequestCredentialTest(request=request):
            return _start_ingestion(
                state,
                request.request.generation,
                request.request.request_id,
                TestCredential(request),
            )
        case RequestCredentialDelete(request=request):
            return _start_ingestion(
                state, request.generation, request.request_id, DeleteCredential(request)
            )
        case CredentialCompleted(result=result):
            current = state.data
            if (
                result.request.generation != current.generation
                or result.request.request_id != current.active_ingestion_id
            ):
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    credential=result.status,
                    ingestion_status=result.outcome,
                    active_ingestion_id=None,
                    error=None
                    if result.outcome is IngestionStatus.COMPLETE
                    else result.status.safe_detail,
                ),
            ), ()
        case RequestFileBrowser(request=request):
            current = state.data
            if request.generation != current.generation:
                return state, ()
            if current.ingestion_view not in {
                DataIngestionView.FILE_IMPORT,
                DataIngestionView.NEW_DATASET,
                DataIngestionView.FILE_BROWSER,
            }:
                raise ValueError("file browser can only open from a file ingestion flow")
            continuing_page = (
                current.ingestion_view is DataIngestionView.FILE_BROWSER
                and request.cursor is not None
                and current.file_browser is not None
                and request.location == current.file_browser.location
                and request.navigation_revision == current.file_browser_navigation_revision
                and request.cursor == current.file_browser.next_cursor
            )
            navigating = request.cursor is None
            if current.ingestion_view is DataIngestionView.FILE_BROWSER and not (
                continuing_page or navigating
            ):
                return state, ()
            expected_revision = current.file_browser_navigation_revision + 1
            if navigating and request.navigation_revision != expected_revision:
                return state, ()
            started, effects = _start_ingestion(
                state,
                request.generation,
                request.request_id,
                BrowseFiles(request),
            )
            return replace(
                started,
                data=replace(
                    started.data,
                    ingestion_view=DataIngestionView.FILE_BROWSER,
                    file_browser=None if navigating else current.file_browser,
                    file_browser_entries=() if navigating else current.file_browser_entries,
                    file_browser_origin=(
                        current.ingestion_view
                        if current.ingestion_view is not DataIngestionView.FILE_BROWSER
                        else current.file_browser_origin
                    ),
                    selected_file_browser_path=(
                        None if navigating else current.selected_file_browser_path
                    ),
                    file_browser_navigation_revision=request.navigation_revision,
                    file_browser_scroll_row=0 if navigating else current.file_browser_scroll_row,
                    file_browser_loading_page=continuing_page,
                ),
            ), effects
        case FileBrowserCompleted(listing=listing):
            current = state.data
            if (
                listing.request.generation != current.generation
                or listing.request.request_id != current.active_ingestion_id
                or current.ingestion_view is not DataIngestionView.FILE_BROWSER
                or listing.request.navigation_revision != current.file_browser_navigation_revision
            ):
                return state, ()
            if listing.request.cursor is None:
                entries = listing.entries
            else:
                entries = current.file_browser_entries + tuple(
                    entry
                    for entry in listing.entries
                    if entry.source_name.casefold()
                    not in {item.source_name.casefold() for item in current.file_browser_entries}
                )
            return replace(
                state,
                data=replace(
                    current,
                    file_browser=listing,
                    file_browser_entries=entries,
                    ingestion_status=IngestionStatus.READY,
                    active_ingestion_id=None,
                    file_browser_loading_page=False,
                    error=None,
                ),
            ), ()
        case SelectFileBrowserEntry(source_name=source_name):
            current = state.data
            if current.ingestion_view is not DataIngestionView.FILE_BROWSER:
                return state, ()
            if source_name not in {entry.source_name for entry in current.file_browser_entries}:
                raise ValueError("selected file browser entry does not exist")
            return replace(state, data=replace(current, selected_file_browser_path=source_name)), ()
        case ScrollFileBrowser(row_delta=row_delta):
            current = state.data
            if current.ingestion_view is not DataIngestionView.FILE_BROWSER or row_delta == 0:
                return state, ()
            maximum = max(0, len(current.file_browser_entries) - 1)
            scroll_row = min(maximum, max(0, current.file_browser_scroll_row + row_delta))
            return replace(state, data=replace(current, file_browser_scroll_row=scroll_row)), ()
        case CloseFileBrowser():
            current = state.data
            if current.ingestion_view is not DataIngestionView.FILE_BROWSER:
                return state, ()
            effects: tuple[Phase7Effect, ...] = (
                (CancelPhase7(current.active_ingestion_id),)
                if current.active_ingestion_id is not None
                else ()
            )
            return replace(
                state,
                data=replace(
                    current,
                    ingestion_view=current.file_browser_origin or DataIngestionView.NEW_DATASET,
                    file_browser=None,
                    file_browser_entries=(),
                    file_browser_origin=None,
                    selected_file_browser_path=None,
                    file_browser_scroll_row=0,
                    file_browser_loading_page=False,
                    ingestion_status=IngestionStatus.IDLE,
                    active_ingestion_id=None,
                    error=None,
                ),
            ), effects
        case RequestFilePreview(request=request):
            current = state.data
            if current.ingestion_view is DataIngestionView.FILE_BROWSER:
                selected = next(
                    (
                        entry
                        for entry in current.file_browser_entries
                        if entry.source_name == current.selected_file_browser_path
                    ),
                    None,
                )
                if (
                    selected is None
                    or selected.kind is not FileBrowserEntryKind.FILE
                    or selected.source_name != request.source_name
                    or selected.source_kind is not request.source_kind
                ):
                    raise ValueError("file preview requires the selected matching file")
            started, effects = _start_ingestion(
                state, request.generation, request.request_id, PreviewFile(request)
            )
            if state.data.ingestion_view is not DataIngestionView.FILE_BROWSER:
                return started, effects
            return replace(
                started,
                data=replace(
                    started.data,
                    ingestion_view=state.data.file_browser_origin or DataIngestionView.NEW_DATASET,
                    file_browser=None,
                    file_browser_entries=(),
                    file_browser_origin=None,
                    selected_file_browser_path=None,
                    file_browser_scroll_row=0,
                    file_browser_loading_page=False,
                ),
            ), effects
        case FilePreviewCompleted(preview=preview):
            current = state.data
            if (
                preview.request.generation != current.generation
                or preview.request.request_id != current.active_ingestion_id
            ):
                return state, ()
            status = (
                IngestionStatus.VALIDATION_FAILED
                if preview.diagnostics
                else IngestionStatus.PREVIEW
            )
            return replace(
                state,
                data=replace(
                    current,
                    file_preview=preview,
                    selected_source_id=None,
                    form_source_kind=preview.request.source_kind,
                    ingestion_status=status,
                    active_ingestion_id=None,
                    error=preview.diagnostics[0].message if preview.diagnostics else None,
                ),
            ), ()
        case UpdateFileMapping(mapping=mapping):
            current = state.data
            preview = current.file_preview
            if preview is None or mapping.source_column not in {
                item.source_column for item in preview.columns
            }:
                raise ValueError("mapped source column is not in the active preview")
            columns = tuple(
                replace(mapping, detected=False)
                if item.source_column == mapping.source_column
                else item
                for item in preview.columns
            )
            return replace(
                state,
                data=replace(
                    current,
                    file_preview=replace(preview, columns=columns),
                    ingestion_status=IngestionStatus.READY,
                    error=None,
                ),
            ), ()
        case SetFileSourceKind(source_kind=source_kind):
            return replace(
                state,
                data=replace(
                    state.data,
                    form_source_kind=source_kind,
                    file_preview=None,
                    ingestion_status=IngestionStatus.IDLE,
                    error=None,
                ),
            ), ()
        case UpdateIngestionForm(
            interval=interval,
            session=session,
            adjustment=adjustment,
            mode=mode,
            source_timezone=source_timezone,
        ):
            if interval not in {"1m", "5m", "15m", "1h", "1d"}:
                raise ValueError("ingestion interval is unsupported")
            if adjustment is AdjustmentPolicy.SPLIT_AND_DIVIDEND:
                raise ValueError("Massive ingestion is not dividend-adjusted")
            if not source_timezone or source_timezone.strip() != source_timezone:
                raise ValueError("source timezone must be non-empty and normalized")
            return replace(
                state,
                data=replace(
                    state.data,
                    form_interval=interval,
                    form_session=session,
                    form_adjustment=adjustment,
                    form_mode=mode,
                    form_source_timezone=source_timezone,
                    ingestion_status=IngestionStatus.READY,
                    error=None,
                ),
            ), ()
        case RequestSymbolDiscovery(request=request):
            return _start_ingestion(
                state, request.generation, request.request_id, DiscoverSymbols(request)
            )
        case SymbolDiscoveryCompleted(result=result):
            current = state.data
            if (
                result.request.generation != current.generation
                or result.request.request_id != current.active_ingestion_id
            ):
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    discovered_symbols=result.symbols,
                    ingestion_status=IngestionStatus.EMPTY
                    if not result.symbols
                    else IngestionStatus.READY,
                    active_ingestion_id=None,
                    error=None,
                ),
            ), ()
        case SetSelectedSymbols(symbols=symbols):
            if tuple(sorted(set(symbols))) != symbols:
                raise ValueError("selected symbols must be unique and sorted")
            available = {item.symbol for item in state.data.discovered_symbols}
            if not set(symbols) <= available:
                raise ValueError("selected symbol is not in discovery results")
            return replace(state, data=replace(state.data, selected_symbols=symbols)), ()
        case ConfirmIngestion(plan=plan):
            current = state.data
            if plan.generation != current.generation:
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    ingestion_status=IngestionStatus.CONFIRMING,
                    active_ingestion_id=plan.request_id,
                    error=None,
                ),
            ), ()
        case RequestFileIngestion(plan=plan):
            return _start_ingestion(
                state,
                plan.generation,
                plan.request_id,
                RunFileIngestion(plan),
                IngestionStatus.QUEUED,
            )
        case RequestMassivePull(plan=plan):
            return _start_ingestion(
                state,
                plan.generation,
                plan.request_id,
                RunMassivePull(plan),
                IngestionStatus.QUEUED,
            )
        case IngestionCompleted(result=result):
            current = state.data
            if (
                result.plan.generation != current.generation
                or result.plan.request_id != current.active_ingestion_id
            ):
                return state, ()
            snapshot = current.snapshot
            if snapshot is not None and result.catalog_entry is not None:
                catalog = tuple(
                    result.catalog_entry
                    if item.dataset_id == result.catalog_entry.dataset_id
                    else item
                    for item in snapshot.catalog
                )
                if not any(item.dataset_id == result.catalog_entry.dataset_id for item in catalog):
                    catalog = (*catalog, result.catalog_entry)
                snapshot = replace(snapshot, catalog=catalog)
            return replace(
                state,
                data=replace(
                    current,
                    snapshot=snapshot,
                    progress=result.progress,
                    ingestion_status=result.progress.status,
                    active_ingestion_id=None,
                    error=result.diagnostics[0].message if result.diagnostics else None,
                ),
            ), ()
        case IngestionProgressed(progress=progress):
            current = state.data
            if (
                progress.generation != current.generation
                or progress.request_id != current.active_ingestion_id
            ):
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    progress=progress,
                    ingestion_status=progress.status,
                    error=None,
                ),
            ), ()
        case RequestIngestionCancellation(request_id=request_id):
            current = state.data
            if current.active_ingestion_id != request_id:
                return state, ()
            return replace(
                state,
                data=replace(current, ingestion_status=IngestionStatus.CANCELLING),
            ), (CancelPhase7(request_id),)
        case IngestionOperationCancelled(request_id=request_id, generation=generation):
            current = state.data
            if generation != current.generation or request_id != current.active_ingestion_id:
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    ingestion_status=IngestionStatus.CANCELLED,
                    active_ingestion_id=None,
                    error="Operation cancelled",
                ),
            ), ()
        case IngestionOperationFailed(
            request_id=request_id, generation=generation, message=message, busy=busy
        ):
            current = state.data
            if generation != current.generation or request_id != current.active_ingestion_id:
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    ingestion_status=IngestionStatus.SATURATED if busy else IngestionStatus.FAILED,
                    active_ingestion_id=None,
                    file_browser_loading_page=False,
                    error=message,
                ),
            ), ()
        case RequestScheduleCommand(command=command):
            return _start_ingestion(
                state,
                command.generation,
                command.request_id,
                ExecuteScheduleCommand(command),
                IngestionStatus.RUNNING,
            )
        case ScheduleCommandCompleted(result=result):
            current = state.data
            if (
                result.command.generation != current.generation
                or result.command.request_id != current.active_ingestion_id
            ):
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    schedules=result.schedules,
                    selected_schedule_id=result.schedule.schedule_id
                    if result.schedule is not None
                    else None,
                    progress=result.progress or current.progress,
                    ingestion_status=IngestionStatus.COMPLETE,
                    active_ingestion_id=None,
                    error=None,
                ),
            ), ()
        case SelectSchedule(schedule_id=schedule_id):
            if schedule_id not in {item.schedule_id for item in state.data.schedules}:
                raise ValueError("selected schedule does not exist")
            return replace(state, data=replace(state.data, selected_schedule_id=schedule_id)), ()
        case RequestReconciliation(request=request):
            return _start_ingestion(
                state,
                request.generation,
                request.request_id,
                ReconcileData(request),
                IngestionStatus.RECONCILING,
            )
        case ReconciliationCompleted(result=result):
            current = state.data
            if (
                result.request.generation != current.generation
                or result.request.request_id != current.active_ingestion_id
            ):
                return state, ()
            snapshot = current.snapshot
            if snapshot is not None:
                snapshot = replace(snapshot, catalog=result.catalog)
            return replace(
                state,
                data=replace(
                    current,
                    snapshot=snapshot,
                    schedules=result.schedules,
                    ingestion_status=IngestionStatus.RECOVERED,
                    active_ingestion_id=None,
                    error=None,
                ),
            ), ()
        case RequestDatasetSave(request=request):
            return _start_ingestion(
                state,
                request.generation,
                request.request_id,
                SaveDataset(request),
                IngestionStatus.RUNNING,
            )
        case DatasetSaveCompleted(result=result):
            current = state.data
            if (
                result.request.generation != current.generation
                or result.request.request_id != current.active_ingestion_id
            ):
                return state, ()
            definitions = (
                *(
                    item
                    for item in current.dataset_definitions
                    if item.dataset_id != result.definition.dataset_id
                ),
                result.definition,
            )
            return replace(
                state,
                data=replace(
                    current,
                    dataset_definitions=definitions,
                    dataset_versions=(*current.dataset_versions, result.version),
                    selected_dataset_id=result.definition.dataset_id,
                    ingestion_status=IngestionStatus.COMPLETE,
                    active_ingestion_id=None,
                    error=None,
                ),
            ), ()
        case RequestDatasetBuild(request=request):
            next_state, effects = _start_ingestion(
                state,
                request.generation,
                request.request_id,
                BuildDataset(request),
                IngestionStatus.RUNNING,
            )
            return replace(
                next_state,
                data=replace(next_state.data, active_dataset_build_id=request.command_id),
            ), effects
        case DatasetBuildCompleted(build=build):
            current = state.data
            if (
                build.generation != current.generation
                or build.correlation_id != current.active_ingestion_id
            ):
                return state, ()
            return replace(
                state,
                data=replace(
                    current,
                    dataset_builds=(*current.dataset_builds, build)[-64:],
                    last_successful_dataset_build=(
                        build
                        if build.binding is not None
                        else current.last_successful_dataset_build
                    ),
                    active_dataset_build_id=None,
                    ingestion_status=(
                        IngestionStatus.COMPLETE
                        if build.binding is not None
                        else IngestionStatus.VALIDATION_FAILED
                    ),
                    active_ingestion_id=None,
                    error=None,
                ),
            ), ()
        case RequestDataImport(request=request):
            return state, (RunDataImport(request),)
        case DataImportCompleted(result=result):
            current = state.data
            snapshot = current.snapshot
            if snapshot is None or result.request.generation != current.generation:
                return state, ()
            catalog = tuple(
                result.catalog_entry if item.dataset_id == result.catalog_entry.dataset_id else item
                for item in snapshot.catalog
            )
            updated_snapshot = replace(
                snapshot, catalog=catalog, imports=(*snapshot.imports, result)[-64:]
            )
            return replace(state, data=replace(current, snapshot=updated_snapshot)), ()
        case EditExperimentDraft(draft=draft):
            current = state.experiments
            if current.draft is not None and draft.revision <= current.draft.revision:
                raise ValueError("draft revision must advance monotonically")
            return replace(
                state,
                experiments=replace(current, draft=draft, evaluation=None, error=None),
            ), ()
        case RequestDraftEvaluation(request_id=request_id, generation=generation, draft=draft):
            return state, (EvaluateDraft(request_id, generation, draft),)
        case DraftEvaluationCompleted(evaluation=evaluation):
            current = state.experiments
            if (
                current.draft is None
                or evaluation.generation != current.generation
                or evaluation.draft.revision != current.draft.revision
            ):
                return state, ()
            return replace(state, experiments=replace(current, evaluation=evaluation)), ()
        case RequestDraftSave(request=request):
            return state, (SaveDraft(request),)
        case DraftSaveCompleted(result=result):
            current = state.experiments
            if current.draft is None or result.request.draft.revision != current.draft.revision:
                return state, ()
            return replace(
                state,
                experiments=replace(current, saved_revision=result.saved_revision),
            ), ()
        case RequestSubmission(request=request):
            return state, (SubmitExperiment(request),)
        case SubmissionCompleted(request=request, definition=definition):
            current = state.experiments
            snapshot = current.snapshot
            if (
                snapshot is None
                or current.draft is None
                or request.generation != current.generation
                or request.draft.revision != current.draft.revision
            ):
                return state, ()
            existing = {item.command_id: item for item in snapshot.experiments}
            existing[definition.command_id] = definition
            updated_snapshot = replace(
                snapshot,
                experiments=tuple(sorted(existing.values(), key=lambda item: item.experiment_id)),
            )
            return replace(state, experiments=replace(current, snapshot=updated_snapshot)), ()
        case _ as unreachable:
            assert_never(unreachable)


def _replace_workspace(
    state: DataExperimentsState,
    workspace: Phase7Workspace,
    updated: Phase7WorkspaceState,
) -> DataExperimentsState:
    return replace(
        state,
        data=updated if workspace is Phase7Workspace.DATA else state.data,
        experiments=updated if workspace is Phase7Workspace.EXPERIMENTS else state.experiments,
    )


def _start_ingestion(
    state: DataExperimentsState,
    generation: int,
    request_id: str,
    effect: Phase7Effect,
    status: IngestionStatus = IngestionStatus.LOADING,
) -> tuple[DataExperimentsState, tuple[Phase7Effect, ...]]:
    current = state.data
    if generation != current.generation:
        return state, ()
    effects: tuple[Phase7Effect, ...] = (effect,)
    if current.active_ingestion_id is not None and current.active_ingestion_id != request_id:
        effects = (CancelPhase7(current.active_ingestion_id), effect)
    return replace(
        state,
        data=replace(
            current,
            ingestion_status=status,
            active_ingestion_id=request_id,
            error=None,
        ),
    ), effects
