"""Agent-facing contract for restrictive credential-file permissions."""

from pathlib import Path
from typing import Protocol


class CredentialPermissionProtocol(Protocol):
    def protect(self, path: Path) -> None: ...

    def verify(self, path: Path) -> None: ...
