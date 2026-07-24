"""Immutable internal Data domain values."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True, slots=True, order=True)
class CanonicalBar:
    symbol: str
    timestamp: datetime
    open: float
    high: float
    low: float
    close: float
    volume: float


@dataclass(frozen=True, slots=True)
class ConnectorBars:
    bars: tuple[CanonicalBar, ...]
    request_ids: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class SessionWindow:
    session_date: datetime
    open_at: datetime
    close_at: datetime
