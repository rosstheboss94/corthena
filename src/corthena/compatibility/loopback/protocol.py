from typing import Protocol

from corthena.compatibility.loopback.models import LoopbackEvidence
from corthena.compatibility.runtime.models import RuntimeCapabilities


class LoopbackProbeProtocol(Protocol):
    def run(self, runtime: RuntimeCapabilities) -> LoopbackEvidence: ...
