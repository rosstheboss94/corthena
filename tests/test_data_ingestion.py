from __future__ import annotations

# pyright: reportMissingTypeStubs=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false, reportUnknownVariableType=false
import json
import subprocess
import threading
import time
from collections.abc import Callable
from datetime import UTC, date, datetime
from pathlib import Path
from threading import Event

import httpx
import pyarrow as pa
import pyarrow.parquet as pq
import pytest
from fastapi.testclient import TestClient
from pydantic import SecretStr, ValidationError

from corthena.config.paths import ApplicationPaths
from corthena.contracts.data import (
    AdjustmentPolicy,
    CatalogRevisionDTO,
    ColumnMappingDTO,
    FilePreviewDTO,
    ImportMode,
    ImportPlanDTO,
    Interval,
    OperationState,
    PreviewRequestDTO,
    ProgressDTO,
    ProvenanceDTO,
    ScheduleCadence,
    ScheduleDTO,
    SessionPolicy,
    SourceKind,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.coordinator.api import create_data_app
from corthena.coordinator.data_service import DataCoordinatorService
from corthena.coordinator.scheduler import ScheduleRuntime
from corthena.data.arrow import ArrowIngestionAdapter
from corthena.data.calendar import XnysCalendar
from corthena.data.errors import (
    AuthenticationDataError,
    CancelledDataError,
    CapacityDataError,
    CredentialDataError,
    PublicationDataError,
    StaleRevisionError,
)
from corthena.data.massive import MassiveConnector
from corthena.data.protocol import MarketCalendarProtocol
from corthena.data.types import CanonicalBar, ConnectorBars
from corthena.persistence.data import DataRepository
from corthena.platform.credentials.store import CredentialStore
from corthena.platform.credentials.windows import WindowsCredentialPermissions
from corthena.ui.client.coordinator import CoordinatorUIClient
from corthena.ui.client.protocol import CancellationSignalProtocol
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy as UiAdjustment,
)
from corthena.ui.data_experiments.models import (
    ColumnMapping as UiColumnMapping,
)
from corthena.ui.data_experiments.models import (
    FileBrowserEntryKind,
    FileBrowserRequest,
    IngestionStatus,
)
from corthena.ui.data_experiments.models import (
    ImportMode as UiImportMode,
)
from corthena.ui.data_experiments.models import (
    IngestionPlan as UiIngestionPlan,
)
from corthena.ui.data_experiments.models import (
    SessionPolicy as UiSession,
)
from corthena.ui.data_experiments.models import (
    SourceKind as UiSourceKind,
)
from corthena.ui.data_experiments.models import (
    UtcRange as UiUtcRange,
)
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig

FIXED_CLOCK = datetime(2026, 7, 20, 15, 0, tzinfo=UTC)


class TestPermissions:
    def protect(self, path: Path) -> None:
        assert path.exists()

    def verify(self, path: Path) -> None:
        assert path.exists()


class UnusedConnector:
    version = "test-connector-v1"

    def test_connection(self, token: str, cancellation: CancellationSignalProtocol) -> str:
        if token == "invalid":
            raise AuthenticationDataError("authentication failed")
        return "request-test"

    def discover_symbols(
        self,
        token: str,
        query: str,
        page_size: int,
        cursor: str | None,
        cancellation: CancellationSignalProtocol,
    ) -> SymbolPageDTO:
        del token, query, page_size, cursor, cancellation
        return SymbolPageDTO(symbols=())

    def pull_bars(
        self,
        token: str,
        symbol: str,
        interval: Interval,
        requested_range: UtcRangeDTO,
        adjustment: AdjustmentPolicy,
        cancellation: CancellationSignalProtocol,
    ) -> ConnectorBars:
        del token, symbol, interval, requested_range, adjustment, cancellation
        return ConnectorBars((), ())

    def close(self) -> None:
        pass


class ScheduleConnector(UnusedConnector):
    def __init__(self) -> None:
        self.pull_count = 0

    def pull_bars(
        self,
        token: str,
        symbol: str,
        interval: Interval,
        requested_range: UtcRangeDTO,
        adjustment: AdjustmentPolicy,
        cancellation: CancellationSignalProtocol,
    ) -> ConnectorBars:
        del token, interval, requested_range, adjustment, cancellation
        self.pull_count += 1
        return ConnectorBars(
            (
                CanonicalBar(
                    symbol,
                    datetime(2026, 7, 20, 14, 59, tzinfo=UTC),
                    10,
                    12,
                    9,
                    11,
                    100,
                ),
            ),
            (f"schedule-request-{self.pull_count}",),
        )


class BlockingFileAdapter:
    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO:
        raise AssertionError(request)

    def read_bars(
        self,
        plan: ImportPlanDTO,
        calendar: MarketCalendarProtocol,
        cancellation: CancellationSignalProtocol,
    ) -> tuple[CanonicalBar, ...]:
        del plan, calendar
        while not cancellation.wait(0.01):
            pass
        raise CancelledDataError("cancelled")

    def read_revision(self, path: Path) -> tuple[CanonicalBar, ...]:
        raise AssertionError(path)

    def write_revision(
        self,
        revisions_root: Path,
        dataset_id: str,
        revision: int,
        bars: tuple[CanonicalBar, ...],
        provenance_factory: Callable[[tuple[tuple[str, str], ...], str], ProvenanceDTO],
    ) -> tuple[Path, ProvenanceDTO]:
        del revisions_root, dataset_id, revision, bars, provenance_factory
        raise AssertionError


class FailingPublishRepository(DataRepository):
    def publish(self, revision: CatalogRevisionDTO, expected_revision: int) -> None:
        del revision, expected_revision
        raise PublicationDataError("injected publication failure")


def _mapping() -> tuple[ColumnMappingDTO, ...]:
    return tuple(
        ColumnMappingDTO(source_column=value, role=value, source_type="detected")
        for value in ("timestamp", "symbol", "open", "high", "low", "close", "volume")
    )


def _plan(path: Path, command_id: str = "command-create") -> ImportPlanDTO:
    return ImportPlanDTO(
        command_id=command_id,
        correlation_id=f"correlation-{command_id}",
        generation=1,
        source_kind=SourceKind.CSV,
        source_path=str(path),
        mapping=_mapping(),
        source_timezone="UTC",
        dataset_id="dataset-bars",
        dataset_name="Imported Bars",
        expected_catalog_revision=0,
        symbols=("AAPL",),
        interval=Interval.MINUTE_1,
        requested_range=UtcRangeDTO(
            start=datetime(2026, 7, 20, 13, 30, tzinfo=UTC),
            end=datetime(2026, 7, 20, 15, 0, tzinfo=UTC),
        ),
        session_policy=SessionPolicy.REGULAR,
        adjustment_policy=AdjustmentPolicy.RAW,
        mode=ImportMode.CREATE,
    )


def _service(tmp_path: Path) -> tuple[DataCoordinatorService, DataRepository]:
    paths = ApplicationPaths.from_root(tmp_path)
    paths.ensure()
    repository = DataRepository(paths.database)
    credentials = CredentialStore(paths.credentials, paths.quarantine, TestPermissions())
    service = DataCoordinatorService(
        paths,
        repository,
        credentials,
        UnusedConnector(),
        XnysCalendar(),
        clock=lambda: FIXED_CLOCK,
    )
    return service, repository


def _wait(service: DataCoordinatorService, operation_id: str) -> OperationState:
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        progress = service.progress(operation_id)
        assert progress is not None
        if progress.state in {
            OperationState.CANCELLED,
            OperationState.COMPLETE,
            OperationState.FAILED,
        }:
            return progress.state
        time.sleep(0.01)
    raise AssertionError("operation did not finish")


def test_file_import_publishes_one_immutable_revision_and_is_idempotent(
    tmp_path: Path,
) -> None:
    source = tmp_path / "bars.csv"
    source.write_text(
        "timestamp,symbol,open,high,low,close,volume\n2026-07-20T14:00:00Z,AAPL,10,12,9,11,100\n",
        encoding="utf-8",
    )
    service, repository = _service(tmp_path)
    try:
        plan = _plan(source)
        first = service.submit(plan)
        assert _wait(service, first.operation_id) is OperationState.COMPLETE
        repeated = service.submit(plan)
        assert repeated.operation_id == first.operation_id
        revision = service.catalog().revisions[0]
        assert revision.revision == 1
        assert revision.row_count == 1
        assert Path(revision.revision_path, "provenance.v1.json").is_file()
        assert tuple(Path(revision.revision_path).glob("symbol=*/bars.parquet"))
        incompatible = plan.model_copy(update={"dataset_name": "Different"})
        with pytest.raises(StaleRevisionError, match="command ID was reused"):
            service.submit(incompatible)
    finally:
        service.close()
        repository.close()


def test_invalid_file_never_advances_catalog(tmp_path: Path) -> None:
    source = tmp_path / "bars.csv"
    source.write_text(
        "timestamp,symbol,open,high,low,close,volume\n2026-07-20T14:00:00Z,AAPL,10,8,9,11,100\n",
        encoding="utf-8",
    )
    service, repository = _service(tmp_path)
    try:
        accepted = service.submit(_plan(source))
        assert _wait(service, accepted.operation_id) is OperationState.FAILED
        assert service.catalog().revisions == ()
        assert not tuple((tmp_path / "data" / "revisions").glob("**/revision-*"))
    finally:
        service.close()
        repository.close()


def test_parquet_timezone_mapping_append_and_range_replacement(tmp_path: Path) -> None:
    parquet = tmp_path / "bars.parquet"
    pq.write_table(
        pa.table(
            {
                "when": [datetime(2026, 7, 20, 10, 0)],
                "ticker": ["AAPL"],
                "o": [10.0],
                "h": [12.0],
                "l": [9.0],
                "c": [11.0],
                "v": [100.0],
            }
        ),
        parquet,
    )
    arbitrary = (
        ColumnMappingDTO(source_column="when", role="timestamp", source_type="mapped"),
        ColumnMappingDTO(source_column="ticker", role="symbol", source_type="mapped"),
        ColumnMappingDTO(source_column="o", role="open", source_type="mapped"),
        ColumnMappingDTO(source_column="h", role="high", source_type="mapped"),
        ColumnMappingDTO(source_column="l", role="low", source_type="mapped"),
        ColumnMappingDTO(source_column="c", role="close", source_type="mapped"),
        ColumnMappingDTO(source_column="v", role="volume", source_type="mapped"),
    )
    service, repository = _service(tmp_path)
    try:
        first_plan = _plan(parquet).model_copy(
            update={
                "source_kind": SourceKind.PARQUET,
                "mapping": arbitrary,
                "source_timezone": "America/New_York",
            }
        )
        first = service.submit(first_plan)
        assert _wait(service, first.operation_id) is OperationState.COMPLETE

        csv = tmp_path / "append.csv"
        csv.write_text(
            "timestamp,symbol,open,high,low,close,volume\n"
            "2026-07-20T14:01:00Z,AAPL,11,13,10,12,110\n",
            encoding="utf-8",
        )
        append = _plan(csv, "command-append").model_copy(
            update={
                "expected_catalog_revision": 1,
                "mode": ImportMode.APPEND,
            }
        )
        accepted = service.submit(append)
        assert _wait(service, accepted.operation_id) is OperationState.COMPLETE

        csv.write_text(
            "timestamp,symbol,open,high,low,close,volume\n"
            "2026-07-20T14:00:00Z,AAPL,10,14,9,13,120\n",
            encoding="utf-8",
        )
        replacement = _plan(csv, "command-replace").model_copy(
            update={
                "expected_catalog_revision": 2,
                "mode": ImportMode.REPLACE_RANGE,
                "requested_range": UtcRangeDTO(
                    start=datetime(2026, 7, 20, 14, 0, tzinfo=UTC),
                    end=datetime(2026, 7, 20, 14, 1, tzinfo=UTC),
                ),
            }
        )
        replaced = service.submit(replacement)
        assert _wait(service, replaced.operation_id) is OperationState.COMPLETE
        revision = service.catalog().revisions[0]
        assert revision.revision == 3
        bars = ArrowIngestionAdapter.read_revision(Path(revision.revision_path))
        assert len(bars) == 2
        assert bars[0].timestamp == datetime(2026, 7, 20, 14, 0, tzinfo=UTC)
        assert bars[0].close == 13
    finally:
        service.close()
        repository.close()


def test_scheduler_coalesces_due_work_and_excludes_duplicate_submission(
    tmp_path: Path,
) -> None:
    paths = ApplicationPaths.from_root(tmp_path)
    paths.ensure()
    repository = DataRepository(paths.database)
    credentials = CredentialStore(paths.credentials, paths.quarantine, TestPermissions())
    credentials.save("schedule-secret")
    connector = ScheduleConnector()
    service = DataCoordinatorService(
        paths,
        repository,
        credentials,
        connector,
        XnysCalendar(),
        clock=lambda: FIXED_CLOCK,
    )
    schedule = ScheduleDTO(
        schedule_id="schedule-hourly",
        revision=1,
        name="Hourly AAPL",
        dataset_id="scheduled-bars",
        symbols=("AAPL",),
        interval=Interval.MINUTE_1,
        session_policy=SessionPolicy.REGULAR,
        adjustment_policy=AdjustmentPolicy.RAW,
        cadence=ScheduleCadence.HOURLY,
        enabled=True,
        range_anchor=datetime(2026, 7, 20, 14, 0, tzinfo=UTC),
    )
    service.create_schedule(schedule, "create-schedule")
    runtime = ScheduleRuntime(service, clock=lambda: FIXED_CLOCK)
    try:
        accepted = runtime.run_due_once()
        assert len(accepted) == 1
        assert runtime.run_due_once() == ()
        assert _wait(service, accepted[0]) is OperationState.COMPLETE
        assert connector.pull_count == 1
        assert service.catalog().revisions[0].revision == 1
    finally:
        service.close()
        repository.close()


def test_queue_saturation_cancellation_and_bounded_worker_cleanup(tmp_path: Path) -> None:
    source = tmp_path / "blocking.csv"
    source.write_text("placeholder", encoding="utf-8")
    paths = ApplicationPaths.from_root(tmp_path)
    paths.ensure()
    repository = DataRepository(paths.database)
    service = DataCoordinatorService(
        paths,
        repository,
        CredentialStore(paths.credentials, paths.quarantine, TestPermissions()),
        UnusedConnector(),
        XnysCalendar(),
        arrow=BlockingFileAdapter(),
        worker_count=1,
        queue_capacity=1,
    )
    before = tuple(thread.name for thread in threading.enumerate())
    try:
        first = service.submit(_plan(source, "blocking-first"))
        with pytest.raises(CapacityDataError, match="queue is full"):
            service.submit(_plan(source, "blocking-second"))
        service.cancel(first.operation_id)
        assert _wait(service, first.operation_id) is OperationState.CANCELLED
    finally:
        service.close()
        repository.close()
    after = tuple(thread.name for thread in threading.enumerate())
    assert sum(name.startswith("data-import") for name in after) == sum(
        name.startswith("data-import") for name in before
    )


def test_restart_recovers_incomplete_operation_and_reports_event_sequence(
    tmp_path: Path,
) -> None:
    paths = ApplicationPaths.from_root(tmp_path)
    paths.ensure()
    repository = DataRepository(paths.database)
    progress = ProgressDTO(
        operation_id="import-interrupted",
        correlation_id="restart-correlation",
        generation=1,
        state=OperationState.RUNNING,
        completed_units=40,
        total_units=100,
        message="Running",
    )
    repository.put_progress(progress, "restart-command")
    repository.close()
    restarted_repository = DataRepository(paths.database)
    service = DataCoordinatorService(
        paths,
        restarted_repository,
        CredentialStore(paths.credentials, paths.quarantine, TestPermissions()),
        UnusedConnector(),
        XnysCalendar(),
    )
    try:
        reconciled = service.reconcile()
        assert reconciled.recovered_operation_ids == ("import-interrupted",)
        assert reconciled.latest_event_sequence == 0
        recovered = service.progress("import-interrupted")
        assert recovered is not None and recovered.state is OperationState.CANCELLED
    finally:
        service.close()
        restarted_repository.close()


def test_atomic_publication_failure_removes_unindexed_revision(tmp_path: Path) -> None:
    source = tmp_path / "bars.csv"
    source.write_text(
        "timestamp,symbol,open,high,low,close,volume\n2026-07-20T14:00:00Z,AAPL,10,12,9,11,100\n",
        encoding="utf-8",
    )
    paths = ApplicationPaths.from_root(tmp_path)
    paths.ensure()
    repository = FailingPublishRepository(paths.database)
    service = DataCoordinatorService(
        paths,
        repository,
        CredentialStore(paths.credentials, paths.quarantine, TestPermissions()),
        UnusedConnector(),
        XnysCalendar(),
        clock=lambda: FIXED_CLOCK,
    )
    try:
        accepted = service.submit(_plan(source))
        assert _wait(service, accepted.operation_id) is OperationState.FAILED
        assert service.catalog().revisions == ()
        assert not tuple(paths.revisions.glob("**/revision-*"))
    finally:
        service.close()
        repository.close()


def test_windows_credential_permissions_reject_an_extra_principal(tmp_path: Path) -> None:
    path = tmp_path / "credential.json"
    path.write_text("{}", encoding="utf-8")
    permissions = WindowsCredentialPermissions()
    permissions.protect(path)
    subprocess.run(
        ("icacls", str(path), "/grant", "*S-1-1-0:(R)"),
        check=True,
        capture_output=True,
        text=True,
        timeout=10,
    )
    with pytest.raises(CredentialDataError, match="could not be verified"):
        permissions.verify(path)


def test_api_never_returns_token_and_quarantines_corruption(tmp_path: Path) -> None:
    service, repository = _service(tmp_path)
    sentinel = "sentinel-token-never-return"
    client = TestClient(create_data_app(service))
    try:
        saved = client.put(
            "/api/v1/settings/api-tokens/massive",
            json={
                "schema_version": 1,
                "command_id": "save-credential",
                "correlation_id": "credential-request",
                "token": sentinel,
            },
        )
        assert saved.status_code == 200
        assert sentinel not in saved.text
        invalid = client.put(
            "/api/v1/settings/api-tokens/massive",
            json={
                "schema_version": 1,
                "command_id": "bad-credential",
                "correlation_id": "bad-request",
                "token": f" {sentinel} ",
            },
        )
        assert invalid.status_code == 422
        assert sentinel not in invalid.text
        credential_path = tmp_path / "credentials" / "massive.v1.json"
        credential_path.write_text("corrupt-secret-content", encoding="utf-8")
        response = client.get("/api/v1/settings/api-tokens/massive")
        assert response.status_code == 500
        assert "corrupt-secret-content" not in response.text
        quarantined = tuple((tmp_path / "quarantine").glob("*.corrupt"))
        assert len(quarantined) == 1
    finally:
        client.close()
        service.close()
        repository.close()


def test_ui_client_to_api_sqlite_and_parquet_integration(tmp_path: Path) -> None:
    source = tmp_path / "ui-bars.csv"
    source.write_text(
        "timestamp,symbol,open,high,low,close,volume\n2026-07-20T14:00:00Z,AAPL,10,12,9,11,100\n",
        encoding="utf-8",
    )
    service, repository = _service(tmp_path)
    transport = TestClient(create_data_app(service), base_url="http://127.0.0.1")
    fallback = DeterministicSimulator(SimulatorConfig(42, FIXED_CLOCK))
    client = CoordinatorUIClient("http://127.0.0.1", fallback, transport)
    plan = UiIngestionPlan(
        "ui-command",
        "ui-correlation",
        1,
        "ui-dataset",
        0,
        ("AAPL",),
        "1m",
        UiUtcRange(
            datetime(2026, 7, 20, 13, 30, tzinfo=UTC),
            datetime(2026, 7, 20, 15, 0, tzinfo=UTC),
        ),
        UiSession.REGULAR,
        UiAdjustment.RAW,
        UiImportMode.CREATE,
        source_path=str(source),
        mapping=tuple(
            UiColumnMapping(item.source_column, item.role, item.source_type) for item in _mapping()
        ),
        dataset_name="UI Dataset",
    )
    try:
        result = client.submit_file_ingestion(plan, Event())
        assert result.progress.status is IngestionStatus.COMPLETE
        assert result.catalog_entry is not None
        assert result.catalog_entry.revision == 1
        assert Path(service.catalog().revisions[0].revision_path).is_dir()
    finally:
        client.close()
        transport.close()
        service.close()
        repository.close()


def test_coordinator_ui_file_browser_lists_folders_csv_parquet_and_pages(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    for index in range(40):
        (tmp_path / f"folder-{index:03d}").mkdir()
        (tmp_path / f"bars-{index:03d}.csv").write_text("close\n1\n", encoding="utf-8")
    (tmp_path / "ignored.parquet").write_bytes(b"not-a-parquet-file")
    alias_a = tmp_path / "alias-a"
    alias_b = tmp_path / "alias-b"
    alias_a.mkdir()
    alias_b.mkdir()
    original_resolve = Path.resolve

    def resolve_aliases(path: Path, strict: bool = False) -> Path:
        if path in {alias_a, alias_b}:
            return tmp_path / "shared-target"
        return original_resolve(path, strict=strict)

    monkeypatch.setattr(Path, "resolve", resolve_aliases)
    fallback = DeterministicSimulator(SimulatorConfig(42, FIXED_CLOCK))
    client = CoordinatorUIClient("http://127.0.0.1", fallback)
    try:
        cursor: str | None = None
        entries = []
        page_number = 0
        while True:
            page_number += 1
            request = FileBrowserRequest(
                f"browser-page-{page_number}",
                1,
                None,
                location=str(tmp_path.resolve()),
                navigation_revision=1,
                cursor=cursor,
                page_size=32,
            )
            listing = client.browse_files(request, Event())
            entries.extend(listing.entries)
            cursor = listing.next_cursor
            if cursor is None:
                break
        assert entries[0].kind is FileBrowserEntryKind.PARENT
        assert sum(entry.kind is FileBrowserEntryKind.FOLDER for entry in entries) == 42
        assert {entry.display_name for entry in entries} >= {"alias-a", "alias-b"}
        files = tuple(entry for entry in entries if entry.kind is FileBrowserEntryKind.FILE)
        assert len(files) == 41
        assert sum(entry.source_kind is UiSourceKind.CSV for entry in files) == 40
        assert sum(entry.source_kind is UiSourceKind.PARQUET for entry in files) == 1
        assert any(entry.display_name == "ignored.parquet" for entry in entries)
        assert len({entry.source_name.casefold() for entry in entries}) == len(entries)
    finally:
        client.close()


def test_massive_connector_uses_bearer_pagination_retry_and_request_ids() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        assert request.headers["Authorization"] == "Bearer secret"
        assert "secret" not in str(request.url)
        if len(requests) == 1:
            return httpx.Response(429, headers={"Retry-After": "0"})
        if "reference/tickers" in request.url.path:
            return httpx.Response(
                200,
                json={
                    "request_id": "ticker-request",
                    "results": [
                        {"ticker": "AAPL", "name": "Apple Inc.", "market": "stocks", "active": True}
                    ],
                },
            )
        return httpx.Response(
            200,
            json={
                "request_id": "bars-request",
                "results": [{"t": 1784556000000, "o": 10, "h": 12, "l": 9, "c": 11, "v": 100}],
            },
        )

    http = httpx.Client(transport=httpx.MockTransport(handler))
    connector = MassiveConnector(http, max_retries=1, sleeper=lambda _: None)
    page = connector.discover_symbols("secret", "apple", 20, None, Event())
    assert page.symbols[0].symbol == "AAPL"
    bars = connector.pull_bars(
        "secret",
        "AAPL",
        Interval.MINUTE_1,
        UtcRangeDTO(
            start=datetime(2026, 7, 20, 13, tzinfo=UTC),
            end=datetime(2026, 7, 20, 15, tzinfo=UTC),
        ),
        AdjustmentPolicy.RAW,
        Event(),
    )
    assert bars.request_ids == ("bars-request",)
    cancelled = Event()
    cancelled.set()
    with pytest.raises(CancelledDataError):
        connector.test_connection("secret", cancelled)
    connector.close()
    http.close()


def test_calendar_admission_covers_holiday_dst_and_early_close() -> None:
    calendar = XnysCalendar()
    assert calendar.session(date(2026, 7, 3)) is None
    assert calendar.session(date(2026, 7, 4)) is None
    before_dst = calendar.session(date(2026, 3, 6))
    after_dst = calendar.session(date(2026, 3, 9))
    early_close = calendar.session(date(2026, 11, 27))
    assert before_dst is not None and before_dst.open_at.hour == 14
    assert after_dst is not None and after_dst.open_at.hour == 13
    assert early_close is not None and early_close.close_at.hour == 18


def test_public_contracts_reject_unknown_fields_and_secrets_are_redacted() -> None:
    from corthena.contracts.data import CredentialSecretDTO

    secret = CredentialSecretDTO(
        command_id="credential-command",
        correlation_id="credential-correlation",
        token=SecretStr("secret-value"),
    )
    assert "secret-value" not in repr(secret)
    assert "secret-value" not in secret.model_dump_json()
    document = json.loads(secret.model_dump_json())
    document["unexpected"] = True
    with pytest.raises(ValidationError):
        CredentialSecretDTO.model_validate(document)
