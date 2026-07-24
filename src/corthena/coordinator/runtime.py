"""Coordinator composition root and dependency-ordered lifecycle."""

from __future__ import annotations

from corthena.config.paths import ApplicationPaths
from corthena.coordinator.data_service import DataCoordinatorService
from corthena.coordinator.scheduler import ScheduleRuntime
from corthena.data.calendar import XnysCalendar
from corthena.data.massive import MassiveConnector
from corthena.persistence.data import DataRepository
from corthena.platform.credentials.store import CredentialStore
from corthena.platform.credentials.windows import WindowsCredentialPermissions


class CoordinatorRuntime:
    """Own the scheduler, service, adapters, and repository in close order."""

    def __init__(self, paths: ApplicationPaths | None = None) -> None:
        self.paths = paths or ApplicationPaths.for_user()
        self.paths.ensure()
        self.repository = DataRepository(self.paths.database)
        permissions = WindowsCredentialPermissions()
        credentials = CredentialStore(
            self.paths.credentials,
            self.paths.quarantine,
            permissions,
        )
        self.service = DataCoordinatorService(
            self.paths,
            self.repository,
            credentials,
            MassiveConnector(),
            XnysCalendar(),
        )
        self.scheduler = ScheduleRuntime(self.service)
        self._started = False
        self._closed = False

    def start(self) -> None:
        if self._closed:
            raise RuntimeError("coordinator runtime is closed")
        if not self._started:
            self.scheduler.start()
            self._started = True

    def close(self) -> None:
        if self._closed:
            return
        if self._started:
            self.scheduler.close()
            self._started = False
        self.service.close()
        self.repository.close()
        self._closed = True

    def __enter__(self) -> CoordinatorRuntime:
        self.start()
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        del exc_type, exc, traceback
        self.close()
