"""Bundled-asset staging outside the native UI boundary."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True, slots=True)
class StagedAsset:
    """Immutable bytes and identity prepared before native initialization."""

    path: Path
    content: bytes
    sha256: str


def legacy_asset_root() -> Path:
    """Locate the canonical bundled assets retained for migration parity."""
    return Path(__file__).resolve().parents[4] / "internal" / "frontend" / "assets"


def stage_assets() -> tuple[StagedAsset, StagedAsset, StagedAsset]:
    """Read and fingerprint all Phase 0 assets before Raylib starts."""
    root = legacy_asset_root()
    paths = (
        root / "fonts" / "InterVariable.ttf",
        root / "fonts" / "JetBrainsMono-Regular.ttf",
        root / "icons" / "lucide-atlas.png",
    )
    assets: list[StagedAsset] = []
    for path in paths:
        content = path.read_bytes()
        if not content:
            raise ValueError(f"bundled asset is empty: {path}")
        assets.append(StagedAsset(path, content, hashlib.sha256(content).hexdigest()))
    return assets[0], assets[1], assets[2]
