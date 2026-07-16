"""Legacy bundled-asset staging adapter."""

from __future__ import annotations

import hashlib
from pathlib import Path

from corthena.compatibility.assets.models import StagedAsset


def legacy_asset_root() -> Path:
    """Locate the canonical bundled assets retained for migration parity."""
    return Path(__file__).resolve().parents[5] / "internal" / "frontend" / "assets"


class LegacyAssetStager:
    """Stage the canonical migration-parity assets from the legacy tree."""

    def stage(self) -> tuple[StagedAsset, StagedAsset, StagedAsset]:
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
