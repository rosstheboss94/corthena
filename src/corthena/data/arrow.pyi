from collections.abc import Callable
from pathlib import Path

from corthena.contracts.data import (
    FilePreviewDTO,
    ImportPlanDTO,
    Interval,
    PreviewRequestDTO,
    ProvenanceDTO,
)
from corthena.data.protocol import MarketCalendarProtocol
from corthena.data.types import CanonicalBar
from corthena.ui.client.protocol import CancellationSignalProtocol

class ArrowIngestionAdapter:
    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO: ...
    def read_bars(
        self,
        plan: ImportPlanDTO,
        calendar: MarketCalendarProtocol,
        cancellation: CancellationSignalProtocol,
    ) -> tuple[CanonicalBar, ...]: ...
    @staticmethod
    def validate(
        bars: tuple[CanonicalBar, ...],
        interval: Interval,
        regular_session: bool,
        calendar: MarketCalendarProtocol,
    ) -> None: ...
    @staticmethod
    def read_revision(path: Path) -> tuple[CanonicalBar, ...]: ...
    @staticmethod
    def write_revision(
        revisions_root: Path,
        dataset_id: str,
        revision: int,
        bars: tuple[CanonicalBar, ...],
        provenance_factory: Callable[[tuple[tuple[str, str], ...], str], ProvenanceDTO],
    ) -> tuple[Path, ProvenanceDTO]: ...

def source_checksum(path: Path) -> str: ...
