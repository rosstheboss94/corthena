"""Closed Phase 7 actions and effects."""

from dataclasses import dataclass

from corthena.ui.data_experiments.models import (
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    ImportRequest,
    ImportResult,
    Phase7Request,
    Phase7Scenario,
    Phase7Snapshot,
    Phase7Workspace,
    SubmissionRequest,
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


Phase7Effect = (
    LoadPhase7 | RunDataImport | EvaluateDraft | SaveDraft | SubmitExperiment | CancelPhase7
)
