"""Data-only contract for loopback compatibility evidence."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class LoopbackEvidence:
    """Observed typed handshake outcome."""

    correlation_id: str
    event_type: str
