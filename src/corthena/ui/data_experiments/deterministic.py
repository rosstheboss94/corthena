"""Deterministic, explicitly synchronized Phase 7 demo service."""

from __future__ import annotations

import hashlib
import threading
from dataclasses import replace
from datetime import UTC, datetime, timedelta

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol
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
    FeatureIdentity,
    FileBrowserEntry,
    FileBrowserEntryKind,
    FileBrowserListing,
    FileBrowserRequest,
    FilePreview,
    FilePreviewRequest,
    ImportMode,
    ImportRequest,
    ImportResult,
    ImportState,
    IngestionPlan,
    IngestionProgress,
    IngestionResult,
    IngestionScenario,
    IngestionStatus,
    Phase7Request,
    Phase7Scenario,
    Phase7Snapshot,
    ReconciliationRequest,
    ReconciliationResult,
    ResourceEstimate,
    ScheduleCadence,
    ScheduleCommand,
    ScheduleCommandKind,
    ScheduleResult,
    SessionPolicy,
    SourceKind,
    StockSymbol,
    SubmissionRequest,
    SymbolDiscoveryRequest,
    SymbolDiscoveryResult,
    UtcRange,
    ValidationDiagnostic,
)
from corthena.ui.datasets.models import (
    ColumnType,
    DatasetBuild,
    DatasetBuildRequest,
    DatasetDefinition,
    DatasetSaveRequest,
    DatasetSaveResult,
    DatasetVersion,
    LaggedReturnStep,
    RollingStatistic,
    RollingStatisticStep,
    RollingVolatilityStep,
    SourceColumn,
    SourceDefinition,
    SourceFamily,
    SourceProvenance,
    SourceProvider,
    SourceSelection,
    SourceSnapshot,
)
from corthena.ui.datasets.workflow import (
    ENGINE_FINGERPRINT,
    REGISTRY_VERSION,
    build_dataset_preview,
    mark_build_stale,
)

FEATURES = (
    FeatureIdentity("ret_5", "1.0.0", 5, "float32[ret_5]", "sha256:feature-ret5-v1"),
    FeatureIdentity(
        "volatility_20", "1.0.0", 20, "float32[volatility_20]", "sha256:feature-vol20-v1"
    ),
    FeatureIdentity("volume_z_30", "1.0.0", 30, "float32[volume_z_30]", "sha256:feature-volz30-v1"),
)


class DataExperimentsDemo:
    """Own mutable demo metadata and publish frozen snapshots under one lock."""

    def __init__(self, seed: int, fixed_clock: datetime) -> None:
        if fixed_clock.tzinfo is None or fixed_clock.utcoffset() != timedelta(0):
            raise ValueError("Phase 7 fixed clock must be UTC")
        self._seed = seed
        self._clock = fixed_clock
        self._lock = threading.Lock()
        self._catalog = _initial_catalog(seed)
        (
            self._sources,
            self._source_snapshots,
            self._dataset_definitions,
            self._dataset_versions,
            self._dataset_builds,
        ) = _initial_dataset_workflow(self._catalog, fixed_clock)
        self._dataset_save_commands: dict[str, DatasetSaveResult] = {}
        self._dataset_build_commands: dict[str, DatasetBuild] = {
            item.command_id: item for item in self._dataset_builds
        }
        self._imports: tuple[ImportResult, ...] = ()
        self._credential_saved = False
        self._credential_test: CredentialStatus | None = None
        self._schedules = _initial_schedules(fixed_clock)
        self._schedule_commands: dict[str, ScheduleResult] = {}
        primary = next(item for item in self._catalog if item.dataset_id == "dataset-us-equities")
        primary_build = next(
            item for item in self._dataset_builds if item.version.dataset_id == primary.dataset_id
        )
        self._draft = default_experiment_draft(primary, primary_build)
        self._saved_revision: int = 0
        self._submissions: dict[str, ExperimentDefinition] = {
            "demo-complete": ExperimentDefinition(
                "experiment-demo-complete", "demo-complete", fixed_clock, self._draft
            ),
            "demo-forest": ExperimentDefinition(
                "experiment-demo-forest", "demo-forest", fixed_clock, self._draft
            ),
        }

    def load(
        self, request: Phase7Request, cancellation: CancellationSignalProtocol
    ) -> Phase7Snapshot:
        _cancel(cancellation, request.request_id)
        if request.scenario is Phase7Scenario.LOADING:
            cancellation.wait()
            raise RequestCancelledError(request.request_id)
        if request.scenario is Phase7Scenario.CANCELLED:
            raise RequestCancelledError(request.request_id)
        if request.scenario is Phase7Scenario.FAILURE:
            raise RuntimeError("deterministic Phase 7 request failed")
        with self._lock:
            catalog = () if request.scenario is Phase7Scenario.EMPTY else self._catalog
            evaluation = evaluate_experiment(request.request_id, request.generation, self._draft)
            return Phase7Snapshot(
                request,
                catalog,
                self._imports,
                self._draft,
                evaluation,
                tuple(sorted(self._submissions.values(), key=lambda item: item.experiment_id)),
                self._seed,
                self._clock,
                request.scenario is Phase7Scenario.DEGRADED,
                replace(
                    self._credential_test or CredentialStatus("massive", False),
                    saved=self._credential_saved,
                ),
                self._schedules,
                self._sources,
                self._source_snapshots,
                self._dataset_definitions,
                self._dataset_versions,
                self._dataset_builds,
            )

    def run_import(
        self, request: ImportRequest, cancellation: CancellationSignalProtocol
    ) -> ImportResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            current = next(
                (item for item in self._catalog if item.dataset_id == request.dataset_id), None
            )
            if current is None:
                raise ValueError(f"unknown dataset {request.dataset_id!r}")
            if current.revision != request.expected_revision:
                result = _rejected_import(
                    request, current, "stale_revision", "Catalog revision changed"
                )
            elif request.interval != current.interval:
                result = _rejected_import(
                    request, current, "interval_mismatch", "Interval does not match dataset"
                )
            elif not set(request.symbols) <= set(current.symbols):
                result = _rejected_import(
                    request, current, "unknown_symbol", "Import contains an unknown symbol"
                )
            elif request.scenario in {"empty", "duplicate", "malformed"}:
                messages = {
                    "empty": ("empty_source", "Source contains no canonical bars"),
                    "duplicate": ("duplicate_key", "Duplicate (symbol, timestamp) key"),
                    "malformed": ("invalid_ohlcv", "OHLC values or volume are invalid"),
                }
                code, message = messages[request.scenario]
                result = _rejected_import(request, current, code, message)
            elif request.scenario == "failure":
                raise RuntimeError("deterministic import adapter failed")
            else:
                rows = 840 if request.mode.value == "append" else 420
                fingerprint = _fingerprint(
                    f"{current.content_fingerprint}|{request.command_id}|{request.mode.value}|{rows}"
                )
                updated = replace(
                    current,
                    revision=current.revision + 1,
                    row_count=current.row_count + rows,
                    content_fingerprint=fingerprint,
                )
                result = ImportResult(request, ImportState.READY, updated, (), rows)
                self._catalog = tuple(
                    updated if item.dataset_id == updated.dataset_id else item
                    for item in self._catalog
                )
                self._publish_legacy_source_refresh(
                    updated, request.command_id, request.correlation_id
                )
            self._imports = (*self._imports, result)[-64:]
            return result

    def credential_status(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            status = self._credential_test or CredentialStatus(
                request.provider, self._credential_saved
            )
            status = replace(status, saved=self._credential_saved)
        return CredentialResult(request, status, IngestionStatus.COMPLETE)

    def save_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        _cancel(cancellation, request.request.request_id)
        # The simulator deliberately consumes but never stores or returns request.secret.
        with self._lock:
            self._credential_saved = True
            status = CredentialStatus(
                request.request.provider,
                True,
                self._clock,
                True,
                "Saved in deterministic simulator memory only",
            )
            self._credential_test = status
        return CredentialResult(request.request, status, IngestionStatus.COMPLETE)

    def test_credential(
        self, request: CredentialSecretRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        _cancel(cancellation, request.request.request_id)
        succeeded = request.secret != "invalid"
        status = CredentialStatus(
            request.request.provider,
            self._credential_saved,
            self._clock,
            succeeded,
            "Connection test succeeded" if succeeded else "Authentication failed",
        )
        with self._lock:
            self._credential_test = status
        return CredentialResult(
            request.request,
            status,
            IngestionStatus.COMPLETE if succeeded else IngestionStatus.AUTHENTICATION_FAILED,
        )

    def delete_credential(
        self, request: CredentialRequest, cancellation: CancellationSignalProtocol
    ) -> CredentialResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            self._credential_saved = False
            self._credential_test = None
        return CredentialResult(
            request, CredentialStatus(request.provider, False), IngestionStatus.COMPLETE
        )

    def preview_file(
        self, request: FilePreviewRequest, cancellation: CancellationSignalProtocol
    ) -> FilePreview:
        _cancel(cancellation, request.request_id)
        columns = tuple(
            ColumnMapping(name, role, kind)
            for name, role, kind in (
                ("timestamp", "timestamp", "timestamp[ms]"),
                ("ticker", "symbol", "string"),
                ("open", "open", "float64"),
                ("high", "high", "float64"),
                ("low", "low", "float64"),
                ("close", "close", "float64"),
                ("volume", "volume", "int64"),
            )
        )
        diagnostics = (
            (ValidationDiagnostic("invalid_ohlcv", "High is below close", "high"),)
            if request.scenario is IngestionScenario.VALIDATION_FAILURE
            else ()
        )
        rows = 0 if request.scenario is IngestionScenario.EMPTY else 12
        return FilePreview(request, columns, rows, diagnostics)

    def browse_files(
        self, request: FileBrowserRequest, cancellation: CancellationSignalProtocol
    ) -> FileBrowserListing:
        """Return a bounded deterministic directory without reading the filesystem."""
        _cancel(cancellation, request.request_id)
        location = request.location or "C:\\Demo Data"
        suffixes = (
            (SourceKind.CSV, SourceKind.PARQUET)
            if request.source_kind is None
            else (request.source_kind,)
        )
        fixtures: dict[str, tuple[tuple[str, FileBrowserEntryKind], ...]] = {
            "C:\\": (("Demo Data", FileBrowserEntryKind.FOLDER),),
            "C:\\Demo Data": (
                ("Archive", FileBrowserEntryKind.FOLDER),
                ("Daily", FileBrowserEntryKind.FOLDER),
                *tuple(
                    (f"daily-us-equities.{source_kind.value}", FileBrowserEntryKind.FILE)
                    for source_kind in suffixes
                ),
                *tuple(
                    (f"intraday-bars.{source_kind.value}", FileBrowserEntryKind.FILE)
                    for source_kind in suffixes
                ),
                *tuple(
                    (f"sample-ohlcv.{source_kind.value}", FileBrowserEntryKind.FILE)
                    for source_kind in suffixes
                ),
            ),
            "C:\\Demo Data\\Archive": tuple(
                (f"archived-bars.{source_kind.value}", FileBrowserEntryKind.FILE)
                for source_kind in suffixes
            ),
            "C:\\Demo Data\\Daily": (
                ("2025", FileBrowserEntryKind.FOLDER),
                *tuple(
                    (f"latest.{source_kind.value}", FileBrowserEntryKind.FILE)
                    for source_kind in suffixes
                ),
            ),
            "C:\\Demo Data\\Daily\\2025": tuple(
                (f"daily-2025.{source_kind.value}", FileBrowserEntryKind.FILE)
                for source_kind in suffixes
            ),
        }
        if location not in fixtures:
            raise ValueError("Cannot open deterministic folder")
        parent_location = None if location == "C:\\" else location.rsplit("\\", 1)[0]
        if parent_location is not None and parent_location.endswith(":"):
            parent_location += "\\"
        entries: list[FileBrowserEntry] = []
        if parent_location is not None:
            entries.append(
                FileBrowserEntry(
                    parent_location,
                    "..",
                    None,
                    FileBrowserEntryKind.PARENT,
                )
            )
        if request.scenario is not IngestionScenario.EMPTY:
            for name, kind in fixtures[location]:
                source_name = f"{location.rstrip('\\')}\\{name}"
                entry_source_kind = (
                    SourceKind.PARQUET if name.casefold().endswith(".parquet") else SourceKind.CSV
                )
                entries.append(
                    FileBrowserEntry(
                        source_name,
                        name,
                        entry_source_kind if kind is FileBrowserEntryKind.FILE else None,
                        kind,
                    )
                )
        entries.sort(key=lambda entry: entry.sort_key)
        offset = 0 if request.cursor is None else int(request.cursor)
        finish = min(len(entries), offset + request.page_size)
        next_cursor = str(finish) if finish < len(entries) else None
        return FileBrowserListing(
            request,
            location,
            tuple(entries[offset:finish]),
            parent_location if request.cursor is None else None,
            next_cursor,
        )

    def discover_symbols(
        self, request: SymbolDiscoveryRequest, cancellation: CancellationSignalProtocol
    ) -> SymbolDiscoveryResult:
        _cancel(cancellation, request.request_id)
        fixtures = (
            StockSymbol("AAPL", "Apple Inc."),
            StockSymbol("AMD", "Advanced Micro Devices, Inc."),
            StockSymbol("MSFT", "Microsoft Corporation"),
            StockSymbol("NVDA", "NVIDIA Corporation"),
            StockSymbol("SPY", "SPDR S&P 500 ETF Trust"),
        )
        query = request.query.upper()
        symbols = tuple(
            item
            for item in fixtures
            if not query or query in item.symbol or query in item.name.upper()
        )
        if request.scenario is IngestionScenario.EMPTY:
            symbols = ()
        return SymbolDiscoveryResult(request, symbols[: request.page_size], False)

    def submit_ingestion(
        self,
        plan: IngestionPlan,
        cancellation: CancellationSignalProtocol,
        *,
        provider: bool,
    ) -> IngestionResult:
        _cancel(cancellation, plan.request_id)
        if provider and plan.source_timezone != "UTC":
            raise ValueError("Massive source timezone must be UTC")
        status = _scenario_status(plan.scenario)
        diagnostics = (
            (ValidationDiagnostic("invalid_ohlcv", "OHLC validation failed", "source"),)
            if status is IngestionStatus.VALIDATION_FAILED
            else ()
        )
        catalog_entry: DatasetCatalogEntry | None = None
        if status in {IngestionStatus.COMPLETE, IngestionStatus.RECOVERED}:
            with self._lock:
                current = next(
                    (item for item in self._catalog if item.dataset_id == plan.dataset_id), None
                )
                if plan.mode is ImportMode.CREATE:
                    if current is not None or plan.expected_revision != 0:
                        raise ValueError("dataset creation identity is stale")
                    catalog_entry = DatasetCatalogEntry(
                        plan.dataset_id,
                        plan.dataset_id.replace("-", " ").title(),
                        1,
                        _fingerprint(f"{self._seed}|{plan.command_id}|create"),
                        "sha256:simulated-canonical-bars-v1",
                        plan.symbols,
                        plan.interval,
                        plan.requested_range,
                        1_260,
                        plan.adjustment,
                    )
                    self._catalog = (*self._catalog, catalog_entry)
                else:
                    if current is None or current.revision != plan.expected_revision:
                        raise ValueError("catalog revision changed")
                    catalog_entry = replace(
                        current,
                        revision=current.revision + 1,
                        row_count=current.row_count + 1_260,
                        content_fingerprint=_fingerprint(
                            f"{current.content_fingerprint}|{plan.command_id}"
                        ),
                    )
                    self._catalog = tuple(
                        catalog_entry if item.dataset_id == plan.dataset_id else item
                        for item in self._catalog
                    )
                self._publish_ingested_source(catalog_entry, plan, provider)
        progress = IngestionProgress(
            plan.request_id,
            plan.generation,
            status,
            0 if status not in {IngestionStatus.COMPLETE, IngestionStatus.RECOVERED} else 100,
            100,
            _status_message(status),
            f"massive-{_fingerprint(plan.command_id)[7:19]}" if provider else None,
            30 if status is IngestionStatus.RATE_LIMITED else None,
        )
        return IngestionResult(plan, progress, catalog_entry, diagnostics)

    def mutate_schedule(
        self, command: ScheduleCommand, cancellation: CancellationSignalProtocol
    ) -> ScheduleResult:
        _cancel(cancellation, command.request_id)
        with self._lock:
            previous = self._schedule_commands.get(command.command_id)
            if previous is not None:
                if previous.command != command:
                    raise ValueError("schedule command ID was reused")
                return previous
            current = next(
                (
                    item
                    for item in self._schedules
                    if item.schedule_id == command.schedule.schedule_id
                ),
                None,
            )
            schedule: DataSchedule | None = command.schedule
            progress: IngestionProgress | None = None
            if command.kind is ScheduleCommandKind.CREATE:
                if current is not None or command.expected_revision != 0:
                    raise ValueError("schedule already exists")
                self._schedules = (*self._schedules, schedule)
            elif current is None or current.revision != command.expected_revision:
                raise ValueError("schedule revision changed")
            elif command.kind is ScheduleCommandKind.DELETE:
                self._schedules = tuple(
                    item for item in self._schedules if item.schedule_id != current.schedule_id
                )
                schedule = None
            elif command.kind is ScheduleCommandKind.RUN_NOW:
                schedule = replace(current, revision=current.revision + 1, last_run_at=self._clock)
                progress = IngestionProgress(
                    command.request_id,
                    command.generation,
                    IngestionStatus.COMPLETE,
                    100,
                    100,
                    "Manual schedule run completed",
                    f"massive-{_fingerprint(command.command_id)[7:19]}",
                )
                self._schedules = tuple(
                    schedule if item.schedule_id == schedule.schedule_id else item
                    for item in self._schedules
                )
            else:
                schedule = replace(command.schedule, revision=current.revision + 1)
                self._schedules = tuple(
                    schedule if item.schedule_id == schedule.schedule_id else item
                    for item in self._schedules
                )
            result = ScheduleResult(command, schedule, self._schedules, progress)
            self._schedule_commands[command.command_id] = result
            return result

    def reconcile(
        self, request: ReconciliationRequest, cancellation: CancellationSignalProtocol
    ) -> ReconciliationResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            return ReconciliationResult(request, self._catalog, self._schedules, ("recovered-1",))

    def save_dataset(
        self, request: DatasetSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetSaveResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            previous = self._dataset_save_commands.get(request.command_id)
            if previous is not None:
                if previous.request != request:
                    raise ValueError("dataset save command ID was reused")
                return previous
            current = next(
                (
                    item
                    for item in self._dataset_definitions
                    if item.dataset_id == request.definition.dataset_id
                ),
                None,
            )
            current_revision = 0 if current is None else current.revision
            if current_revision != request.expected_revision:
                raise ValueError("dataset definition revision changed")
            next_version = 1 if current is None else current.latest_version + 1
            if request.version.version != next_version:
                raise ValueError("dataset version must advance exactly once")
            definition = replace(
                request.definition,
                latest_version=next_version,
                revision=current_revision + 1,
            )
            version = request.version
            self._dataset_definitions = (
                *(
                    item
                    for item in self._dataset_definitions
                    if item.dataset_id != definition.dataset_id
                ),
                definition,
            )
            self._dataset_versions = (*self._dataset_versions, version)
            result = DatasetSaveResult(request, definition, version)
            self._dataset_save_commands[request.command_id] = result
            return result

    def build_dataset(
        self, request: DatasetBuildRequest, cancellation: CancellationSignalProtocol
    ) -> DatasetBuild:
        _cancel(cancellation, request.request_id)
        with self._lock:
            previous = self._dataset_build_commands.get(request.command_id)
            if previous is not None:
                if (
                    previous.version != request.version
                    or previous.source_snapshot != request.source_snapshot
                ):
                    raise ValueError("dataset build command ID was reused")
                return previous
            source = next(
                (item for item in self._sources if item.source_id == request.version.source_id),
                None,
            )
            if source is None:
                raise ValueError("unknown dataset source")
            latest = next(
                (
                    item
                    for item in reversed(self._source_snapshots)
                    if item.source_id == request.version.source_id
                ),
                None,
            )
            if latest is None or latest != request.source_snapshot:
                raise ValueError("dataset build source snapshot is stale")
            build = build_dataset_preview(request, source, self._clock)
            self._dataset_builds = (*self._dataset_builds, build)[-64:]
            self._dataset_build_commands[request.command_id] = build
            return build

    def evaluate(
        self,
        request_id: str,
        generation: int,
        draft: ExperimentDraft,
        cancellation: CancellationSignalProtocol,
    ) -> DraftEvaluation:
        _cancel(cancellation, request_id)
        return evaluate_experiment(request_id, generation, draft)

    def save(
        self, request: DraftSaveRequest, cancellation: CancellationSignalProtocol
    ) -> DraftSaveResult:
        _cancel(cancellation, request.request_id)
        with self._lock:
            if request.expected_saved_revision != self._saved_revision:
                raise ValueError("stale draft save revision")
            if request.draft.revision < self._draft.revision:
                raise ValueError("late draft save cannot overwrite a newer edit")
            self._draft = request.draft
            self._saved_revision = request.draft.revision
            return DraftSaveResult(request, self._saved_revision)

    def submit(
        self, request: SubmissionRequest, cancellation: CancellationSignalProtocol
    ) -> ExperimentDefinition:
        _cancel(cancellation, request.request_id)
        evaluation = evaluate_experiment(request.request_id, request.generation, request.draft)
        if not evaluation.valid:
            raise ValueError("invalid experiment draft cannot be submitted")
        with self._lock:
            existing = self._submissions.get(request.command_id)
            if existing is not None:
                if existing.draft != request.draft:
                    raise ValueError("accepted command cannot be reused with a different draft")
                return existing
            current = next(
                (item for item in self._catalog if item.dataset_id == request.draft.dataset_id),
                None,
            )
            if request.draft.dataset_binding is None:
                if current is None or (
                    current.revision != request.draft.dataset_revision
                    or current.content_fingerprint != request.draft.dataset_fingerprint
                ):
                    raise ValueError("draft catalog revision or fingerprint is stale")
            elif not any(
                build.binding == request.draft.dataset_binding
                for build in self._dataset_builds
                if build.binding is not None
            ):
                raise ValueError("draft dataset binding is not a published build")
            definition = ExperimentDefinition(
                f"experiment-{len(self._submissions) + 1:06d}",
                request.command_id,
                self._clock + timedelta(milliseconds=request.generation),
                request.draft,
            )
            self._submissions[request.command_id] = definition
            return definition

    def _publish_legacy_source_refresh(
        self, catalog: DatasetCatalogEntry, command_id: str, correlation_id: str
    ) -> None:
        source_id = f"source-{catalog.dataset_id.removeprefix('dataset-')}"
        previous = next(
            item for item in reversed(self._source_snapshots) if item.source_id == source_id
        )
        snapshot = SourceSnapshot(
            f"snapshot-{source_id}-{catalog.revision}",
            source_id,
            previous.source_revision,
            previous.snapshot_revision + 1,
            catalog.content_fingerprint,
            previous.selection,
            catalog.row_count,
            SourceProvenance(
                self._clock, command_id, correlation_id, parent_snapshot_id=previous.snapshot_id
            ),
        )
        self._source_snapshots = (*self._source_snapshots, snapshot)
        self._dataset_builds = tuple(
            mark_build_stale(build, snapshot.snapshot_id)
            if build.version.source_id == source_id
            else build
            for build in self._dataset_builds
        )

    def _publish_ingested_source(
        self, catalog: DatasetCatalogEntry, plan: IngestionPlan, provider: bool
    ) -> None:
        source_id = f"source-{catalog.dataset_id.removeprefix('dataset-')}"
        existing = next((item for item in self._sources if item.source_id == source_id), None)
        if existing is not None:
            self._publish_legacy_source_refresh(catalog, plan.command_id, plan.correlation_id)
            return
        template = self._sources[0]
        source = SourceDefinition(
            source_id,
            1,
            f"{catalog.name} source",
            SourceFamily.MARKET_BARS,
            SourceProvider.MASSIVE
            if provider
            else SourceProvider.PARQUET
            if plan.source_path is not None and plan.source_path.casefold().endswith(".parquet")
            else SourceProvider.CSV,
            template.schema,
            _fingerprint(f"{plan.mapping!r}|{plan.source_timezone}"),
        )
        snapshot = SourceSnapshot(
            f"snapshot-{source_id}-1",
            source_id,
            1,
            1,
            catalog.content_fingerprint,
            SourceSelection(
                catalog.symbols,
                catalog.interval,
                catalog.coverage.start,
                catalog.coverage.end,
                plan.session.value,
                plan.adjustment.value,
            ),
            catalog.row_count,
            SourceProvenance(
                self._clock,
                plan.command_id,
                plan.correlation_id,
                f"massive-{_fingerprint(plan.command_id)[7:19]}" if provider else None,
            ),
        )
        self._sources = (*self._sources, source)
        self._source_snapshots = (*self._source_snapshots, snapshot)


def default_experiment_draft(
    dataset: DatasetCatalogEntry, build: DatasetBuild | None = None
) -> ExperimentDraft:
    binding = None if build is None else build.binding
    return ExperimentDraft(
        "draft-daily-equity-baseline",
        1,
        dataset.dataset_id,
        dataset.revision,
        dataset.content_fingerprint if binding is None else binding.build_fingerprint,
        FEATURES[:2],
        5,
        1000,
        250,
        250,
        5,
        "hist_gradient_boosting",
        6,
        300,
        1_000_000.0,
        1.5,
        (),
        4,
        binding,
    )


def evaluate_experiment(
    request_id: str, generation: int, draft: ExperimentDraft
) -> DraftEvaluation:
    diagnostics: list[ValidationDiagnostic] = []
    if not draft.features or len(
        {(item.name, item.semantic_version) for item in draft.features}
    ) != len(draft.features):
        diagnostics.append(
            ValidationDiagnostic("features_unique", "Select unique compiled features", "features")
        )
    if not 1 <= draft.target_horizon <= 256:
        diagnostics.append(
            ValidationDiagnostic(
                "target_horizon", "Target horizon must be 1-256 bars", "target_horizon"
            )
        )
    if min(draft.train_bars, draft.validation_bars, draft.test_bars) < 1:
        diagnostics.append(
            ValidationDiagnostic("split_size", "Every split must contain rows", "split")
        )
    if draft.purge_bars < draft.target_horizon:
        diagnostics.append(
            ValidationDiagnostic(
                "purge_horizon", "Purge must cover the target horizon", "purge_bars"
            )
        )
    if not 1 <= draft.max_depth <= 32 or not 1 <= draft.estimator_count <= 10_000:
        diagnostics.append(
            ValidationDiagnostic("model_bounds", "Model settings exceed bounds", "model")
        )
    if draft.initial_capital <= 0 or draft.fee_bps < 0:
        diagnostics.append(
            ValidationDiagnostic("portfolio_bounds", "Portfolio values are invalid", "portfolio")
        )
    if not 1 <= draft.cpu_limit <= 64:
        diagnostics.append(ValidationDiagnostic("cpu_limit", "CPU limit must be 1-64", "cpu_limit"))
    if len(draft.sweep_values) > 128 or any(value < 1 for value in draft.sweep_values):
        diagnostics.append(
            ValidationDiagnostic("sweep_bounds", "Sweep values exceed bounds", "sweep")
        )
    rows = max(0, draft.train_bars + draft.validation_bars + draft.test_bars - draft.purge_bars * 2)
    columns = max(1, len(draft.features))
    feature_bytes = rows * columns * 4
    candidates = max(1, len(draft.sweep_values))
    estimate = ResourceEstimate(
        rows,
        feature_bytes,
        feature_bytes * 3 + rows * 8,
        round(
            rows
            * columns
            * draft.estimator_count
            * candidates
            / max(1, draft.cpu_limit)
            / 2_000_000,
            6,
        ),
    )
    return DraftEvaluation(request_id, generation, draft, tuple(diagnostics), estimate)


def _initial_catalog(seed: int) -> tuple[DatasetCatalogEntry, ...]:
    coverage = UtcRange(datetime(2020, 7, 9, tzinfo=UTC), datetime(2026, 7, 9, tzinfo=UTC))
    return (
        DatasetCatalogEntry(
            "dataset-us-equities",
            "US equities daily",
            18,
            _fingerprint(f"{seed}|us-equities|18"),
            "sha256:canonical-bars-v1",
            ("AAPL", "AMD", "MSFT", "NVDA"),
            "1d",
            coverage,
            955_353,
            AdjustmentPolicy.SPLIT_AND_DIVIDEND,
        ),
        DatasetCatalogEntry(
            "dataset-index-watchlist",
            "Index watchlist hourly",
            7,
            _fingerprint(f"{seed}|index-watchlist|7"),
            "sha256:canonical-bars-v1",
            ("DIA", "IWM", "QQQ", "SPY"),
            "1h",
            coverage,
            214_840,
            AdjustmentPolicy.RAW,
            "validation",
        ),
    )


def _initial_schedules(clock: datetime) -> tuple[DataSchedule, ...]:
    return (
        DataSchedule(
            "schedule-us-equities-daily",
            3,
            "US equities daily refresh",
            "dataset-us-equities",
            ("AAPL", "AMD", "MSFT", "NVDA"),
            "1d",
            SessionPolicy.REGULAR,
            AdjustmentPolicy.PROVIDER_SPLIT_ADJUSTED,
            ScheduleCadence.DAILY,
            True,
            clock - timedelta(days=1),
            clock + timedelta(days=1),
        ),
    )


def _initial_dataset_workflow(
    catalog: tuple[DatasetCatalogEntry, ...], clock: datetime
) -> tuple[
    tuple[SourceDefinition, ...],
    tuple[SourceSnapshot, ...],
    tuple[DatasetDefinition, ...],
    tuple[DatasetVersion, ...],
    tuple[DatasetBuild, ...],
]:
    schema = (
        SourceColumn("timestamp", ColumnType.TIMESTAMP),
        SourceColumn("symbol", ColumnType.SYMBOL),
        SourceColumn("open", ColumnType.FLOAT64),
        SourceColumn("high", ColumnType.FLOAT64),
        SourceColumn("low", ColumnType.FLOAT64),
        SourceColumn("close", ColumnType.FLOAT64),
        SourceColumn("volume", ColumnType.FLOAT64),
    )
    sources: list[SourceDefinition] = []
    snapshots: list[SourceSnapshot] = []
    definitions: list[DatasetDefinition] = []
    versions: list[DatasetVersion] = []
    builds: list[DatasetBuild] = []
    for entry in catalog:
        suffix = entry.dataset_id.removeprefix("dataset-")
        source = SourceDefinition(
            f"source-{suffix}",
            1,
            f"{entry.name} source",
            SourceFamily.MARKET_BARS,
            SourceProvider.EXISTING,
            schema,
            "sha256:legacy-canonical-ohlcv-mapping-v1",
        )
        selection = SourceSelection(
            entry.symbols,
            entry.interval,
            entry.coverage.start,
            entry.coverage.end,
            "regular",
            entry.adjustment.value,
        )
        snapshot = SourceSnapshot(
            f"snapshot-{source.source_id}-{entry.revision}",
            source.source_id,
            source.revision,
            entry.revision,
            entry.content_fingerprint,
            selection,
            entry.row_count,
            SourceProvenance(clock, "legacy-catalog-migration", f"migration-{suffix}"),
        )
        steps = (
            LaggedReturnStep("ret_5", periods=5),
            RollingVolatilityStep("volatility_20", window=20),
            RollingStatisticStep("volume_z_30", "volume", 30, RollingStatistic.STANDARD_DEVIATION),
        )
        recipe_fingerprint = _fingerprint(
            f"{entry.dataset_id}|1|{source.source_id}|{selection!r}|{steps!r}"
        )
        version = DatasetVersion(
            entry.dataset_id,
            entry.revision,
            source.source_id,
            source.revision,
            selection,
            steps,
            REGISTRY_VERSION,
            ENGINE_FINGERPRINT,
            recipe_fingerprint,
        )
        definition = DatasetDefinition(entry.dataset_id, entry.name, version.version, 1)
        build = build_dataset_preview(
            DatasetBuildRequest(
                f"legacy-build-{suffix}",
                f"legacy-build-{suffix}",
                1,
                version,
                snapshot,
            ),
            source,
            clock,
        )
        sources.append(source)
        snapshots.append(snapshot)
        definitions.append(definition)
        versions.append(version)
        builds.append(build)
    return tuple(sources), tuple(snapshots), tuple(definitions), tuple(versions), tuple(builds)


def _scenario_status(scenario: IngestionScenario) -> IngestionStatus:
    return {
        IngestionScenario.SUCCESS: IngestionStatus.COMPLETE,
        IngestionScenario.VALIDATION_FAILURE: IngestionStatus.VALIDATION_FAILED,
        IngestionScenario.AUTHENTICATION_FAILURE: IngestionStatus.AUTHENTICATION_FAILED,
        IngestionScenario.ENTITLEMENT_FAILURE: IngestionStatus.ENTITLEMENT_FAILED,
        IngestionScenario.RATE_LIMIT: IngestionStatus.RATE_LIMITED,
        IngestionScenario.FAILURE: IngestionStatus.FAILED,
        IngestionScenario.EMPTY: IngestionStatus.EMPTY,
        IngestionScenario.RECOVERY: IngestionStatus.RECOVERED,
    }[scenario]


def _status_message(status: IngestionStatus) -> str:
    return {
        IngestionStatus.COMPLETE: "Simulated ingestion completed",
        IngestionStatus.VALIDATION_FAILED: "Validation rejected the simulated import",
        IngestionStatus.AUTHENTICATION_FAILED: "Massive authentication failed",
        IngestionStatus.ENTITLEMENT_FAILED: "Massive entitlement does not permit this request",
        IngestionStatus.RATE_LIMITED: "Massive rate limit reached",
        IngestionStatus.FAILED: "Simulated ingestion failed",
        IngestionStatus.EMPTY: "The requested range contains no bars",
        IngestionStatus.RECOVERED: "Reconnected and reconciled simulated ingestion",
    }[status]


def _rejected_import(
    request: ImportRequest, current: DatasetCatalogEntry, code: str, message: str
) -> ImportResult:
    return ImportResult(
        request,
        ImportState.REJECTED,
        current,
        (ValidationDiagnostic(code, message, "source"),),
        0,
    )


def _fingerprint(value: str) -> str:
    return f"sha256:{hashlib.sha256(value.encode()).hexdigest()}"


def _cancel(cancellation: CancellationSignalProtocol, request_id: str) -> None:
    if cancellation.is_set():
        raise RequestCancelledError(request_id)
