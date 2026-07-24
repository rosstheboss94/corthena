"""Coordinator-backed UIClientProtocol adapter for real Data operations."""

# pyright: reportUnknownArgumentType=false, reportUnknownMemberType=false
# pyright: reportUnknownVariableType=false

from __future__ import annotations

import time
from collections.abc import Mapping
from dataclasses import replace
from datetime import UTC, datetime
from pathlib import Path
from typing import Literal
from urllib.parse import urlparse

import httpx
from pydantic import SecretStr

from corthena.contracts.data import (
    AdjustmentPolicy as ApiAdjustment,
)
from corthena.contracts.data import (
    CatalogDTO,
    CatalogRevisionDTO,
    ColumnMappingDTO,
    CommandDTO,
    CredentialSecretDTO,
    CredentialStatusDTO,
    CredentialTestDTO,
    DeleteCommandDTO,
    FilePreviewDTO,
    ImportPlanDTO,
    Interval,
    OperationAcceptedDTO,
    OperationState,
    PreviewRequestDTO,
    ProgressDTO,
    ReconciliationDTO,
    ScheduleCommandDTO,
    ScheduleDTO,
    SchedulePatchDTO,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.contracts.data import (
    ImportMode as ApiImportMode,
)
from corthena.contracts.data import (
    ScheduleCadence as ApiCadence,
)
from corthena.contracts.data import (
    SessionPolicy as ApiSession,
)
from corthena.contracts.data import (
    SourceKind as ApiSource,
)
from corthena.ui.client.protocol import CancellationSignalProtocol, UIClientProtocol
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    ColumnMapping,
    CredentialRequest,
    CredentialResult,
    CredentialSecretRequest,
    CredentialStatus,
    DataSchedule,
    DatasetCatalogEntry,
    DraftEvaluation,
    DraftSaveRequest,
    DraftSaveResult,
    ExperimentDefinition,
    ExperimentDraft,
    FileBrowserEntry,
    FileBrowserEntryKind,
    FileBrowserListing,
    FileBrowserRequest,
    FilePreview,
    FilePreviewRequest,
    ImportRequest,
    ImportResult,
    IngestionPlan,
    IngestionProgress,
    IngestionResult,
    IngestionStatus,
    Phase7Request,
    Phase7Snapshot,
    ReconciliationRequest,
    ReconciliationResult,
    ScheduleCommand,
    ScheduleCommandKind,
    ScheduleResult,
    SourceKind,
    StockSymbol,
    SubmissionRequest,
    SymbolDiscoveryRequest,
    SymbolDiscoveryResult,
    UtcRange,
)
from corthena.ui.datasets.models import (
    DatasetBuild,
    DatasetBuildRequest,
    DatasetSaveRequest,
    DatasetSaveResult,
)
from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    JobCommand,
    JobCommandResult,
    Phase8Request,
    Phase8Snapshot,
    RunComparison,
)
from corthena.ui.models_inference.models import (
    AliasCommand,
    AliasResult,
    ExportRequest,
    ExportResult,
    InferenceQuery,
    InferenceSnapshot,
    Phase9Request,
    Phase9Snapshot,
)
from corthena.ui.research.models import ResearchQuery, ResearchSnapshot
from corthena.ui.state import Snapshot


class CoordinatorUIClient:
    """Translate immutable UI values to the versioned loopback API."""

    def __init__(
        self,
        base_url: str,
        fallback: UIClientProtocol,
        client: httpx.Client | None = None,
        *,
        poll_seconds: float = 0.05,
        operation_timeout_seconds: float = 120.0,
    ) -> None:
        parsed = urlparse(base_url)
        if parsed.scheme not in {"http", "https"} or parsed.hostname not in {
            "127.0.0.1",
            "::1",
            "localhost",
        }:
            raise ValueError("coordinator URL must use loopback HTTP")
        self._fallback = fallback
        self._owns_client = client is None
        self._client = client or httpx.Client(base_url=base_url, timeout=20.0)
        self._poll_seconds = poll_seconds
        self._operation_timeout = operation_timeout_seconds

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def load_snapshot(
        self,
        request_id: str,
        generation: int,
        cancellation: CancellationSignalProtocol,
    ) -> Snapshot:
        return self._fallback.load_snapshot(request_id, generation, cancellation)

    def load_research(
        self, query: ResearchQuery, cancellation: CancellationSignalProtocol
    ) -> ResearchSnapshot:
        return self._fallback.load_research(query, cancellation)

    def load_phase7(
        self, request: Phase7Request, cancellation: CancellationSignalProtocol
    ) -> Phase7Snapshot:
        simulated = self._fallback.load_phase7(request, cancellation)
        reconciled = self.reconcile_data(
            ReconciliationRequest(request.request_id, request.generation), cancellation
        )
        credential = self.credential_status(
            CredentialRequest(request.request_id, request.request_id, request.generation),
            cancellation,
        ).status
        return replace(
            simulated,
            catalog=reconciled.catalog,
            schedules=reconciled.schedules,
            credential=credential,
        )

    def import_data(
        self, request: ImportRequest, cancellation: CancellationSignalProtocol
    ) -> ImportResult:
        return self._fallback.import_data(request, cancellation)

    def credential_status(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        self._cancel(cancellation)
        response = self._request("GET", "/api/v1/settings/api-tokens/massive")
        status = self._credential(CredentialStatusDTO.model_validate(response.json()))
        return CredentialResult(request, status, IngestionStatus.COMPLETE)

    def save_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        self._cancel(cancellation)
        body = CredentialSecretDTO(
            command_id=request.request.command_id,
            correlation_id=request.request.request_id,
            token=SecretStr(request.secret),
        )
        response = self._request(
            "PUT",
            "/api/v1/settings/api-tokens/massive",
            json={
                "schema_version": body.schema_version,
                "command_id": body.command_id,
                "correlation_id": body.correlation_id,
                "token": body.token.get_secret_value(),
            },
        )
        return CredentialResult(
            request.request,
            self._credential(CredentialStatusDTO.model_validate(response.json())),
            IngestionStatus.COMPLETE,
        )

    def test_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        self._cancel(cancellation)
        body = CredentialTestDTO(
            command_id=request.request.command_id,
            correlation_id=request.request.request_id,
            token=SecretStr(request.secret),
        )
        response = self._request(
            "POST",
            "/api/v1/settings/api-tokens/massive/test",
            json={
                "schema_version": body.schema_version,
                "command_id": body.command_id,
                "correlation_id": body.correlation_id,
                "token": None if body.token is None else body.token.get_secret_value(),
            },
        )
        status = self._credential(CredentialStatusDTO.model_validate(response.json()))
        outcome = (
            IngestionStatus.COMPLETE
            if status.last_test_succeeded is not False
            else IngestionStatus.AUTHENTICATION_FAILED
        )
        return CredentialResult(request.request, status, outcome)

    def delete_credential(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        self._cancel(cancellation)
        body = CommandDTO(command_id=request.command_id, correlation_id=request.request_id)
        response = self._request(
            "DELETE",
            "/api/v1/settings/api-tokens/massive",
            json=body.model_dump(mode="json"),
        )
        return CredentialResult(
            request,
            self._credential(CredentialStatusDTO.model_validate(response.json())),
            IngestionStatus.COMPLETE,
        )

    def preview_file(
        self, request: FilePreviewRequest, cancellation: CancellationSignalProtocol
    ) -> FilePreview:
        self._cancel(cancellation)
        body = PreviewRequestDTO(
            command_id=request.request_id,
            correlation_id=request.request_id,
            path=request.source_name,
            source_kind=(
                ApiSource.CSV if request.source_kind.value == "csv" else ApiSource.PARQUET
            ),
            max_rows=request.max_rows,
            max_bytes=request.max_bytes,
        )
        response = self._request(
            "POST", "/api/v1/data/files/preview", json=body.model_dump(mode="json")
        )
        preview = FilePreviewDTO.model_validate(response.json())
        columns = tuple(
            self._ui_column(item.source_column, item.role, item.source_type)
            for item in preview.columns
        )
        return FilePreview(request, columns, len(preview.rows))

    def browse_files(
        self, request: FileBrowserRequest, cancellation: CancellationSignalProtocol
    ) -> FileBrowserListing:
        self._cancel(cancellation)
        location = Path.home() if request.location is None else Path(request.location)
        try:
            location = location.resolve(strict=True)
        except OSError as error:
            raise ValueError(f"Cannot open folder: {error}") from error
        if not location.is_dir():
            raise ValueError("Cannot open folder: path is not a directory")
        suffixes = (
            frozenset((".csv", ".parquet"))
            if request.source_kind is None
            else frozenset((f".{request.source_kind.value}",))
        )
        entries: list[FileBrowserEntry] = []
        parent = location.parent.resolve()
        parent_location = None if parent == location else str(parent)
        if parent_location is not None:
            entries.append(
                FileBrowserEntry(
                    parent_location,
                    "..",
                    None,
                    FileBrowserEntryKind.PARENT,
                )
            )
        try:
            children = tuple(location.iterdir())
        except OSError as error:
            raise ValueError(f"Cannot list folder: {error}") from error
        for path in children:
            self._cancel(cancellation)
            try:
                source_name = str(path)
                if path.is_dir():
                    entries.append(
                        FileBrowserEntry(
                            source_name,
                            path.name,
                            None,
                            FileBrowserEntryKind.FOLDER,
                        )
                    )
                elif path.is_file() and path.suffix.casefold() in suffixes:
                    source_kind = (
                        SourceKind.PARQUET
                        if path.suffix.casefold() == ".parquet"
                        else SourceKind.CSV
                    )
                    entries.append(
                        FileBrowserEntry(
                            source_name,
                            path.name,
                            source_kind,
                            FileBrowserEntryKind.FILE,
                        )
                    )
            except OSError:
                continue
        entries.sort(key=lambda entry: entry.sort_key)
        offset = 0 if request.cursor is None else int(request.cursor)
        if offset > len(entries):
            raise ValueError("File browser cursor is outside the directory listing")
        finish = min(len(entries), offset + request.page_size)
        next_cursor = str(finish) if finish < len(entries) else None
        return FileBrowserListing(
            request,
            str(location),
            tuple(entries[offset:finish]),
            parent_location if request.cursor is None else None,
            next_cursor,
        )

    def discover_symbols(
        self, request: SymbolDiscoveryRequest, cancellation: CancellationSignalProtocol
    ) -> SymbolDiscoveryResult:
        cursor: str | None = None
        page: SymbolPageDTO | None = None
        for _ in range(request.page):
            self._cancel(cancellation)
            response = self._request(
                "GET",
                "/api/v1/data/providers/massive/symbols",
                params={
                    "query": request.query,
                    "limit": str(request.page_size),
                    **({} if cursor is None else {"cursor": cursor}),
                },
            )
            page = SymbolPageDTO.model_validate(response.json())
            cursor = page.next_cursor
            if cursor is None:
                break
        if page is None:
            raise RuntimeError("symbol discovery did not execute")
        symbols = tuple(StockSymbol(item.symbol, item.name) for item in page.symbols)
        return SymbolDiscoveryResult(request, symbols, page.next_cursor is not None)

    def submit_file_ingestion(
        self, plan: IngestionPlan, cancellation: CancellationSignalProtocol
    ) -> IngestionResult:
        return self._submit(plan, False, cancellation)

    def submit_massive_pull(
        self, plan: IngestionPlan, cancellation: CancellationSignalProtocol
    ) -> IngestionResult:
        return self._submit(plan, True, cancellation)

    def mutate_schedule(
        self, command: ScheduleCommand, cancellation: CancellationSignalProtocol
    ) -> ScheduleResult:
        self._cancel(cancellation)
        schedule = self._api_schedule(command.schedule)
        if command.kind is ScheduleCommandKind.CREATE:
            body = ScheduleCommandDTO(
                command_id=command.command_id,
                correlation_id=command.request_id,
                schedule=schedule,
                expected_revision=command.expected_revision,
            )
            response = self._request(
                "POST", "/api/v1/data/schedules", json=body.model_dump(mode="json")
            )
            returned = self._ui_schedule(ScheduleDTO.model_validate(response.json()))
            return ScheduleResult(command, returned, self._get_schedules())
        if command.kind is ScheduleCommandKind.DELETE:
            body = DeleteCommandDTO(
                command_id=command.command_id,
                correlation_id=command.request_id,
                expected_revision=command.expected_revision,
            )
            self._request(
                "DELETE",
                f"/api/v1/data/schedules/{command.schedule.schedule_id}",
                json=body.model_dump(mode="json"),
            )
            return ScheduleResult(command, None, self._get_schedules())
        if command.kind is ScheduleCommandKind.RUN_NOW:
            body = CommandDTO(command_id=command.command_id, correlation_id=command.request_id)
            response = self._request(
                "POST",
                f"/api/v1/data/schedules/{command.schedule.schedule_id}/run",
                json=body.model_dump(mode="json"),
            )
            accepted = OperationAcceptedDTO.model_validate(response.json())
            progress = self._poll(accepted, cancellation)
            schedules = self._get_schedules()
            returned = next(
                item for item in schedules if item.schedule_id == command.schedule.schedule_id
            )
            return ScheduleResult(command, returned, schedules, self._ui_progress(progress))
        body = SchedulePatchDTO(
            command_id=command.command_id,
            correlation_id=command.request_id,
            schedule=schedule,
            expected_revision=command.expected_revision,
        )
        response = self._request(
            "PATCH",
            f"/api/v1/data/schedules/{command.schedule.schedule_id}",
            json=body.model_dump(mode="json"),
        )
        returned = self._ui_schedule(ScheduleDTO.model_validate(response.json()))
        return ScheduleResult(command, returned, self._get_schedules())

    def reconcile_data(
        self, request: ReconciliationRequest, cancellation: CancellationSignalProtocol
    ) -> ReconciliationResult:
        self._cancel(cancellation)
        response = self._request("GET", "/api/v1/data/reconciliation")
        reconciled = ReconciliationDTO.model_validate(response.json())
        return ReconciliationResult(
            request,
            tuple(self._ui_catalog(item) for item in reconciled.catalog.revisions),
            tuple(self._ui_schedule(item) for item in reconciled.schedules),
            reconciled.recovered_operation_ids,
        )

    def save_dataset(
        self, request: DatasetSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetSaveResult:
        return self._fallback.save_dataset(request, cancellation)

    def build_dataset(
        self, request: DatasetBuildRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetBuild:
        return self._fallback.build_dataset(request, cancellation)

    def evaluate_draft(
        self,
        request_id: str,
        generation: int,
        draft: ExperimentDraft,
        cancellation: CancellationSignalProtocol,
    ) -> DraftEvaluation:
        return self._fallback.evaluate_draft(request_id, generation, draft, cancellation)

    def save_draft(
        self, request: DraftSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DraftSaveResult:
        return self._fallback.save_draft(request, cancellation)

    def submit_experiment(
        self, request: SubmissionRequest, cancellation: CancellationSignalProtocol
    ) -> ExperimentDefinition:
        return self._fallback.submit_experiment(request, cancellation)

    def load_phase8(
        self, request: Phase8Request, cancellation: CancellationSignalProtocol
    ) -> Phase8Snapshot:
        return self._fallback.load_phase8(request, cancellation)

    def command_job(
        self, command: JobCommand, cancellation: CancellationSignalProtocol
    ) -> JobCommandResult:
        return self._fallback.command_job(command, cancellation)

    def compare_runs(
        self, query: ComparisonQuery, cancellation: CancellationSignalProtocol
    ) -> RunComparison:
        return self._fallback.compare_runs(query, cancellation)

    def load_phase9(
        self, request: Phase9Request, cancellation: CancellationSignalProtocol
    ) -> Phase9Snapshot:
        return self._fallback.load_phase9(request, cancellation)

    def assign_alias(
        self, command: AliasCommand, cancellation: CancellationSignalProtocol
    ) -> AliasResult:
        return self._fallback.assign_alias(command, cancellation)

    def score_inference(
        self, query: InferenceQuery, cancellation: CancellationSignalProtocol
    ) -> InferenceSnapshot:
        return self._fallback.score_inference(query, cancellation)

    def prepare_export(
        self, request: ExportRequest, cancellation: CancellationSignalProtocol
    ) -> ExportResult:
        return self._fallback.prepare_export(request, cancellation)

    def _submit(
        self,
        plan: IngestionPlan,
        provider: bool,
        cancellation: CancellationSignalProtocol,
    ) -> IngestionResult:
        source = (
            ApiSource.MASSIVE if provider else ApiSource(Path(plan.source_path or "").suffix[1:])
        )
        mapping = tuple(
            ColumnMappingDTO(
                source_column=item.source_column,
                role=self._api_role(item.role),
                source_type=item.source_type,
            )
            for item in plan.mapping
        )
        body = ImportPlanDTO(
            command_id=plan.command_id,
            correlation_id=plan.correlation_id,
            generation=plan.generation,
            source_kind=source,
            source_path=None if provider else plan.source_path,
            mapping=() if provider else mapping,
            source_timezone="UTC" if provider else plan.source_timezone,
            dataset_id=plan.dataset_id,
            dataset_name=plan.dataset_name or plan.dataset_id,
            expected_catalog_revision=plan.expected_revision,
            symbols=plan.symbols,
            interval=Interval(plan.interval),
            requested_range=UtcRangeDTO(
                start=plan.requested_range.start, end=plan.requested_range.end
            ),
            session_policy=ApiSession(plan.session.value),
            adjustment_policy=ApiAdjustment(plan.adjustment.value),
            mode=ApiImportMode(plan.mode.value),
        )
        endpoint = "/api/v1/data/providers/massive/pulls" if provider else "/api/v1/data/imports"
        response = self._request("POST", endpoint, json=body.model_dump(mode="json"))
        progress = self._poll(OperationAcceptedDTO.model_validate(response.json()), cancellation)
        catalog_entry = None
        if progress.state is OperationState.COMPLETE:
            catalog = CatalogDTO.model_validate(self._request("GET", "/api/v1/data/catalog").json())
            revision = next(
                item for item in catalog.revisions if item.dataset_id == plan.dataset_id
            )
            catalog_entry = self._ui_catalog(revision)
        return IngestionResult(plan, self._ui_progress(progress), catalog_entry)

    def _poll(
        self,
        accepted: OperationAcceptedDTO,
        cancellation: CancellationSignalProtocol,
    ) -> ProgressDTO:
        deadline = time.monotonic() + self._operation_timeout
        while time.monotonic() < deadline:
            if cancellation.is_set():
                body = CommandDTO(
                    command_id=f"cancel-{accepted.operation_id}",
                    correlation_id=accepted.correlation_id,
                )
                self._request(
                    "POST",
                    f"/api/v1/data/imports/{accepted.operation_id}/cancel",
                    json=body.model_dump(mode="json"),
                )
            response = self._request("GET", accepted.reconciliation_path)
            progress = ProgressDTO.model_validate(response.json())
            if progress.state in {
                OperationState.CANCELLED,
                OperationState.COMPLETE,
                OperationState.FAILED,
            }:
                return progress
            cancellation.wait(self._poll_seconds)
        raise TimeoutError("coordinator operation reconciliation timed out")

    def _get_schedules(self) -> tuple[DataSchedule, ...]:
        response = self._request("GET", "/api/v1/data/schedules")
        value = response.json()
        if not isinstance(value, list):
            raise RuntimeError("coordinator schedule response is invalid")
        return tuple(self._ui_schedule(ScheduleDTO.model_validate(item)) for item in value)

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
            try:
                document = response.json()
                message = document.get("message", "Coordinator request failed")
                field = document.get("field")
                if field:
                    message = f"{message}: {field}"
            except TypeError, ValueError:
                message = "Coordinator request failed"
            raise RuntimeError(str(message))
        return response

    @staticmethod
    def _credential(value: CredentialStatusDTO) -> CredentialStatus:
        return CredentialStatus(
            value.provider.value,
            value.saved,
            value.last_tested_at,
            value.last_test_succeeded,
            value.safe_detail,
        )

    @staticmethod
    def _ui_catalog(value: CatalogRevisionDTO) -> DatasetCatalogEntry:
        return DatasetCatalogEntry(
            value.dataset_id,
            value.name,
            value.revision,
            value.content_fingerprint,
            "sha256:real-ingestion-v1",
            value.symbols,
            value.interval.value,
            UtcRange(value.coverage.start, value.coverage.end),
            value.row_count,
            AdjustmentPolicy(value.adjustment_policy.value),
        )

    @staticmethod
    def _api_schedule(value: DataSchedule) -> ScheduleDTO:
        anchor = value.range_anchor or value.last_run_at or value.next_run_at
        if anchor is None:
            anchor = datetime(2000, 1, 1, tzinfo=UTC)
        return ScheduleDTO(
            schedule_id=value.schedule_id,
            revision=value.revision,
            name=value.name,
            dataset_id=value.dataset_id,
            symbols=value.symbols,
            interval=Interval(value.interval),
            session_policy=ApiSession(value.session.value),
            adjustment_policy=ApiAdjustment(value.adjustment.value),
            cadence=ApiCadence(value.cadence.value),
            enabled=value.enabled,
            range_anchor=anchor,
            last_run_at=value.last_run_at,
            next_run_at=value.next_run_at,
        )

    @staticmethod
    def _ui_schedule(value: ScheduleDTO) -> DataSchedule:
        from corthena.ui.data_experiments.models import ScheduleCadence, SessionPolicy

        return DataSchedule(
            value.schedule_id,
            value.revision,
            value.name,
            value.dataset_id,
            value.symbols,
            value.interval.value,
            SessionPolicy(value.session_policy.value),
            AdjustmentPolicy(value.adjustment_policy.value),
            ScheduleCadence(value.cadence.value),
            value.enabled,
            value.last_run_at,
            value.next_run_at,
            value.range_anchor,
        )

    @staticmethod
    def _ui_progress(value: ProgressDTO) -> IngestionProgress:
        statuses = {
            OperationState.QUEUED: IngestionStatus.QUEUED,
            OperationState.RUNNING: IngestionStatus.RUNNING,
            OperationState.CANCELLING: IngestionStatus.CANCELLING,
            OperationState.CANCELLED: IngestionStatus.CANCELLED,
            OperationState.COMPLETE: IngestionStatus.COMPLETE,
            OperationState.FAILED: IngestionStatus.FAILED,
        }
        lowered = value.message.casefold()
        status = statuses[value.state]
        if value.state is OperationState.FAILED:
            if "authentication" in lowered:
                status = IngestionStatus.AUTHENTICATION_FAILED
            elif "entitlement" in lowered:
                status = IngestionStatus.ENTITLEMENT_FAILED
            elif "rate limit" in lowered:
                status = IngestionStatus.RATE_LIMITED
            elif "validation" in lowered or "invalid" in lowered:
                status = IngestionStatus.VALIDATION_FAILED
        return IngestionProgress(
            value.correlation_id,
            value.generation,
            status,
            value.completed_units,
            value.total_units,
            value.message,
            value.provider_request_id,
            None if value.retry_after_seconds is None else int(value.retry_after_seconds),
        )

    @staticmethod
    def _ui_column(source_column: str, role: str, source_type: str) -> ColumnMapping:
        return ColumnMapping(source_column, role, source_type)

    @staticmethod
    def _api_role(
        role: str,
    ) -> (
        Literal["timestamp"]
        | Literal["symbol"]
        | Literal["open"]
        | Literal["high"]
        | Literal["low"]
        | Literal["close"]
        | Literal["volume"]
        | Literal["ignore"]
    ):
        match role:
            case "timestamp":
                return "timestamp"
            case "symbol":
                return "symbol"
            case "open":
                return "open"
            case "high":
                return "high"
            case "low":
                return "low"
            case "close":
                return "close"
            case "volume":
                return "volume"
            case "ignore":
                return "ignore"
            case _:
                raise ValueError("unsupported column role")

    @staticmethod
    def _cancel(cancellation: CancellationSignalProtocol) -> None:
        if cancellation.is_set():
            raise RuntimeError("coordinator UI operation was cancelled")
