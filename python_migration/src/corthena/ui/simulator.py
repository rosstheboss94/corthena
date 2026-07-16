"""Deterministic Phase 2 simulator implementing only ``UIClient``."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.research.deterministic import build_research_snapshot
from corthena.ui.research.models import ResearchQuery, ResearchSnapshot
from corthena.ui.state import Snapshot, SnapshotItem


@dataclass(frozen=True, slots=True)
class SimulatorConfig:
    """Explicit replay inputs for deterministic demo behavior."""

    seed: int
    fixed_clock: datetime
    delay_seconds: float = 0.0

    def __post_init__(self) -> None:
        if self.fixed_clock.tzinfo is None:
            raise ValueError("fixed_clock must be timezone-aware")
        if self.delay_seconds < 0:
            raise ValueError("delay_seconds must be non-negative")


class DeterministicSimulator:
    """Seeded, scheduling-independent client adapter for pre-coordinator use."""

    def __init__(self, config: SimulatorConfig) -> None:
        self._config = config

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignalProtocol,
    ) -> Snapshot:
        """Derive stable demo values solely from explicit replay inputs."""
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(request_id)
        if cancellation.is_set():
            raise RequestCancelledError(request_id)
        symbols = ("AAPL", "MSFT", "NVDA", "SPY")
        items = tuple(
            SnapshotItem(index, symbol, self._value(request_id, generation, symbol))
            for index, symbol in enumerate(symbols)
        )
        return Snapshot(
            request_id=request_id,
            generation=generation,
            seed=self._config.seed,
            as_of=self._config.fixed_clock,
            items=items,
        )

    def load_research(
        self,
        query: ResearchQuery,
        cancellation: CancellationSignalProtocol,
    ) -> ResearchSnapshot:
        """Prepare deterministic Research data off the render thread."""
        if cancellation.wait(self._config.delay_seconds):
            raise RequestCancelledError(query.request_id)
        return build_research_snapshot(
            self._config.seed,
            self._config.fixed_clock,
            query,
            cancellation,
        )

    def _value(self, request_id: str, generation: int, symbol: str) -> int:
        payload = f"{self._config.seed}\0{request_id}\0{generation}\0{symbol}".encode()
        digest = hashlib.sha256(payload).digest()
        return 10_000_000 + int.from_bytes(digest[:8], "big") % 990_000_001


__all__ = ["DeterministicSimulator", "SimulatorConfig"]
