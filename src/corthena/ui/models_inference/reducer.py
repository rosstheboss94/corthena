"""Pure Phase 9 reducer with generation-safe immutable publication."""

from dataclasses import replace
from typing import assert_never

from corthena.ui.models_inference.actions import (
    AliasAssignmentCompleted,
    AssignAlias,
    CancelPhase9,
    ExportCompleted,
    ExportFailed,
    InferenceCancelled,
    InferenceCompleted,
    InferenceFailed,
    LoadPhase9,
    Phase9Action,
    Phase9Cancelled,
    Phase9Completed,
    Phase9Effect,
    Phase9Failed,
    PrepareExport,
    RequestAliasAssignment,
    RequestExport,
    RequestInference,
    RequestPhase9,
    ScoreInference,
    SelectModel,
    SelectTree,
    SetPhase9Scenario,
)
from corthena.ui.models_inference.models import (
    ExportState,
    ModelsInferenceState,
    Phase9LoadState,
    Phase9Request,
    Phase9Scenario,
    Phase9Workspace,
)


def reduce_models_inference(
    state: ModelsInferenceState, action: Phase9Action
) -> tuple[ModelsInferenceState, tuple[Phase9Effect, ...]]:
    match action:
        case RequestPhase9(request=request):
            if request.workspace is Phase9Workspace.MODELS:
                current = state.models
            else:
                current = state.inference
            if request.generation <= current.generation:
                raise ValueError("Phase 9 generation must advance monotonically")
            effects: tuple[Phase9Effect, ...] = (LoadPhase9(request),)
            if current.active_request is not None and current.state is Phase9LoadState.LOADING:
                effects = (CancelPhase9(current.active_request.request_id), *effects)
            updated = replace(
                current,
                generation=request.generation,
                state=Phase9LoadState.LOADING,
                scenario=request.scenario,
                active_request=request,
                error=None,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, request.workspace, updated), effects
        case Phase9Completed(snapshot=snapshot):
            request = snapshot.request
            if request.workspace is Phase9Workspace.MODELS:
                current = state.models
            else:
                current = state.inference
            if current.active_request != request or current.generation != request.generation:
                return state, ()
            contents = snapshot.models
            status = (
                Phase9LoadState.LOADING
                if request.scenario
                in {Phase9Scenario.MODELS_LOADING, Phase9Scenario.INFERENCE_LOADING}
                else Phase9LoadState.DEGRADED
                if snapshot.degraded
                else Phase9LoadState.RECOVERED
                if request.scenario
                in {Phase9Scenario.MODELS_RECOVERED, Phase9Scenario.INFERENCE_RECOVERED}
                else Phase9LoadState.EMPTY
                if not contents
                else Phase9LoadState.READY
            )
            if request.workspace is Phase9Workspace.MODELS:
                models_state = state.models
                selected = models_state.selected_model_id
                ids = {item.model_id for item in snapshot.models}
                if selected not in ids:
                    selected = snapshot.models[0].model_id if snapshot.models else None
                return replace(
                    state,
                    models=replace(
                        models_state,
                        state=status,
                        active_request=None,
                        snapshot=snapshot,
                        selected_model_id=selected,
                        selected_tree_index=0,
                        error=None,
                        stale=False,
                    ),
                ), ()
            return replace(
                state,
                inference=replace(
                    current,
                    state=status,
                    active_request=None,
                    snapshot=snapshot,
                    history=snapshot.inference_history,
                    error=None,
                    stale=False,
                ),
            ), ()
        case Phase9Failed(workspace=workspace, generation=generation, message=message, busy=busy):
            current = state.models if workspace is Phase9Workspace.MODELS else state.inference
            if current.generation != generation:
                return state, ()
            updated = replace(
                current,
                state=Phase9LoadState.BUSY if busy else Phase9LoadState.FAILED,
                active_request=None,
                error=message,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, workspace, updated), ()
        case Phase9Cancelled(workspace=workspace, generation=generation):
            current = state.models if workspace is Phase9Workspace.MODELS else state.inference
            if current.generation != generation:
                return state, ()
            return _replace_workspace(
                state,
                workspace,
                replace(
                    current,
                    state=Phase9LoadState.CANCELLED,
                    active_request=None,
                    error="Request cancelled",
                    stale=current.snapshot is not None,
                ),
            ), ()
        case SetPhase9Scenario(workspace=workspace, scenario=scenario):
            current = state.models if workspace is Phase9Workspace.MODELS else state.inference
            generation = current.generation + 1
            return reduce_models_inference(
                state,
                RequestPhase9(
                    Phase9Request(
                        f"phase9-{workspace.value}-{generation:020d}",
                        generation,
                        workspace,
                        scenario,
                    )
                ),
            )
        case SelectModel(model_id=model_id):
            snapshot = state.models.snapshot
            if snapshot is None or model_id not in {item.model_id for item in snapshot.models}:
                raise ValueError("selected model is not registered")
            return replace(
                state,
                models=replace(state.models, selected_model_id=model_id, selected_tree_index=0),
            ), ()
        case SelectTree(tree_index=index):
            snapshot = state.models.snapshot
            model = (
                next(
                    (
                        item
                        for item in snapshot.models
                        if item.model_id == state.models.selected_model_id
                    ),
                    None,
                )
                if snapshot
                else None
            )
            if model is None or not 0 <= index < len(model.trees):
                raise ValueError("selected tree index is invalid")
            return replace(state, models=replace(state.models, selected_tree_index=index)), ()
        case RequestAliasAssignment(command=command):
            if command.generation != state.models.generation:
                return state, ()
            return state, (AssignAlias(command),)
        case AliasAssignmentCompleted(result=result):
            current = state.models
            snapshot = current.snapshot
            if snapshot is None or result.command.generation != current.generation:
                return state, ()
            aliases = dict(snapshot.aliases)
            aliases[result.event.alias] = result.event.model_id
            updated = replace(
                snapshot,
                aliases=tuple(sorted(aliases.items())),
                alias_history=(*snapshot.alias_history, result.event),
            )
            return replace(state, models=replace(current, snapshot=updated)), ()
        case RequestInference(query=query):
            current = state.inference
            if query.generation <= current.generation:
                raise ValueError("inference generation must advance monotonically")
            effects: tuple[Phase9Effect, ...] = (ScoreInference(query),)
            if current.active_inference is not None:
                effects = (CancelPhase9(current.active_inference.request_id), *effects)
            return replace(
                state,
                inference=replace(
                    current,
                    generation=query.generation,
                    state=Phase9LoadState.LOADING,
                    active_inference=query,
                    inference=None,
                    error=None,
                ),
            ), effects
        case InferenceCompleted(snapshot=snapshot):
            current = state.inference
            if (
                current.active_inference != snapshot.query
                or current.generation != snapshot.query.generation
            ):
                return state, ()
            status = (
                Phase9LoadState.READY
                if snapshot.compatibility.compatible
                else Phase9LoadState.FAILED
            )
            history = (
                (*current.history, snapshot)
                if snapshot.compatibility.compatible
                else current.history
            )
            return replace(
                state,
                inference=replace(
                    current,
                    state=status,
                    active_inference=None,
                    inference=snapshot,
                    history=history,
                    error=None
                    if snapshot.compatibility.compatible
                    else "; ".join(item.message for item in snapshot.compatibility.issues),
                ),
            ), ()
        case InferenceFailed(
            request_id=request_id, generation=generation, message=message, busy=busy
        ):
            current = state.inference
            if (
                current.active_inference is None
                or current.active_inference.request_id != request_id
                or current.generation != generation
            ):
                return state, ()
            return replace(
                state,
                inference=replace(
                    current,
                    state=Phase9LoadState.BUSY if busy else Phase9LoadState.FAILED,
                    active_inference=None,
                    error=message,
                ),
            ), ()
        case InferenceCancelled(request_id=request_id, generation=generation):
            current = state.inference
            if (
                current.active_inference is None
                or current.active_inference.request_id != request_id
                or current.generation != generation
            ):
                return state, ()
            return replace(
                state,
                inference=replace(
                    current,
                    state=Phase9LoadState.CANCELLED,
                    active_inference=None,
                    error="Inference cancelled",
                ),
            ), ()
        case RequestExport(request=request):
            current = state.inference
            if current.inference is None or current.inference.inference_id != request.inference_id:
                raise ValueError("export requires the current completed inference")
            return replace(
                state,
                inference=replace(
                    current,
                    export_state=ExportState.PREPARING,
                    active_export=request,
                    export_result=None,
                ),
            ), (PrepareExport(request),)
        case ExportCompleted(result=result):
            current = state.inference
            if current.active_export != result.request:
                return state, ()
            return replace(
                state,
                inference=replace(
                    current,
                    export_state=ExportState.READY,
                    active_export=None,
                    export_result=result,
                ),
            ), ()
        case ExportFailed(request_id=request_id, generation=generation, message=message):
            current = state.inference
            if (
                current.active_export is None
                or current.active_export.request_id != request_id
                or current.active_export.generation != generation
            ):
                return state, ()
            return replace(
                state,
                inference=replace(
                    current, export_state=ExportState.FAILED, active_export=None, error=message
                ),
            ), ()
        case _ as unreachable:
            assert_never(unreachable)


def _replace_workspace(
    state: ModelsInferenceState, workspace: Phase9Workspace, updated: object
) -> ModelsInferenceState:
    if workspace is Phase9Workspace.MODELS:
        if not isinstance(updated, type(state.models)):
            raise TypeError("Models workspace replacement has the wrong type")
        return replace(state, models=updated)
    if not isinstance(updated, type(state.inference)):
        raise TypeError("Inference workspace replacement has the wrong type")
    return replace(state, inference=updated)
