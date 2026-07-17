"""Pure Phase 7 reducer with generation and draft-revision safety."""

from dataclasses import replace
from typing import assert_never

from corthena.ui.data_experiments.actions import (
    CancelPhase7,
    DataImportCompleted,
    DraftEvaluationCompleted,
    DraftSaveCompleted,
    EditExperimentDraft,
    EvaluateDraft,
    LoadPhase7,
    Phase7Action,
    Phase7Cancelled,
    Phase7Completed,
    Phase7Effect,
    Phase7Failed,
    RequestDataImport,
    RequestDraftEvaluation,
    RequestDraftSave,
    RequestPhase7,
    RequestSubmission,
    RunDataImport,
    SaveDraft,
    SelectDataset,
    SetPhase7Scenario,
    SubmissionCompleted,
    SubmitExperiment,
)
from corthena.ui.data_experiments.models import (
    DataExperimentsState,
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
