"""Versioned atomic credential document store."""

# pyright: reportUnknownArgumentType=false, reportUnknownVariableType=false

from __future__ import annotations

import json
import os
import secrets
import threading
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

from corthena.data.errors import CredentialDataError
from corthena.platform.credentials.protocol import CredentialPermissionProtocol


@dataclass(frozen=True, slots=True)
class StoredCredentialStatus:
    saved: bool
    updated_at: datetime | None = None


class CredentialStore:
    """Own one secret document without ever returning it as metadata."""

    def __init__(
        self,
        path: Path,
        quarantine: Path,
        permissions: CredentialPermissionProtocol,
        *,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
    ) -> None:
        self._path = path
        self._quarantine = quarantine
        self._permissions = permissions
        self._clock = clock
        self._lock = threading.RLock()

    def status(self) -> StoredCredentialStatus:
        with self._lock:
            if not self._path.exists():
                return StoredCredentialStatus(False)
            document = self._read_document()
            return StoredCredentialStatus(True, datetime.fromisoformat(document["updated_at"]))

    def load(self) -> str:
        with self._lock:
            return self._read_document()["token"]

    def save(self, token: str) -> StoredCredentialStatus:
        with self._lock:
            return self._save_locked(token)

    def _save_locked(self, token: str) -> StoredCredentialStatus:
        normalized = token.strip()
        if not normalized or normalized != token or len(token) > 4096:
            raise CredentialDataError("credential token is invalid")
        self._path.parent.mkdir(parents=True, exist_ok=True)
        now = self._clock()
        document = {
            "version": 1,
            "provider": "massive",
            "token": token,
            "updated_at": now.isoformat(),
        }
        temporary = self._path.with_name(f".{self._path.name}.{secrets.token_hex(8)}.tmp")
        try:
            with temporary.open("x", encoding="utf-8", newline="\n") as stream:
                json.dump(
                    document, stream, ensure_ascii=True, separators=(",", ":"), sort_keys=True
                )
                stream.flush()
                os.fsync(stream.fileno())
            self._permissions.protect(temporary)
            os.replace(temporary, self._path)
            self._permissions.verify(self._path)
        except Exception as error:
            temporary.unlink(missing_ok=True)
            if isinstance(error, CredentialDataError):
                raise
            raise CredentialDataError("credential document could not be saved") from error
        return StoredCredentialStatus(True, now)

    def delete(self) -> None:
        with self._lock:
            if self._path.exists():
                self._permissions.verify(self._path)
                self._path.unlink()

    def _read_document(self) -> dict[str, str]:
        try:
            self._permissions.verify(self._path)
            raw = self._path.read_text(encoding="utf-8")
            value = json.loads(raw)
            if not isinstance(value, dict) or set(value) != {
                "version",
                "provider",
                "token",
                "updated_at",
            }:
                raise ValueError
            if value["version"] != 1 or value["provider"] != "massive":
                raise ValueError
            token = value["token"]
            timestamp = value["updated_at"]
            if not isinstance(token, str) or not token or not isinstance(timestamp, str):
                raise ValueError
            datetime.fromisoformat(timestamp)
            return {"token": token, "updated_at": timestamp}
        except Exception as error:
            self._quarantine_corrupt()
            raise CredentialDataError(
                "credential document is corrupt and was quarantined"
            ) from error

    def _quarantine_corrupt(self) -> None:
        if not self._path.exists():
            return
        self._quarantine.mkdir(parents=True, exist_ok=True)
        destination = (
            self._quarantine
            / f"credential-{self._clock().strftime('%Y%m%dT%H%M%S')}-{secrets.token_hex(4)}.corrupt"
        )
        try:
            os.replace(self._path, destination)
        except OSError as error:
            raise CredentialDataError(
                "corrupt credential document could not be quarantined"
            ) from error
