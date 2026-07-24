"""Immutable typed values for Phase 7 Data and Experiments workflows."""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from enum import StrEnum
from pathlib import Path

from corthena.ui.datasets.models import (
    DatasetBinding,
    DatasetBuild,
    DatasetDefinition,
    DatasetVersion,
    FeatureStep,
    SourceDefinition,
    SourceSnapshot,
)


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


class FileBrowserEntryKind(StrEnum):
    PARENT = "parent"
    FOLDER = "folder"
    FILE = "file"


class DataIngestionView(StrEnum):
    CATALOG = "catalog"
    NEW_DATASET = "new_dataset"
    FILE_IMPORT = "file_import"
    FILE_BROWSER = "file_browser"
    MASSIVE_PULL = "massive_pull"
    SCHEDULES = "schedules"
    API_TOKENS = "api_tokens"


class DatasetWizardStep(StrEnum):
    SELECT_SOURCE = "select_source"
    MAP_SCHEMA = "map_schema"
    SOURCE_SELECTION = "source_selection"
    FEATURES = "features"
    REVIEW = "review"
    BUILD = "build"


class DataIngestionFixture(StrEnum):
    API_TOKENS = "api_tokens"
    FILE_BROWSER = "file_browser"
    FILE_MAPPING = "file_mapping"
    MASSIVE_PULL = "massive_pull"
    SCHEDULE_EDITING = "schedule_editing"
    ACTIVE_PROGRESS = "active_progress"
    VALIDATION_ERROR = "validation_error"
    AUTHENTICATION_ERROR = "authentication_error"
    RECOVERED = "recovered"


class IngestionStatus(StrEnum):
    IDLE = "idle"
    LOADING = "loading"
    PREVIEW = "preview"
    READY = "ready"
    CONFIRMING = "confirming"
    QUEUED = "queued"
    RUNNING = "running"
    CANCELLING = "cancelling"
    CANCELLED = "cancelled"
    VALIDATING = "validating"
    VALIDATION_FAILED = "validation_failed"
    AUTHENTICATION_FAILED = "authentication_failed"
    ENTITLEMENT_FAILED = "entitlement_failed"
    RATE_LIMITED = "rate_limited"
    FAILED = "failed"
    RECONNECTING = "reconnecting"
    RECONCILING = "reconciling"
    RECOVERED = "recovered"
    EMPTY = "empty"
    COMPLETE = "complete"
    SATURATED = "saturated"


class IngestionScenario(StrEnum):
    SUCCESS = "success"
    VALIDATION_FAILURE = "validation_failure"
    AUTHENTICATION_FAILURE = "authentication_failure"
    ENTITLEMENT_FAILURE = "entitlement_failure"
    RATE_LIMIT = "rate_limit"
    FAILURE = "failure"
    EMPTY = "empty"
    RECOVERY = "recovery"


class SessionPolicy(StrEnum):
    REGULAR = "regular"
    ALL = "all"


class ScheduleCadence(StrEnum):
    HOURLY = "hourly"
    DAILY = "daily"


class ScheduleCommandKind(StrEnum):
    CREATE = "create"
    UPDATE = "update"
    SET_ENABLED = "set_enabled"
    RUN_NOW = "run_now"
    DELETE = "delete"


class ImportMode(StrEnum):
    CREATE = "create"
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
    PROVIDER_SPLIT_ADJUSTED = "provider_split_adjusted"
    RAW = "raw"


@dataclass(frozen=True, slots=True)
class CredentialStatus:
    provider: str
    saved: bool
    last_tested_at: datetime | None = None
    last_test_succeeded: bool | None = None
    safe_detail: str | None = None

    def __post_init__(self) -> None:
        _identity(self.provider)
        if self.last_tested_at is not None and (
            self.last_tested_at.tzinfo is None or self.last_tested_at.utcoffset() != timedelta(0)
        ):
            raise ValueError("credential test time must be UTC")
        if self.safe_detail is not None and not self.safe_detail:
            raise ValueError("credential test detail cannot be empty")


@dataclass(frozen=True, slots=True)
class CredentialRequest:
    request_id: str
    command_id: str
    generation: int
    provider: str = "massive"

    def __post_init__(self) -> None:
        _identity(self.request_id)
        _identity(self.command_id)
        _identity(self.provider)
        if self.generation < 1:
            raise ValueError("credential generation must be positive")


@dataclass(frozen=True, slots=True)
class CredentialSecretRequest:
    request: CredentialRequest
    secret: str = field(repr=False)

    def __post_init__(self) -> None:
        if not self.secret or self.secret.strip() != self.secret:
            raise ValueError("credential token must be non-empty and normalized")


@dataclass(frozen=True, slots=True)
class CredentialResult:
    request: CredentialRequest
    status: CredentialStatus
    outcome: IngestionStatus

    def __post_init__(self) -> None:
        if self.outcome not in {
            IngestionStatus.COMPLETE,
            IngestionStatus.AUTHENTICATION_FAILED,
            IngestionStatus.FAILED,
        }:
            raise ValueError("credential outcome is unsupported")


@dataclass(frozen=True, slots=True)
class ColumnMapping:
    source_column: str
    role: str
    source_type: str
    detected: bool = True

    def __post_init__(self) -> None:
        _identity(self.source_column)
        _identity(self.source_type)
        if self.role not in {
            "timestamp",
            "symbol",
            "open",
            "high",
            "low",
            "close",
            "volume",
            "ignore",
        }:
            raise ValueError("unsupported column role")


@dataclass(frozen=True, slots=True)
class FilePreviewRequest:
    request_id: str
    generation: int
    source_name: str
    source_kind: SourceKind
    scenario: IngestionScenario = IngestionScenario.SUCCESS
    max_rows: int = 50
    max_bytes: int = 1_048_576

    def __post_init__(self) -> None:
        _identity(self.request_id)
        _identity(self.source_name)
        if self.generation < 1:
            raise ValueError("file preview generation must be positive")
        if not 1 <= self.max_rows <= 500 or not 1024 <= self.max_bytes <= 16_777_216:
            raise ValueError("file preview bounds are invalid")


@dataclass(frozen=True, slots=True)
class FileBrowserRequest:
    request_id: str
    generation: int
    source_kind: SourceKind | None
    scenario: IngestionScenario = IngestionScenario.SUCCESS
    location: str | None = None
    navigation_revision: int = 1
    cursor: str | None = None
    page_size: int = 256

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("file browser generation must be positive")
        if self.location is not None:
            _absolute_path(self.location)
        if self.navigation_revision < 1:
            raise ValueError("file browser navigation revision must be positive")
        if self.cursor is not None and (not self.cursor.isdecimal() or int(self.cursor) < 1):
            raise ValueError("file browser cursor must be a positive decimal offset")
        if not 1 <= self.page_size <= 512:
            raise ValueError("file browser page size is invalid")


@dataclass(frozen=True, slots=True)
class FileBrowserEntry:
    source_name: str
    display_name: str
    source_kind: SourceKind | None
    kind: FileBrowserEntryKind = FileBrowserEntryKind.FILE

    def __post_init__(self) -> None:
        _absolute_path(self.source_name)
        _identity(self.display_name)
        if self.kind is FileBrowserEntryKind.FILE:
            if self.source_kind is None:
                raise ValueError("file browser file entries require a source kind")
            expected_suffix = f".{self.source_kind.value}"
            if not self.source_name.casefold().endswith(expected_suffix):
                raise ValueError("browser entry extension does not match its source kind")
        elif self.source_kind is not None:
            raise ValueError("file browser directory entries cannot have a source kind")
        if self.kind is FileBrowserEntryKind.PARENT and self.display_name != "..":
            raise ValueError("file browser parent entry must use the '..' label")

    @property
    def sort_key(self) -> tuple[int, str, str]:
        rank = {
            FileBrowserEntryKind.PARENT: 0,
            FileBrowserEntryKind.FOLDER: 1,
            FileBrowserEntryKind.FILE: 2,
        }[self.kind]
        return rank, self.display_name.casefold(), self.source_name.casefold()


@dataclass(frozen=True, slots=True)
class FileBrowserListing:
    request: FileBrowserRequest
    location: str
    entries: tuple[FileBrowserEntry, ...]
    parent_location: str | None = None
    next_cursor: str | None = None

    def __post_init__(self) -> None:
        _absolute_path(self.location)
        if self.parent_location is not None:
            _absolute_path(self.parent_location)
        if len(self.entries) > self.request.page_size:
            raise ValueError("file browser page exceeds its bound")
        if self.entries != tuple(sorted(self.entries, key=lambda entry: entry.sort_key)):
            raise ValueError("file browser entries must be stably sorted")
        names = tuple(entry.source_name.casefold() for entry in self.entries)
        if len(names) != len(set(names)):
            raise ValueError("file browser entries must be unique")
        if any(
            entry.kind is FileBrowserEntryKind.FILE
            and self.request.source_kind is not None
            and entry.source_kind is not self.request.source_kind
            for entry in self.entries
        ):
            raise ValueError("file browser entries must match the requested source kind")
        parent_entries = tuple(
            entry for entry in self.entries if entry.kind is FileBrowserEntryKind.PARENT
        )
        if len(parent_entries) > 1 or (parent_entries and self.request.cursor is not None):
            raise ValueError("file browser parent entry may appear only on the first page")
        has_parent_metadata = self.parent_location is not None and self.request.cursor is None
        if bool(parent_entries) != has_parent_metadata:
            raise ValueError("file browser parent metadata and row must agree")
        if self.next_cursor is not None and (
            not self.next_cursor.isdecimal() or int(self.next_cursor) < 1
        ):
            raise ValueError("file browser next cursor must be a positive decimal offset")


@dataclass(frozen=True, slots=True)
class FilePreview:
    request: FilePreviewRequest
    columns: tuple[ColumnMapping, ...]
    representative_rows: int
    diagnostics: tuple[ValidationDiagnostic, ...] = ()

    def __post_init__(self) -> None:
        if self.representative_rows < 0 or len(self.columns) > 64:
            raise ValueError("file preview bounds are invalid")
        source_columns = tuple(item.source_column for item in self.columns)
        if len(source_columns) != len(set(source_columns)):
            raise ValueError("file preview columns must be unique")


@dataclass(frozen=True, slots=True)
class SymbolDiscoveryRequest:
    request_id: str
    generation: int
    query: str
    page: int = 1
    page_size: int = 20
    scenario: IngestionScenario = IngestionScenario.SUCCESS

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1 or self.page < 1 or not 1 <= self.page_size <= 100:
            raise ValueError("symbol discovery bounds are invalid")


@dataclass(frozen=True, slots=True, order=True)
class StockSymbol:
    symbol: str
    name: str

    def __post_init__(self) -> None:
        _identity(self.symbol)
        _identity(self.name)


@dataclass(frozen=True, slots=True)
class SymbolDiscoveryResult:
    request: SymbolDiscoveryRequest
    symbols: tuple[StockSymbol, ...]
    has_more: bool

    def __post_init__(self) -> None:
        if tuple(sorted(set(self.symbols))) != self.symbols:
            raise ValueError("discovered symbols must be unique and sorted")


@dataclass(frozen=True, slots=True)
class IngestionPlan:
    command_id: str
    correlation_id: str
    generation: int
    dataset_id: str
    expected_revision: int
    symbols: tuple[str, ...]
    interval: str
    requested_range: UtcRange
    session: SessionPolicy
    adjustment: AdjustmentPolicy
    mode: ImportMode
    scenario: IngestionScenario = IngestionScenario.SUCCESS
    source_timezone: str = "UTC"
    source_path: str | None = None
    mapping: tuple[ColumnMapping, ...] = ()
    dataset_name: str | None = None

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id, self.dataset_id):
            _identity(value)
        if self.generation < 1 or self.expected_revision < 0:
            raise ValueError("ingestion identity is invalid")
        if not self.symbols or tuple(sorted(set(self.symbols))) != self.symbols:
            raise ValueError("ingestion symbols must be unique and sorted")
        if self.interval not in {"1m", "5m", "15m", "1h", "1d"}:
            raise ValueError("ingestion interval is unsupported")
        if self.adjustment is AdjustmentPolicy.SPLIT_AND_DIVIDEND:
            raise ValueError("Massive ingestion is not dividend-adjusted")
        _identity(self.source_timezone)
        if self.source_path == "":
            raise ValueError("source path cannot be empty")
        if self.dataset_name == "":
            raise ValueError("dataset name cannot be empty")

    @property
    def request_id(self) -> str:
        return self.correlation_id


@dataclass(frozen=True, slots=True)
class IngestionProgress:
    request_id: str
    generation: int
    status: IngestionStatus
    completed_units: int
    total_units: int
    message: str
    provider_request_id: str | None = None
    retry_after_seconds: int | None = None

    def __post_init__(self) -> None:
        _identity(self.request_id)
        _identity(self.message)
        if self.generation < 1 or not 0 <= self.completed_units <= self.total_units:
            raise ValueError("progress bounds are invalid")
        if self.retry_after_seconds is not None and self.retry_after_seconds < 0:
            raise ValueError("retry delay cannot be negative")


@dataclass(frozen=True, slots=True)
class IngestionResult:
    plan: IngestionPlan
    progress: IngestionProgress
    catalog_entry: DatasetCatalogEntry | None
    diagnostics: tuple[ValidationDiagnostic, ...] = ()

    def __post_init__(self) -> None:
        if (
            self.progress.request_id != self.plan.request_id
            or self.progress.generation != self.plan.generation
        ):
            raise ValueError("ingestion result identity does not match its plan")


@dataclass(frozen=True, slots=True)
class DataSchedule:
    schedule_id: str
    revision: int
    name: str
    dataset_id: str
    symbols: tuple[str, ...]
    interval: str
    session: SessionPolicy
    adjustment: AdjustmentPolicy
    cadence: ScheduleCadence
    enabled: bool
    last_run_at: datetime | None = None
    next_run_at: datetime | None = None
    range_anchor: datetime | None = None

    def __post_init__(self) -> None:
        for value in (self.schedule_id, self.name, self.dataset_id):
            _identity(value)
        if (
            self.revision < 1
            or not self.symbols
            or tuple(sorted(set(self.symbols))) != self.symbols
        ):
            raise ValueError("schedule identity is invalid")
        if self.interval not in {"1m", "5m", "15m", "1h", "1d"}:
            raise ValueError("schedule interval is unsupported")
        if self.adjustment is AdjustmentPolicy.SPLIT_AND_DIVIDEND:
            raise ValueError("Massive schedules are not dividend-adjusted")
        for timestamp in (self.last_run_at, self.next_run_at, self.range_anchor):
            if timestamp is not None and (
                timestamp.tzinfo is None or timestamp.utcoffset() != timedelta(0)
            ):
                raise ValueError("schedule timestamps must be UTC")


@dataclass(frozen=True, slots=True)
class ScheduleCommand:
    request_id: str
    command_id: str
    generation: int
    kind: ScheduleCommandKind
    schedule: DataSchedule
    expected_revision: int

    def __post_init__(self) -> None:
        _identity(self.request_id)
        _identity(self.command_id)
        if self.generation < 1 or self.expected_revision < 0:
            raise ValueError("schedule command identity is invalid")


@dataclass(frozen=True, slots=True)
class ScheduleResult:
    command: ScheduleCommand
    schedule: DataSchedule | None
    schedules: tuple[DataSchedule, ...]
    progress: IngestionProgress | None = None


@dataclass(frozen=True, slots=True)
class ReconciliationRequest:
    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("reconciliation generation must be positive")


@dataclass(frozen=True, slots=True)
class ReconciliationResult:
    request: ReconciliationRequest
    catalog: tuple[DatasetCatalogEntry, ...]
    schedules: tuple[DataSchedule, ...]
    recovered_request_ids: tuple[str, ...]


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
        if self.interval not in {"1m", "5m", "15m", "1h", "1d"}:
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
        if self.interval not in {"1m", "5m", "15m", "1h", "1d"}:
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
    dataset_binding: DatasetBinding | None = None

    def __post_init__(self) -> None:
        for value in (self.draft_id, self.dataset_id, self.dataset_fingerprint, self.model_kind):
            _identity(value)
        if self.revision < 1 or self.dataset_revision < 1:
            raise ValueError("draft and dataset revisions must be positive")
        if not math.isfinite(self.initial_capital) or not math.isfinite(self.fee_bps):
            raise ValueError("portfolio values must be finite")
        if self.dataset_binding is not None and (
            self.dataset_binding.dataset_id != self.dataset_id
            or self.dataset_binding.dataset_version != self.dataset_revision
            or self.dataset_binding.build_fingerprint != self.dataset_fingerprint
        ):
            raise ValueError("experiment dataset binding disagrees with legacy dataset identity")


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
    credential: CredentialStatus = field(default_factory=lambda: CredentialStatus("massive", False))
    schedules: tuple[DataSchedule, ...] = ()
    sources: tuple[SourceDefinition, ...] = ()
    source_snapshots: tuple[SourceSnapshot, ...] = ()
    dataset_definitions: tuple[DatasetDefinition, ...] = ()
    dataset_versions: tuple[DatasetVersion, ...] = ()
    dataset_builds: tuple[DatasetBuild, ...] = ()

    def __post_init__(self) -> None:
        identities = tuple(item.dataset_id for item in self.catalog)
        if len(identities) != len(set(identities)):
            raise ValueError("catalog entries must have unique identities")
        if self.replay_clock.tzinfo is None or self.replay_clock.utcoffset() != timedelta(0):
            raise ValueError("Phase 7 replay clock must be UTC")
        if len({item.source_id for item in self.sources}) != len(self.sources):
            raise ValueError("source definitions must have unique identities")
        if len({item.dataset_id for item in self.dataset_definitions}) != len(
            self.dataset_definitions
        ):
            raise ValueError("dataset definitions must have unique identities")


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
    ingestion_view: DataIngestionView = DataIngestionView.CATALOG
    ingestion_status: IngestionStatus = IngestionStatus.IDLE
    ingestion_scenario: IngestionScenario = IngestionScenario.SUCCESS
    credential: CredentialStatus = field(default_factory=lambda: CredentialStatus("massive", False))
    file_browser: FileBrowserListing | None = None
    file_browser_entries: tuple[FileBrowserEntry, ...] = ()
    file_browser_origin: DataIngestionView | None = None
    selected_file_browser_path: str | None = None
    file_browser_navigation_revision: int = 0
    file_browser_scroll_row: int = 0
    file_browser_loading_page: bool = False
    file_preview: FilePreview | None = None
    discovered_symbols: tuple[StockSymbol, ...] = ()
    selected_symbols: tuple[str, ...] = ()
    active_ingestion_id: str | None = None
    progress: IngestionProgress | None = None
    schedules: tuple[DataSchedule, ...] = ()
    selected_schedule_id: str | None = None
    form_source_kind: SourceKind = SourceKind.CSV
    form_interval: str = "1d"
    form_session: SessionPolicy = SessionPolicy.REGULAR
    form_adjustment: AdjustmentPolicy = AdjustmentPolicy.RAW
    form_mode: ImportMode = ImportMode.APPEND
    form_source_timezone: str = "UTC"
    sources: tuple[SourceDefinition, ...] = ()
    source_snapshots: tuple[SourceSnapshot, ...] = ()
    dataset_definitions: tuple[DatasetDefinition, ...] = ()
    dataset_versions: tuple[DatasetVersion, ...] = ()
    dataset_builds: tuple[DatasetBuild, ...] = ()
    selected_source_id: str | None = None
    active_dataset_build_id: str | None = None
    dataset_wizard_step: DatasetWizardStep = DatasetWizardStep.SELECT_SOURCE
    dataset_recipe_steps: tuple[FeatureStep, ...] = ()
    last_successful_dataset_build: DatasetBuild | None = None


@dataclass(frozen=True, slots=True)
class DataExperimentsState:
    data: Phase7WorkspaceState = field(default_factory=Phase7WorkspaceState)
    experiments: Phase7WorkspaceState = field(default_factory=Phase7WorkspaceState)

    def workspace(self, value: Phase7Workspace) -> Phase7WorkspaceState:
        return self.data if value is Phase7Workspace.DATA else self.experiments


def _identity(value: str) -> None:
    if not value or value.strip() != value:
        raise ValueError("identity values must be non-empty and normalized")


def _absolute_path(value: str) -> None:
    _identity(value)
    if not Path(value).is_absolute():
        raise ValueError("file browser paths must be absolute")
