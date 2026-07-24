from collections.abc import Callable
from datetime import datetime
from pathlib import Path

from corthena.platform.credentials.protocol import CredentialPermissionProtocol

class StoredCredentialStatus:
    @property
    def saved(self) -> bool: ...
    @property
    def updated_at(self) -> datetime | None: ...

class CredentialStore:
    def __init__(
        self,
        path: Path,
        quarantine: Path,
        permissions: CredentialPermissionProtocol,
        *,
        clock: Callable[[], datetime] = ...,
    ) -> None: ...
    def status(self) -> StoredCredentialStatus: ...
    def load(self) -> str: ...
    def save(self, token: str) -> StoredCredentialStatus: ...
    def delete(self) -> None: ...
