"""Deterministic, explicitly synchronized Phase 7 demo service."""

from __future__ import annotations

import hashlib
import threading
from dataclasses import replace
from datetime import UTC, datetime, timedelta

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    DatasetCatalogEntry,
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    FeatureIdentity,
    ImportRequest,
    ImportResult,
    ImportState,
    Phase7Request,
    Phase7Scenario,
    Phase7Snapshot,
    ResourceEstimate,
    SubmissionRequest,
    UtcRange,
    ValidationDiagnostic,
)

FEATURES = (
    FeatureIdentity("ret_5", "1.0.0", 5, "float32[ret_5]", "sha256:feature-ret5-v1"),
    FeatureIdentity(
        "volatility_20", "1.0.0", 20, "float32[volatility_20]", "sha256:feature-vol20-v1"
    ),
    FeatureIdentity("volume_z_30", "1.0.0", 30, "float32[volume_z_30]", "sha256:feature-volz30-v1"),
)


class DataExperimentsDemo:
    """Own mutable demo metadata and publish frozen snapshots under one lock."""

    def __init__(self, seed: int, fixed_clock: datetime) -> None:
        if fixed_clock.tzinfo is None or fixed_clock.utcoffset() != timedelta(0):
            raise ValueError("Phase 7 fixed clock must be UTC")
        self._seed = seed
        self._clock = fixed_clock
        self._lock = threading.Lock()
        self._catalog = _initial_catalog(seed)
        self._imports: tuple[ImportResult, ...] = ()
        primary = next(item for item in self._catalog if item.dataset_id == "dataset-us-equities")
        self._draft = default_experiment_draft(primary)
        self._saved_revision: int = 0
        self._submissions: dict[str, ExperimentDefinition] = {
            "demo-complete": ExperimentDefinition(
                "experiment-demo-complete", "demo-complete", fixed_clock, self._draft
            ),
            "demo-forest": ExperimentDefinition(
                "experiment-demo-forest", "demo-forest", fixed_clock, self._draft
            ),
        }

    def load(
        self, request: Phase7Request, cancellation: CancellationSignalProtocol
    ) -> Phase7Snapshot:
        _cancel(cancellation, request.request_id)
        if request.scenario is Phase7Scenario.LOADING:
            cancellation.wait()
            raise RequestCancelledError(request.request_id)
        if request.scenario is Phase7Scenario.CANCELLED:
            raise RequestCancelledError(request.request_id)
        if request.scenario is Phase7Scenario.FAILURE:
            raise RuntimeError("deterministic Phase 7 request failed")
        with self._lock:
            catalog = () if request.scenario is Phase7Scenario.EMPTY else self._catalog
            evaluation = evaluate_experiment(request.request_id, request.generation, self._draft)
            return Phase7Snapshot(
                request,
                catalog,
                self._imports,
                self._draft,
                evaluation,
                tuple(sorted(self._submissions.values(), key=lambda item: item.experiment_id)),
                self._seed,
                self._clock,
                request.scenario is Phase7Scenario.DEGRADED,
            )

    def run_import(
        self, request: ImportRequest, cancellation: CancellationSignalProtocol
    ) -> ImportResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            current = next(
                (item for item in self._catalog if item.dataset_id == request.dataset_id), None
            )
            if current is None:
                raise ValueError(f"unknown dataset {request.dataset_id!r}")
            if current.revision != request.expected_revision:
                result = _rejected_import(
                    request, current, "stale_revision", "Catalog revision changed"
                )
            elif request.interval != current.interval:
                result = _rejected_import(
                    request, current, "interval_mismatch", "Interval does not match dataset"
                )
            elif not set(request.symbols) <= set(current.symbols):
                result = _rejected_import(
                    request, current, "unknown_symbol", "Import contains an unknown symbol"
                )
            elif request.scenario in {"empty", "duplicate", "malformed"}:
                messages = {
                    "empty": ("empty_source", "Source contains no canonical bars"),
                    "duplicate": ("duplicate_key", "Duplicate (symbol, timestamp) key"),
                    "malformed": ("invalid_ohlcv", "OHLC values or volume are invalid"),
                }
                code, message = messages[request.scenario]
                result = _rejected_import(request, current, code, message)
            elif request.scenario == "failure":
                raise RuntimeError("deterministic import adapter failed")
            else:
                rows = 840 if request.mode.value == "append" else 420
                fingerprint = _fingerprint(
                    f"{current.content_fingerprint}|{request.command_id}|{request.mode.value}|{rows}"
                )
                updated = replace(
                    current,
                    revision=current.revision + 1,
                    row_count=current.row_count + rows,
                    content_fingerprint=fingerprint,
                )
                result = ImportResult(request, ImportState.READY, updated, (), rows)
                self._catalog = tuple(
                    updated if item.dataset_id == updated.dataset_id else item
                    for item in self._catalog
                )
            self._imports = (*self._imports, result)[-64:]
            return result

    def evaluate(
        self,
        request_id: str,
        generation: int,
        draft: ExperimentDraft,
        cancellation: CancellationSignalProtocol,
    ) -> DraftEvaluation:
        _cancel(cancellation, request_id)
        return evaluate_experiment(request_id, generation, draft)

    def save(
        self, request: DraftSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DraftSaveResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            if request.expected_saved_revision != self._saved_revision:
                raise ValueError("stale draft save revision")
            if request.draft.revision < self._draft.revision:
                raise ValueError("late draft save cannot overwrite a newer edit")
            self._draft = request.draft
            self._saved_revision = request.draft.revision
            return DraftSaveResult(request, self._saved_revision)

    def submit(
        self, request: SubmissionRequest, cancellation: CancellationSignalProtocol
    ) -> ExperimentDefinition:
        _cancel(cancellation, request.request_id)
        evaluation = evaluate_experiment(request.request_id, request.generation, request.draft)
        if not evaluation.valid:
            raise ValueError("invalid experiment draft cannot be submitted")
        with self._lock:
            existing = self._submissions.get(request.command_id)
            if existing is not None:
                if existing.draft != request.draft:
                    raise ValueError("accepted command cannot be reused with a different draft")
                return existing
            current = next(
                (item for item in self._catalog if item.dataset_id == request.draft.dataset_id),
                None,
            )
            if current is None or (
                current.revision != request.draft.dataset_revision
                or current.content_fingerprint != request.draft.dataset_fingerprint
            ):
                raise ValueError("draft catalog revision or fingerprint is stale")
            definition = ExperimentDefinition(
                f"experiment-{len(self._submissions) + 1:06d}",
                request.command_id,
                self._clock + timedelta(milliseconds=request.generation),
                request.draft,
            )
            self._submissions[request.command_id] = definition
            return definition


def default_experiment_draft(dataset: DatasetCatalogEntry) -> ExperimentDraft:
    return ExperimentDraft(
        "draft-daily-equity-baseline",
        1,
        dataset.dataset_id,
        dataset.revision,
        dataset.content_fingerprint,
        FEATURES[:2],
        5,
        1000,
        250,
        250,
        5,
        "hist_gradient_boosting",
        6,
        300,
        1_000_000.0,
        1.5,
        (),
        4,
    )


def evaluate_experiment(
    request_id: str, generation: int, draft: ExperimentDraft
) -> DraftEvaluation:
    diagnostics: list[ValidationDiagnostic] = []
    if not draft.features or len(
        {(item.name, item.semantic_version) for item in draft.features}
    ) != len(draft.features):
        diagnostics.append(
            ValidationDiagnostic("features_unique", "Select unique compiled features", "features")
        )
    if not 1 <= draft.target_horizon <= 256:
        diagnostics.append(
            ValidationDiagnostic(
                "target_horizon", "Target horizon must be 1-256 bars", "target_horizon"
            )
        )
    if min(draft.train_bars, draft.validation_bars, draft.test_bars) < 1:
        diagnostics.append(
            ValidationDiagnostic("split_size", "Every split must contain rows", "split")
        )
    if draft.purge_bars < draft.target_horizon:
        diagnostics.append(
            ValidationDiagnostic(
                "purge_horizon", "Purge must cover the target horizon", "purge_bars"
            )
        )
    if not 1 <= draft.max_depth <= 32 or not 1 <= draft.estimator_count <= 10_000:
        diagnostics.append(
            ValidationDiagnostic("model_bounds", "Model settings exceed bounds", "model")
        )
    if draft.initial_capital <= 0 or draft.fee_bps < 0:
        diagnostics.append(
            ValidationDiagnostic("portfolio_bounds", "Portfolio values are invalid", "portfolio")
        )
    if not 1 <= draft.cpu_limit <= 64:
        diagnostics.append(ValidationDiagnostic("cpu_limit", "CPU limit must be 1-64", "cpu_limit"))
    if len(draft.sweep_values) > 128 or any(value < 1 for value in draft.sweep_values):
        diagnostics.append(
            ValidationDiagnostic("sweep_bounds", "Sweep values exceed bounds", "sweep")
        )
    rows = max(0, draft.train_bars + draft.validation_bars + draft.test_bars - draft.purge_bars * 2)
    columns = max(1, len(draft.features))
    feature_bytes = rows * columns * 4
    candidates = max(1, len(draft.sweep_values))
    estimate = ResourceEstimate(
        rows,
        feature_bytes,
        feature_bytes * 3 + rows * 8,
        round(
            rows
            * columns
            * draft.estimator_count
            * candidates
            / max(1, draft.cpu_limit)
            / 2_000_000,
            6,
        ),
    )
    return DraftEvaluation(request_id, generation, draft, tuple(diagnostics), estimate)


def _initial_catalog(seed: int) -> tuple[DatasetCatalogEntry, ...]:
    coverage = UtcRange(datetime(2020, 7, 9, tzinfo=UTC), datetime(2026, 7, 9, tzinfo=UTC))
    return (
        DatasetCatalogEntry(
            "dataset-us-equities",
            "US equities daily",
            18,
            _fingerprint(f"{seed}|us-equities|18"),
            "sha256:canonical-bars-v1",
            ("AAPL", "AMD", "MSFT", "NVDA"),
            "1d",
            coverage,
            955_353,
            AdjustmentPolicy.SPLIT_AND_DIVIDEND,
        ),
        DatasetCatalogEntry(
            "dataset-index-watchlist",
            "Index watchlist hourly",
            7,
            _fingerprint(f"{seed}|index-watchlist|7"),
            "sha256:canonical-bars-v1",
            ("DIA", "IWM", "QQQ", "SPY"),
            "1h",
            coverage,
            214_840,
            AdjustmentPolicy.RAW,
            "validation",
        ),
    )


def _rejected_import(
    request: ImportRequest, current: DatasetCatalogEntry, code: str, message: str
) -> ImportResult:
    return ImportResult(
        request,
        ImportState.REJECTED,
        current,
        (ValidationDiagnostic(code, message, "source"),),
        0,
    )


def _fingerprint(value: str) -> str:
    return f"sha256:{hashlib.sha256(value.encode()).hexdigest()}"


def _cancel(cancellation: CancellationSignalProtocol, request_id: str) -> None:
    if cancellation.is_set():
        raise RequestCancelledError(request_id)
