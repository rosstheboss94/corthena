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
        self._effects: queue.Queue[LoadSnapshot | LoadResearch | None] = queue.Queue(
            config.effect_capacity
        )
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
            if isinstance(effect, (CancelRequest, CancelResearch)):
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
                else:
                    research = self._client.load_research(effect.query, cancellation)
                    action = ResearchCompleted(research)
            except RequestCancelledError:
                if isinstance(effect, LoadSnapshot):
                    continue
                action = ResearchCancelled(effect.query.group_id, effect.query.generation)
            except Exception as error:
                action = (
                    SnapshotFailed(effect.request_id, effect.generation, str(error))
                    if isinstance(effect, LoadSnapshot)
                    else ResearchFailed(
                        effect.query.group_id,
                        effect.query.generation,
                        str(error),
                    )
                )
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
    def _busy_action(effect: LoadSnapshot | LoadResearch) -> UIAction:
        if isinstance(effect, LoadSnapshot):
            return RuntimeBusy(effect.request_id, effect.generation)
        return ResearchFailed(
            effect.query.group_id,
            effect.query.generation,
            "Research effect queue is busy",
            busy=True,
        )


__all__ = [
    "EffectsRuntime",
    "EnqueueResult",
    "EnqueueState",
    "RuntimeClosedError",
    "RuntimeConfig",
]
