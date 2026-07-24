"""Agent-facing contracts for backend-swappable UI operations."""

from __future__ import annotations

from typing import Protocol

from corthena.ui.data_experiments.models import (
    CredentialRequest,
    CredentialResult,
    CredentialSecretRequest,
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    FileBrowserListing,
    FileBrowserRequest,
    FilePreview,
    FilePreviewRequest,
    ImportRequest,
    ImportResult,
    IngestionPlan,
    IngestionResult,
    Phase7Request,
    Phase7Snapshot,
    ReconciliationRequest,
    ReconciliationResult,
    ScheduleCommand,
    ScheduleResult,
    SubmissionRequest,
    SymbolDiscoveryRequest,
    SymbolDiscoveryResult,
)
from corthena.ui.datasets.models import (
    DatasetBuild,
    DatasetBuildRequest,
    DatasetSaveRequest,
    DatasetSaveResult,
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

    def credential_status(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult: ...

    def save_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult: ...

    def test_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult: ...

    def delete_credential(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult: ...

    def preview_file(
        self, request: FilePreviewRequest, cancellation: CancellationSignalProtocol
    ) -> FilePreview: ...

    def browse_files(
        self, request: FileBrowserRequest, cancellation: CancellationSignalProtocol
    ) -> FileBrowserListing: ...

    def discover_symbols(
        self, request: SymbolDiscoveryRequest, cancellation: CancellationSignalProtocol
    ) -> SymbolDiscoveryResult: ...

    def submit_file_ingestion(
        self, plan: IngestionPlan, cancellation: CancellationSignalProtocol
    ) -> IngestionResult: ...

    def submit_massive_pull(
        self, plan: IngestionPlan, cancellation: CancellationSignalProtocol
    ) -> IngestionResult: ...

    def mutate_schedule(
        self, command: ScheduleCommand, cancellation: CancellationSignalProtocol
    ) -> ScheduleResult: ...

    def reconcile_data(
        self, request: ReconciliationRequest, cancellation: CancellationSignalProtocol
    ) -> ReconciliationResult: ...

    def save_dataset(
        self, request: DatasetSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetSaveResult: ...

    def build_dataset(
        self, request: DatasetBuildRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetBuild: ...

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
