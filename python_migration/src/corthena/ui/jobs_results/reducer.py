"""Pure Phase 8 reducer with request and comparison generation safety."""

from dataclasses import replace
from typing import assert_never

from corthena.ui.jobs_results.actions import (
    CancelPhase8,
    CompareRuns,
    ComparisonCancelled,
    ComparisonCompleted,
    ComparisonFailed,
    ExecuteJobCommand,
    JobCommandCompleted,
    LoadPhase8,
    Phase8Action,
    Phase8Cancelled,
    Phase8Completed,
    Phase8Effect,
    Phase8Failed,
    RequestComparison,
    RequestJobCommand,
    RequestPhase8,
    SelectComparisonRuns,
    SelectJob,
    SetPhase8Scenario,
)
from corthena.ui.jobs_results.models import (
    JobsResultsState,
    Phase8LoadState,
    Phase8Request,
    Phase8Scenario,
    Phase8Workspace,
)


def reduce_jobs_results(
    state: JobsResultsState, action: Phase8Action
) -> tuple[JobsResultsState, tuple[Phase8Effect, ...]]:
    match action:
        case RequestPhase8(request=request):
            current = state.jobs if request.workspace is Phase8Workspace.JOBS else state.results
            if request.generation <= current.generation:
                raise ValueError("Phase 8 generation must advance monotonically")
            effects: tuple[Phase8Effect, ...] = (LoadPhase8(request),)
            if current.active_request is not None and current.state is Phase8LoadState.LOADING:
                effects = (CancelPhase8(current.active_request.request_id), *effects)
            updated = replace(
                current,
                generation=request.generation,
                state=Phase8LoadState.LOADING,
                scenario=request.scenario,
                active_request=request,
                error=None,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, request.workspace, updated), effects
        case Phase8Completed(snapshot=snapshot):
            request = snapshot.request
            status = (
                Phase8LoadState.DEGRADED
                if snapshot.degraded
                else Phase8LoadState.RECOVERED
                if request.scenario is Phase8Scenario.RESULTS_RECOVERED
                else Phase8LoadState.EMPTY
                if not (
                    snapshot.jobs if request.workspace is Phase8Workspace.JOBS else snapshot.runs
                )
                else Phase8LoadState.READY
            )
            if request.workspace is Phase8Workspace.JOBS:
                current_jobs = state.jobs
                if (
                    current_jobs.active_request != request
                    or current_jobs.generation != request.generation
                ):
                    return state, ()
                valid_ids = {item.summary.job_id for item in snapshot.jobs}
                selected = current_jobs.selected_job_id
                if selected not in valid_ids:
                    selected = snapshot.jobs[0].summary.job_id if snapshot.jobs else None
                return replace(
                    state,
                    jobs=replace(
                        current_jobs,
                        state=status,
                        active_request=None,
                        snapshot=snapshot,
                        selected_job_id=selected,
                        error=None,
                        stale=False,
                    ),
                ), ()
            current_results = state.results
            if (
                current_results.active_request != request
                or current_results.generation != request.generation
            ):
                return state, ()
            valid_run_ids = {item.run_id for item in snapshot.runs}
            selected_runs = tuple(
                run_id for run_id in current_results.selected_run_ids if run_id in valid_run_ids
            )
            if not selected_runs and snapshot.runs:
                selected_runs = (snapshot.runs[0].run_id,)
            return replace(
                state,
                results=replace(
                    current_results,
                    state=status,
                    active_request=None,
                    snapshot=snapshot,
                    selected_run_ids=selected_runs,
                    error=None,
                    stale=False,
                ),
            ), ()
        case Phase8Failed(workspace=workspace, generation=generation, message=message, busy=busy):
            current = state.jobs if workspace is Phase8Workspace.JOBS else state.results
            if current.generation != generation:
                return state, ()
            updated = replace(
                current,
                state=Phase8LoadState.BUSY if busy else Phase8LoadState.FAILED,
                active_request=None,
                error=message,
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, workspace, updated), ()
        case Phase8Cancelled(workspace=workspace, generation=generation):
            current = state.jobs if workspace is Phase8Workspace.JOBS else state.results
            if current.generation != generation:
                return state, ()
            updated = replace(
                current,
                state=Phase8LoadState.CANCELLED,
                active_request=None,
                error="Request cancelled",
                stale=current.snapshot is not None,
            )
            return _replace_workspace(state, workspace, updated), ()
        case SetPhase8Scenario(workspace=workspace, scenario=scenario):
            current = state.jobs if workspace is Phase8Workspace.JOBS else state.results
            generation = current.generation + 1
            return reduce_jobs_results(
                state,
                RequestPhase8(
                    Phase8Request(
                        f"phase8-{workspace.value}-{generation:020d}",
                        generation,
                        workspace,
                        scenario,
                    )
                ),
            )
        case SelectJob(job_id=job_id):
            snapshot = state.jobs.snapshot
            if snapshot is None or job_id not in {item.summary.job_id for item in snapshot.jobs}:
                raise ValueError("selected job is not in the current queue")
            return replace(state, jobs=replace(state.jobs, selected_job_id=job_id)), ()
        case RequestJobCommand(command=command):
            if command.generation != state.jobs.generation:
                return state, ()
            return state, (ExecuteJobCommand(command),)
        case JobCommandCompleted(result=result):
            current = state.jobs
            snapshot = current.snapshot
            if snapshot is None or result.command.generation != current.generation:
                return state, ()
            jobs = tuple(
                result.detail if item.summary.job_id == result.detail.summary.job_id else item
                for item in snapshot.jobs
            )
            updated_snapshot = replace(snapshot, jobs=jobs)
            return replace(state, jobs=replace(current, snapshot=updated_snapshot)), ()
        case SelectComparisonRuns(run_ids=run_ids):
            if len(run_ids) > 4 or len(set(run_ids)) != len(run_ids):
                raise ValueError("select zero to four unique comparison runs")
            snapshot = state.results.snapshot
            if snapshot is None or not set(run_ids) <= {item.run_id for item in snapshot.runs}:
                raise ValueError("comparison selection contains an unknown run")
            return replace(state, results=replace(state.results, selected_run_ids=run_ids)), ()
        case RequestComparison(query=query):
            current = state.results
            if query.generation <= current.comparison_generation:
                raise ValueError("comparison generation must advance monotonically")
            effects = (CompareRuns(query),)
            if current.active_comparison is not None:
                effects = (CancelPhase8(current.active_comparison.request_id), *effects)
            return replace(
                state,
                results=replace(
                    current,
                    comparison_generation=query.generation,
                    active_comparison=query,
                    comparison_error=None,
                ),
            ), effects
        case ComparisonCompleted(comparison=comparison):
            current = state.results
            if current.active_comparison != comparison.query or (
                comparison.query.generation != current.comparison_generation
            ):
                return state, ()
            return replace(
                state,
                results=replace(
                    current,
                    active_comparison=None,
                    comparison=comparison,
                    comparison_error=None,
                ),
            ), ()
        case ComparisonFailed(request_id=request_id, generation=generation, message=message):
            current = state.results
            if (
                current.active_comparison is None
                or current.active_comparison.request_id != request_id
                or current.comparison_generation != generation
            ):
                return state, ()
            return replace(
                state,
                results=replace(current, active_comparison=None, comparison_error=message),
            ), ()
        case ComparisonCancelled(request_id=request_id, generation=generation):
            current = state.results
            if (
                current.active_comparison is None
                or current.active_comparison.request_id != request_id
                or current.comparison_generation != generation
            ):
                return state, ()
            return replace(
                state,
                results=replace(
                    current,
                    active_comparison=None,
                    comparison_error="Comparison cancelled",
                ),
            ), ()
        case _ as unreachable:
            assert_never(unreachable)


def _replace_workspace(
    state: JobsResultsState, workspace: Phase8Workspace, updated: object
) -> JobsResultsState:
    if workspace is Phase8Workspace.JOBS:
        if not isinstance(updated, type(state.jobs)):
            raise TypeError("Jobs workspace replacement has the wrong type")
        return replace(state, jobs=updated)
    if not isinstance(updated, type(state.results)):
        raise TypeError("Results workspace replacement has the wrong type")
    return replace(state, results=updated)
