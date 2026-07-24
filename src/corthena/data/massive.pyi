from collections.abc import Callable
from datetime import datetime

import httpx

from corthena.contracts.data import (
    AdjustmentPolicy,
    Interval,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.data.types import ConnectorBars
from corthena.ui.client.protocol import CancellationSignalProtocol

class MassiveConnector:
    version: str
    def __init__(
        self,
        client: httpx.Client | None = ...,
        *,
        max_retries: int = ...,
        timeout_seconds: float = ...,
        sleeper: Callable[[float], None] = ...,
        clock: Callable[[], datetime] = ...,
    ) -> None: ...
    def close(self) -> None: ...
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
