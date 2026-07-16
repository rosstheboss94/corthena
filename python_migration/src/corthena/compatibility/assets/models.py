"""Data-only contracts for bundled-asset compatibility evidence."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True, slots=True)
class StagedAsset:
    """Immutable bytes and identity prepared before native initialization."""

    path: Path
    content: bytes
    sha256: str
