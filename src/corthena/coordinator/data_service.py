"""Coordinator-owned real Data ingestion application service."""

from __future__ import annotations

import hashlib
import shutil
import threading
import time
from collections.abc import Callable
from concurrent.futures import Future, ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from pathlib import Path

from corthena.config.paths import ApplicationPaths
from corthena.contracts.data import (
    CatalogDTO,
    CatalogRevisionDTO,
    CredentialStatusDTO,
    FilePreviewDTO,
    ImportMode,
    ImportPlanDTO,
    Interval,
    OperationAcceptedDTO,
    OperationState,
    PreviewRequestDTO,
    ProgressDTO,
    ProvenanceDTO,
    ReconciliationDTO,
    ScheduleDTO,
    SourceKind,
    SymbolPageDTO,
    UtcRangeDTO,
)
from corthena.data.arrow import ArrowIngestionAdapter, source_checksum
from corthena.data.errors import (
    AuthenticationDataError,
    CancelledDataError,
    CapacityDataError,
    DataError,
    EntitlementDataError,
    PublicationDataError,
    RateLimitDataError,
    StaleRevisionError,
    ValidationDataError,
)
from corthena.data.protocol import (
    FileIngestionProtocol,
    MarketCalendarProtocol,
    MarketDataConnectorProtocol,
)
from corthena.data.types import CanonicalBar
from corthena.events.data import DataEventBus
from corthena.persistence.protocol import DataRepositoryProtocol
from corthena.platform.credentials.store import CredentialStore
from corthena.ui.client.protocol import CancellationSignalProtocol

_INTERVAL_DURATION = {
    Interval.MINUTE_1: timedelta(minutes=1),
    Interval.MINUTE_5: timedelta(minutes=5),
    Interval.MINUTE_15: timedelta(minutes=15),
    Interval.HOUR_1: timedelta(hours=1),
    Interval.DAY_1: timedelta(days=1),
}


class CancellationSignal:
    """Explicit owned cancellation event."""

    def __init__(self) -> None:
        self._event = threading.Event()

    def set(self) -> None:
        self._event.set()

    def is_set(self) -> bool:
        return self._event.is_set()

    def wait(self, timeout: float | None = None) -> bool:
        return self._event.wait(timeout)


class DataCoordinatorService:
    """Own bounded imports, durable metadata, provider calls, and publication."""

    def __init__(
        self,
        paths: ApplicationPaths,
        repository: DataRepositoryProtocol,
        credentials: CredentialStore,
        connector: MarketDataConnectorProtocol,
        calendar: MarketCalendarProtocol,
        *,
        arrow: FileIngestionProtocol | None = None,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
        worker_count: int = 2,
        queue_capacity: int = 8,
        events: DataEventBus | None = None,
        max_schedule_catchup: timedelta = timedelta(days=30),
    ) -> None:
        if worker_count < 1 or queue_capacity < worker_count:
            raise ValueError("ingestion worker bounds are invalid")
        if max_schedule_catchup <= timedelta(0):
            raise ValueError("schedule catch-up bound must be positive")
        self._paths = paths
        self._repository = repository
        self._credentials = credentials
        self._connector = connector
        self._calendar = calendar
        self._arrow = arrow or ArrowIngestionAdapter()
        self._clock = clock
        self._executor = ThreadPoolExecutor(
            max_workers=worker_count, thread_name_prefix="data-import"
        )
        self._capacity = threading.BoundedSemaphore(queue_capacity)
        self._lock = threading.RLock()
        self._condition = threading.Condition(self._lock)
        self._cancellations: dict[str, CancellationSignal] = {}
        self._futures: dict[str, Future[None]] = {}
        self._active_auxiliary = 0
        self._stopping = False
        self._closed = False
        self.events = events or DataEventBus(clock=clock)
        self._recovered = repository.recover_incomplete()
        self._max_schedule_catchup = max_schedule_catchup

    @property
    def recovered_operation_ids(self) -> tuple[str, ...]:
        return self._recovered

    def close(self, timeout_seconds: float = 10.0) -> None:
        with self._lock:
            if self._closed:
                return
            self._stopping = True
            signals = tuple(self._cancellations.values())
        for signal in signals:
            signal.set()
        deadline = time.monotonic() + timeout_seconds
        with self._condition:
            while self._futures or self._active_auxiliary:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    break
                self._condition.wait(remaining)
            unfinished = bool(self._futures or self._active_auxiliary)
        self._executor.shutdown(wait=not unfinished, cancel_futures=True)
        self.events.close()
        self._connector.close()
        with self._lock:
            self._closed = True
        if unfinished:
            raise RuntimeError("data workers did not stop within the shutdown deadline")

    def catalog(self) -> CatalogDTO:
        return CatalogDTO(revisions=self._repository.catalog())

    def reconcile(self) -> ReconciliationDTO:
        return ReconciliationDTO(
            catalog=self.catalog(),
            schedules=self.schedules(),
            recovered_operation_ids=self._recovered,
            latest_event_sequence=self.events.sequence,
        )

    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO:
        return self._bounded_call(lambda: self._arrow.preview(request), timeout_seconds=30.0)

    def credential_status(self) -> CredentialStatusDTO:
        stored = self._credentials.status()
        tested_at, succeeded, detail = self._repository.credential_test_status()
        return CredentialStatusDTO(
            saved=stored.saved,
            last_tested_at=None if tested_at is None else datetime.fromisoformat(tested_at),
            last_test_succeeded=succeeded,
            safe_detail=detail,
        )

    def save_credential(self, token: str) -> CredentialStatusDTO:
        self._credentials.save(token)
        return self.credential_status()

    def test_credential(
        self, token: str | None, cancellation: CancellationSignalProtocol
    ) -> CredentialStatusDTO:
        selected = token if token is not None else self._credentials.load()
        now = self._clock()
        try:
            self._bounded_call(
                lambda: self._connector.test_connection(selected, cancellation),
                timeout_seconds=30.0,
            )
        except AuthenticationDataError:
            self._repository.set_credential_test_status(
                now.isoformat(), False, "Authentication failed"
            )
            return self.credential_status()
        self._repository.set_credential_test_status(now.isoformat(), True, "Connection succeeded")
        return self.credential_status()

    def delete_credential(self) -> CredentialStatusDTO:
        self._credentials.delete()
        self._repository.clear_credential_test_status()
        return self.credential_status()

    def discover_symbols(
        self,
        query: str,
        page_size: int,
        cursor: str | None,
        cancellation: CancellationSignalProtocol,
    ) -> SymbolPageDTO:
        token = self._credentials.load()
        return self._bounded_call(
            lambda: self._connector.discover_symbols(token, query, page_size, cursor, cancellation),
            timeout_seconds=30.0,
        )

    def submit(self, plan: ImportPlanDTO) -> OperationAcceptedDTO:
        operation_id = f"import-{hashlib.sha256(plan.command_id.encode()).hexdigest()[:24]}"
        fingerprint = hashlib.sha256(plan.model_dump_json().encode()).hexdigest()
        prior = self._repository.progress_by_command(plan.command_id)
        if prior is not None:
            self._repository.claim_command(
                plan.command_id, "import", prior.operation_id, fingerprint
            )
            return OperationAcceptedDTO(
                operation_id=prior.operation_id,
                correlation_id=prior.correlation_id,
                state=prior.state,
                reconciliation_path=f"/api/v1/data/imports/{prior.operation_id}",
            )
        with self._lock:
            if self._stopping:
                raise CapacityDataError("coordinator is stopping")
            if not self._capacity.acquire(blocking=False):
                raise CapacityDataError("ingestion queue is full")
            cancellation = CancellationSignal()
            queued = ProgressDTO(
                operation_id=operation_id,
                correlation_id=plan.correlation_id,
                generation=plan.generation,
                state=OperationState.QUEUED,
                completed_units=0,
                total_units=100,
                message="Import queued",
            )
            try:
                claimed = self._repository.claim_command(
                    plan.command_id, "import", operation_id, fingerprint
                )
                if not claimed:
                    prior = self._repository.progress_by_command(plan.command_id)
                    if prior is None:
                        raise CapacityDataError("accepted command is awaiting reconciliation")
                    self._capacity.release()
                    return OperationAcceptedDTO(
                        operation_id=prior.operation_id,
                        correlation_id=prior.correlation_id,
                        state=prior.state,
                        reconciliation_path=f"/api/v1/data/imports/{prior.operation_id}",
                    )
                self._repository.put_progress(queued, plan.command_id)
                future = self._executor.submit(self._run_import, operation_id, plan, cancellation)
            except Exception:
                self._capacity.release()
                raise
            self._cancellations[operation_id] = cancellation
            self._futures[operation_id] = future
            future.add_done_callback(
                lambda completed: self._release_operation(operation_id, completed)
            )
        return OperationAcceptedDTO(
            operation_id=operation_id,
            correlation_id=plan.correlation_id,
            state=OperationState.QUEUED,
            reconciliation_path=f"/api/v1/data/imports/{operation_id}",
        )

    def progress(self, operation_id: str) -> ProgressDTO | None:
        return self._repository.progress(operation_id)

    def cancel(self, operation_id: str) -> ProgressDTO:
        progress = self._repository.progress(operation_id)
        if progress is None:
            raise KeyError(operation_id)
        with self._lock:
            signal = self._cancellations.get(operation_id)
        if signal is None or progress.state in {
            OperationState.CANCELLED,
            OperationState.COMPLETE,
            OperationState.FAILED,
        }:
            return progress
        signal.set()
        updated = progress.model_copy(
            update={"state": OperationState.CANCELLING, "message": "Cancellation requested"}
        )
        self._repository.put_progress(updated, self._command_id(operation_id))
        return updated

    def schedules(self) -> tuple[ScheduleDTO, ...]:
        return self._repository.schedules()

    def create_schedule(self, schedule: ScheduleDTO, command_id: str) -> ScheduleDTO:
        fingerprint = hashlib.sha256(schedule.model_dump_json().encode()).hexdigest()
        if not self._repository.claim_command(
            command_id, "schedule.create", schedule.schedule_id, fingerprint
        ):
            current = self._repository.schedule(schedule.schedule_id)
            if current is None:
                raise StaleRevisionError("accepted schedule command is not reconcilable")
            return current
        return self._repository.create_schedule(schedule)

    def update_schedule(
        self, schedule: ScheduleDTO, expected_revision: int, command_id: str
    ) -> ScheduleDTO:
        fingerprint = hashlib.sha256(
            f"{expected_revision}:{schedule.model_dump_json()}".encode()
        ).hexdigest()
        if not self._repository.claim_command(
            command_id, "schedule.update", schedule.schedule_id, fingerprint
        ):
            current = self._repository.schedule(schedule.schedule_id)
            if current is None:
                raise StaleRevisionError("accepted schedule command is not reconcilable")
            return current
        return self._repository.update_schedule(schedule, expected_revision)

    def delete_schedule(self, schedule_id: str, expected_revision: int, command_id: str) -> None:
        fingerprint = hashlib.sha256(f"{schedule_id}:{expected_revision}".encode()).hexdigest()
        if not self._repository.claim_command(
            command_id, "schedule.delete", schedule_id, fingerprint
        ):
            return
        self._repository.delete_schedule(schedule_id, expected_revision)

    def run_schedule(
        self, schedule_id: str, command_id: str, correlation_id: str
    ) -> OperationAcceptedDTO:
        schedule = self._repository.schedule(schedule_id)
        if schedule is None:
            raise KeyError(schedule_id)
        current = self._repository.current_revision(schedule.dataset_id)
        expected = 0 if current is None else current.revision
        now = self._clock()
        completed = self._calendar.last_completed_bar_start(schedule.interval, now)
        start = schedule.range_anchor
        if schedule.last_run_at is not None:
            start = min(schedule.last_run_at - _INTERVAL_DURATION[schedule.interval], completed)
        end = completed + _INTERVAL_DURATION[schedule.interval]
        start = max(start, end - self._max_schedule_catchup)
        if start >= end:
            start = end - _INTERVAL_DURATION[schedule.interval]
        plan = ImportPlanDTO(
            command_id=command_id,
            correlation_id=correlation_id,
            generation=schedule.revision,
            source_kind=SourceKind.MASSIVE,
            mapping=(),
            dataset_id=schedule.dataset_id,
            dataset_name=schedule.name,
            expected_catalog_revision=expected,
            symbols=schedule.symbols,
            interval=schedule.interval,
            requested_range=UtcRangeDTO(start=start, end=end),
            session_policy=schedule.session_policy,
            adjustment_policy=schedule.adjustment_policy,
            mode=ImportMode.CREATE if current is None else ImportMode.REPLACE_RANGE,
        )
        accepted = self.submit(plan)
        next_run = self._next_run(schedule.cadence.value, now)
        self._repository.update_schedule(
            schedule.model_copy(update={"last_run_at": now, "next_run_at": next_run}),
            schedule.revision,
        )
        return accepted

    def _run_import(
        self, operation_id: str, plan: ImportPlanDTO, cancellation: CancellationSignal
    ) -> None:
        current = self._repository.current_revision(plan.dataset_id)
        started = self._clock()
        promoted: Path | None = None
        provider_request_ids: tuple[str, ...] = ()
        try:
            self._set_progress(operation_id, plan, OperationState.RUNNING, 5, "Validating import")
            observed_revision = 0 if current is None else current.revision
            if observed_revision != plan.expected_catalog_revision:
                raise StaleRevisionError("catalog revision changed")
            if plan.source_kind is SourceKind.MASSIVE:
                token = self._credentials.load()
                pulled: list[CanonicalBar] = []
                identities: list[str] = []
                for index, symbol in enumerate(plan.symbols):
                    self._cancel(cancellation)
                    result = self._connector.pull_bars(
                        token,
                        symbol,
                        plan.interval,
                        plan.requested_range,
                        plan.adjustment_policy,
                        cancellation,
                    )
                    pulled.extend(result.bars)
                    identities.extend(result.request_ids)
                    self._set_progress(
                        operation_id,
                        plan,
                        OperationState.RUNNING,
                        10 + int(35 * (index + 1) / len(plan.symbols)),
                        "Downloading provider bars",
                        result.request_ids[-1] if result.request_ids else None,
                    )
                new_bars = tuple(sorted(pulled))
                cutoff = self._calendar.last_completed_bar_start(plan.interval, self._clock())
                if plan.interval is Interval.DAY_1:
                    new_bars = tuple(
                        bar for bar in new_bars if bar.timestamp.date() <= cutoff.date()
                    )
                else:
                    new_bars = tuple(bar for bar in new_bars if bar.timestamp <= cutoff)
                provider_request_ids = tuple(identities)
                ArrowIngestionAdapter.validate(
                    new_bars,
                    plan.interval,
                    plan.session_policy.value == "regular",
                    self._calendar,
                )
                source_identity = "massive:" + ",".join(provider_request_ids)
            else:
                if plan.source_path is None:
                    raise AssertionError("validated file plan lost its source path")
                file_path = Path(plan.source_path)
                checksum_before = source_checksum(file_path)
                new_bars = self._arrow.read_bars(plan, self._calendar, cancellation)
                source_identity = source_checksum(file_path)
                if source_identity != checksum_before:
                    raise ValidationDataError("source file changed during ingestion")
            previous = (
                () if current is None else self._arrow.read_revision(Path(current.revision_path))
            )
            merged = self._merge(previous, new_bars, plan)
            self._cancel(cancellation)
            self._set_progress(operation_id, plan, OperationState.RUNNING, 60, "Writing revision")
            completed = self._clock()
            duration = _INTERVAL_DURATION[plan.interval]
            actual_range = UtcRangeDTO(
                start=merged[0].timestamp, end=merged[-1].timestamp + duration
            )

            def provenance(
                checksums: tuple[tuple[str, str], ...], fingerprint: str
            ) -> ProvenanceDTO:
                return ProvenanceDTO(
                    source_kind=plan.source_kind,
                    source_identity=source_identity,
                    mapping=plan.mapping,
                    symbols=tuple(sorted({bar.symbol for bar in merged})),
                    interval=plan.interval,
                    session_policy=plan.session_policy,
                    adjustment_policy=plan.adjustment_policy,
                    requested_range=plan.requested_range,
                    actual_range=actual_range,
                    command_id=plan.command_id,
                    correlation_id=plan.correlation_id,
                    operation_id=operation_id,
                    dataset_id=plan.dataset_id,
                    parent_revision=observed_revision,
                    started_at=started,
                    completed_at=completed,
                    application_version="0.0.0",
                    connector_version=(
                        getattr(self._connector, "version", "market-data-connector-v1")
                        if plan.source_kind is SourceKind.MASSIVE
                        else "file-v1"
                    ),
                    calendar_version=self._calendar.version,
                    partition_checksums=checksums,
                    content_fingerprint=fingerprint,
                )

            promoted, manifest = self._arrow.write_revision(
                self._paths.revisions,
                plan.dataset_id,
                observed_revision + 1,
                merged,
                provenance,
            )
            revision = CatalogRevisionDTO(
                dataset_id=plan.dataset_id,
                name=plan.dataset_name,
                revision=observed_revision + 1,
                content_fingerprint=manifest.content_fingerprint,
                symbols=manifest.symbols,
                interval=plan.interval,
                session_policy=plan.session_policy,
                adjustment_policy=plan.adjustment_policy,
                coverage=actual_range,
                row_count=len(merged),
                revision_path=str(promoted),
                provenance=manifest,
            )
            self._repository.publish(revision, observed_revision)
            self._set_progress(
                operation_id,
                plan,
                OperationState.COMPLETE,
                100,
                "Import complete",
                provider_request_ids[-1] if provider_request_ids else None,
            )
            completed_progress = self._repository.progress(operation_id)
            if completed_progress is not None:
                self.events.publish_catalog(completed_progress, plan.dataset_id)
        except CancelledDataError:
            self._set_progress(operation_id, plan, OperationState.CANCELLED, 0, "Import cancelled")
        except DataError as error:
            if promoted is not None:
                self._remove_unindexed(promoted)
            self._set_progress(
                operation_id,
                plan,
                OperationState.FAILED,
                0,
                self._safe_failure_message(error),
                error.provider_request_id,
                error.retry_after_seconds,
            )
        except Exception:
            if promoted is not None:
                self._remove_unindexed(promoted)
            self._set_progress(operation_id, plan, OperationState.FAILED, 0, "Import failed")

    @staticmethod
    def _merge(
        previous: tuple[CanonicalBar, ...],
        incoming: tuple[CanonicalBar, ...],
        plan: ImportPlanDTO,
    ) -> tuple[CanonicalBar, ...]:
        existing = {(bar.symbol, bar.timestamp): bar for bar in previous}
        incoming_map = {(bar.symbol, bar.timestamp): bar for bar in incoming}
        if len(incoming_map) != len(incoming):
            raise PublicationDataError("incoming bars contain duplicate keys")
        if plan.mode is ImportMode.CREATE:
            if previous:
                raise StaleRevisionError("dataset already exists")
            merged = incoming_map
        elif plan.mode is ImportMode.APPEND:
            overlap = set(existing).intersection(incoming_map)
            if overlap:
                raise PublicationDataError("append contains existing keys")
            merged = existing | incoming_map
        else:
            merged = {
                key: bar
                for key, bar in existing.items()
                if not (
                    key[0] in plan.symbols
                    and plan.requested_range.start <= key[1] < plan.requested_range.end
                )
            }
            merged.update(incoming_map)
        if not merged:
            raise PublicationDataError("revision cannot be empty")
        return tuple(sorted(merged.values()))

    def _set_progress(
        self,
        operation_id: str,
        plan: ImportPlanDTO,
        state: OperationState,
        completed: int,
        message: str,
        provider_request_id: str | None = None,
        retry_after_seconds: float | None = None,
    ) -> None:
        progress = ProgressDTO(
            operation_id=operation_id,
            correlation_id=plan.correlation_id,
            generation=plan.generation,
            state=state,
            completed_units=completed,
            total_units=100,
            message=message,
            provider_request_id=provider_request_id,
            retry_after_seconds=retry_after_seconds,
        )
        self._repository.put_progress(progress, plan.command_id)
        self.events.publish_progress(progress)

    def _release_operation(self, operation_id: str, future: Future[None]) -> None:
        del future
        with self._lock:
            self._cancellations.pop(operation_id, None)
            self._futures.pop(operation_id, None)
            self._capacity.release()
            self._condition.notify_all()

    def _bounded_call[T](self, operation: Callable[[], T], *, timeout_seconds: float) -> T:
        with self._condition:
            if self._stopping:
                raise CapacityDataError("coordinator is stopping")
            if not self._capacity.acquire(blocking=False):
                raise CapacityDataError("coordinator Data queue is full")
            self._active_auxiliary += 1
            try:
                future = self._executor.submit(operation)
            except Exception:
                self._active_auxiliary -= 1
                self._capacity.release()
                self._condition.notify_all()
                raise
            future.add_done_callback(self._release_auxiliary)
        try:
            return future.result(timeout=timeout_seconds)
        except TimeoutError as error:
            future.cancel()
            raise CapacityDataError("coordinator Data operation timed out") from error

    def _release_auxiliary[T](self, future: Future[T]) -> None:
        del future
        with self._condition:
            self._active_auxiliary -= 1
            self._capacity.release()
            self._condition.notify_all()

    def _command_id(self, operation_id: str) -> str:
        del operation_id
        # put_progress only uses command_id on initial insertion; updates conflict by operation ID.
        return "existing-operation"

    @staticmethod
    def _safe_failure_message(error: DataError) -> str:
        if isinstance(error, AuthenticationDataError):
            return "Authentication failed"
        if isinstance(error, EntitlementDataError):
            return "Provider entitlement denied"
        if isinstance(error, RateLimitDataError):
            return "Provider rate limit exhausted"
        if isinstance(error, StaleRevisionError):
            return "Catalog revision changed"
        return str(error)

    def _remove_unindexed(self, path: Path) -> None:
        resolved = path.resolve()
        root = self._paths.revisions.resolve()
        if resolved.name.startswith("revision-") and root in resolved.parents and resolved.exists():
            shutil.rmtree(resolved)
            return
        raise PublicationDataError("unpublished revision path failed safety validation")

    @staticmethod
    def _cancel(cancellation: CancellationSignalProtocol) -> None:
        if cancellation.is_set():
            raise CancelledDataError("operation was cancelled")

    @staticmethod
    def _next_run(cadence: str, now: datetime) -> datetime:
        if cadence == "hourly":
            return now.replace(minute=0, second=0, microsecond=0) + timedelta(hours=1)
        return now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(days=1)
