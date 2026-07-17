"""Bounded, owned Phase 2 effects runtime.

The UI thread is the sole effect sender and action receiver. Worker threads are
the effect receivers and result senders. ``close`` owns cancellation, queue
closure, and bounded thread joins. Both queues are bounded; enqueue is always
nonblocking and saturation is returned as a typed ``RuntimeBusy`` action.
"""

from __future__ import annotations

import queue
import threading
from dataclasses import dataclass
from enum import StrEnum

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import UIClientProtocol
from corthena.ui.data_experiments.actions import (
    CancelPhase7,
    DataImportCompleted,
    DraftEvaluationCompleted,
    DraftSaveCompleted,
    EvaluateDraft,
    LoadPhase7,
    Phase7Cancelled,
    Phase7Completed,
    Phase7Effect,
    Phase7Failed,
    RunDataImport,
    SaveDraft,
    SubmissionCompleted,
    SubmitExperiment,
)
from corthena.ui.data_experiments.models import Phase7Workspace
from corthena.ui.jobs_results.actions import (
    CancelPhase8,
    CompareRuns,
    ComparisonCancelled,
    ComparisonCompleted,
    ComparisonFailed,
    ExecuteJobCommand,
    JobCommandCompleted,
    LoadPhase8,
    Phase8Cancelled,
    Phase8Completed,
    Phase8Effect,
    Phase8Failed,
)
from corthena.ui.jobs_results.models import Phase8Workspace
from corthena.ui.models_inference.actions import (
    AliasAssignmentCompleted,
    AssignAlias,
    CancelPhase9,
    ExportCompleted,
    ExportFailed,
    InferenceCancelled,
    InferenceCompleted,
    InferenceFailed,
    LoadPhase9,
    Phase9Cancelled,
    Phase9Completed,
    Phase9Effect,
    Phase9Failed,
    PrepareExport,
    ScoreInference,
)
from corthena.ui.models_inference.models import Phase9Workspace
from corthena.ui.research.actions import (
    CancelResearch,
    LoadResearch,
    ResearchCancelled,
    ResearchCompleted,
    ResearchFailed,
)
from corthena.ui.state import (
    CancelRequest,
    LoadSnapshot,
    RuntimeBusy,
    SnapshotCompleted,
    SnapshotFailed,
    UIAction,
    UIEffect,
)


class RuntimeClosedError(RuntimeError):
    """Raised when work is submitted after runtime ownership has ended."""


class EnqueueState(StrEnum):
    """Nonblocking submission outcome."""

    ACCEPTED = "accepted"
    CANCELLED = "cancelled"
    BUSY = "busy"


@dataclass(frozen=True, slots=True)
class EnqueueResult:
    """Typed saturation result for a render-thread submission."""

    state: EnqueueState
    action: UIAction | None = None


@dataclass(frozen=True, slots=True)
class RuntimeConfig:
    """Bounds for queues, draining, workers, and shutdown."""

    worker_count: int = 1
    effect_capacity: int = 8
    action_capacity: int = 16
    max_actions_per_drain: int = 4
    shutdown_timeout_seconds: float = 2.0

    def __post_init__(self) -> None:
        bounds = (
            self.worker_count,
            self.effect_capacity,
            self.action_capacity,
            self.max_actions_per_drain,
        )
        if min(bounds) < 1:
            raise ValueError("worker and queue bounds must be positive")
        if self.shutdown_timeout_seconds <= 0:
            raise ValueError("shutdown timeout must be positive")


class EffectsRuntime:
    """Execute client effects on owned workers without blocking the UI sender."""

    def __init__(self, client: UIClientProtocol, config: RuntimeConfig | None = None) -> None:
        if config is None:
            config = RuntimeConfig()
        self._client = client
        self._config = config
        self._effects: queue.Queue[
            LoadSnapshot | LoadResearch | Phase7Effect | Phase8Effect | Phase9Effect | None
        ] = queue.Queue(config.effect_capacity)
        self._actions: queue.Queue[UIAction] = queue.Queue(config.action_capacity)
        self._lock = threading.Lock()
        self._cancellations: dict[str, threading.Event] = {}
        self._closed = False
        self._threads = tuple(
            threading.Thread(target=self._worker, name=f"corthena-effects-{index}", daemon=False)
            for index in range(config.worker_count)
        )
        for thread in self._threads:
            thread.start()

    @property
    def worker_names(self) -> tuple[str, ...]:
        """Expose stable worker identities for lifecycle evidence."""
        return tuple(thread.name for thread in self._threads)

    def enqueue(self, effect: UIEffect) -> EnqueueResult:
        """Submit without blocking, or cancel synchronously by identity."""
        with self._lock:
            if self._closed:
                raise RuntimeClosedError("effects runtime is closed")
            if isinstance(
                effect, (CancelRequest, CancelResearch, CancelPhase7, CancelPhase8, CancelPhase9)
            ):
                cancellation = self._cancellations.get(effect.request_id)
                if cancellation is not None:
                    cancellation.set()
                return EnqueueResult(EnqueueState.CANCELLED)
            if effect.request_id in self._cancellations:
                return EnqueueResult(
                    EnqueueState.BUSY,
                    self._busy_action(effect),
                )
            cancellation = threading.Event()
            try:
                self._effects.put_nowait(effect)
            except queue.Full:
                return EnqueueResult(
                    EnqueueState.BUSY,
                    self._busy_action(effect),
                )
            self._cancellations[effect.request_id] = cancellation
            return EnqueueResult(EnqueueState.ACCEPTED)

    def drain(self, limit: int | None = None) -> tuple[UIAction, ...]:
        """Drain at most the configured per-frame bound in FIFO publication order."""
        bound = self._config.max_actions_per_drain if limit is None else limit
        if bound < 0 or bound > self._config.max_actions_per_drain:
            raise ValueError("drain limit exceeds the configured per-frame bound")
        actions: list[UIAction] = []
        for _ in range(bound):
            try:
                actions.append(self._actions.get_nowait())
            except queue.Empty:
                break
        return tuple(actions)

    def close(self) -> None:
        """Cancel work and join every owned worker within the configured bound."""
        with self._lock:
            if self._closed:
                return
            self._closed = True
            for cancellation in self._cancellations.values():
                cancellation.set()
        # Workers use timed result publication, so cancellation always lets them progress.
        for _ in self._threads:
            while True:
                try:
                    self._effects.put(None, timeout=0.01)
                    break
                except queue.Full:
                    self._discard_pending()
        for thread in self._threads:
            thread.join(self._config.shutdown_timeout_seconds)
        alive = tuple(thread.name for thread in self._threads if thread.is_alive())
        if alive:
            raise RuntimeError(f"effects workers did not terminate: {alive!r}")

    def __enter__(self) -> EffectsRuntime:
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.close()

    def _discard_pending(self) -> None:
        try:
            pending = self._effects.get_nowait()
        except queue.Empty:
            return
        if pending is not None:
            with self._lock:
                cancellation = self._cancellations.pop(pending.request_id, None)
                if cancellation is not None:
                    cancellation.set()

    def _worker(self) -> None:
        while True:
            effect = self._effects.get()
            if effect is None:
                return
            if isinstance(effect, (CancelPhase7, CancelPhase8, CancelPhase9)):
                continue
            with self._lock:
                cancellation = self._cancellations.get(effect.request_id)
            if cancellation is None:
                continue
            try:
                if isinstance(effect, LoadSnapshot):
                    snapshot = self._client.load_snapshot(
                        effect.request_id, effect.generation, cancellation
                    )
                    action: UIAction = SnapshotCompleted(snapshot)
                elif isinstance(effect, LoadResearch):
                    research = self._client.load_research(effect.query, cancellation)
                    action = ResearchCompleted(research)
                elif isinstance(effect, LoadPhase7):
                    action = Phase7Completed(self._client.load_phase7(effect.request, cancellation))
                elif isinstance(effect, RunDataImport):
                    action = DataImportCompleted(
                        self._client.import_data(effect.request, cancellation)
                    )
                elif isinstance(effect, EvaluateDraft):
                    action = DraftEvaluationCompleted(
                        self._client.evaluate_draft(
                            effect.request_id, effect.generation, effect.draft, cancellation
                        )
                    )
                elif isinstance(effect, SaveDraft):
                    action = DraftSaveCompleted(
                        self._client.save_draft(effect.request, cancellation)
                    )
                elif isinstance(effect, SubmitExperiment):
                    definition = self._client.submit_experiment(effect.request, cancellation)
                    action = SubmissionCompleted(effect.request, definition)
                elif isinstance(effect, LoadPhase8):
                    action = Phase8Completed(self._client.load_phase8(effect.request, cancellation))
                elif isinstance(effect, LoadPhase9):
                    action = Phase9Completed(self._client.load_phase9(effect.request, cancellation))
                elif isinstance(effect, AssignAlias):
                    action = AliasAssignmentCompleted(
                        self._client.assign_alias(effect.command, cancellation)
                    )
                elif isinstance(effect, ScoreInference):
                    action = InferenceCompleted(
                        self._client.score_inference(effect.query, cancellation)
                    )
                elif isinstance(effect, PrepareExport):
                    action = ExportCompleted(
                        self._client.prepare_export(effect.request, cancellation)
                    )
                elif isinstance(effect, ExecuteJobCommand):
                    action = JobCommandCompleted(
                        self._client.command_job(effect.command, cancellation)
                    )
                else:
                    action = ComparisonCompleted(
                        self._client.compare_runs(effect.query, cancellation)
                    )
            except RequestCancelledError:
                if isinstance(effect, LoadSnapshot):
                    continue
                elif isinstance(effect, LoadResearch):
                    action = ResearchCancelled(effect.query.group_id, effect.query.generation)
                elif isinstance(effect, (LoadPhase8, ExecuteJobCommand)):
                    workspace, generation = self._phase8_identity(effect)
                    action = Phase8Cancelled(workspace, generation)
                elif isinstance(effect, LoadPhase9):
                    action = Phase9Cancelled(effect.request.workspace, effect.request.generation)
                elif isinstance(effect, ScoreInference):
                    action = InferenceCancelled(effect.query.request_id, effect.query.generation)
                elif isinstance(effect, (AssignAlias, PrepareExport)):
                    continue
                elif isinstance(
                    effect, (LoadPhase7, RunDataImport, EvaluateDraft, SaveDraft, SubmitExperiment)
                ):
                    workspace, generation = self._phase7_identity(effect)
                    action = Phase7Cancelled(workspace, generation)
                else:
                    action = ComparisonCancelled(effect.query.request_id, effect.query.generation)
            except Exception as error:
                if isinstance(effect, LoadSnapshot):
                    action = SnapshotFailed(effect.request_id, effect.generation, str(error))
                elif isinstance(effect, LoadResearch):
                    action = ResearchFailed(
                        effect.query.group_id,
                        effect.query.generation,
                        str(error),
                    )
                elif isinstance(effect, CompareRuns):
                    action = ComparisonFailed(
                        effect.query.request_id,
                        effect.query.generation,
                        str(error),
                    )
                elif isinstance(effect, (LoadPhase8, ExecuteJobCommand)):
                    workspace, generation = self._phase8_identity(effect)
                    action = Phase8Failed(workspace, generation, str(error))
                elif isinstance(effect, LoadPhase9):
                    action = Phase9Failed(
                        effect.request.workspace, effect.request.generation, str(error)
                    )
                elif isinstance(effect, ScoreInference):
                    action = InferenceFailed(
                        effect.query.request_id, effect.query.generation, str(error)
                    )
                elif isinstance(effect, PrepareExport):
                    action = ExportFailed(
                        effect.request.request_id, effect.request.generation, str(error)
                    )
                elif isinstance(effect, AssignAlias):
                    action = Phase9Failed(
                        Phase9Workspace.MODELS, effect.command.generation, str(error)
                    )
                else:
                    workspace, generation = self._phase7_identity(effect)
                    action = Phase7Failed(workspace, generation, str(error))
            finally:
                with self._lock:
                    self._cancellations.pop(effect.request_id, None)
            while not cancellation.is_set():
                try:
                    self._actions.put(action, timeout=0.01)
                    break
                except queue.Full:
                    continue

    @staticmethod
    def _busy_action(
        effect: LoadSnapshot | LoadResearch | Phase7Effect | Phase8Effect | Phase9Effect,
    ) -> UIAction:
        if isinstance(effect, LoadSnapshot):
            return RuntimeBusy(effect.request_id, effect.generation)
        if isinstance(effect, LoadResearch):
            return ResearchFailed(
                effect.query.group_id,
                effect.query.generation,
                "Research effect queue is busy",
                busy=True,
            )
        if isinstance(effect, CompareRuns):
            return ComparisonFailed(
                effect.query.request_id,
                effect.query.generation,
                "Phase 8 comparison queue is busy",
                busy=True,
            )
        if isinstance(effect, (LoadPhase8, ExecuteJobCommand)):
            workspace, generation = EffectsRuntime._phase8_identity(effect)
            return Phase8Failed(workspace, generation, "Phase 8 effect queue is busy", busy=True)
        if isinstance(effect, LoadPhase9):
            return Phase9Failed(
                effect.request.workspace,
                effect.request.generation,
                "Phase 9 effect queue is busy",
                busy=True,
            )
        if isinstance(effect, ScoreInference):
            return InferenceFailed(
                effect.query.request_id,
                effect.query.generation,
                "Phase 9 inference queue is busy",
                busy=True,
            )
        if isinstance(effect, PrepareExport):
            return ExportFailed(
                effect.request.request_id,
                effect.request.generation,
                "Phase 9 export queue is busy",
                busy=True,
            )
        if isinstance(effect, AssignAlias):
            return Phase9Failed(
                Phase9Workspace.MODELS,
                effect.command.generation,
                "Phase 9 alias queue is busy",
                busy=True,
            )
        if isinstance(effect, (CancelPhase7, CancelPhase8, CancelPhase9)):
            raise AssertionError("cancellation effects are handled synchronously")
        workspace, generation = EffectsRuntime._phase7_identity(effect)
        return Phase7Failed(workspace, generation, "Phase 7 effect queue is busy", busy=True)

    @staticmethod
    def _phase7_identity(effect: Phase7Effect) -> tuple[Phase7Workspace, int]:
        if isinstance(effect, LoadPhase7):
            return effect.request.workspace, effect.request.generation
        if isinstance(effect, RunDataImport):
            return Phase7Workspace.DATA, effect.request.generation
        if isinstance(effect, EvaluateDraft):
            return Phase7Workspace.EXPERIMENTS, effect.generation
        if isinstance(effect, (SaveDraft, SubmitExperiment)):
            return Phase7Workspace.EXPERIMENTS, effect.request.generation
        return Phase7Workspace.EXPERIMENTS, 0

    @staticmethod
    def _phase8_identity(effect: LoadPhase8 | ExecuteJobCommand) -> tuple[Phase8Workspace, int]:
        if isinstance(effect, LoadPhase8):
            return effect.request.workspace, effect.request.generation
        return Phase8Workspace.JOBS, effect.command.generation


__all__ = [
    "EffectsRuntime",
    "EnqueueResult",
    "EnqueueState",
    "RuntimeClosedError",
    "RuntimeConfig",
]
