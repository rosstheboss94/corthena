"""Deterministic, explicitly synchronized Phase 8 demo service."""

from __future__ import annotations

import math
import threading
from dataclasses import replace
from datetime import datetime, timedelta

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.jobs_results.models import (
    ChartPoint,
    ChartSeries,
    CheckpointCompatibility,
    CheckpointStatus,
    ComparisonQuery,
    ConfigurationDifference,
    ConfigurationValue,
    CpuLease,
    Distribution,
    FoldWindow,
    JobCommand,
    JobCommandKind,
    JobCommandResult,
    JobDetail,
    JobLog,
    JobStage,
    JobState,
    JobSummary,
    LiveMetric,
    MetricPartition,
    MetricValue,
    Phase8Request,
    Phase8Scenario,
    Phase8Snapshot,
    Phase8Workspace,
    PredictionOverlayPoint,
    RunComparison,
    RunDetail,
    RunSummary,
    StageState,
    WorkerHealth,
)


class JobsResultsDemo:
    """Own demo lifecycle metadata and publish frozen values under one lock."""

    def __init__(self, seed: int, fixed_clock: datetime) -> None:
        if fixed_clock.tzinfo is None or fixed_clock.utcoffset() != timedelta(0):
            raise ValueError("Phase 8 fixed clock must be UTC")
        self._seed = seed
        self._clock = fixed_clock
        self._lock = threading.Lock()
        self._base_jobs = _build_jobs(fixed_clock)
        self._jobs = {item.summary.job_id: item for item in self._base_jobs}
        self._jobs_scenario: Phase8Scenario | None = None
        self._runs = {item.summary.run_id: item for item in _build_runs(fixed_clock)}
        self._commands: dict[str, JobCommandResult] = {}

    def load(
        self, request: Phase8Request, cancellation: CancellationSignalProtocol
    ) -> Phase8Snapshot:
        _cancel(cancellation, request.request_id)
        if request.scenario is Phase8Scenario.RESULTS_LOADING:
            cancellation.wait()
            raise RequestCancelledError(request.request_id)
        if request.scenario in {Phase8Scenario.RESULTS_FAILURE, Phase8Scenario.QUEUE_SATURATED}:
            raise RuntimeError("deterministic Phase 8 request failed")
        with self._lock:
            if request.workspace is Phase8Workspace.JOBS:
                if request.scenario != self._jobs_scenario:
                    jobs = _jobs_for_scenario(self._base_jobs, request.scenario, self._clock)
                    self._jobs = {item.summary.job_id: item for item in jobs}
                    self._jobs_scenario = request.scenario
                else:
                    jobs = tuple(
                        sorted(
                            self._jobs.values(),
                            key=lambda item: (
                                item.summary.stable_queue_ordinal,
                                item.summary.job_id,
                            ),
                        )
                    )
                runs: tuple[RunSummary, ...] = ()
            else:
                jobs = ()
                run_details = tuple(
                    sorted(self._runs.values(), key=lambda item: item.summary.run_id)
                )
                runs = (
                    ()
                    if request.scenario is Phase8Scenario.RESULTS_EMPTY
                    else tuple(item.summary for item in run_details)
                )
            return Phase8Snapshot(
                request,
                jobs,
                runs,
                self._seed,
                self._clock,
                request.scenario is Phase8Scenario.RESULTS_DEGRADED,
            )

    def command(
        self, command: JobCommand, cancellation: CancellationSignalProtocol
    ) -> JobCommandResult:
        _cancel(cancellation, command.request_id)
        with self._lock:
            replay = self._commands.get(command.command_id)
            if replay is not None:
                if replay.command != command:
                    raise ValueError("accepted command ID cannot be reused with different input")
                return replace(replay, replayed=True)
            current = self._jobs.get(command.job_id)
            if current is None:
                raise ValueError("job command identifies an unknown job")
            updated = _apply_command(current, command, self._clock)
            self._jobs[command.job_id] = updated
            result = JobCommandResult(
                command,
                updated,
                self._clock + timedelta(microseconds=len(self._commands) + 1),
            )
            self._commands[command.command_id] = result
            return result

    def compare(
        self, query: ComparisonQuery, cancellation: CancellationSignalProtocol
    ) -> RunComparison:
        _cancel(cancellation, query.request_id)
        with self._lock:
            try:
                runs = tuple(self._runs[run_id] for run_id in query.run_ids)
            except KeyError as error:
                raise ValueError("comparison identifies an unknown immutable run") from error
        if query.filters.model_kind is not None and any(
            item.summary.model_kind != query.filters.model_kind for item in runs
        ):
            raise ValueError("comparison selection does not satisfy the model filter")
        if query.filters.text and any(
            query.filters.text.casefold() not in item.summary.run_id.casefold() for item in runs
        ):
            raise ValueError("comparison selection does not satisfy the text filter")
        paths = sorted({item.path for run in runs for item in run.configuration})
        differences = tuple(
            ConfigurationDifference(
                path,
                tuple(
                    (run.summary.run_id, _configuration(run).get(path, "<missing>")) for run in runs
                ),
            )
            for path in paths
            if len({_configuration(run).get(path, "<missing>") for run in runs}) > 1
        )
        return RunComparison(query, runs, differences)


def _apply_command(detail: JobDetail, command: JobCommand, clock: datetime) -> JobDetail:
    state = detail.summary.state
    if command.kind is JobCommandKind.PAUSE:
        if state is not JobState.RUNNING:
            raise ValueError("pause is valid only for running jobs")
        next_state = JobState.PAUSED
        stage_title = "paused at durable checkpoint"
    elif command.kind is JobCommandKind.RESUME:
        if state not in {JobState.PAUSED, JobState.INTERRUPTED}:
            raise ValueError("resume is valid only for paused or interrupted jobs")
        if detail.checkpoint.compatibility is CheckpointCompatibility.INCOMPATIBLE:
            raise ValueError("checkpoint is incompatible; resume fails closed")
        next_state = JobState.RUNNING
        stage_title = "resumed from committed checkpoint"
    elif command.kind is JobCommandKind.CANCEL:
        if state not in {
            JobState.QUEUED,
            JobState.RUNNING,
            JobState.PAUSE_REQUESTED,
            JobState.PAUSED,
            JobState.INTERRUPTED,
        }:
            raise ValueError("cancel is valid only for active or queued jobs")
        next_state = JobState.CANCELLED
        stage_title = "cancelled cooperatively"
    else:
        raise AssertionError(f"unhandled job command {command.kind!r}")
    summary = replace(detail.summary, state=next_state, stage_title=stage_title)
    stages = tuple(
        replace(item, state=StageState.CANCELLED)
        if next_state is JobState.CANCELLED
        and item.state in {StageState.PENDING, StageState.RUNNING}
        else item
        for item in detail.stages
    )
    log = JobLog(
        len(detail.logs),
        clock + timedelta(seconds=len(detail.logs)),
        "INFO",
        f"accepted {command.kind.value} command {command.command_id}",
    )
    lease = detail.lease
    if lease is not None:
        lease = replace(
            lease,
            active_slots=lease.granted_slots if next_state is JobState.RUNNING else 0,
        )
    worker = detail.worker
    if worker is not None:
        worker = replace(
            worker,
            healthy=next_state is JobState.RUNNING,
            detail=(
                "healthy"
                if next_state is JobState.RUNNING
                else "paused"
                if next_state is JobState.PAUSED
                else "stopped"
            ),
        )
    return replace(
        detail,
        summary=summary,
        stages=stages,
        lease=lease,
        worker=worker,
        logs=(*detail.logs, log),
    )


def _build_jobs(clock: datetime) -> tuple[JobDetail, ...]:
    primary = _job_detail(
        "job-phase8-primary", 0, JobState.RUNNING, 0.63, clock, "walk-forward fold 3 / tree 82"
    )
    completed = _job_detail(
        "job-phase8-complete",
        1,
        JobState.COMPLETED,
        1.0,
        clock - timedelta(days=1),
        "immutable result published",
    )
    queued = tuple(
        _job_detail(
            "job-phase8-queued" if index == 0 else f"job-queued-{index:04d}",
            index + 2,
            JobState.QUEUED,
            0.0,
            clock + timedelta(seconds=index),
            "waiting for 4 CPU slots",
        )
        for index in range(8)
    )
    return (primary, completed, *queued)


def _job_detail(
    job_id: str,
    ordinal: int,
    state: JobState,
    progress: float,
    clock: datetime,
    stage_title: str,
) -> JobDetail:
    completed = state is JobState.COMPLETED
    summary = JobSummary(
        job_id,
        "experiment-demo-forest",
        ordinal,
        state,
        stage_title,
        progress,
        4,
        clock,
        "run-phase8-a" if completed else None,
    )
    running_units = round(progress * 100)
    stages = (
        JobStage(0, "materialize", "Materialize features", StageState.COMPLETED, 100, 100),
        JobStage(1, "folds", "Prepare walk-forward folds", StageState.COMPLETED, 4, 4),
        JobStage(
            2,
            "train",
            "Train deterministic estimators",
            StageState.COMPLETED
            if completed
            else StageState.RUNNING
            if state is JobState.RUNNING
            else StageState.PENDING,
            100 if completed else running_units,
            100,
        ),
        JobStage(
            3,
            "evaluate",
            "Evaluate and publish",
            StageState.COMPLETED if completed else StageState.PENDING,
            1 if completed else 0,
            1,
        ),
    )
    metrics = (
        LiveMetric(0, "validation_mse", 0.012 + ordinal * 0.0001, MetricPartition.VALIDATION),
        LiveMetric(1, "validation_ic", 0.061 - ordinal * 0.0002, MetricPartition.VALIDATION),
    )
    active = 0 if state in {JobState.QUEUED, JobState.PAUSED, JobState.INTERRUPTED} else 4
    lease = None if state is JobState.QUEUED else CpuLease(f"lease-{job_id}", 4, active, 0, 4)
    worker = (
        None
        if state is JobState.QUEUED
        else WorkerHealth(
            f"worker-{job_id}",
            7300 + ordinal,
            state not in {JobState.FAILED, JobState.INTERRUPTED},
            "healthy" if state not in {JobState.FAILED, JobState.INTERRUPTED} else state.value,
            clock,
        )
    )
    checkpoint = CheckpointStatus(
        f"checkpoint-{job_id}-11" if state is not JobState.QUEUED else None,
        CheckpointCompatibility.COMPATIBLE
        if state is not JobState.QUEUED
        else CheckpointCompatibility.NOT_AVAILABLE,
        11 if state is not JobState.QUEUED else 0,
        f"checkpoint-{job_id}-10" if state is not JobState.QUEUED else None,
        1,
        "durably committed" if state is not JobState.QUEUED else "awaiting worker",
    )
    logs = (
        JobLog(0, clock, "INFO", "published read-only materialization"),
        JobLog(1, clock + timedelta(minutes=30), "DEBUG", "durably committed checkpoint 11"),
        JobLog(2, clock + timedelta(minutes=38), "INFO", stage_title),
    )
    return JobDetail(summary, stages, metrics, lease, worker, checkpoint, logs)


def _jobs_for_scenario(
    jobs: tuple[JobDetail, ...], scenario: Phase8Scenario, clock: datetime
) -> tuple[JobDetail, ...]:
    primary = jobs[0]
    if scenario is Phase8Scenario.JOBS_SUCCESS:
        updated = primary
    elif scenario is Phase8Scenario.JOBS_PAUSE_RESUME:
        updated = replace(
            primary,
            summary=replace(
                primary.summary, state=JobState.PAUSED, stage_title="paused at checkpoint 11"
            ),
            lease=replace(primary.lease, active_slots=0) if primary.lease is not None else None,
            worker=replace(primary.worker, healthy=False, detail="paused")
            if primary.worker is not None
            else None,
        )
    elif scenario is Phase8Scenario.JOBS_CANCELLATION:
        updated = replace(
            primary,
            summary=replace(
                primary.summary, state=JobState.CANCELLED, stage_title="cancelled cooperatively"
            ),
            stages=tuple(
                replace(item, state=StageState.CANCELLED)
                if item.state in {StageState.PENDING, StageState.RUNNING}
                else item
                for item in primary.stages
            ),
            lease=replace(primary.lease, active_slots=0) if primary.lease is not None else None,
            worker=replace(primary.worker, healthy=False, detail="stopped")
            if primary.worker is not None
            else None,
        )
    elif scenario is Phase8Scenario.JOBS_INTERRUPTION:
        updated = replace(
            primary,
            summary=replace(
                primary.summary, state=JobState.INTERRUPTED, stage_title="explicit resume required"
            ),
            worker=replace(primary.worker, healthy=False, detail="heartbeat expired")
            if primary.worker is not None
            else None,
            lease=replace(primary.lease, active_slots=0) if primary.lease is not None else None,
        )
    elif scenario is Phase8Scenario.JOBS_FAILURE:
        updated = replace(
            primary,
            summary=replace(primary.summary, state=JobState.FAILED, stage_title="worker failed"),
            stages=tuple(
                replace(item, state=StageState.FAILED) if item.state is StageState.RUNNING else item
                for item in primary.stages
            ),
            lease=replace(primary.lease, active_slots=0) if primary.lease is not None else None,
            worker=replace(primary.worker, healthy=False, detail="failed")
            if primary.worker is not None
            else None,
        )
    elif scenario is Phase8Scenario.JOBS_CHECKPOINT_INCOMPATIBLE:
        updated = replace(
            primary,
            summary=replace(
                primary.summary,
                state=JobState.INTERRUPTED,
                stage_title="checkpoint incompatible",
            ),
            checkpoint=replace(
                primary.checkpoint,
                compatibility=CheckpointCompatibility.INCOMPATIBLE,
                detail="dependency fingerprint mismatch; previous valid checkpoint retained",
            ),
            lease=replace(primary.lease, active_slots=0) if primary.lease is not None else None,
            worker=replace(primary.worker, healthy=False, detail="resume blocked")
            if primary.worker is not None
            else None,
        )
    else:
        raise ValueError("unsupported Jobs scenario")
    log = JobLog(3, clock + timedelta(minutes=40), "INFO", updated.summary.stage_title)
    return (replace(updated, logs=(*updated.logs[:3], log)), *jobs[1:])


def _build_runs(clock: datetime) -> tuple[RunDetail, ...]:
    return tuple(
        _run_detail(
            f"run-phase8-{letter}",
            clock - timedelta(days=index),
            "random_forest" if index != 1 else "hist_gradient_boosting",
            0.012 + index * 0.0017,
            8 + index * 2,
        )
        for index, letter in enumerate(("a", "b", "c"))
    )


def _run_detail(
    run_id: str, completed_at: datetime, model_kind: str, mse: float, max_depth: int
) -> RunDetail:
    summary = RunSummary(
        run_id,
        "job-phase8-complete",
        "experiment-demo-forest",
        completed_at,
        "validation_mse",
        mse,
        model_kind,
    )
    metrics = (
        MetricValue("mse", MetricPartition.VALIDATION, None, mse),
        MetricValue("mae", MetricPartition.VALIDATION, None, mse * 6.55),
        MetricValue("pearson_ic", MetricPartition.VALIDATION, None, 0.0626 - mse),
        MetricValue("pearson_ic", MetricPartition.TEST, None, 0.0487 - mse / 10),
        MetricValue("sharpe", MetricPartition.TEST, None, 1.5221 - mse),
        MetricValue("max_drawdown", MetricPartition.TEST, None, -0.1123 - mse),
    )
    folds = tuple(
        FoldWindow(
            index,
            completed_at - timedelta(days=900 - index * 180),
            completed_at - timedelta(days=540 - index * 180),
            completed_at - timedelta(days=450 - index * 180),
            completed_at - timedelta(days=360 - index * 180),
        )
        for index in range(3)
    )
    points = tuple(
        ChartPoint(
            index,
            completed_at - timedelta(days=23 - index),
            1_000_000 * (1 + index * 0.004 + math.sin(index / 3) * 0.006),
        )
        for index in range(24)
    )
    drawdown_points = tuple(
        ChartPoint(
            item.logical_index,
            item.timestamp,
            min(0.0, (item.value / 1_000_000 - 1) - item.logical_index * 0.004),
        )
        for item in points
    )
    edges = tuple(round(-0.1 + index * 0.02, 6) for index in range(11))
    counts = (1, 3, 6, 10, 14, 14, 12, 8, 4, 1)
    overlay = tuple(
        PredictionOverlayPoint(
            index,
            completed_at - timedelta(days=23 - index),
            math.sin(index / 4) * 0.03,
            math.cos(index / 5) * 0.02,
        )
        for index in range(24)
    )
    configuration = (
        ConfigurationValue("model.kind", model_kind),
        ConfigurationValue("model.max_depth", str(max_depth)),
        ConfigurationValue("evaluation.selection_partition", "validation"),
    )
    provenance = (
        ConfigurationValue("dataset.fingerprint", "sha256:dataset-phase8-v1"),
        ConfigurationValue("engine.version", "demo-engine-1"),
    )
    disclosure = (
        "Reference long/short backtest; borrow, impact, and survivorship correction unmodeled"
    )
    return RunDetail(
        summary,
        metrics,
        folds,
        ChartSeries(f"{run_id}-equity", "Equity", points),
        ChartSeries(f"{run_id}-drawdown", "Drawdown", drawdown_points),
        Distribution(f"{run_id}-ic", "IC distribution", edges, counts),
        Distribution(f"{run_id}-prediction", "Prediction distribution", edges, counts[::-1]),
        overlay,
        configuration,
        provenance,
        disclosure,
    )


def _cancel(cancellation: CancellationSignalProtocol, request_id: str) -> None:
    if cancellation.is_set():
        raise RequestCancelledError(request_id)


def _configuration(run: RunDetail) -> dict[str, str]:
    return {item.path: item.value for item in run.configuration}
