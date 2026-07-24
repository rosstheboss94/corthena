"""Coordinator-owned application-data paths."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from platformdirs import user_data_path


@dataclass(frozen=True, slots=True)
class ApplicationPaths:
    root: Path
    database: Path
    revisions: Path
    credentials: Path
    quarantine: Path

    @classmethod
    def for_user(cls) -> ApplicationPaths:
        return cls.from_root(Path(user_data_path("Corthena", appauthor="Corthena", roaming=False)))

    @classmethod
    def from_root(cls, root: Path) -> ApplicationPaths:
        resolved = root.expanduser().resolve()
        return cls(
            resolved,
            resolved / "corthena.sqlite3",
            resolved / "data" / "revisions",
            resolved / "credentials" / "massive.v1.json",
            resolved / "quarantine",
        )

    def ensure(self) -> None:
        for directory in (
            self.root,
            self.database.parent,
            self.revisions,
            self.credentials.parent,
            self.quarantine,
        ):
            directory.mkdir(parents=True, exist_ok=True)
