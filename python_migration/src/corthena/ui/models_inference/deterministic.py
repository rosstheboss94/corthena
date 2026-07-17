"""Seeded deterministic Phase 9 registry and inference simulator."""

from __future__ import annotations

import hashlib
from dataclasses import replace
from datetime import datetime, timedelta

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.models_inference.models import (
    AliasCommand,
    AliasEvent,
    AliasResult,
    ArtifactMetadata,
    CompatibilityIssue,
    CompatibilityReport,
    ExportRequest,
    ExportResult,
    FeatureDescriptor,
    InferenceMode,
    InferenceQuery,
    InferenceSnapshot,
    ModelRecord,
    Phase9Request,
    Phase9Scenario,
    Phase9Snapshot,
    Phase9Workspace,
    Prediction,
    RankedPrediction,
    ScoreDistribution,
    TreeArrays,
)


class ModelsInferenceDemo:
    def __init__(self, seed: int, clock: datetime) -> None:
        self._seed = seed
        self._clock = clock
        self._aliases: dict[str, str] = {
            "candidate": "model-phase9-b",
            "champion": "model-phase9-a",
        }
        self._history: list[AliasEvent] = []
        self._commands: dict[str, AliasResult] = {}
        self._inference_history: list[InferenceSnapshot] = []

    def load(
        self, request: Phase9Request, cancellation: CancellationSignalProtocol
    ) -> Phase9Snapshot:
        self._cancel(request.request_id, cancellation)
        if request.scenario in {Phase9Scenario.MODELS_FAILURE, Phase9Scenario.INFERENCE_FAILURE}:
            raise RuntimeError("simulated Phase 9 failure")
        empty = request.scenario in {Phase9Scenario.MODELS_EMPTY, Phase9Scenario.INFERENCE_EMPTY}
        degraded = request.scenario in {
            Phase9Scenario.MODELS_DEGRADED,
            Phase9Scenario.INFERENCE_DEGRADED,
        }
        models = () if empty else self._models()
        report = (
            self._compatibility(
                models[0], incompatible=request.scenario is Phase9Scenario.INFERENCE_INCOMPATIBLE
            )
            if models and request.workspace is Phase9Workspace.INFERENCE
            else None
        )
        return Phase9Snapshot(
            request,
            models,
            tuple(sorted(self._aliases.items())) if models else (),
            tuple(self._history),
            report,
            tuple(self._inference_history),
            self._seed,
            self._clock,
            degraded,
        )

    def assign_alias(
        self, command: AliasCommand, cancellation: CancellationSignalProtocol
    ) -> AliasResult:
        self._cancel(command.request_id, cancellation)
        replay = self._commands.get(command.command_id)
        if replay is not None:
            return replace(replay, replayed=True)
        if command.model_id not in {item.model_id for item in self._models()}:
            raise ValueError("alias target is not a registered final-refit model")
        previous = self._aliases.get(command.alias)
        event = AliasEvent(
            f"alias-event-{len(self._history) + 1:04d}",
            command.alias,
            previous,
            command.model_id,
            command.command_id,
            self._clock + timedelta(seconds=len(self._history) + 1),
        )
        self._aliases[command.alias] = command.model_id
        self._history.append(event)
        result = AliasResult(command, event)
        self._commands[command.command_id] = result
        return result

    def score(
        self, query: InferenceQuery, cancellation: CancellationSignalProtocol
    ) -> InferenceSnapshot:
        self._cancel(query.request_id, cancellation)
        aliases = self._aliases
        model_id = aliases.get(query.model_or_alias, query.model_or_alias)
        model = next((item for item in self._models() if item.model_id == model_id), None)
        if model is None:
            raise ValueError("selected model or alias does not resolve")
        incompatible = query.dataset_fingerprint != "dataset-fingerprint-phase9"
        report = self._compatibility(model, incompatible=incompatible)
        inference_id = f"inference-{query.generation:04d}-{model.model_id}"
        if incompatible:
            return InferenceSnapshot(query, inference_id, report, (), (), None, None, None)
        timestamp = self._clock if query.mode is InferenceMode.LATEST else query.end
        if timestamp is None:
            raise AssertionError("validated historical query has an end")
        fingerprints = tuple(item.implementation_fingerprint for item in model.features)
        raw = (
            ("AAPL", 0.72, True),
            ("MSFT", 0.72, True),
            ("NVDA", 0.41, True),
            ("SPY", None, False),
        )
        scored = tuple(
            Prediction(
                symbol,
                timestamp,
                model.model_id,
                model.run_id,
                query.dataset_fingerprint,
                fingerprints,
                score,
                eligible,
            )
            for symbol, score, eligible in raw
        )
        ordered = sorted(
            (item for item in scored if item.score is not None), key=_prediction_sort_key
        )
        rank_by_symbol = {item.symbol_id: index + 1 for index, item in enumerate(ordered)}
        predictions = tuple(ordered) + tuple(item for item in scored if item.score is None)
        rankings = tuple(
            RankedPrediction(rank_by_symbol.get(item.symbol_id), item) for item in predictions
        )
        distribution = ScoreDistribution((0.0, 0.5, 0.75, 1.0), (1, 2, 0))
        checksum = self._digest(inference_id, *(item.symbol_id for item in predictions))
        snapshot = InferenceSnapshot(
            query, inference_id, report, predictions, rankings, distribution, checksum, self._clock
        )
        self._inference_history.append(snapshot)
        return snapshot

    def export(
        self, request: ExportRequest, cancellation: CancellationSignalProtocol
    ) -> ExportResult:
        self._cancel(request.request_id, cancellation)
        snapshot = next(
            (item for item in self._inference_history if item.inference_id == request.inference_id),
            None,
        )
        if snapshot is None or snapshot.checksum is None:
            raise ValueError("only complete checksummed inference can be exported")
        return ExportResult(
            request,
            self._digest("export", snapshot.checksum),
            len(snapshot.predictions),
            self._clock,
        )

    def _models(self) -> tuple[ModelRecord, ...]:
        features = (
            FeatureDescriptor("return_5", self._digest("feature", "return_5"), "float64"),
            FeatureDescriptor("volatility_20", self._digest("feature", "volatility_20"), "float64"),
        )
        tree = TreeArrays(
            (1, -1, -1),
            (2, -1, -1),
            (0, -1, -1),
            (0.015, 0.0, 0.0),
            (0.0, -0.2, 0.4),
            (True, False, False),
            2,
        )
        values: list[ModelRecord] = []
        for suffix, kind, score in (
            ("a", "hist_gradient_boosting", (0.61, 0.39)),
            ("b", "random_forest", (0.55, 0.45)),
        ):
            files = (
                ("manifest.json", self._digest("manifest", suffix)),
                ("model.bin", self._digest("model", suffix)),
            )
            artifact = ArtifactMetadata(
                1,
                "demo-engine-1",
                kind,
                (("max_depth", "6"), ("trees", "128")),
                "forward_return_5",
                self._digest("training", suffix),
                self._clock - timedelta(days=30),
                self._seed,
                "phase9-demo-v1",
                "python-migration-phase9",
                files,
            )
            values.append(
                ModelRecord(
                    f"model-phase9-{suffix}",
                    f"run-phase8-{suffix}",
                    f"Phase 9 {kind.replace('_', ' ').title()}",
                    self._clock - timedelta(days=2 if suffix == "a" else 4),
                    artifact,
                    features,
                    score,
                    (tree, tree),
                )
            )
        return tuple(values)

    def _compatibility(self, model: ModelRecord, *, incompatible: bool) -> CompatibilityReport:
        issues = (
            (
                CompatibilityIssue(
                    "dataset.fingerprint",
                    "dataset_fingerprint_mismatch",
                    "Dataset fingerprint does not match the selected model",
                ),
            )
            if incompatible
            else ()
        )
        return CompatibilityReport(
            model.model_id,
            "dataset-us-equities",
            "dataset-fingerprint-phase9",
            not incompatible,
            issues,
        )

    @staticmethod
    def _cancel(request_id: str, cancellation: CancellationSignalProtocol) -> None:
        if cancellation.is_set():
            raise RequestCancelledError(request_id)

    def _digest(self, *parts: str) -> str:
        return hashlib.sha256((str(self._seed) + "\0" + "\0".join(parts)).encode()).hexdigest()


def _prediction_sort_key(item: Prediction) -> tuple[float, str]:
    if item.score is None:
        raise ValueError("only scored predictions can be ranked")
    return -item.score, item.symbol_id
