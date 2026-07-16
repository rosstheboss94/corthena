"""Agent-facing contracts for snapshot loading."""

from __future__ import annotations

from typing import Protocol

from corthena.ui.research.models import ResearchQuery, ResearchSnapshot
from corthena.ui.state import Snapshot


class CancellationSignalProtocol(Protocol):
    """Read-only cancellation view for client operations."""

    def is_set(self) -> bool: ...

    def wait(self, timeout: float | None = None) -> bool: ...


class UIClientProtocol(Protocol):
    """Operations consumed by the UI effects runtime."""

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignalProtocol,
    ) -> Snapshot: ...

    def load_research(
        self,
        query: ResearchQuery,
        cancellation: CancellationSignalProtocol,
    ) -> ResearchSnapshot: ...
