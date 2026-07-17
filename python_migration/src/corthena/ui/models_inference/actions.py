"""Closed Phase 9 actions and effects."""

from dataclasses import dataclass

from corthena.ui.models_inference.models import (
    AliasCommand,
    AliasResult,
    ExportRequest,
    ExportResult,
    InferenceQuery,
    InferenceSnapshot,
    Phase9Request,
    Phase9Scenario,
    Phase9Snapshot,
    Phase9Workspace,
)


@dataclass(frozen=True, slots=True)
class RequestPhase9:
    request: Phase9Request


@dataclass(frozen=True, slots=True)
class Phase9Completed:
    snapshot: Phase9Snapshot


@dataclass(frozen=True, slots=True)
class Phase9Failed:
    workspace: Phase9Workspace
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class Phase9Cancelled:
    workspace: Phase9Workspace
    generation: int


@dataclass(frozen=True, slots=True)
class SetPhase9Scenario:
    workspace: Phase9Workspace
    scenario: Phase9Scenario


@dataclass(frozen=True, slots=True)
class SelectModel:
    model_id: str


@dataclass(frozen=True, slots=True)
class SelectTree:
    tree_index: int


@dataclass(frozen=True, slots=True)
class RequestAliasAssignment:
    command: AliasCommand


@dataclass(frozen=True, slots=True)
class AliasAssignmentCompleted:
    result: AliasResult


@dataclass(frozen=True, slots=True)
class RequestInference:
    query: InferenceQuery


@dataclass(frozen=True, slots=True)
class InferenceCompleted:
    snapshot: InferenceSnapshot


@dataclass(frozen=True, slots=True)
class InferenceFailed:
    request_id: str
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class InferenceCancelled:
    request_id: str
    generation: int


@dataclass(frozen=True, slots=True)
class RequestExport:
    request: ExportRequest


@dataclass(frozen=True, slots=True)
class ExportCompleted:
    result: ExportResult


@dataclass(frozen=True, slots=True)
class ExportFailed:
    request_id: str
    generation: int
    message: str
    busy: bool = False


Phase9Action = (
    RequestPhase9
    | Phase9Completed
    | Phase9Failed
    | Phase9Cancelled
    | SetPhase9Scenario
    | SelectModel
    | SelectTree
    | RequestAliasAssignment
    | AliasAssignmentCompleted
    | RequestInference
    | InferenceCompleted
    | InferenceFailed
    | InferenceCancelled
    | RequestExport
    | ExportCompleted
    | ExportFailed
)
PHASE9_ACTION_TYPES = (
    RequestPhase9,
    Phase9Completed,
    Phase9Failed,
    Phase9Cancelled,
    SetPhase9Scenario,
    SelectModel,
    SelectTree,
    RequestAliasAssignment,
    AliasAssignmentCompleted,
    RequestInference,
    InferenceCompleted,
    InferenceFailed,
    InferenceCancelled,
    RequestExport,
    ExportCompleted,
    ExportFailed,
)


@dataclass(frozen=True, slots=True)
class LoadPhase9:
    request: Phase9Request

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class AssignAlias:
    command: AliasCommand

    @property
    def request_id(self) -> str:
        return self.command.request_id


@dataclass(frozen=True, slots=True)
class ScoreInference:
    query: InferenceQuery

    @property
    def request_id(self) -> str:
        return self.query.request_id


@dataclass(frozen=True, slots=True)
class PrepareExport:
    request: ExportRequest

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class CancelPhase9:
    request_id: str


Phase9Effect = LoadPhase9 | AssignAlias | ScoreInference | PrepareExport | CancelPhase9
