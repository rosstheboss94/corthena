import tempfile
from pathlib import Path

from corthena.compatibility.ui import capture_hidden_frame


def test_hidden_ui_capture_and_cleanup() -> None:
    with tempfile.TemporaryDirectory(prefix="corthena-phase0-test-") as directory:
        evidence = capture_hidden_frame(Path(directory) / "hidden-frame.png")
        assert evidence.capture.is_file()
        assert all(len(fingerprint) == 64 for fingerprint in evidence.asset_sha256)
