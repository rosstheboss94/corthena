"""Immutable source, dataset recipe, build, and binding values."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum


class SourceFamily(StrEnum):
    MARKET_BARS = "market_bars"
    OPTIONS = "options"


class SourceProvider(StrEnum):
    EXISTING = "existing"
    CSV = "csv"
    PARQUET = "parquet"
    MASSIVE = "massive"


class ColumnType(StrEnum):
    TIMESTAMP = "timestamp"
    SYMBOL = "symbol"
    FLOAT64 = "float64"


class FeatureCategory(StrEnum):
    RETURNS = "returns"
    ROLLING = "rolling"
    RATIO = "ratio"
    VOLATILITY = "volatility"
    CROSS_SECTIONAL = "cross_sectional"


class RollingStatistic(StrEnum):
    MEAN = "mean"
    MINIMUM = "minimum"
    MAXIMUM = "maximum"
    STANDARD_DEVIATION = "standard_deviation"


class CrossSectionalMethod(StrEnum):
    RANK = "rank"
    Z_SCORE = "z_score"


class DatasetBuildState(StrEnum):
    QUEUED = "queued"
    BUILDING = "building"
    READY = "ready"
    FAILED = "failed"
    CANCELLED = "cancelled"
    SATURATED = "saturated"
    STALE = "stale"


@dataclass(frozen=True, slots=True)
class SourceColumn:
    name: str
    type: ColumnType
    nullable: bool = False

    def __post_init__(self) -> None:
        _identity(self.name)


@dataclass(frozen=True, slots=True)
class SourceSelection:
    symbols: tuple[str, ...]
    interval: str
    range_start: datetime
    range_end: datetime
    session: str
    adjustment: str

    def __post_init__(self) -> None:
        if not self.symbols or tuple(sorted(set(self.symbols))) != self.symbols:
            raise ValueError("source symbols must be unique and sorted")
        if self.interval not in {"1m", "5m", "15m", "1h", "1d"}:
            raise ValueError("source interval is unsupported")
        if self.session not in {"regular", "all"}:
            raise ValueError("source session is unsupported")
        if self.adjustment not in {"raw", "provider_split_adjusted", "split_and_dividend"}:
            raise ValueError("source adjustment is unsupported")
        _utc_range(self.range_start, self.range_end)


@dataclass(frozen=True, slots=True)
class SourceDefinition:
    source_id: str
    revision: int
    name: str
    family: SourceFamily
    provider: SourceProvider
    schema: tuple[SourceColumn, ...]
    mapping_fingerprint: str

    def __post_init__(self) -> None:
        for value in (self.source_id, self.name, self.mapping_fingerprint):
            _identity(value)
        if self.revision < 1:
            raise ValueError("source definition revision must be positive")
        names = tuple(column.name for column in self.schema)
        if len(names) != len(set(names)) or not names:
            raise ValueError("source schema columns must be non-empty and unique")
        if self.family is SourceFamily.MARKET_BARS:
            _validate_market_bar_schema(self.schema)


@dataclass(frozen=True, slots=True)
class SourceProvenance:
    acquired_at: datetime
    command_id: str
    correlation_id: str
    provider_request_id: str | None = None
    parent_snapshot_id: str | None = None

    def __post_init__(self) -> None:
        _utc(self.acquired_at)
        for value in (self.command_id, self.correlation_id):
            _identity(value)
        if self.provider_request_id is not None:
            _identity(self.provider_request_id)
        if self.parent_snapshot_id is not None:
            _identity(self.parent_snapshot_id)


@dataclass(frozen=True, slots=True)
class SourceSnapshot:
    snapshot_id: str
    source_id: str
    source_revision: int
    snapshot_revision: int
    content_fingerprint: str
    selection: SourceSelection
    row_count: int
    provenance: SourceProvenance

    def __post_init__(self) -> None:
        for value in (self.snapshot_id, self.source_id, self.content_fingerprint):
            _identity(value)
        if min(self.source_revision, self.snapshot_revision) < 1 or self.row_count < 0:
            raise ValueError("source snapshot revisions and rows are invalid")


@dataclass(frozen=True, slots=True)
class LaggedReturnStep:
    output_name: str
    input_column: str = "close"
    periods: int = 1
    log_return: bool = False
    kind: str = "lagged_return"


@dataclass(frozen=True, slots=True)
class RollingStatisticStep:
    output_name: str
    input_column: str
    window: int
    statistic: RollingStatistic
    kind: str = "rolling_statistic"


@dataclass(frozen=True, slots=True)
class PriceVolumeRatioStep:
    output_name: str
    price_column: str = "close"
    volume_column: str = "volume"
    kind: str = "price_volume_ratio"


@dataclass(frozen=True, slots=True)
class RollingVolatilityStep:
    output_name: str
    input_column: str = "close"
    window: int = 20
    kind: str = "rolling_volatility"


@dataclass(frozen=True, slots=True)
class RollingRangeStep:
    output_name: str
    high_column: str = "high"
    low_column: str = "low"
    window: int = 20
    kind: str = "rolling_range"


@dataclass(frozen=True, slots=True)
class CrossSectionalStep:
    output_name: str
    input_column: str
    method: CrossSectionalMethod
    timestamp_column: str = "timestamp"
    symbol_column: str = "symbol"
    kind: str = "cross_sectional"


FeatureStep = (
    LaggedReturnStep
    | RollingStatisticStep
    | PriceVolumeRatioStep
    | RollingVolatilityStep
    | RollingRangeStep
    | CrossSectionalStep
)


@dataclass(frozen=True, slots=True)
class DatasetDefinition:
    dataset_id: str
    name: str
    latest_version: int
    revision: int

    def __post_init__(self) -> None:
        _identity(self.dataset_id)
        _identity(self.name)
        if min(self.latest_version, self.revision) < 1:
            raise ValueError("dataset definition revisions must be positive")


@dataclass(frozen=True, slots=True)
class DatasetVersion:
    dataset_id: str
    version: int
    source_id: str
    source_revision: int
    selection: SourceSelection
    steps: tuple[FeatureStep, ...]
    registry_version: str
    implementation_fingerprint: str
    recipe_fingerprint: str

    def __post_init__(self) -> None:
        for value in (
            self.dataset_id,
            self.source_id,
            self.registry_version,
            self.implementation_fingerprint,
            self.recipe_fingerprint,
        ):
            _identity(value)
        if min(self.version, self.source_revision) < 1:
            raise ValueError("dataset version identities must be positive")


@dataclass(frozen=True, slots=True)
class EngineeredColumn:
    name: str
    type: ColumnType
    category: FeatureCategory
    lookback: int
    implementation_fingerprint: str
    inputs: tuple[str, ...]

    def __post_init__(self) -> None:
        _identity(self.name)
        _identity(self.implementation_fingerprint)
        if self.lookback < 0 or not self.inputs:
            raise ValueError("engineered column metadata is invalid")


@dataclass(frozen=True, slots=True)
class DatasetDiagnostic:
    code: str
    message: str
    field: str | None = None
    step_index: int | None = None

    def __post_init__(self) -> None:
        _identity(self.code)
        _identity(self.message)
        if self.field == "" or (self.step_index is not None and self.step_index < 0):
            raise ValueError("dataset diagnostic location is invalid")


@dataclass(frozen=True, slots=True)
class DatasetValidation:
    version: DatasetVersion
    columns: tuple[EngineeredColumn, ...]
    diagnostics: tuple[DatasetDiagnostic, ...]
    accumulated_lookback: int

    @property
    def valid(self) -> bool:
        return not self.diagnostics


@dataclass(frozen=True, slots=True)
class DatasetBinding:
    dataset_id: str
    dataset_version: int
    source_snapshots: tuple[str, ...]
    recipe_fingerprint: str
    build_fingerprint: str
    feature_fingerprints: tuple[str, ...]
    feature_columns: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        for value in (self.dataset_id, self.recipe_fingerprint, self.build_fingerprint):
            _identity(value)
        if self.dataset_version < 1 or len(self.source_snapshots) != 1:
            raise ValueError("dataset binding must pin one source snapshot and a positive version")
        _identity(self.source_snapshots[0])
        if len(self.feature_fingerprints) != len(set(self.feature_fingerprints)):
            raise ValueError("dataset binding feature fingerprints must be unique")
        if self.feature_columns and (
            len(self.feature_columns) != len(self.feature_fingerprints)
            or len(self.feature_columns) != len(set(self.feature_columns))
        ):
            raise ValueError("dataset binding feature columns must align with fingerprints")


@dataclass(frozen=True, slots=True)
class DatasetBuild:
    build_id: str
    command_id: str
    correlation_id: str
    generation: int
    version: DatasetVersion
    source_snapshot: SourceSnapshot
    state: DatasetBuildState
    validation: DatasetValidation
    build_fingerprint: str
    preview_rows: int
    completed_at: datetime | None = None

    def __post_init__(self) -> None:
        for value in (
            self.build_id,
            self.command_id,
            self.correlation_id,
            self.build_fingerprint,
        ):
            _identity(value)
        if self.generation < 1 or not 0 <= self.preview_rows <= 10_000:
            raise ValueError("dataset build bounds are invalid")
        if self.source_snapshot.source_id != self.version.source_id:
            raise ValueError("dataset build source does not match the recipe")
        if self.state is DatasetBuildState.READY and not self.validation.valid:
            raise ValueError("a ready dataset build must be valid")
        if self.completed_at is not None:
            _utc(self.completed_at)

    @property
    def binding(self) -> DatasetBinding | None:
        if self.state not in {DatasetBuildState.READY, DatasetBuildState.STALE}:
            return None
        return DatasetBinding(
            self.version.dataset_id,
            self.version.version,
            (self.source_snapshot.snapshot_id,),
            self.version.recipe_fingerprint,
            self.build_fingerprint,
            tuple(column.implementation_fingerprint for column in self.validation.columns),
            tuple(column.name for column in self.validation.columns),
        )


@dataclass(frozen=True, slots=True)
class DatasetSaveRequest:
    command_id: str
    correlation_id: str
    generation: int
    expected_revision: int
    definition: DatasetDefinition
    version: DatasetVersion

    @property
    def request_id(self) -> str:
        return self.correlation_id

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id):
            _identity(value)
        if self.generation < 1 or self.expected_revision < 0:
            raise ValueError("dataset save identity is invalid")
        if self.definition.dataset_id != self.version.dataset_id:
            raise ValueError("dataset definition and version identities must match")


@dataclass(frozen=True, slots=True)
class DatasetSaveResult:
    request: DatasetSaveRequest
    definition: DatasetDefinition
    version: DatasetVersion


@dataclass(frozen=True, slots=True)
class DatasetBuildRequest:
    command_id: str
    correlation_id: str
    generation: int
    version: DatasetVersion
    source_snapshot: SourceSnapshot
    preview_limit: int = 500

    @property
    def request_id(self) -> str:
        return self.correlation_id

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id):
            _identity(value)
        if self.generation < 1 or not 1 <= self.preview_limit <= 10_000:
            raise ValueError("dataset build request bounds are invalid")


@dataclass(frozen=True, slots=True)
class DatasetCancelRequest:
    command_id: str
    correlation_id: str
    generation: int
    build_id: str

    @property
    def request_id(self) -> str:
        return self.correlation_id

    def __post_init__(self) -> None:
        for value in (self.command_id, self.correlation_id, self.build_id):
            _identity(value)
        if self.generation < 1:
            raise ValueError("dataset cancellation generation must be positive")


@dataclass(frozen=True, slots=True)
class DatasetReconciliationRequest:
    correlation_id: str
    generation: int

    @property
    def request_id(self) -> str:
        return self.correlation_id

    def __post_init__(self) -> None:
        _identity(self.correlation_id)
        if self.generation < 1:
            raise ValueError("dataset reconciliation generation must be positive")


@dataclass(frozen=True, slots=True)
class DatasetReconciliation:
    request: DatasetReconciliationRequest
    generation: int
    sources: tuple[SourceDefinition, ...]
    snapshots: tuple[SourceSnapshot, ...]
    definitions: tuple[DatasetDefinition, ...]
    versions: tuple[DatasetVersion, ...]
    builds: tuple[DatasetBuild, ...]

    def __post_init__(self) -> None:
        if self.generation != self.request.generation:
            raise ValueError("dataset reconciliation generation mismatch")


def _identity(value: str) -> None:
    if not value or value.strip() != value:
        raise ValueError("identity values must be non-empty and normalized")


def _utc(value: datetime) -> None:
    if value.tzinfo is None or value.utcoffset() != timedelta(0):
        raise ValueError("timestamp must be UTC")


def _utc_range(start: datetime, end: datetime) -> None:
    _utc(start)
    _utc(end)
    if start >= end:
        raise ValueError("range start must precede end")


def _validate_market_bar_schema(schema: tuple[SourceColumn, ...]) -> None:
    actual = {column.name: column.type for column in schema}
    required = {
        "timestamp": ColumnType.TIMESTAMP,
        "symbol": ColumnType.SYMBOL,
        "open": ColumnType.FLOAT64,
        "high": ColumnType.FLOAT64,
        "low": ColumnType.FLOAT64,
        "close": ColumnType.FLOAT64,
        "volume": ColumnType.FLOAT64,
    }
    if any(actual.get(name) is not type_ for name, type_ in required.items()):
        raise ValueError("market-bars sources require the canonical typed OHLCV schema")
