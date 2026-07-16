from pathlib import Path
from typing import Protocol

from corthena.compatibility.ui.models import UiEvidence


class UiProbeProtocol(Protocol):
    def capture(self, capture: Path) -> UiEvidence: ...
