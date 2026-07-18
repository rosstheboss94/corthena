"""Data-only contract for native UI compatibility evidence."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True, slots=True)
class UiEvidence:
    """Native UI evidence containing no Raylib values."""

    owner_thread: int
    asset_sha256: tuple[str, str, str]
    capture: Path
