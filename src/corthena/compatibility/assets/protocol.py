from typing import Protocol

from corthena.compatibility.assets.models import StagedAsset


class AssetStagerProtocol(Protocol):
    def stage(self) -> tuple[StagedAsset, StagedAsset, StagedAsset]: ...
