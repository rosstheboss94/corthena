"""Massive REST adapter with bounded retries and typed result containment."""

# pyright: reportUnknownArgumentType=false, reportUnknownMemberType=false
# pyright: reportUnknownVariableType=false

from __future__ import annotations

import json
import math
import time
from collections.abc import Callable
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime
from urllib.parse import quote, urlparse

import httpx

from corthena.contracts.data import (
    AdjustmentPolicy,
    Interval,
    SymbolDTO,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.data.errors import (
    AuthenticationDataError,
    CancelledDataError,
    EntitlementDataError,
    ProviderDataError,
    RateLimitDataError,
    ValidationDataError,
)
from corthena.data.types import CanonicalBar, ConnectorBars
from corthena.ui.client.protocol import CancellationSignalProtocol

_BASE_URL = "https://api.massive.com"
_INTERVAL = {
    Interval.MINUTE_1: (1, "minute"),
    Interval.MINUTE_5: (5, "minute"),
    Interval.MINUTE_15: (15, "minute"),
    Interval.HOUR_1: (1, "hour"),
    Interval.DAY_1: (1, "day"),
}


class MassiveConnector:
    """Only provider implementation shipped by Milestone 1b."""

    version = "massive-rest-v1"

    def __init__(
        self,
        client: httpx.Client | None = None,
        *,
        max_retries: int = 3,
        timeout_seconds: float = 20.0,
        sleeper: Callable[[float], None] = time.sleep,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
    ) -> None:
        if not 0 <= max_retries <= 8:
            raise ValueError("Massive retry bound is invalid")
        self._owns_client = client is None
        self._client = client or httpx.Client(timeout=timeout_seconds, follow_redirects=False)
        self._max_retries = max_retries
        self._sleeper = sleeper
        self._clock = clock

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def test_connection(self, token: str, cancellation: CancellationSignalProtocol) -> str:
        document = self._get(
            f"{_BASE_URL}/v3/reference/tickers",
            token,
            {"market": "stocks", "active": "true", "limit": "1", "sort": "ticker"},
            cancellation,
        )
        return self._text(document, "request_id", required=False) or "not-provided"

    def discover_symbols(
        self,
        token: str,
        query: str,
        page_size: int,
        cursor: str | None,
        cancellation: CancellationSignalProtocol,
    ) -> SymbolPageDTO:
        if not 1 <= page_size <= 1000:
            raise ValidationDataError("symbol page size is invalid", field="page_size")
        url = cursor or f"{_BASE_URL}/v3/reference/tickers"
        params = (
            None
            if cursor
            else {
                "market": "stocks",
                "active": "true",
                "search": query,
                "limit": str(page_size),
                "sort": "ticker",
                "order": "asc",
            }
        )
        document = self._get(url, token, params, cancellation)
        results = self._list(document, "results", required=False)
        symbols = tuple(
            sorted(
                [
                    SymbolDTO(
                        symbol=self._required_text(item, "ticker"),
                        name=self._required_text(item, "name"),
                    )
                    for item in results
                    if self._text(item, "market", required=False) in {None, "stocks"}
                    and self._boolean(item, "active", default=True)
                ],
                key=lambda item: item.symbol,
            )
        )
        next_url = self._safe_next_url(self._text(document, "next_url", required=False))
        return SymbolPageDTO(
            symbols=symbols,
            next_cursor=next_url,
            provider_request_id=self._text(document, "request_id", required=False),
        )

    def pull_bars(
        self,
        token: str,
        symbol: str,
        interval: Interval,
        requested_range: UtcRangeDTO,
        adjustment: AdjustmentPolicy,
        cancellation: CancellationSignalProtocol,
    ) -> ConnectorBars:
        multiplier, timespan = _INTERVAL[interval]
        start_ms = int(requested_range.start.timestamp() * 1000)
        end_ms = int(requested_range.end.timestamp() * 1000)
        url: str | None = (
            f"{_BASE_URL}/v2/aggs/ticker/{quote(symbol, safe='')}/range/"
            f"{multiplier}/{timespan}/{start_ms}/{end_ms}"
        )
        params: dict[str, str] | None = {
            "adjusted": "true"
            if adjustment is AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED
            else "false",
            "sort": "asc",
            "limit": "50000",
        }
        bars: list[CanonicalBar] = []
        request_ids: list[str] = []
        pages = 0
        while url is not None:
            self._cancel(cancellation)
            pages += 1
            if pages > 10_000:
                raise ProviderDataError("Massive pagination exceeded its bound")
            document = self._get(url, token, params, cancellation)
            params = None
            request_id = self._text(document, "request_id", required=False)
            if request_id:
                request_ids.append(request_id)
            for item in self._list(document, "results", required=False):
                timestamp = datetime.fromtimestamp(self._number(item, "t") / 1000.0, UTC)
                bar = CanonicalBar(
                    symbol=symbol.upper(),
                    timestamp=timestamp,
                    open=self._number(item, "o"),
                    high=self._number(item, "h"),
                    low=self._number(item, "l"),
                    close=self._number(item, "c"),
                    volume=self._number(item, "v"),
                )
                if not all(
                    math.isfinite(value)
                    for value in (bar.open, bar.high, bar.low, bar.close, bar.volume)
                ):
                    raise ProviderDataError("Massive returned non-finite aggregate values")
                if requested_range.start <= bar.timestamp < requested_range.end:
                    bars.append(bar)
            url = self._safe_next_url(self._text(document, "next_url", required=False))
        unique: dict[tuple[str, datetime], CanonicalBar] = {}
        for bar in bars:
            unique[(bar.symbol, bar.timestamp)] = bar
        return ConnectorBars(tuple(sorted(unique.values())), tuple(request_ids))

    def _get(
        self,
        url: str,
        token: str,
        params: dict[str, str] | None,
        cancellation: CancellationSignalProtocol,
    ) -> dict[str, object]:
        self._safe_next_url(url)
        for attempt in range(self._max_retries + 1):
            self._cancel(cancellation)
            try:
                response = self._client.get(
                    url,
                    params=params,
                    headers={"Authorization": f"Bearer {token}", "Accept": "application/json"},
                )
            except httpx.HTTPError as error:
                if attempt >= self._max_retries:
                    raise ProviderDataError("Massive request failed") from error
                self._bounded_wait(0.25 * (2**attempt), cancellation)
                continue
            request_id = response.headers.get("X-Request-ID")
            if response.status_code == 401:
                raise AuthenticationDataError(
                    "Massive authentication failed", provider_request_id=request_id
                )
            if response.status_code == 403:
                raise EntitlementDataError(
                    "Massive entitlement denied", provider_request_id=request_id
                )
            if response.status_code == 429:
                retry_after = self._retry_after(response.headers.get("Retry-After"))
                if attempt >= self._max_retries:
                    raise RateLimitDataError(
                        "Massive rate limit exhausted",
                        retry_after_seconds=retry_after,
                        provider_request_id=request_id,
                    )
                self._bounded_wait(retry_after, cancellation)
                continue
            if response.status_code >= 500:
                if attempt >= self._max_retries:
                    raise ProviderDataError(
                        "Massive service failed", provider_request_id=request_id
                    )
                self._bounded_wait(0.25 * (2**attempt), cancellation)
                continue
            if response.status_code >= 400:
                raise ProviderDataError(
                    "Massive rejected the request", provider_request_id=request_id
                )
            try:
                document = json.loads(response.text)
            except (TypeError, ValueError) as error:
                raise ProviderDataError("Massive returned malformed JSON") from error
            if not isinstance(document, dict):
                raise ProviderDataError("Massive returned an invalid response shape")
            return document
        raise AssertionError("bounded retry loop did not terminate")

    def _bounded_wait(self, seconds: float, cancellation: CancellationSignalProtocol) -> None:
        remaining = min(max(seconds, 0.0), 60.0)
        while remaining > 0:
            self._cancel(cancellation)
            step = min(remaining, 0.1)
            self._sleeper(step)
            remaining -= step

    @staticmethod
    def _cancel(cancellation: CancellationSignalProtocol) -> None:
        if cancellation.is_set():
            raise CancelledDataError("Massive operation was cancelled")

    def _retry_after(self, value: str | None) -> float:
        if value is None:
            return 1.0
        try:
            return min(max(float(value), 0.0), 60.0)
        except ValueError:
            try:
                parsed = parsedate_to_datetime(value).astimezone(UTC)
            except TypeError, ValueError:
                return 1.0
            return min(max((parsed - self._clock()).total_seconds(), 0.0), 60.0)

    @staticmethod
    def _safe_next_url(value: str | None) -> str | None:
        if value is None:
            return None
        parsed = urlparse(value)
        if (
            parsed.scheme != "https"
            or parsed.hostname != "api.massive.com"
            or parsed.username
            or parsed.password
        ):
            raise ProviderDataError("Massive pagination URL was rejected")
        if "apikey=" in parsed.query.casefold():
            raise ProviderDataError("Massive pagination URL contained a credential")
        return value

    @staticmethod
    def _list(
        document: dict[str, object], key: str, *, required: bool = True
    ) -> list[dict[str, object]]:
        value = document.get(key)
        if value is None and not required:
            return []
        if not isinstance(value, list) or any(not isinstance(item, dict) for item in value):
            raise ProviderDataError(f"Massive response field {key} is invalid")
        return value

    @staticmethod
    def _text(document: dict[str, object], key: str, *, required: bool = True) -> str | None:
        value = document.get(key)
        if value is None and not required:
            return None
        if not isinstance(value, str) or not value:
            raise ProviderDataError(f"Massive response field {key} is invalid")
        return value

    @staticmethod
    def _required_text(document: dict[str, object], key: str) -> str:
        value = MassiveConnector._text(document, key)
        if value is None:
            raise ProviderDataError(f"Massive response field {key} is invalid")
        return value

    @staticmethod
    def _boolean(document: dict[str, object], key: str, *, default: bool) -> bool:
        value = document.get(key, default)
        if not isinstance(value, bool):
            raise ProviderDataError(f"Massive response field {key} is invalid")
        return value

    @staticmethod
    def _number(document: dict[str, object], key: str) -> float:
        value = document.get(key)
        if isinstance(value, bool) or not isinstance(value, int | float):
            raise ProviderDataError(f"Massive response field {key} is invalid")
        return float(value)
