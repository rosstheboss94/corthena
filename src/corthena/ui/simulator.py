"""Deterministic Phase 2 simulator implementing only ``UIClient``."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.data_experiments.deterministic import DataExperimentsDemo
from corthena.ui.data_experiments.models import (
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    ImportRequest,
    ImportResult,
    Phase7Request,
    Phase7Snapshot,
    SubmissionRequest,
)
from corthena.ui.jobs_results.deterministic import JobsResultsDemo
from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    JobCommand,
    JobCommandResult,
    Phase8Request,
    Phase8Snapshot,
    RunComparison,
)
from corthena.ui.models_inference.deterministic import ModelsInferenceDemo
from corthena.ui.models_inference.models import (
    AliasCommand,
    AliasResult,
    ExportRequest,
    ExportResult,
    InferenceQuery,
    InferenceSnapshot,
    Phase9Request,
    Phase9Snapshot,
)
from corthena.ui.research.deterministic import build_research_snapshot
from corthena.ui.research.models import ResearchQuery, ResearchSnapshot
from corthena.ui.state import Snapshot, SnapshotItem


@dataclass(frozen=True, slots=True)
class SimulatorConfig:
    """Explicit replay inputs for deterministic demo behavior."""

    seed: int
    fixed_clock: datetime
    delay_seconds: float = 0.0

    def __post_init__(self) -> None:
        if self.fixed_clock.tzinfo is None:
            raise ValueError("fixed_clock must be timezone-aware")
        if self.delay_seconds < 0:
            raise ValueError("delay_seconds must be non-negative")


class DeterministicSimulator:
    """Seeded, scheduling-independent client adapter for pre-coordinator use."""

    def __init__(self, config: SimulatorConfig) -> None:
        self._config = config
        self._phase7 = DataExperimentsDemo(config.seed, config.fixed_clock)
        self._phase8 = JobsResultsDemo(config.seed, config.fixed_clock)
        self._phase9 = ModelsInferenceDemo(config.seed, config.fixed_clock)

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignalProtocol,
    ) -> Snapshot:
        """Derive stable demo values solely from explicit replay inputs."""
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request_id)
        if cancellation.is_set():
            raise RequestCancelledError(request_id)
        symbols = ("AAPL", "MSFT", "NVDA", "SPY")
        items = tuple(
            SnapshotItem(index, symbol, self._value(request_id, generation, symbol))
            for index, symbol in enumerate(symbols)
        )
        return Snapshot(
            request_id=request_id,
            generation=generation,
            seed=self._config.seed,
            as_of=self._config.fixed_clock,
            items=items,
        )

    def load_research(
        self,
        query: ResearchQuery,
        cancellation: CancellationSignalProtocol,
    ) -> ResearchSnapshot:
        """Prepare deterministic Research data off the render thread."""
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(query.request_id)
        return build_research_snapshot(
            self._config.seed,
            self._config.fixed_clock,
            query,
            cancellation,
        )

    def load_phase7(
        self, request: Phase7Request, cancellation: CancellationSignalProtocol
    ) -> Phase7Snapshot:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase7.load(request, cancellation)

    def import_data(
        self, request: ImportRequest, cancellation: CancellationSignalProtocol
    ) -> ImportResult:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase7.run_import(request, cancellation)

    def evaluate_draft(
        self,
        request_id: str,
        generation: int,
        draft: ExperimentDraft,
        cancellation: CancellationSignalProtocol,
    ) -> DraftEvaluation:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request_id)
        return self._phase7.evaluate(request_id, generation, draft, cancellation)

    def save_draft(
        self, request: DraftSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DraftSaveResult:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase7.save(request, cancellation)

    def submit_experiment(
        self, request: SubmissionRequest, cancellation: CancellationSignalProtocol
    ) -> ExperimentDefinition:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase7.submit(request, cancellation)

    def load_phase8(
        self, request: Phase8Request, cancellation: CancellationSignalProtocol
    ) -> Phase8Snapshot:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase8.load(request, cancellation)

    def command_job(
        self, command: JobCommand, cancellation: CancellationSignalProtocol
    ) -> JobCommandResult:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(command.request_id)
        return self._phase8.command(command, cancellation)

    def compare_runs(
        self, query: ComparisonQuery, cancellation: CancellationSignalProtocol
    ) -> RunComparison:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(query.request_id)
        return self._phase8.compare(query, cancellation)

    def load_phase9(
        self, request: Phase9Request, cancellation: CancellationSignalProtocol
    ) -> Phase9Snapshot:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase9.load(request, cancellation)

    def assign_alias(
        self, command: AliasCommand, cancellation: CancellationSignalProtocol
    ) -> AliasResult:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(command.request_id)
        return self._phase9.assign_alias(command, cancellation)

    def score_inference(
        self, query: InferenceQuery, cancellation: CancellationSignalProtocol
    ) -> InferenceSnapshot:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(query.request_id)
        return self._phase9.score(query, cancellation)

    def prepare_export(
        self, request: ExportRequest, cancellation: CancellationSignalProtocol
    ) -> ExportResult:
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request.request_id)
        return self._phase9.export(request, cancellation)

    def _value(self, request_id: str, generation: int, symbol: str) -> int:
        payload = f"{self._config.seed}\0{request_id}\0{generation}\0{symbol}".encode()
        digest = hashlib.sha256(payload).digest()
        return 10_000_000 + int.from_bytes(digest[:8], "big") % 990_000_001


__all__ = ["DeterministicSimulator", "SimulatorConfig"]
