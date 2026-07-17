"""Closed Phase 8 actions and effects."""

from dataclasses import dataclass

from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    JobCommand,
    JobCommandResult,
    Phase8Request,
    Phase8Scenario,
    Phase8Snapshot,
    Phase8Workspace,
    RunComparison,
)


@dataclass(frozen=True, slots=True)
class RequestPhase8:
    request: Phase8Request


@dataclass(frozen=True, slots=True)
class Phase8Completed:
    snapshot: Phase8Snapshot


@dataclass(frozen=True, slots=True)
class Phase8Failed:
    workspace: Phase8Workspace
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class Phase8Cancelled:
    workspace: Phase8Workspace
    generation: int


@dataclass(frozen=True, slots=True)
class SetPhase8Scenario:
    workspace: Phase8Workspace
    scenario: Phase8Scenario


@dataclass(frozen=True, slots=True)
class SelectJob:
    job_id: str


@dataclass(frozen=True, slots=True)
class RequestJobCommand:
    command: JobCommand


@dataclass(frozen=True, slots=True)
class JobCommandCompleted:
    result: JobCommandResult


@dataclass(frozen=True, slots=True)
class SelectComparisonRuns:
    run_ids: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class RequestComparison:
    query: ComparisonQuery


@dataclass(frozen=True, slots=True)
class ComparisonCompleted:
    comparison: RunComparison


@dataclass(frozen=True, slots=True)
class ComparisonFailed:
    request_id: str
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class ComparisonCancelled:
    request_id: str
    generation: int


Phase8Action = (
    RequestPhase8
    | Phase8Completed
    | Phase8Failed
    | Phase8Cancelled
    | SetPhase8Scenario
    | SelectJob
    | RequestJobCommand
    | JobCommandCompleted
    | SelectComparisonRuns
    | RequestComparison
    | ComparisonCompleted
    | ComparisonFailed
    | ComparisonCancelled
)
PHASE8_ACTION_TYPES = (
    RequestPhase8,
    Phase8Completed,
    Phase8Failed,
    Phase8Cancelled,
    SetPhase8Scenario,
    SelectJob,
    RequestJobCommand,
    JobCommandCompleted,
    SelectComparisonRuns,
    RequestComparison,
    ComparisonCompleted,
    ComparisonFailed,
    ComparisonCancelled,
)


@dataclass(frozen=True, slots=True)
class LoadPhase8:
    request: Phase8Request

    @property
    def request_id(self) -> str:
        return self.request.request_id


@dataclass(frozen=True, slots=True)
class ExecuteJobCommand:
    command: JobCommand

    @property
    def request_id(self) -> str:
        return self.command.request_id


@dataclass(frozen=True, slots=True)
class CompareRuns:
    query: ComparisonQuery

    @property
    def request_id(self) -> str:
        return self.query.request_id


@dataclass(frozen=True, slots=True)
class CancelPhase8:
    request_id: str


Phase8Effect = LoadPhase8 | ExecuteJobCommand | CompareRuns | CancelPhase8
