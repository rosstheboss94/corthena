"""Immutable typed values for Phase 7 Data and Experiments workflows."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum


class Phase7Workspace(StrEnum):
    DATA = "data"
    EXPERIMENTS = "experiments"


class Phase7Scenario(StrEnum):
    NORMAL = "normal"
    LOADING = "loading"
    FAILURE = "failure"
    DEGRADED = "degraded"
    RECOVERED = "recovered"
    EMPTY = "empty"
    CANCELLED = "cancelled"
    QUEUE_SATURATED = "queue_saturated"


class Phase7LoadState(StrEnum):
    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    EMPTY = "empty"
    FAILED = "failure"
    DEGRADED = "degraded"
    RECOVERED = "recovered"
    CANCELLED = "cancelled"
    BUSY = "queue_saturated"


class SourceKind(StrEnum):
    CSV = "csv"
    PARQUET = "parquet"


class ImportMode(StrEnum):
    APPEND = "append"
    REPLACE_RANGE = "replace_range"


class ImportState(StrEnum):
    IDLE = "idle"
    QUEUED = "queued"
    VALIDATING = "validating"
    IMPORTING = "importing"
    READY = "ready"
    REJECTED = "rejected"
    FAILED = "failed"
    CANCELLED = "cancelled"
    SATURATED = "saturated"


class AdjustmentPolicy(StrEnum):
    SPLIT_AND_DIVIDEND = "split_and_dividend"
    RAW = "raw"


@dataclass(frozen=True, slots=True)
class UtcRange:
    start: datetime
    end: datetime

    def __post_init__(self) -> None:
        if self.start.tzinfo is None or self.end.tzinfo is None:
            raise ValueError("range timestamps must be timezone-aware")
        if self.start.utcoffset() != timedelta(0) or self.end.utcoffset() != timedelta(0):
            raise ValueError("range timestamps must be UTC")
        if self.start >= self.end:
            raise ValueError("range start must precede end")


@dataclass(frozen=True, slots=True)
class Phase7Request:
    request_id: str
    generation: int
    workspace: Phase7Workspace
    scenario: Phase7Scenario = Phase7Scenario.NORMAL

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("Phase 7 generation must be positive")


@dataclass(frozen=True, slots=True)
class DatasetCatalogEntry:
    dataset_id: str
    name: str
    revision: int
    content_fingerprint: str
    implementation_fingerprint: str
    symbols: tuple[str, ...]
    interval: str
    coverage: UtcRange
    row_count: int
    adjustment: AdjustmentPolicy
    status: str = "ready"

    def __post_init__(self) -> None:
        for value in (
            self.dataset_id,
            self.name,
            self.content_fingerprint,
            self.implementation_fingerprint,
        ):
            _identity(value)
        if self.revision < 1 or self.row_count < 0 or not self.symbols:
            raise ValueError("catalog revision, rows, and symbols must be valid")
        if tuple(sorted(set(self.symbols))) != self.symbols:
            raise ValueError("catalog symbols must be unique and sorted")
        if self.interval not in {"1d", "1h"}:
            raise ValueError("catalog interval is unsupported")


@dataclass(frozen=True, slots=True)
class ValidationDiagnostic:
    code: str
    message: str
    field: str | None = None

    def __post_init__(self) -> None:
        _identity(self.code)
        _identity(self.message)
        if self.field == "":
            raise ValueError("diagnostic field cannot be empty")


@dataclass(frozen=True, slots=True)
class ImportRequest:
    command_id: str
    correlation_id: str
    generation: int
    dataset_id: str
    expected_revision: int
    source_kind: SourceKind
    source_name: str
    symbols: tuple[str, ...]
    interval: str
    adjustment: AdjustmentPolicy
    mode: ImportMode
    replacement_range: UtcRange | None = None
    scenario: str = "normal"

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id, self.dataset_id, self.source_name):
            _identity(value)
        if self.generation < 1 or self.expected_revision < 1:
            raise ValueError("import generation and expected revision must be positive")
        if not self.symbols or tuple(sorted(set(self.symbols))) != self.symbols:
            raise ValueError("import symbols must be unique and sorted")
        if self.interval not in {"1d", "1h"}:
            raise ValueError("import interval is unsupported")
        if (self.mode is ImportMode.REPLACE_RANGE) != (self.replacement_range is not None):
            raise ValueError("replacement bounds are required only for range replacement")
        if self.scenario not in {"normal", "empty", "duplicate", "malformed", "failure"}:
            raise ValueError("unknown import scenario")

    @property
    def request_id(self) -> str:
        return self.correlation_id


@dataclass(frozen=True, slots=True)
class ImportResult:
    request: ImportRequest
    state: ImportState
    catalog_entry: DatasetCatalogEntry
    diagnostics: tuple[ValidationDiagnostic, ...]
    imported_rows: int

    def __post_init__(self) -> None:
        if self.imported_rows < 0:
            raise ValueError("imported rows cannot be negative")
        accepted = self.state is ImportState.READY
        if accepted and self.catalog_entry.revision != self.request.expected_revision + 1:
            raise ValueError("accepted import must publish exactly one revision")


@dataclass(frozen=True, slots=True)
class FeatureIdentity:
    name: str
    semantic_version: str
    lookback: int
    output_schema: str
    fingerprint: str

    def __post_init__(self) -> None:
        for value in (self.name, self.semantic_version, self.output_schema, self.fingerprint):
            _identity(value)
        if self.lookback < 1:
            raise ValueError("feature lookback must be positive")


@dataclass(frozen=True, slots=True)
class ExperimentDraft:
    draft_id: str
    revision: int
    dataset_id: str
    dataset_revision: int
    dataset_fingerprint: str
    features: tuple[FeatureIdentity, ...]
    target_horizon: int
    train_bars: int
    validation_bars: int
    test_bars: int
    purge_bars: int
    model_kind: str
    max_depth: int
    estimator_count: int
    initial_capital: float
    fee_bps: float
    sweep_values: tuple[int, ...] = ()
    cpu_limit: int = 1

    def __post_init__(self) -> None:
        for value in (self.draft_id, self.dataset_id, self.dataset_fingerprint, self.model_kind):
            _identity(value)
        if self.revision < 1 or self.dataset_revision < 1:
            raise ValueError("draft and dataset revisions must be positive")
        if not math.isfinite(self.initial_capital) or not math.isfinite(self.fee_bps):
            raise ValueError("portfolio values must be finite")


@dataclass(frozen=True, slots=True)
class ResourceEstimate:
    rows: int
    feature_bytes: int
    peak_bytes: int
    cpu_seconds: float

    def __post_init__(self) -> None:
        if min(self.rows, self.feature_bytes, self.peak_bytes) < 0 or self.cpu_seconds < 0:
            raise ValueError("resource estimate values cannot be negative")


@dataclass(frozen=True, slots=True)
class DraftEvaluation:
    request_id: str
    generation: int
    draft: ExperimentDraft
    diagnostics: tuple[ValidationDiagnostic, ...]
    estimate: ResourceEstimate

    @property
    def valid(self) -> bool:
        return not self.diagnostics


@dataclass(frozen=True, slots=True)
class DraftSaveRequest:
    request_id: str
    generation: int
    draft: ExperimentDraft
    expected_saved_revision: int

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1 or self.expected_saved_revision < 0:
            raise ValueError("draft save identity is invalid")


@dataclass(frozen=True, slots=True)
class DraftSaveResult:
    request: DraftSaveRequest
    saved_revision: int


@dataclass(frozen=True, slots=True)
class SubmissionRequest:
    request_id: str
    command_id: str
    generation: int
    draft: ExperimentDraft

    def __post_init__(self) -> None:
        _identity(self.request_id)
        _identity(self.command_id)
        if self.generation < 1:
            raise ValueError("submission generation must be positive")


@dataclass(frozen=True, slots=True)
class ExperimentDefinition:
    experiment_id: str
    command_id: str
    accepted_at: datetime
    draft: ExperimentDraft

    def __post_init__(self) -> None:
        _identity(self.experiment_id)
        _identity(self.command_id)
        if self.accepted_at.tzinfo is None or self.accepted_at.utcoffset() != timedelta(0):
            raise ValueError("submission time must be UTC")


@dataclass(frozen=True, slots=True)
class Phase7Snapshot:
    request: Phase7Request
    catalog: tuple[DatasetCatalogEntry, ...]
    imports: tuple[ImportResult, ...]
    draft: ExperimentDraft
    evaluation: DraftEvaluation
    experiments: tuple[ExperimentDefinition, ...]
    replay_seed: int
    replay_clock: datetime
    degraded: bool = False

    def __post_init__(self) -> None:
        identities = tuple(item.dataset_id for item in self.catalog)
        if len(identities) != len(set(identities)):
            raise ValueError("catalog entries must have unique identities")
        if self.replay_clock.tzinfo is None or self.replay_clock.utcoffset() != timedelta(0):
            raise ValueError("Phase 7 replay clock must be UTC")


@dataclass(frozen=True, slots=True)
class Phase7WorkspaceState:
    generation: int = 0
    state: Phase7LoadState = Phase7LoadState.IDLE
    scenario: Phase7Scenario = Phase7Scenario.NORMAL
    active_request: Phase7Request | None = None
    snapshot: Phase7Snapshot | None = None
    draft: ExperimentDraft | None = None
    evaluation: DraftEvaluation | None = None
    saved_revision: int = 0
    selected_dataset_id: str | None = None
    selected_import_id: str | None = None
    error: str | None = None
    stale: bool = False


@dataclass(frozen=True, slots=True)
class DataExperimentsState:
    data: Phase7WorkspaceState = Phase7WorkspaceState()
    experiments: Phase7WorkspaceState = Phase7WorkspaceState()

    def workspace(self, value: Phase7Workspace) -> Phase7WorkspaceState:
        return self.data if value is Phase7Workspace.DATA else self.experiments


def _identity(value: str) -> None:
    if not value or value.strip() != value:
        raise ValueError("identity values must be non-empty and normalized")
