"""Agent-facing contracts for market data and session capabilities."""

from __future__ import annotations

from collections.abc import Callable
from datetime import date, datetime
from pathlib import Path
from typing import Protocol

from corthena.contracts.data import (
    AdjustmentPolicy,
    FilePreviewDTO,
    ImportPlanDTO,
    Interval,
    PreviewRequestDTO,
    ProvenanceDTO,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.data.types import CanonicalBar, ConnectorBars, SessionWindow
from corthena.ui.client.protocol import CancellationSignalProtocol


class MarketDataConnectorProtocol(Protocol):
    def test_connection(self, token: str, cancellation: CancellationSignalProtocol) -> str: ...

    def discover_symbols(
        self,
        token: str,
        query: str,
        page_size: int,
        cursor: str | None,
        cancellation: CancellationSignalProtocol,
    ) -> SymbolPageDTO: ...

    def pull_bars(
        self,
        token: str,
        symbol: str,
        interval: Interval,
        requested_range: UtcRangeDTO,
        adjustment: AdjustmentPolicy,
        cancellation: CancellationSignalProtocol,
    ) -> ConnectorBars: ...

    def close(self) -> None: ...


class MarketCalendarProtocol(Protocol):
    @property
    def version(self) -> str: ...

    def session(self, value: date) -> SessionWindow | None: ...

    def is_regular_bar(self, bar: CanonicalBar, interval: Interval) -> bool: ...

    def last_completed_bar_start(self, interval: Interval, now: datetime) -> datetime: ...


class FileIngestionProtocol(Protocol):
    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO: ...

    def read_bars(
        self,
        plan: ImportPlanDTO,
        calendar: MarketCalendarProtocol,
        cancellation: CancellationSignalProtocol,
    ) -> tuple[CanonicalBar, ...]: ...

    def read_revision(self, path: Path) -> tuple[CanonicalBar, ...]: ...

    def write_revision(
        self,
        revisions_root: Path,
        dataset_id: str,
        revision: int,
        bars: tuple[CanonicalBar, ...],
        provenance_factory: Callable[[tuple[tuple[str, str], ...], str], ProvenanceDTO],
    ) -> tuple[Path, ProvenanceDTO]: ...
