"""Agent-facing contract for the supported Data client."""

from __future__ import annotations

from collections.abc import Iterator
from typing import Protocol

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
    SymbolPageDTO,
)
from corthena.ui.client.protocol import CancellationSignalProtocol


class DataClientProtocol(Protocol):
    def catalog(self) -> CatalogDTO: ...

    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO: ...

    def submit(self, plan: ImportPlanDTO) -> OperationAcceptedDTO: ...

    def progress(self, operation_id: str) -> ProgressDTO: ...

    def credential_status(self) -> CredentialStatusDTO: ...

    def save_credential(self, request: CredentialSecretDTO) -> CredentialStatusDTO: ...

    def discover_symbols(
        self, query: str, limit: int = 20, cursor: str | None = None
    ) -> SymbolPageDTO: ...

    def events(self, cancellation: CancellationSignalProtocol) -> Iterator[DataEventDTO]: ...

    def close(self) -> None: ...
