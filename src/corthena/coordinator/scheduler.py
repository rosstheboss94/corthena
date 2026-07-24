"""Bounded coordinator-owned hourly/daily schedule runtime."""

from __future__ import annotations

import threading
from collections.abc import Callable
from datetime import UTC, datetime

from corthena.contracts.data import ScheduleDTO
from corthena.coordinator.data_service import DataCoordinatorService
from corthena.data.errors import CapacityDataError, DataError


class ScheduleRuntime:
    """Coalesce due work to one submission per schedule while open."""

    def __init__(
        self,
        service: DataCoordinatorService,
        *,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
        poll_seconds: float = 1.0,
        max_due_per_poll: int = 8,
    ) -> None:
        if poll_seconds <= 0 or max_due_per_poll < 1:
            raise ValueError("scheduler bounds are invalid")
        self._service = service
        self._clock = clock
        self._poll_seconds = poll_seconds
        self._max_due = max_due_per_poll
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        if self._thread is not None:
            raise RuntimeError("scheduler is already running")
        self._thread = threading.Thread(target=self._run, name="data-scheduler", daemon=False)
        self._thread.start()

    def close(self, timeout_seconds: float = 5.0) -> None:
        self._stop.set()
        thread = self._thread
        if thread is not None:
            thread.join(timeout_seconds)
            if thread.is_alive():
                raise RuntimeError("scheduler did not stop within its deadline")
        self._thread = None

    def run_due_once(self) -> tuple[str, ...]:
        now = self._clock()
        due = tuple(
            schedule
            for schedule in self._service.schedules()
            if schedule.enabled and (schedule.next_run_at is None or schedule.next_run_at <= now)
        )[: self._max_due]
        accepted: list[str] = []
        for schedule in due:
            identity = self._command_identity(schedule, now)
            try:
                operation = self._service.run_schedule(
                    schedule.schedule_id,
                    identity,
                    f"schedule-{schedule.schedule_id}-{identity[-12:]}",
                )
            except CapacityDataError, DataError:
                continue
            accepted.append(operation.operation_id)
        return tuple(accepted)

    def _run(self) -> None:
        while not self._stop.wait(self._poll_seconds):
            self.run_due_once()

    @staticmethod
    def _command_identity(schedule: ScheduleDTO, now: datetime) -> str:
        bucket = (
            now.strftime("%Y%m%d%H")
            if schedule.cadence.value == "hourly"
            else now.strftime("%Y%m%d")
        )
        return f"scheduled-{schedule.schedule_id}-{bucket}"
