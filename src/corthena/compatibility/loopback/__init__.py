"""Owned loopback compatibility capability."""

from corthena.compatibility.loopback.models import LoopbackEvidence
from corthena.compatibility.loopback.protocol import LoopbackProbeProtocol
from corthena.compatibility.loopback.uvicorn_probe import UvicornLoopbackProbe
from corthena.compatibility.runtime.models import RuntimeCapabilities


def run_loopback_probe(runtime: RuntimeCapabilities) -> LoopbackEvidence:
    """Run the default owned loopback compatibility probe."""
    return UvicornLoopbackProbe().run(runtime)


__all__ = (
    "LoopbackEvidence",
    "LoopbackProbeProtocol",
    "UvicornLoopbackProbe",
    "run_loopback_probe",
)
