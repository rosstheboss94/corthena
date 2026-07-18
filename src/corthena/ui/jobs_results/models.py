"""Immutable typed values for Phase 8 Jobs and Results workflows."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum


class Phase8Workspace(StrEnum):
    JOBS = "jobs"
    RESULTS = "results"


class Phase8Scenario(StrEnum):
    JOBS_SUCCESS = "jobs_success"
    JOBS_PAUSE_RESUME = "jobs_pause_resume"
    JOBS_CANCELLATION = "jobs_cancellation"
    JOBS_INTERRUPTION = "jobs_interruption"
    JOBS_FAILURE = "jobs_failure"
    JOBS_CHECKPOINT_INCOMPATIBLE = "jobs_checkpoint_incompatible"
    RESULTS_NORMAL = "results_normal"
    RESULTS_LOADING = "results_loading"
    RESULTS_FAILURE = "results_failure"
    RESULTS_DEGRADED = "results_degraded"
    RESULTS_RECOVERED = "results_recovered"
    RESULTS_EMPTY = "results_empty"
    QUEUE_SATURATED = "queue_saturated"


class Phase8LoadState(StrEnum):
    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    EMPTY = "empty"
    FAILED = "failed"
    DEGRADED = "degraded"
    RECOVERED = "recovered"
    CANCELLED = "cancelled"
    BUSY = "queue_saturated"


class JobState(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    PAUSE_REQUESTED = "pause_requested"
    PAUSED = "paused"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"
    INTERRUPTED = "interrupted"


class JobCommandKind(StrEnum):
    PAUSE = "pause"
    RESUME = "resume"
    CANCEL = "cancel"


class StageState(StrEnum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class MetricPartition(StrEnum):
    VALIDATION = "validation"
    TEST = "test"


class CheckpointCompatibility(StrEnum):
    COMPATIBLE = "compatible"
    INCOMPATIBLE = "incompatible"
    NOT_AVAILABLE = "not_available"


@dataclass(frozen=True, slots=True)
class Phase8Request:
    request_id: str
    generation: int
    workspace: Phase8Workspace
    scenario: Phase8Scenario

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("Phase 8 generation must be positive")
        if (
            self.workspace is Phase8Workspace.JOBS
            and not self.scenario.value.startswith("jobs_")
            and self.scenario is not Phase8Scenario.QUEUE_SATURATED
        ):
            raise ValueError("Jobs requests require a Jobs scenario")
        if (
            self.workspace is Phase8Workspace.RESULTS
            and not self.scenario.value.startswith("results_")
            and self.scenario is not Phase8Scenario.QUEUE_SATURATED
        ):
            raise ValueError("Results requests require a Results scenario")


@dataclass(frozen=True, slots=True)
class JobStage:
    ordinal: int
    stage_id: str
    title: str
    state: StageState
    completed_units: int
    total_units: int

    def __post_init__(self) -> None:
        _identity(self.stage_id)
        _identity(self.title)
        if self.ordinal < 0 or self.total_units < 1:
            raise ValueError("stage ordinal and total must be valid")
        if not 0 <= self.completed_units <= self.total_units:
            raise ValueError("stage progress is outside its bounds")


@dataclass(frozen=True, slots=True)
class LiveMetric:
    sequence: int
    name: str
    value: float
    partition: MetricPartition

    def __post_init__(self) -> None:
        _identity(self.name)
        if self.sequence < 0 or not math.isfinite(self.value):
            raise ValueError("live metric must have a finite value and sequence")


@dataclass(frozen=True, slots=True)
class CpuLease:
    lease_id: str
    granted_slots: int
    active_slots: int
    queued_tasks: int
    peak_slots: int

    def __post_init__(self) -> None:
        _identity(self.lease_id)
        if min(self.granted_slots, self.active_slots, self.queued_tasks, self.peak_slots) < 0:
            raise ValueError("lease counts cannot be negative")
        if self.active_slots > self.granted_slots or self.peak_slots > self.granted_slots:
            raise ValueError("lease use cannot exceed the grant")


@dataclass(frozen=True, slots=True)
class WorkerHealth:
    worker_id: str
    process_id: int
    healthy: bool
    detail: str
    last_heartbeat: datetime

    def __post_init__(self) -> None:
        _identity(self.worker_id)
        _identity(self.detail)
        if self.process_id < 1:
            raise ValueError("worker process ID must be positive")
        _utc(self.last_heartbeat, "worker heartbeat")


@dataclass(frozen=True, slots=True)
class CheckpointStatus:
    checkpoint_id: str | None
    compatibility: CheckpointCompatibility
    committed_sequence: int
    previous_valid_checkpoint_id: str | None
    schema_version: int
    detail: str

    def __post_init__(self) -> None:
        if self.checkpoint_id is not None:
            _identity(self.checkpoint_id)
        if self.previous_valid_checkpoint_id is not None:
            _identity(self.previous_valid_checkpoint_id)
        _identity(self.detail)
        if self.committed_sequence < 0 or self.schema_version < 1:
            raise ValueError("checkpoint sequence and schema must be valid")
        if (
            self.compatibility is CheckpointCompatibility.NOT_AVAILABLE
            and self.checkpoint_id is not None
        ):
            raise ValueError("an unavailable checkpoint cannot have an identity")


@dataclass(frozen=True, slots=True)
class JobLog:
    sequence: int
    timestamp: datetime
    level: str
    message: str

    def __post_init__(self) -> None:
        if self.sequence < 0:
            raise ValueError("log sequence cannot be negative")
        _utc(self.timestamp, "log timestamp")
        _identity(self.level)
        _identity(self.message)


@dataclass(frozen=True, slots=True)
class JobSummary:
    job_id: str
    experiment_id: str
    stable_queue_ordinal: int
    state: JobState
    stage_title: str
    progress: float
    requested_slots: int
    created_at: datetime
    completed_run_id: str | None = None

    def __post_init__(self) -> None:
        _identity(self.job_id)
        _identity(self.experiment_id)
        _identity(self.stage_title)
        if self.completed_run_id is not None:
            _identity(self.completed_run_id)
        if self.stable_queue_ordinal < 0 or self.requested_slots < 1:
            raise ValueError("job queue ordinal and slots must be valid")
        if not math.isfinite(self.progress) or not 0 <= self.progress <= 1:
            raise ValueError("job progress must be in [0, 1]")
        _utc(self.created_at, "job creation timestamp")
        if (self.state is JobState.COMPLETED) != (self.completed_run_id is not None):
            raise ValueError("only completed jobs identify an immutable run")


@dataclass(frozen=True, slots=True)
class JobDetail:
    summary: JobSummary
    stages: tuple[JobStage, ...]
    metrics: tuple[LiveMetric, ...]
    lease: CpuLease | None
    worker: WorkerHealth | None
    checkpoint: CheckpointStatus
    logs: tuple[JobLog, ...]

    def __post_init__(self) -> None:
        if tuple(item.ordinal for item in self.stages) != tuple(range(len(self.stages))):
            raise ValueError("job stages must be in contiguous logical order")
        if tuple(item.sequence for item in self.logs) != tuple(
            sorted(item.sequence for item in self.logs)
        ):
            raise ValueError("job logs must be in stable sequence order")


@dataclass(frozen=True, slots=True)
class JobCommand:
    command_id: str
    correlation_id: str
    generation: int
    job_id: str
    kind: JobCommandKind

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id, self.job_id):
            _identity(value)
        if self.generation < 1:
            raise ValueError("job command generation must be positive")

    @property
    def request_id(self) -> str:
        return self.correlation_id


@dataclass(frozen=True, slots=True)
class JobCommandResult:
    command: JobCommand
    detail: JobDetail
    accepted_at: datetime
    replayed: bool = False

    def __post_init__(self) -> None:
        _utc(self.accepted_at, "job command acceptance timestamp")


@dataclass(frozen=True, slots=True)
class MetricValue:
    metric: str
    partition: MetricPartition
    fold: int | None
    value: float | None

    def __post_init__(self) -> None:
        _identity(self.metric)
        if self.fold is not None and self.fold < 0:
            raise ValueError("metric fold cannot be negative")
        if self.value is not None and not math.isfinite(self.value):
            raise ValueError("metric values must be finite or missing")


@dataclass(frozen=True, slots=True)
class FoldWindow:
    fold: int
    train_start: datetime
    train_end: datetime
    validation_end: datetime
    test_end: datetime

    def __post_init__(self) -> None:
        if self.fold < 0:
            raise ValueError("fold ordinal cannot be negative")
        for value in (self.train_start, self.train_end, self.validation_end, self.test_end):
            _utc(value, "fold timestamp")
        if not self.train_start < self.train_end < self.validation_end < self.test_end:
            raise ValueError("fold windows must be chronological")


@dataclass(frozen=True, slots=True)
class ChartPoint:
    logical_index: int
    timestamp: datetime
    value: float

    def __post_init__(self) -> None:
        if self.logical_index < 0 or not math.isfinite(self.value):
            raise ValueError("chart points require a finite value and logical index")
        _utc(self.timestamp, "chart timestamp")


@dataclass(frozen=True, slots=True)
class ChartSeries:
    series_id: str
    label: str
    points: tuple[ChartPoint, ...]

    def __post_init__(self) -> None:
        _identity(self.series_id)
        _identity(self.label)
        if tuple(item.logical_index for item in self.points) != tuple(range(len(self.points))):
            raise ValueError("chart points must use contiguous logical indexes")


@dataclass(frozen=True, slots=True)
class Distribution:
    distribution_id: str
    label: str
    bin_edges: tuple[float, ...]
    counts: tuple[int, ...]

    def __post_init__(self) -> None:
        _identity(self.distribution_id)
        _identity(self.label)
        if len(self.bin_edges) != len(self.counts) + 1 or not self.counts:
            raise ValueError("distribution edges must bound every bin")
        if any(not math.isfinite(value) for value in self.bin_edges):
            raise ValueError("distribution edges must be finite")
        if tuple(sorted(self.bin_edges)) != self.bin_edges or min(self.counts) < 0:
            raise ValueError("distribution bins must be ordered and non-negative")


@dataclass(frozen=True, slots=True)
class PredictionOverlayPoint:
    logical_index: int
    timestamp: datetime
    prediction: float
    market_return: float

    def __post_init__(self) -> None:
        if self.logical_index < 0 or not all(
            math.isfinite(value) for value in (self.prediction, self.market_return)
        ):
            raise ValueError("overlay values must be finite")
        _utc(self.timestamp, "overlay timestamp")


@dataclass(frozen=True, slots=True)
class ConfigurationValue:
    path: str
    value: str

    def __post_init__(self) -> None:
        _identity(self.path)
        _identity(self.value)


@dataclass(frozen=True, slots=True)
class RunSummary:
    run_id: str
    job_id: str
    experiment_id: str
    completed_at: datetime
    selection_metric: str
    selection_value: float
    model_kind: str

    def __post_init__(self) -> None:
        for value in (
            self.run_id,
            self.job_id,
            self.experiment_id,
            self.selection_metric,
            self.model_kind,
        ):
            _identity(value)
        _utc(self.completed_at, "run completion timestamp")
        if not math.isfinite(self.selection_value):
            raise ValueError("selection value must be finite")


@dataclass(frozen=True, slots=True)
class RunDetail:
    summary: RunSummary
    metrics: tuple[MetricValue, ...]
    folds: tuple[FoldWindow, ...]
    equity: ChartSeries
    drawdown: ChartSeries
    ic_distribution: Distribution
    prediction_distribution: Distribution
    overlay: tuple[PredictionOverlayPoint, ...]
    configuration: tuple[ConfigurationValue, ...]
    provenance: tuple[ConfigurationValue, ...]
    backtest_disclosure: str

    def __post_init__(self) -> None:
        _identity(self.backtest_disclosure)
        if tuple(item.fold for item in self.folds) != tuple(range(len(self.folds))):
            raise ValueError("run folds must be chronological and contiguous")
        partitions = frozenset(item.partition for item in self.metrics)
        if partitions != frozenset({MetricPartition.VALIDATION, MetricPartition.TEST}):
            raise ValueError("run metrics must keep validation and test partitions")
        if tuple(item.logical_index for item in self.overlay) != tuple(range(len(self.overlay))):
            raise ValueError("overlay points must be in stable logical order")


@dataclass(frozen=True, slots=True)
class RunFilter:
    text: str = ""
    model_kind: str | None = None

    def __post_init__(self) -> None:
        if self.text.strip() != self.text:
            raise ValueError("run filter text must be normalized")
        if self.model_kind is not None:
            _identity(self.model_kind)


@dataclass(frozen=True, slots=True)
class ComparisonQuery:
    request_id: str
    generation: int
    run_ids: tuple[str, ...]
    filters: RunFilter = RunFilter()

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("comparison generation must be positive")
        if len(self.run_ids) > 4 or len(set(self.run_ids)) != len(self.run_ids):
            raise ValueError("comparisons require zero to four unique runs")
        for run_id in self.run_ids:
            _identity(run_id)


@dataclass(frozen=True, slots=True)
class ConfigurationDifference:
    path: str
    values: tuple[tuple[str, str], ...]

    def __post_init__(self) -> None:
        _identity(self.path)
        if not self.values or len({run_id for run_id, _ in self.values}) != len(self.values):
            raise ValueError("configuration differences require unique runs")


@dataclass(frozen=True, slots=True)
class RunComparison:
    query: ComparisonQuery
    runs: tuple[RunDetail, ...]
    differences: tuple[ConfigurationDifference, ...]

    def __post_init__(self) -> None:
        if tuple(item.summary.run_id for item in self.runs) != self.query.run_ids:
            raise ValueError("comparison runs must preserve query order")


@dataclass(frozen=True, slots=True)
class Phase8Snapshot:
    request: Phase8Request
    jobs: tuple[JobDetail, ...]
    runs: tuple[RunSummary, ...]
    replay_seed: int
    replay_clock: datetime
    degraded: bool = False

    def __post_init__(self) -> None:
        _utc(self.replay_clock, "Phase 8 replay clock")
        job_ids = tuple(item.summary.job_id for item in self.jobs)
        if len(job_ids) != len(set(job_ids)):
            raise ValueError("job identities must be unique")
        if job_ids != tuple(
            item.summary.job_id
            for item in sorted(
                self.jobs,
                key=lambda item: (item.summary.stable_queue_ordinal, item.summary.job_id),
            )
        ):
            raise ValueError("jobs must use stable persisted queue order")
        run_ids = tuple(item.run_id for item in self.runs)
        if len(run_ids) != len(set(run_ids)):
            raise ValueError("run identities must be unique")


@dataclass(frozen=True, slots=True)
class JobsWorkspaceState:
    generation: int = 0
    state: Phase8LoadState = Phase8LoadState.IDLE
    scenario: Phase8Scenario = Phase8Scenario.JOBS_SUCCESS
    active_request: Phase8Request | None = None
    snapshot: Phase8Snapshot | None = None
    selected_job_id: str | None = None
    error: str | None = None
    stale: bool = False


@dataclass(frozen=True, slots=True)
class ResultsWorkspaceState:
    generation: int = 0
    state: Phase8LoadState = Phase8LoadState.IDLE
    scenario: Phase8Scenario = Phase8Scenario.RESULTS_NORMAL
    active_request: Phase8Request | None = None
    snapshot: Phase8Snapshot | None = None
    selected_run_ids: tuple[str, ...] = ()
    comparison_generation: int = 0
    active_comparison: ComparisonQuery | None = None
    comparison: RunComparison | None = None
    comparison_error: str | None = None
    error: str | None = None
    stale: bool = False


@dataclass(frozen=True, slots=True)
class JobsResultsState:
    jobs: JobsWorkspaceState = JobsWorkspaceState()
    results: ResultsWorkspaceState = ResultsWorkspaceState()


def _identity(value: str) -> None:
    if not value or value.strip() != value:
        raise ValueError("identity values must be non-empty and normalized")


def _utc(value: datetime, label: str) -> None:
    if value.tzinfo is None or value.utcoffset() != timedelta(0):
        raise ValueError(f"{label} must be UTC")
