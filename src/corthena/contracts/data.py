"""Versioned Data API DTOs with strict inbound validation."""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, SecretStr, field_validator, model_validator

API_VERSION = 1
SCHEMA_VERSION = 1
Identifier = Annotated[str, Field(min_length=1, max_length=128, pattern=r"^[A-Za-z0-9._:-]+$")]


class StrictDTO(BaseModel):
    """Base for JSON boundaries that reject unrecognized fields."""

    model_config = ConfigDict(extra="forbid", frozen=True)


class Provider(StrEnum):
    MASSIVE = "massive"


class ProviderIdentityDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    provider: Provider
    connector_version: str = Field(min_length=1, max_length=128)


class SourceKind(StrEnum):
    CSV = "csv"
    PARQUET = "parquet"
    MASSIVE = "massive"


class Interval(StrEnum):
    MINUTE_1 = "1m"
    MINUTE_5 = "5m"
    MINUTE_15 = "15m"
    HOUR_1 = "1h"
    DAY_1 = "1d"


class SessionPolicy(StrEnum):
    REGULAR = "regular"
    ALL = "all"


class AdjustmentPolicy(StrEnum):
    RAW = "raw"
    PROVIDER_SPLIT_ADJUSTED = "provider_split_adjusted"


class ImportMode(StrEnum):
    CREATE = "create"
    APPEND = "append"
    REPLACE_RANGE = "replace_range"


class OperationState(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    CANCELLING = "cancelling"
    CANCELLED = "cancelled"
    FAILED = "failed"
    COMPLETE = "complete"


class ScheduleCadence(StrEnum):
    HOURLY = "hourly"
    DAILY = "daily"


class UtcRangeDTO(StrictDTO):
    start: datetime
    end: datetime

    @model_validator(mode="after")
    def validate_range(self) -> UtcRangeDTO:
        if self.start.tzinfo is None or self.end.tzinfo is None:
            raise ValueError("range timestamps must be timezone-aware")
        start_offset = self.start.utcoffset()
        end_offset = self.end.utcoffset()
        if (
            start_offset is None
            or end_offset is None
            or start_offset.total_seconds() != 0
            or end_offset.total_seconds() != 0
        ):
            raise ValueError("range timestamps must be UTC")
        if self.start >= self.end:
            raise ValueError("range start must precede end")
        return self


class CommandDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    command_id: Identifier
    correlation_id: Identifier


class CredentialSecretDTO(CommandDTO):
    token: SecretStr

    @field_validator("token")
    @classmethod
    def token_is_normalized(cls, value: SecretStr) -> SecretStr:
        token = value.get_secret_value()
        if not token or token.strip() != token or len(token) > 4096:
            raise ValueError("token must be non-empty, normalized, and bounded")
        return value


class CredentialTestDTO(CommandDTO):
    token: SecretStr | None = None


class CredentialStatusDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    provider: Provider = Provider.MASSIVE
    saved: bool
    last_tested_at: datetime | None = None
    last_test_succeeded: bool | None = None
    safe_detail: str | None = Field(default=None, max_length=256)


class ColumnMappingDTO(StrictDTO):
    source_column: str = Field(min_length=1, max_length=256)
    role: Literal["timestamp", "symbol", "open", "high", "low", "close", "volume", "ignore"]
    source_type: str = Field(min_length=1, max_length=128)


class PreviewRequestDTO(CommandDTO):
    path: str = Field(min_length=1, max_length=32767)
    source_kind: Literal[SourceKind.CSV, SourceKind.PARQUET]
    max_rows: int = Field(default=50, ge=1, le=500)
    max_bytes: int = Field(default=1_048_576, ge=1024, le=16_777_216)


class PreviewRowDTO(StrictDTO):
    values: tuple[str | int | float | bool | None, ...]


class FilePreviewDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    path: str
    source_kind: Literal[SourceKind.CSV, SourceKind.PARQUET]
    columns: tuple[ColumnMappingDTO, ...]
    rows: tuple[PreviewRowDTO, ...]
    total_rows_seen: int = Field(ge=0)
    truncated: bool
    diagnostics: tuple[str, ...] = ()


class ImportPlanDTO(CommandDTO):
    generation: int = Field(ge=1)
    source_kind: SourceKind
    source_path: str | None = Field(default=None, max_length=32767)
    mapping: tuple[ColumnMappingDTO, ...]
    source_timezone: str = Field(default="UTC", min_length=1, max_length=128)
    dataset_id: Identifier
    dataset_name: str = Field(min_length=1, max_length=256)
    expected_catalog_revision: int = Field(ge=0)
    symbols: tuple[str, ...] = Field(min_length=1, max_length=10_000)
    interval: Interval
    requested_range: UtcRangeDTO
    session_policy: SessionPolicy
    adjustment_policy: AdjustmentPolicy
    mode: ImportMode

    @field_validator("symbols")
    @classmethod
    def symbols_are_normalized(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        normalized = tuple(sorted({symbol.strip().upper() for symbol in value if symbol.strip()}))
        if normalized != value:
            raise ValueError("symbols must be uppercase, unique, and sorted")
        return value

    @model_validator(mode="after")
    def source_is_valid(self) -> ImportPlanDTO:
        if self.source_kind in {SourceKind.CSV, SourceKind.PARQUET} and not self.source_path:
            raise ValueError("file imports require source_path")
        if self.source_kind is SourceKind.MASSIVE and self.mapping:
            raise ValueError("Massive imports do not use file mappings")
        if self.mode is ImportMode.CREATE and self.expected_catalog_revision != 0:
            raise ValueError("dataset creation expects revision zero")
        return self


class PullRequestDTO(ImportPlanDTO):
    @model_validator(mode="after")
    def validate_pull(self) -> PullRequestDTO:
        if self.source_kind is not SourceKind.MASSIVE:
            raise ValueError("pull request requires the Massive source")
        return self


class OperationAcceptedDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    operation_id: Identifier
    correlation_id: Identifier
    state: OperationState
    reconciliation_path: str


class ProgressDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    operation_id: Identifier
    correlation_id: Identifier
    generation: int = Field(ge=1)
    state: OperationState
    completed_units: int = Field(ge=0)
    total_units: int = Field(ge=0)
    message: str = Field(min_length=1, max_length=512)
    provider_request_id: str | None = Field(default=None, max_length=256)
    retry_after_seconds: float | None = Field(default=None, ge=0, le=3600)


class ProvenanceDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    source_kind: SourceKind
    source_identity: str
    mapping: tuple[ColumnMappingDTO, ...]
    symbols: tuple[str, ...]
    interval: Interval
    session_policy: SessionPolicy
    adjustment_policy: AdjustmentPolicy
    requested_range: UtcRangeDTO
    actual_range: UtcRangeDTO
    command_id: Identifier
    correlation_id: Identifier
    operation_id: Identifier
    dataset_id: Identifier
    parent_revision: int = Field(ge=0)
    started_at: datetime
    completed_at: datetime
    application_version: str
    connector_version: str
    calendar_version: str
    partition_checksums: tuple[tuple[str, str], ...]
    content_fingerprint: str


class CatalogRevisionDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    dataset_id: Identifier
    name: str
    revision: int = Field(ge=1)
    content_fingerprint: str
    symbols: tuple[str, ...]
    interval: Interval
    session_policy: SessionPolicy
    adjustment_policy: AdjustmentPolicy
    coverage: UtcRangeDTO
    row_count: int = Field(ge=0)
    revision_path: str
    provenance: ProvenanceDTO


class CatalogDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    revisions: tuple[CatalogRevisionDTO, ...]


class SymbolDTO(StrictDTO):
    symbol: str
    name: str


class SymbolPageDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    symbols: tuple[SymbolDTO, ...]
    next_cursor: str | None = None
    provider_request_id: str | None = None


class ScheduleDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    schedule_id: Identifier
    revision: int = Field(ge=1)
    name: str = Field(min_length=1, max_length=256)
    dataset_id: Identifier
    symbols: tuple[str, ...]
    interval: Interval
    session_policy: SessionPolicy
    adjustment_policy: AdjustmentPolicy
    cadence: ScheduleCadence
    enabled: bool
    range_anchor: datetime
    last_run_at: datetime | None = None
    next_run_at: datetime | None = None

    @model_validator(mode="after")
    def validate_schedule(self) -> ScheduleDTO:
        normalized = tuple(sorted({symbol.strip().upper() for symbol in self.symbols}))
        if not normalized or normalized != self.symbols:
            raise ValueError("schedule symbols must be uppercase, unique, and sorted")
        for value in (self.range_anchor, self.last_run_at, self.next_run_at):
            if value is None:
                continue
            offset = value.utcoffset()
            if value.tzinfo is None or offset is None:
                raise ValueError("schedule timestamps must be UTC")
            if offset.total_seconds() != 0:
                raise ValueError("schedule timestamps must be UTC")
        return self


class ScheduleCommandDTO(CommandDTO):
    schedule: ScheduleDTO
    expected_revision: int = Field(ge=0)


class SchedulePatchDTO(CommandDTO):
    schedule: ScheduleDTO
    expected_revision: int = Field(ge=1)


class DeleteCommandDTO(CommandDTO):
    expected_revision: int = Field(ge=1)


class ErrorDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    code: str
    message: str
    correlation_id: str
    retryable: bool
    field: str | None = None
    retry_after_seconds: float | None = None
    provider_request_id: str | None = None


class DataEventDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    event_id: Identifier
    sequence: int = Field(ge=1)
    event_type: Literal["import.progress", "catalog.invalidated", "schedule.invalidated"]
    timestamp: datetime
    correlation_id: Identifier | None = None
    operation_id: Identifier | None = None
    dataset_id: Identifier | None = None
    schedule_id: Identifier | None = None
    progress: ProgressDTO | None = None


class ReconciliationDTO(StrictDTO):
    schema_version: Literal[1] = SCHEMA_VERSION
    catalog: CatalogDTO
    schedules: tuple[ScheduleDTO, ...]
    recovered_operation_ids: tuple[Identifier, ...]
    latest_event_sequence: int = Field(ge=0)
