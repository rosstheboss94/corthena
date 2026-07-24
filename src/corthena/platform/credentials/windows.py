"""Fail-closed Windows DACL adapter for plaintext credential documents."""

from __future__ import annotations

import csv
import io
import os
import re
import subprocess
import tempfile
from pathlib import Path

from corthena.data.errors import CredentialDataError

_SYSTEM_SID = "S-1-5-18"


class WindowsCredentialPermissions:
    """Restrict a file to the current user and Windows SYSTEM."""

    def __init__(self) -> None:
        if os.name != "nt":
            raise CredentialDataError("credential permissions require Windows")
        self._user_sid = self._read_user_sid()

    def protect(self, path: Path) -> None:
        self._run(
            "icacls",
            str(path),
            "/inheritance:r",
            "/grant:r",
            f"*{self._user_sid}:(F)",
            f"*{_SYSTEM_SID}:(F)",
        )
        self.verify(path)

    def verify(self, path: Path) -> None:
        with tempfile.NamedTemporaryFile(
            prefix="corthena-acl-", suffix=".txt", delete=False
        ) as stream:
            acl_path = Path(stream.name)
        try:
            acl_path.unlink()
            self._run("icacls", str(path), "/save", str(acl_path), "/c")
            acl = acl_path.read_text(encoding="utf-16-le").lstrip("\ufeff")
        finally:
            acl_path.unlink(missing_ok=True)
        aces = tuple(match.split(";") for match in re.findall(r"\(([^()]*)\)", acl))
        if len(aces) != 2 or any(
            len(ace) != 6 or ace[0] != "A" or ace[1] or ace[2] != "FA" for ace in aces
        ):
            raise CredentialDataError("credential permissions could not be verified")
        principals = {ace[5] for ace in aces}
        system_present = "SY" in principals or _SYSTEM_SID in principals
        if principals - {"SY", _SYSTEM_SID, self._user_sid}:
            raise CredentialDataError("credential permissions could not be verified")
        if self._user_sid not in principals or not system_present or not acl.startswith(path.name):
            raise CredentialDataError("credential permissions could not be verified")

    @staticmethod
    def _read_user_sid() -> str:
        result = WindowsCredentialPermissions._run("whoami", "/user", "/fo", "csv", "/nh")
        rows = tuple(csv.reader(io.StringIO(result.stdout)))
        if len(rows) != 1 or len(rows[0]) < 2 or not rows[0][1].startswith("S-1-"):
            raise CredentialDataError("current Windows user identity could not be determined")
        return rows[0][1]

    @staticmethod
    def _run(*arguments: str) -> subprocess.CompletedProcess[str]:
        try:
            return subprocess.run(
                arguments,
                check=True,
                capture_output=True,
                text=True,
                timeout=10,
            )
        except (OSError, subprocess.SubprocessError) as error:
            raise CredentialDataError("credential permission operation failed") from error
