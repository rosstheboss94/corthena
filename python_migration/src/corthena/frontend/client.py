"""Consumer-owned client boundary used by the frontend effects runtime."""

from __future__ import annotations

from typing import Protocol

from corthena.frontend.state import Snapshot


class CancellationSignal(Protocol):
    """Narrow cancellation view; implementations retain mutation ownership."""

    def is_set(self) -> bool:
        """Return whether cancellation has been requested."""
        ...

    def wait(self, timeout: float | None = None) -> bool:
        """Wait up to ``timeout`` seconds for cancellation."""
        ...


class RequestCancelledError(Exception):
    """Raised by a client when cooperative cancellation wins."""


class FrontendClient(Protocol):
    """Operations needed by Phase 2 frontend consumers."""

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignal,
    ) -> Snapshot:
        """Prepare an immutable startup snapshot off the UI thread."""
        ...


__all__ = ["CancellationSignal", "FrontendClient", "RequestCancelledError"]
