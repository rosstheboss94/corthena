import tempfile
from pathlib import Path

from corthena.compatibility.assets.legacy import LegacyAssetStager
from corthena.compatibility.ui.protocol import UiProbeProtocol
from corthena.compatibility.ui.raylib_probe import RaylibUiProbe


def test_hidden_ui_capture_and_cleanup() -> None:
    with tempfile.TemporaryDirectory(prefix="corthena-phase0-test-") as directory:
        probe: UiProbeProtocol = RaylibUiProbe(LegacyAssetStager())
        evidence = probe.capture(Path(directory) / "hidden-frame.png")
        assert evidence.capture.is_file()
        assert all(len(fingerprint) == 64 for fingerprint in evidence.asset_sha256)
