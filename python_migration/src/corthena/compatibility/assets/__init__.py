"""Bundled-asset compatibility capability."""

from corthena.compatibility.assets.legacy import LegacyAssetStager, legacy_asset_root
from corthena.compatibility.assets.models import StagedAsset
from corthena.compatibility.assets.protocol import AssetStagerProtocol


def stage_assets() -> tuple[StagedAsset, StagedAsset, StagedAsset]:
    """Stage the canonical Phase 0 assets with the default adapter."""
    return LegacyAssetStager().stage()


__all__ = (
    "AssetStagerProtocol",
    "LegacyAssetStager",
    "StagedAsset",
    "legacy_asset_root",
    "stage_assets",
)
