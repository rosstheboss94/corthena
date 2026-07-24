"""Bounded thread-safe Data event history for WebSocket hints."""

from __future__ import annotations

import threading
from collections import deque
from collections.abc import Callable
from datetime import UTC, datetime

from corthena.contracts.data import DataEventDTO, ProgressDTO


class DataEventBus:
    """Own a bounded event sequence and wake waiting subscribers."""

    def __init__(
        self,
        capacity: int = 512,
        *,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
    ) -> None:
        if capacity < 1:
            raise ValueError("event capacity must be positive")
        self._events: deque[DataEventDTO] = deque(maxlen=capacity)
        self._condition = threading.Condition()
        self._sequence = 0
        self._closed = False
        self._clock = clock

    def publish_progress(self, progress: ProgressDTO) -> DataEventDTO:
        with self._condition:
            self._sequence += 1
            event = DataEventDTO(
                event_id=f"event-{self._sequence:020d}",
                sequence=self._sequence,
                event_type="import.progress",
                timestamp=self._clock(),
                correlation_id=progress.correlation_id,
                operation_id=progress.operation_id,
                progress=progress,
            )
            self._events.append(event)
            self._condition.notify_all()
            return event

    def publish_catalog(self, progress: ProgressDTO, dataset_id: str) -> DataEventDTO:
        with self._condition:
            self._sequence += 1
            event = DataEventDTO(
                event_id=f"event-{self._sequence:020d}",
                sequence=self._sequence,
                event_type="catalog.invalidated",
                timestamp=self._clock(),
                correlation_id=progress.correlation_id,
                operation_id=progress.operation_id,
                dataset_id=dataset_id,
            )
            self._events.append(event)
            self._condition.notify_all()
            return event

    def after(self, sequence: int, timeout: float = 1.0) -> tuple[DataEventDTO, ...]:
        with self._condition:
            if not self._closed and not any(item.sequence > sequence for item in self._events):
                self._condition.wait(timeout)
            return tuple(item for item in self._events if item.sequence > sequence)

    def close(self) -> None:
        with self._condition:
            self._closed = True
            self._condition.notify_all()

    @property
    def sequence(self) -> int:
        with self._condition:
            return self._sequence
