"""Cancellable typed HTTP/WebSocket coordinator client."""

# pyright: reportMissingTypeStubs=false, reportUnknownMemberType=false

from __future__ import annotations

from collections.abc import Iterator, Mapping
from urllib.parse import urlparse, urlunparse

import httpx
from websockets.exceptions import ConnectionClosed
from websockets.sync.client import connect

from corthena.contracts.data import (
    CatalogDTO,
    CredentialSecretDTO,
    CredentialStatusDTO,
    DataEventDTO,
    FilePreviewDTO,
    ImportPlanDTO,
    OperationAcceptedDTO,
    PreviewRequestDTO,
    ProgressDTO,
    SourceKind,
    SymbolPageDTO,
)
from corthena.ui.client.protocol import CancellationSignalProtocol


class EventSequenceGap(RuntimeError):
    """Signals that the caller must reconcile authoritative REST state."""


class DataClient:
    """Public typed Data client with explicit close and bounded reconnect."""

    def __init__(
        self,
        base_url: str,
        client: httpx.Client | None = None,
        *,
        reconnect_attempts: int = 3,
    ) -> None:
        if not 0 <= reconnect_attempts <= 8:
            raise ValueError("reconnect bound is invalid")
        parsed = urlparse(base_url)
        if parsed.scheme not in {"http", "https"} or parsed.hostname not in {
            "127.0.0.1",
            "::1",
            "localhost",
        }:
            raise ValueError("coordinator URL must use loopback HTTP")
        self._base_url = base_url.rstrip("/")
        self._owns_client = client is None
        self._client = client or httpx.Client(base_url=self._base_url, timeout=20.0)
        self._reconnect_attempts = reconnect_attempts

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def catalog(self) -> CatalogDTO:
        return CatalogDTO.model_validate_json(self._request("GET", "/api/v1/data/catalog").text)

    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO:
        response = self._request(
            "POST", "/api/v1/data/files/preview", json=request.model_dump(mode="json")
        )
        return FilePreviewDTO.model_validate_json(response.text)

    def submit(self, plan: ImportPlanDTO) -> OperationAcceptedDTO:
        path = (
            "/api/v1/data/providers/massive/pulls"
            if plan.source_kind is SourceKind.MASSIVE
            else "/api/v1/data/imports"
        )
        response = self._request("POST", path, json=plan.model_dump(mode="json"))
        return OperationAcceptedDTO.model_validate_json(response.text)

    def progress(self, operation_id: str) -> ProgressDTO:
        response = self._request("GET", f"/api/v1/data/imports/{operation_id}")
        return ProgressDTO.model_validate_json(response.text)

    def credential_status(self) -> CredentialStatusDTO:
        response = self._request("GET", "/api/v1/settings/api-tokens/massive")
        return CredentialStatusDTO.model_validate_json(response.text)

    def save_credential(self, request: CredentialSecretDTO) -> CredentialStatusDTO:
        response = self._request(
            "PUT",
            "/api/v1/settings/api-tokens/massive",
            json={
                "schema_version": request.schema_version,
                "command_id": request.command_id,
                "correlation_id": request.correlation_id,
                "token": request.token.get_secret_value(),
            },
        )
        return CredentialStatusDTO.model_validate_json(response.text)

    def discover_symbols(
        self, query: str, limit: int = 20, cursor: str | None = None
    ) -> SymbolPageDTO:
        params = {"query": query, "limit": str(limit)}
        if cursor is not None:
            params["cursor"] = cursor
        response = self._request("GET", "/api/v1/data/providers/massive/symbols", params=params)
        return SymbolPageDTO.model_validate_json(response.text)

    def events(self, cancellation: CancellationSignalProtocol) -> Iterator[DataEventDTO]:
        sequence = 0
        failures = 0
        while not cancellation.is_set():
            try:
                with connect(
                    self._event_url(),
                    open_timeout=10,
                    close_timeout=5,
                    max_size=1_048_576,
                    max_queue=16,
                    proxy=None,
                ) as websocket:
                    failures = 0
                    while not cancellation.is_set():
                        try:
                            message = websocket.recv(timeout=0.5)
                        except TimeoutError:
                            continue
                        event = DataEventDTO.model_validate_json(message)
                        if sequence and event.sequence != sequence + 1:
                            raise EventSequenceGap(
                                "event sequence gap requires REST reconciliation"
                            )
                        sequence = event.sequence
                        yield event
            except ConnectionClosed:
                failures += 1
                if failures > self._reconnect_attempts:
                    raise RuntimeError("coordinator event reconnect exhausted") from None
                if cancellation.wait(min(0.25 * (2 ** (failures - 1)), 2.0)):
                    return

    def _request(
        self,
        method: str,
        path: str,
        *,
        json: object | None = None,
        params: Mapping[str, str] | None = None,
    ) -> httpx.Response:
        response = self._client.request(method, path, json=json, params=params)
        if response.is_error:
            raise RuntimeError(f"coordinator request failed with status {response.status_code}")
        return response

    def _event_url(self) -> str:
        parsed = urlparse(self._base_url)
        scheme = "wss" if parsed.scheme == "https" else "ws"
        return urlunparse((scheme, parsed.netloc, "/api/v1/events", "", "", ""))
