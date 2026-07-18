"""Immutable typed values for Phase 9 Models and Inference workflows."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum


class Phase9Workspace(StrEnum):
    MODELS = "models"
    INFERENCE = "inference"


class Phase9Scenario(StrEnum):
    MODELS_NORMAL = "models_normal"
    MODELS_LOADING = "models_loading"
    MODELS_EMPTY = "models_empty"
    MODELS_FAILURE = "models_failure"
    MODELS_DEGRADED = "models_degraded"
    MODELS_RECOVERED = "models_recovered"
    INFERENCE_NORMAL = "inference_normal"
    INFERENCE_LOADING = "inference_loading"
    INFERENCE_LATEST = "inference_latest"
    INFERENCE_INCOMPATIBLE = "inference_incompatible"
    INFERENCE_EMPTY = "inference_empty"
    INFERENCE_FAILURE = "inference_failure"
    INFERENCE_DEGRADED = "inference_degraded"
    INFERENCE_RECOVERED = "inference_recovered"
    QUEUE_SATURATED = "queue_saturated"


class Phase9LoadState(StrEnum):
    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    EMPTY = "empty"
    FAILED = "failed"
    DEGRADED = "degraded"
    RECOVERED = "recovered"
    CANCELLED = "cancelled"
    BUSY = "queue_saturated"


class ArtifactStatus(StrEnum):
    COMPLETE = "complete"
    INCOMPATIBLE = "incompatible"


class InferenceMode(StrEnum):
    HISTORICAL = "historical"
    LATEST = "latest"


class ExportState(StrEnum):
    IDLE = "idle"
    PREPARING = "preparing"
    READY = "ready"
    FAILED = "failed"


@dataclass(frozen=True, slots=True)
class Phase9Request:
    request_id: str
    generation: int
    workspace: Phase9Workspace
    scenario: Phase9Scenario

    def __post_init__(self) -> None:
        _identity(self.request_id)
        if self.generation < 1:
            raise ValueError("Phase 9 generation must be positive")
        prefix = "models_" if self.workspace is Phase9Workspace.MODELS else "inference_"
        if (
            not self.scenario.value.startswith(prefix)
            and self.scenario is not Phase9Scenario.QUEUE_SATURATED
        ):
            raise ValueError("Phase 9 scenario does not match its workspace")


@dataclass(frozen=True, slots=True)
class FeatureDescriptor:
    name: str
    implementation_fingerprint: str
    dtype: str

    def __post_init__(self) -> None:
        _identities(self.name, self.implementation_fingerprint, self.dtype)


@dataclass(frozen=True, slots=True)
class ArtifactMetadata:
    schema_version: int
    engine_version: str
    model_kind: str
    model_configuration: tuple[tuple[str, str], ...]
    target_definition: str
    training_fingerprint: str
    training_cutoff: datetime
    seed: int
    generator_version: str
    build_revision: str
    files: tuple[tuple[str, str], ...]
    status: ArtifactStatus = ArtifactStatus.COMPLETE

    def __post_init__(self) -> None:
        if self.schema_version < 1 or self.seed < 0:
            raise ValueError("artifact schema and seed must be valid")
        _identities(
            self.engine_version,
            self.model_kind,
            self.target_definition,
            self.training_fingerprint,
            self.generator_version,
            self.build_revision,
        )
        _utc(self.training_cutoff, "training cutoff")
        _unique_pairs(self.model_configuration, "model configuration")
        _unique_pairs(self.files, "artifact files")
        if any(len(checksum) != 64 for _, checksum in self.files):
            raise ValueError("artifact checksums must be SHA-256 hex digests")


@dataclass(frozen=True, slots=True)
class TreeArrays:
    left: tuple[int, ...]
    right: tuple[int, ...]
    feature: tuple[int, ...]
    threshold: tuple[float, ...]
    value: tuple[float, ...]
    missing_left: tuple[bool, ...]
    feature_count: int

    def __post_init__(self) -> None:
        size = len(self.left)
        if size < 1 or any(
            len(values) != size
            for values in (self.right, self.feature, self.threshold, self.value, self.missing_left)
        ):
            raise ValueError("tree arrays must be non-empty and have identical lengths")
        if self.feature_count < 1 or any(
            not math.isfinite(value) for value in (*self.threshold, *self.value)
        ):
            raise ValueError("tree arrays require finite values and features")
        for node in range(size):
            children = (self.left[node], self.right[node])
            leaf = children == (-1, -1)
            if leaf:
                if self.feature[node] != -1:
                    raise ValueError("leaf nodes cannot identify split features")
                continue
            if -1 in children or any(child < 0 or child >= size for child in children):
                raise ValueError("split child indices are invalid")
            if not 0 <= self.feature[node] < self.feature_count:
                raise ValueError("split feature index is invalid")
        visited: set[int] = set()
        active: set[int] = set()

        def walk(node: int) -> None:
            if node in active:
                raise ValueError("tree contains a cycle")
            if node in visited:
                raise ValueError("tree nodes must have one parent")
            active.add(node)
            visited.add(node)
            if self.left[node] != -1:
                walk(self.left[node])
                walk(self.right[node])
            active.remove(node)

        walk(0)
        if len(visited) != size:
            raise ValueError("tree contains unreachable nodes")


@dataclass(frozen=True, slots=True)
class ModelRecord:
    model_id: str
    run_id: str
    display_name: str
    completed_at: datetime
    artifact: ArtifactMetadata
    features: tuple[FeatureDescriptor, ...]
    feature_importance: tuple[float, ...]
    trees: tuple[TreeArrays, ...]
    final_refit: bool = True

    def __post_init__(self) -> None:
        _identities(self.model_id, self.run_id, self.display_name)
        _utc(self.completed_at, "model completion")
        if not self.final_refit:
            raise ValueError("the inference registry contains final-refit models only")
        if len(self.features) != len(self.feature_importance) or not self.features:
            raise ValueError("feature importance must align with features")
        if any(not math.isfinite(value) or value < 0 for value in self.feature_importance):
            raise ValueError("feature importance must be finite and non-negative")
        if any(tree.feature_count != len(self.features) for tree in self.trees):
            raise ValueError("tree feature bounds must match the model feature schema")


@dataclass(frozen=True, slots=True)
class AliasEvent:
    event_id: str
    alias: str
    previous_model_id: str | None
    model_id: str
    command_id: str
    assigned_at: datetime

    def __post_init__(self) -> None:
        _identities(self.event_id, self.alias, self.model_id, self.command_id)
        if self.previous_model_id is not None:
            _identity(self.previous_model_id)
        _utc(self.assigned_at, "alias assignment")


@dataclass(frozen=True, slots=True)
class AliasCommand:
    command_id: str
    correlation_id: str
    generation: int
    alias: str
    model_id: str
    confirmed: bool

    def __post_init__(self) -> None:
        _identities(self.command_id, self.correlation_id, self.alias, self.model_id)
        if self.generation < 1 or not self.confirmed:
            raise ValueError("alias assignment requires a generation and explicit confirmation")

    @property
    def request_id(self) -> str:
        return self.correlation_id


@dataclass(frozen=True, slots=True)
class AliasResult:
    command: AliasCommand
    event: AliasEvent
    replayed: bool = False


@dataclass(frozen=True, slots=True)
class CompatibilityIssue:
    field: str
    code: str
    message: str

    def __post_init__(self) -> None:
        _identities(self.field, self.code, self.message)


@dataclass(frozen=True, slots=True)
class CompatibilityReport:
    model_id: str
    dataset_id: str
    dataset_fingerprint: str
    compatible: bool
    issues: tuple[CompatibilityIssue, ...]

    def __post_init__(self) -> None:
        _identities(self.model_id, self.dataset_id, self.dataset_fingerprint)
        if self.compatible == bool(self.issues):
            raise ValueError("compatibility status and diagnostics disagree")


@dataclass(frozen=True, slots=True)
class InferenceQuery:
    request_id: str
    generation: int
    model_or_alias: str
    dataset_id: str
    dataset_fingerprint: str
    mode: InferenceMode
    start: datetime | None = None
    end: datetime | None = None

    def __post_init__(self) -> None:
        _identities(self.request_id, self.model_or_alias, self.dataset_id, self.dataset_fingerprint)
        if self.generation < 1:
            raise ValueError("inference generation must be positive")
        if self.mode is InferenceMode.HISTORICAL:
            if self.start is None or self.end is None or not self.start < self.end:
                raise ValueError("historical inference requires a valid range")
        elif self.start is not None or self.end is not None:
            raise ValueError("latest inference cannot include a historical range")
        if self.start is not None:
            _utc(self.start, "inference start")
        if self.end is not None:
            _utc(self.end, "inference end")


@dataclass(frozen=True, slots=True)
class Prediction:
    symbol_id: str
    timestamp: datetime
    model_id: str
    run_id: str
    dataset_fingerprint: str
    feature_fingerprints: tuple[str, ...]
    score: float | None
    eligible: bool

    def __post_init__(self) -> None:
        _identities(self.symbol_id, self.model_id, self.run_id, self.dataset_fingerprint)
        _utc(self.timestamp, "prediction timestamp")
        for value in self.feature_fingerprints:
            _identity(value)
        if self.score is not None and not math.isfinite(self.score):
            raise ValueError("prediction scores must be finite or explicitly missing")
        if not self.eligible and self.score is not None:
            raise ValueError("ineligible predictions cannot carry scores")


@dataclass(frozen=True, slots=True)
class RankedPrediction:
    rank: int | None
    prediction: Prediction

    def __post_init__(self) -> None:
        if self.rank is None:
            if self.prediction.score is not None:
                raise ValueError("scored predictions require a rank")
        elif self.rank < 1 or self.prediction.score is None:
            raise ValueError("ranks are positive and require a score")


@dataclass(frozen=True, slots=True)
class ScoreDistribution:
    edges: tuple[float, ...]
    counts: tuple[int, ...]

    def __post_init__(self) -> None:
        if len(self.edges) != len(self.counts) + 1 or not self.counts:
            raise ValueError("distribution edges must bound all counts")
        if tuple(sorted(self.edges)) != self.edges or min(self.counts) < 0:
            raise ValueError("distribution bins must be ordered and non-negative")


@dataclass(frozen=True, slots=True)
class InferenceSnapshot:
    query: InferenceQuery
    inference_id: str
    compatibility: CompatibilityReport
    predictions: tuple[Prediction, ...]
    rankings: tuple[RankedPrediction, ...]
    distribution: ScoreDistribution | None
    checksum: str | None
    completed_at: datetime | None

    def __post_init__(self) -> None:
        _identity(self.inference_id)
        if not self.compatibility.compatible:
            if self.predictions or self.rankings or self.distribution is not None or self.checksum:
                raise ValueError("incompatible inference cannot publish output")
            return
        if (
            len(self.predictions) != len(self.rankings)
            or tuple(item.prediction for item in self.rankings) != self.predictions
        ):
            raise ValueError("rankings must align with predictions")
        expected = tuple(
            sorted(
                (item for item in self.predictions if item.score is not None),
                key=_prediction_sort_key,
            )
        )
        actual = tuple(item.prediction for item in self.rankings if item.rank is not None)
        if actual != expected:
            raise ValueError("rankings must use descending score and stable symbol tie-breaking")
        if self.checksum is None or len(self.checksum) != 64 or self.completed_at is None:
            raise ValueError("completed inference requires checksum and timestamp")
        _utc(self.completed_at, "inference completion")


@dataclass(frozen=True, slots=True)
class ExportRequest:
    request_id: str
    generation: int
    inference_id: str

    def __post_init__(self) -> None:
        _identities(self.request_id, self.inference_id)
        if self.generation < 1:
            raise ValueError("export generation must be positive")


@dataclass(frozen=True, slots=True)
class ExportResult:
    request: ExportRequest
    checksum: str
    row_count: int
    prepared_at: datetime

    def __post_init__(self) -> None:
        if len(self.checksum) != 64 or self.row_count < 0:
            raise ValueError("export result requires checksum and row count")
        _utc(self.prepared_at, "export preparation")


@dataclass(frozen=True, slots=True)
class Phase9Snapshot:
    request: Phase9Request
    models: tuple[ModelRecord, ...]
    aliases: tuple[tuple[str, str], ...]
    alias_history: tuple[AliasEvent, ...]
    compatibility: CompatibilityReport | None
    inference_history: tuple[InferenceSnapshot, ...]
    replay_seed: int
    replay_clock: datetime
    degraded: bool = False

    def __post_init__(self) -> None:
        _utc(self.replay_clock, "Phase 9 replay clock")
        model_ids = tuple(item.model_id for item in self.models)
        if len(model_ids) != len(set(model_ids)) or model_ids != tuple(sorted(model_ids)):
            raise ValueError("model registry requires unique stable model ordering")
        _unique_pairs(self.aliases, "aliases")
        if any(model_id not in set(model_ids) for _, model_id in self.aliases):
            raise ValueError("aliases must target registered models")


@dataclass(frozen=True, slots=True)
class ModelsWorkspaceState:
    generation: int = 0
    state: Phase9LoadState = Phase9LoadState.IDLE
    scenario: Phase9Scenario = Phase9Scenario.MODELS_NORMAL
    active_request: Phase9Request | None = None
    snapshot: Phase9Snapshot | None = None
    selected_model_id: str | None = None
    selected_tree_index: int = 0
    error: str | None = None
    stale: bool = False


@dataclass(frozen=True, slots=True)
class InferenceWorkspaceState:
    generation: int = 0
    state: Phase9LoadState = Phase9LoadState.IDLE
    scenario: Phase9Scenario = Phase9Scenario.INFERENCE_NORMAL
    active_request: Phase9Request | None = None
    snapshot: Phase9Snapshot | None = None
    selected_model_or_alias: str = "champion"
    active_inference: InferenceQuery | None = None
    inference: InferenceSnapshot | None = None
    history: tuple[InferenceSnapshot, ...] = ()
    export_state: ExportState = ExportState.IDLE
    active_export: ExportRequest | None = None
    export_result: ExportResult | None = None
    error: str | None = None
    stale: bool = False


@dataclass(frozen=True, slots=True)
class ModelsInferenceState:
    models: ModelsWorkspaceState = ModelsWorkspaceState()
    inference: InferenceWorkspaceState = InferenceWorkspaceState()


def _identity(value: str) -> None:
    if not value or value.strip() != value:
        raise ValueError("identity values must be non-empty and normalized")


def _identities(*values: str) -> None:
    for value in values:
        _identity(value)


def _utc(value: datetime, label: str) -> None:
    if value.tzinfo is None or value.utcoffset() != timedelta(0):
        raise ValueError(f"{label} must be UTC")


def _unique_pairs(values: tuple[tuple[str, str], ...], label: str) -> None:
    for key, value in values:
        _identities(key, value)
    if len({key for key, _ in values}) != len(values):
        raise ValueError(f"{label} keys must be unique")


def _prediction_sort_key(item: Prediction) -> tuple[float, str]:
    if item.score is None:
        raise ValueError("only scored predictions can be ranked")
    return -item.score, item.symbol_id
