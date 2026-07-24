"""Closed Phase 7 actions and effects."""

from dataclasses import dataclass

from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    ColumnMapping,
    CredentialRequest,
    CredentialResult,
    CredentialSecretRequest,
    DataIngestionView,
    DatasetWizardStep,
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    FileBrowserListing,
    FileBrowserRequest,
    FilePreview,
    FilePreviewRequest,
    ImportMode,
    ImportRequest,
    ImportResult,
    IngestionPlan,
    IngestionProgress,
    IngestionResult,
    IngestionScenario,
    Phase7Request,
    Phase7Scenario,
    Phase7Snapshot,
    Phase7Workspace,
    ReconciliationRequest,
    ReconciliationResult,
    ScheduleCommand,
    ScheduleResult,
    SessionPolicy,
    SourceKind,
    SubmissionRequest,
    SymbolDiscoveryRequest,
    SymbolDiscoveryResult,
)
from corthena.ui.datasets.models import (
    DatasetBuild,
    DatasetBuildRequest,
    DatasetSaveRequest,
    DatasetSaveResult,
    FeatureStep,
)


@dataclass(frozen=True, slots=True)
class RequestPhase7:
    request: Phase7Request


@dataclass(frozen=True, slots=True)
class Phase7Completed:
    snapshot: Phase7Snapshot


@dataclass(frozen=True, slots=True)
class Phase7Failed:
    workspace: Phase7Workspace
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class Phase7Cancelled:
    workspace: Phase7Workspace
    generation: int


@dataclass(frozen=True, slots=True)
class SetPhase7Scenario:
    workspace: Phase7Workspace
    scenario: Phase7Scenario


@dataclass(frozen=True, slots=True)
class SelectDataset:
    dataset_id: str


@dataclass(frozen=True, slots=True)
class SetDataIngestionView:
    view: DataIngestionView


@dataclass(frozen=True, slots=True)
class SetDatasetWizardStep:
    step: DatasetWizardStep


@dataclass(frozen=True, slots=True)
class SelectDatasetSource:
    source_id: str


@dataclass(frozen=True, slots=True)
class AddDatasetFeatureStep:
    step: FeatureStep


@dataclass(frozen=True, slots=True)
class RemoveDatasetFeatureStep:
    index: int


@dataclass(frozen=True, slots=True)
class MoveDatasetFeatureStep:
    index: int
    offset: int


@dataclass(frozen=True, slots=True)
class SetIngestionScenario:
    scenario: IngestionScenario


@dataclass(frozen=True, slots=True)
class RequestCredentialStatus:
    request: CredentialRequest


@dataclass(frozen=True, slots=True)
class RequestCredentialSave:
    request: CredentialSecretRequest


@dataclass(frozen=True, slots=True)
class RequestCredentialTest:
    request: CredentialSecretRequest


@dataclass(frozen=True, slots=True)
class RequestCredentialDelete:
    request: CredentialRequest


@dataclass(frozen=True, slots=True)
class CredentialCompleted:
    result: CredentialResult


@dataclass(frozen=True, slots=True)
class RequestFileBrowser:
    request: FileBrowserRequest


@dataclass(frozen=True, slots=True)
class FileBrowserCompleted:
    listing: FileBrowserListing


@dataclass(frozen=True, slots=True)
class CloseFileBrowser:
    pass


@dataclass(frozen=True, slots=True)
class SelectFileBrowserEntry:
    source_name: str


@dataclass(frozen=True, slots=True)
class ScrollFileBrowser:
    row_delta: int


@dataclass(frozen=True, slots=True)
class RequestFilePreview:
    request: FilePreviewRequest


@dataclass(frozen=True, slots=True)
class FilePreviewCompleted:
    preview: FilePreview


@dataclass(frozen=True, slots=True)
class UpdateFileMapping:
    mapping: ColumnMapping


@dataclass(frozen=True, slots=True)
class SetFileSourceKind:
    source_kind: SourceKind


@dataclass(frozen=True, slots=True)
class UpdateIngestionForm:
    interval: str
    session: SessionPolicy
    adjustment: AdjustmentPolicy
    mode: ImportMode
    source_timezone: str = "UTC"


@dataclass(frozen=True, slots=True)
class RequestSymbolDiscovery:
    request: SymbolDiscoveryRequest


@dataclass(frozen=True, slots=True)
class SymbolDiscoveryCompleted:
    result: SymbolDiscoveryResult


@dataclass(frozen=True, slots=True)
class SetSelectedSymbols:
    symbols: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class ConfirmIngestion:
    plan: IngestionPlan


@dataclass(frozen=True, slots=True)
class RequestFileIngestion:
    plan: IngestionPlan


@dataclass(frozen=True, slots=True)
class RequestMassivePull:
    plan: IngestionPlan


@dataclass(frozen=True, slots=True)
class IngestionCompleted:
    result: IngestionResult


@dataclass(frozen=True, slots=True)
class IngestionProgressed:
    progress: IngestionProgress


@dataclass(frozen=True, slots=True)
class IngestionOperationFailed:
    request_id: str
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class IngestionOperationCancelled:
    request_id: str
    generation: int


@dataclass(frozen=True, slots=True)
class RequestIngestionCancellation:
    request_id: str


@dataclass(frozen=True, slots=True)
class RequestScheduleCommand:
    command: ScheduleCommand


@dataclass(frozen=True, slots=True)
class ScheduleCommandCompleted:
    result: ScheduleResult


@dataclass(frozen=True, slots=True)
class SelectSchedule:
    schedule_id: str


@dataclass(frozen=True, slots=True)
class RequestReconciliation:
    request: ReconciliationRequest


@dataclass(frozen=True, slots=True)
class ReconciliationCompleted:
    result: ReconciliationResult


@dataclass(frozen=True, slots=True)
class RequestDatasetSave:
    request: DatasetSaveRequest


@dataclass(frozen=True, slots=True)
class DatasetSaveCompleted:
    result: DatasetSaveResult


@dataclass(frozen=True, slots=True)
class RequestDatasetBuild:
    request: DatasetBuildRequest


@dataclass(frozen=True, slots=True)
class DatasetBuildCompleted:
    build: DatasetBuild


@dataclass(frozen=True, slots=True)
class RequestDataImport:
    request: ImportRequest


@dataclass(frozen=True, slots=True)
class DataImportCompleted:
    result: ImportResult


@dataclass(frozen=True, slots=True)
class EditExperimentDraft:
    draft: ExperimentDraft


@dataclass(frozen=True, slots=True)
class RequestDraftEvaluation:
    request_id: str
    generation: int
    draft: ExperimentDraft


@dataclass(frozen=True, slots=True)
class DraftEvaluationCompleted:
    evaluation: DraftEvaluation


@dataclass(frozen=True, slots=True)
class RequestDraftSave:
    request: DraftSaveRequest


@dataclass(frozen=True, slots=True)
class DraftSaveCompleted:
    result: DraftSaveResult


@dataclass(frozen=True, slots=True)
class RequestSubmission:
    request: SubmissionRequest


@dataclass(frozen=True, slots=True)
class SubmissionCompleted:
    request: SubmissionRequest
    definition: ExperimentDefinition


Phase7Action = (
    RequestPhase7
    | Phase7Completed
    | Phase7Failed
    | Phase7Cancelled
    | SetPhase7Scenario
    | SelectDataset
    | SetDataIngestionView
    | SetDatasetWizardStep
    | SelectDatasetSource
    | AddDatasetFeatureStep
    | RemoveDatasetFeatureStep
    | MoveDatasetFeatureStep
    | SetIngestionScenario
    | RequestCredentialStatus
    | RequestCredentialSave
    | RequestCredentialTest
    | RequestCredentialDelete
    | CredentialCompleted
    | RequestFileBrowser
    | FileBrowserCompleted
    | CloseFileBrowser
    | SelectFileBrowserEntry
    | ScrollFileBrowser
    | RequestFilePreview
    | FilePreviewCompleted
    | UpdateFileMapping
    | SetFileSourceKind
    | UpdateIngestionForm
    | RequestSymbolDiscovery
    | SymbolDiscoveryCompleted
    | SetSelectedSymbols
    | ConfirmIngestion
    | RequestFileIngestion
    | RequestMassivePull
    | IngestionCompleted
    | IngestionProgressed
    | IngestionOperationFailed
    | IngestionOperationCancelled
    | RequestIngestionCancellation
    | RequestScheduleCommand
    | ScheduleCommandCompleted
    | SelectSchedule
    | RequestReconciliation
    | ReconciliationCompleted
    | RequestDatasetSave
    | DatasetSaveCompleted
    | RequestDatasetBuild
    | DatasetBuildCompleted
    | RequestDataImport
    | DataImportCompleted
    | EditExperimentDraft
    | RequestDraftEvaluation
    | DraftEvaluationCompleted
    | RequestDraftSave
    | DraftSaveCompleted
    | RequestSubmission
    | SubmissionCompleted
)
PHASE7_ACTION_TYPES = (
    RequestPhase7,
    Phase7Completed,
    Phase7Failed,
    Phase7Cancelled,
    SetPhase7Scenario,
    SelectDataset,
    SetDataIngestionView,
    SetDatasetWizardStep,
    SelectDatasetSource,
    AddDatasetFeatureStep,
    RemoveDatasetFeatureStep,
    MoveDatasetFeatureStep,
    SetIngestionScenario,
    RequestCredentialStatus,
    RequestCredentialSave,
    RequestCredentialTest,
    RequestCredentialDelete,
    CredentialCompleted,
    RequestFileBrowser,
    FileBrowserCompleted,
    CloseFileBrowser,
    SelectFileBrowserEntry,
    ScrollFileBrowser,
    RequestFilePreview,
    FilePreviewCompleted,
    UpdateFileMapping,
    SetFileSourceKind,
    UpdateIngestionForm,
    RequestSymbolDiscovery,
    SymbolDiscoveryCompleted,
    SetSelectedSymbols,
    ConfirmIngestion,
    RequestFileIngestion,
    RequestMassivePull,
    IngestionCompleted,
    IngestionProgressed,
    IngestionOperationFailed,
    IngestionOperationCancelled,
    RequestIngestionCancellation,
    RequestScheduleCommand,
    ScheduleCommandCompleted,
    SelectSchedule,
    RequestReconciliation,
    ReconciliationCompleted,
    RequestDatasetSave,
    DatasetSaveCompleted,
    RequestDatasetBuild,
    DatasetBuildCompleted,
    RequestDataImport,
    DataImportCompleted,
    EditExperimentDraft,
    RequestDraftEvaluation,
    DraftEvaluationCompleted,
    RequestDraftSave,
    DraftSaveCompleted,
    RequestSubmission,
    SubmissionCompleted,
)


@dataclass(frozen=True, slots=True)
class LoadPhase7:
    request: Phase7Request

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class RunDataImport:
    request: ImportRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class EvaluateDraft:
    request_id: str
    generation: int
    draft: ExperimentDraft


@dataclass(frozen=True, slots=True)
class SaveDraft:
    request: DraftSaveRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class SubmitExperiment:
    request: SubmissionRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class CancelPhase7:
    request_id: str


@dataclass(frozen=True, slots=True)
class LoadCredentialStatus:
    request: CredentialRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class SaveCredential:
    request: CredentialSecretRequest

    @property
    def request_id(self) -> str:
        return self.request.request.request_id


@dataclass(frozen=True, slots=True)
class TestCredential:
    request: CredentialSecretRequest

    @property
    def request_id(self) -> str:
        return self.request.request.request_id


@dataclass(frozen=True, slots=True)
class DeleteCredential:
    request: CredentialRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class PreviewFile:
    request: FilePreviewRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class BrowseFiles:
    request: FileBrowserRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class DiscoverSymbols:
    request: SymbolDiscoveryRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class RunFileIngestion:
    plan: IngestionPlan

    @property
    def request_id(self) -> str:
        return self.plan.request_id


@dataclass(frozen=True, slots=True)
class RunMassivePull:
    plan: IngestionPlan

    @property
    def request_id(self) -> str:
        return self.plan.request_id


@dataclass(frozen=True, slots=True)
class ExecuteScheduleCommand:
    command: ScheduleCommand

    @property
    def request_id(self) -> str:
        return self.command.request_id


@dataclass(frozen=True, slots=True)
class ReconcileData:
    request: ReconciliationRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class SaveDataset:
    request: DatasetSaveRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class BuildDataset:
    request: DatasetBuildRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


Phase7Effect = (
    LoadPhase7
    | RunDataImport
    | EvaluateDraft
    | SaveDraft
    | SubmitExperiment
    | CancelPhase7
    | LoadCredentialStatus
    | SaveCredential
    | TestCredential
    | DeleteCredential
    | BrowseFiles
    | PreviewFile
    | DiscoverSymbols
    | RunFileIngestion
    | RunMassivePull
    | ExecuteScheduleCommand
    | ReconcileData
    | SaveDataset
    | BuildDataset
)
