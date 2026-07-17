"""Agent-facing contracts for snapshot loading."""

from __future__ import annotations

from typing import Protocol

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
from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    JobCommand,
    JobCommandResult,
    Phase8Request,
    Phase8Snapshot,
    RunComparison,
)
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
from corthena.ui.research.models import ResearchQuery, ResearchSnapshot
from corthena.ui.state import Snapshot


class CancellationSignalProtocol(Protocol):
    """Read-only cancellation view for client operations."""

    def is_set(self) -> bool: ...

    def wait(self, timeout: float | None = None) -> bool: ...


class UIClientProtocol(Protocol):
    """Operations consumed by the UI effects runtime."""

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignalProtocol,
    ) -> Snapshot: ...

    def load_research(
        self,
        query: ResearchQuery,
        cancellation: CancellationSignalProtocol,
    ) -> ResearchSnapshot: ...

    def load_phase7(
        self, request: Phase7Request, cancellation: CancellationSignalProtocol
    ) -> Phase7Snapshot: ...

    def import_data(
        self, request: ImportRequest, cancellation: CancellationSignalProtocol
    ) -> ImportResult: ...

    def evaluate_draft(
        self,
        request_id: str,
        generation: int,
        draft: ExperimentDraft,
        cancellation: CancellationSignalProtocol,
    ) -> DraftEvaluation: ...

    def save_draft(
        self, request: DraftSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DraftSaveResult: ...

    def submit_experiment(
        self, request: SubmissionRequest, cancellation: CancellationSignalProtocol
    ) -> ExperimentDefinition: ...

    def load_phase8(
        self, request: Phase8Request, cancellation: CancellationSignalProtocol
    ) -> Phase8Snapshot: ...

    def command_job(
        self, command: JobCommand, cancellation: CancellationSignalProtocol
    ) -> JobCommandResult: ...

    def compare_runs(
        self, query: ComparisonQuery, cancellation: CancellationSignalProtocol
    ) -> RunComparison: ...

    def load_phase9(
        self, request: Phase9Request, cancellation: CancellationSignalProtocol
    ) -> Phase9Snapshot: ...

    def assign_alias(
        self, command: AliasCommand, cancellation: CancellationSignalProtocol
    ) -> AliasResult: ...

    def score_inference(
        self, query: InferenceQuery, cancellation: CancellationSignalProtocol
    ) -> InferenceSnapshot: ...

    def prepare_export(
        self, request: ExportRequest, cancellation: CancellationSignalProtocol
    ) -> ExportResult: ...
