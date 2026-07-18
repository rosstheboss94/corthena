"""Raylib UI compatibility capability."""

from pathlib import Path

from corthena.compatibility.assets.legacy import LegacyAssetStager
from corthena.compatibility.ui.models import UiEvidence
from corthena.compatibility.ui.protocol import UiProbeProtocol
from corthena.compatibility.ui.raylib_probe import RaylibUiProbe


def capture_hidden_frame(capture: Path) -> UiEvidence:
    """Capture a hidden frame with the default Raylib and asset adapters."""
    return RaylibUiProbe(LegacyAssetStager()).capture(capture)


__all__ = ("RaylibUiProbe", "UiEvidence", "UiProbeProtocol", "capture_hidden_frame")
