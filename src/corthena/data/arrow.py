"""PyArrow file parsing, canonical validation, and immutable revision writing."""

# pyright: reportMissingTypeStubs=false, reportUnknownMemberType=false
# pyright: reportUnknownVariableType=false, reportUnknownArgumentType=false
# pyright: reportUnknownParameterType=false

from __future__ import annotations

import hashlib
import math
import os
import secrets
import shutil
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Literal
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

import pyarrow as pa
import pyarrow.csv as pacsv
import pyarrow.parquet as pq

from corthena.contracts.data import (
    ColumnMappingDTO,
    FilePreviewDTO,
    ImportPlanDTO,
    Interval,
    PreviewRequestDTO,
    PreviewRowDTO,
    ProvenanceDTO,
    SourceKind,
)
from corthena.data.errors import CancelledDataError, PublicationDataError, ValidationDataError
from corthena.data.protocol import MarketCalendarProtocol
from corthena.data.types import CanonicalBar
from corthena.ui.client.protocol import CancellationSignalProtocol

_ROLES = ("timestamp", "symbol", "open", "high", "low", "close", "volume")
type ColumnRole = Literal["timestamp", "symbol", "open", "high", "low", "close", "volume", "ignore"]
_DURATION_SECONDS = {
    Interval.MINUTE_1: 60,
    Interval.MINUTE_5: 300,
    Interval.MINUTE_15: 900,
    Interval.HOUR_1: 3600,
    Interval.DAY_1: 86400,
}


class ArrowIngestionAdapter:
    """Contain PyArrow values and publish only immutable typed values."""

    def preview(self, request: PreviewRequestDTO) -> FilePreviewDTO:
        path = self._validated_path(request.path, request.source_kind)
        sample, source_truncated = self._read_preview(
            path, request.source_kind, request.max_rows, request.max_bytes
        )
        mappings = tuple(
            ColumnMappingDTO(
                source_column=name,
                role=self._detect_role(name),
                source_type=str(sample.schema.field(name).type),
            )
            for name in sample.column_names[:64]
        )
        rows = tuple(
            PreviewRowDTO(
                values=tuple(self._safe_preview_value(column[row]) for column in sample.columns)
            )
            for row in range(sample.num_rows)
        )
        encoded_bytes = sum(len(str(value).encode("utf-8")) for row in rows for value in row.values)
        if encoded_bytes > request.max_bytes:
            bounded: list[PreviewRowDTO] = []
            used = 0
            for row in rows:
                size = sum(len(str(value).encode("utf-8")) for value in row.values)
                if used + size > request.max_bytes:
                    break
                bounded.append(row)
                used += size
            rows = tuple(bounded)
        return FilePreviewDTO(
            path=str(path),
            source_kind=request.source_kind,
            columns=mappings,
            rows=rows,
            total_rows_seen=sample.num_rows,
            truncated=source_truncated or sample.num_rows > len(rows),
        )

    def read_bars(
        self,
        plan: ImportPlanDTO,
        calendar: MarketCalendarProtocol,
        cancellation: CancellationSignalProtocol,
    ) -> tuple[CanonicalBar, ...]:
        if plan.source_path is None or plan.source_kind is SourceKind.MASSIVE:
            raise ValidationDataError("file import source is missing", field="source_path")
        path = self._validated_path(plan.source_path, plan.source_kind)
        table = self._read_table(path, plan.source_kind)
        mapping = {item.role: item.source_column for item in plan.mapping if item.role != "ignore"}
        if set(mapping) != set(_ROLES):
            raise ValidationDataError(
                "mapping must define each canonical role once", field="mapping"
            )
        if len(mapping) != len(tuple(item for item in plan.mapping if item.role != "ignore")):
            raise ValidationDataError("mapping roles must be unique", field="mapping")
        missing = tuple(name for name in mapping.values() if name not in table.column_names)
        if missing:
            raise ValidationDataError("mapped source column is missing", field=missing[0])
        try:
            timezone = ZoneInfo(plan.source_timezone)
        except ZoneInfoNotFoundError as error:
            raise ValidationDataError(
                "source timezone is unknown", field="source_timezone"
            ) from error
        columns = {role: table[mapping[role]].combine_chunks() for role in _ROLES}
        bars: list[CanonicalBar] = []
        for index in range(table.num_rows):
            self._cancel(cancellation)
            timestamp = self._timestamp(columns["timestamp"][index].as_py(), timezone)
            symbol = str(columns["symbol"][index].as_py()).strip().upper()
            if symbol not in plan.symbols or not (
                plan.requested_range.start <= timestamp < plan.requested_range.end
            ):
                continue
            try:
                bar = CanonicalBar(
                    symbol,
                    timestamp,
                    float(columns["open"][index].as_py()),
                    float(columns["high"][index].as_py()),
                    float(columns["low"][index].as_py()),
                    float(columns["close"][index].as_py()),
                    float(columns["volume"][index].as_py()),
                )
            except (TypeError, ValueError) as error:
                raise ValidationDataError("OHLCV value is malformed", field="ohlcv") from error
            bars.append(bar)
        self.validate(tuple(bars), plan.interval, plan.session_policy.value == "regular", calendar)
        return tuple(bars)

    @staticmethod
    def validate(
        bars: tuple[CanonicalBar, ...],
        interval: Interval,
        regular_session: bool,
        calendar: MarketCalendarProtocol,
    ) -> None:
        if not bars:
            raise ValidationDataError("source contains no canonical bars")
        seen: set[tuple[str, datetime]] = set()
        previous: dict[str, datetime] = {}
        duration = _DURATION_SECONDS[interval]
        for bar in bars:
            key = (bar.symbol, bar.timestamp)
            if key in seen:
                raise ValidationDataError("duplicate (symbol, timestamp) key")
            seen.add(key)
            values = (bar.open, bar.high, bar.low, bar.close, bar.volume)
            if not all(math.isfinite(value) for value in values) or bar.volume < 0:
                raise ValidationDataError("prices must be finite and volume nonnegative")
            if (
                bar.low > min(bar.open, bar.close)
                or bar.high < max(bar.open, bar.close)
                or bar.low > bar.high
            ):
                raise ValidationDataError("OHLC relationship is invalid")
            prior = previous.get(bar.symbol)
            if prior is not None:
                delta = int((bar.timestamp - prior).total_seconds())
                if delta <= 0 or (interval is not Interval.DAY_1 and delta % duration != 0):
                    raise ValidationDataError("bars are unordered or interval-inconsistent")
            previous[bar.symbol] = bar.timestamp
            if regular_session and not calendar.is_regular_bar(bar, interval):
                raise ValidationDataError("bar falls outside the declared regular session")

    @staticmethod
    def read_revision(path: Path) -> tuple[CanonicalBar, ...]:
        bars: list[CanonicalBar] = []
        for partition in sorted(path.glob("symbol=*/bars.parquet")):
            table = pq.ParquetFile(partition).read()
            for index in range(table.num_rows):
                bars.append(
                    CanonicalBar(
                        str(table["symbol"][index].as_py()),
                        table["timestamp"][index].as_py().astimezone(UTC),
                        float(table["open"][index].as_py()),
                        float(table["high"][index].as_py()),
                        float(table["low"][index].as_py()),
                        float(table["close"][index].as_py()),
                        float(table["volume"][index].as_py()),
                    )
                )
        return tuple(sorted(bars))

    @staticmethod
    def write_revision(
        revisions_root: Path,
        dataset_id: str,
        revision: int,
        bars: tuple[CanonicalBar, ...],
        provenance_factory: Callable[[tuple[tuple[str, str], ...], str], ProvenanceDTO],
    ) -> tuple[Path, ProvenanceDTO]:
        dataset_root = revisions_root / dataset_id
        dataset_root.mkdir(parents=True, exist_ok=True)
        final = dataset_root / f"revision-{revision:08d}"
        temporary = dataset_root / f".revision-{revision:08d}-{secrets.token_hex(6)}.tmp"
        if final.exists():
            raise PublicationDataError("revision directory already exists")
        checksums: list[tuple[str, str]] = []
        try:
            temporary.mkdir()
            symbols = sorted({bar.symbol for bar in bars})
            for symbol in symbols:
                selected = tuple(bar for bar in bars if bar.symbol == symbol)
                partition = temporary / f"symbol={symbol}"
                partition.mkdir()
                path = partition / "bars.parquet"
                table = pa.table(
                    {
                        "timestamp": pa.array(
                            (bar.timestamp for bar in selected), type=pa.timestamp("ns", tz="UTC")
                        ),
                        "symbol": pa.array((bar.symbol for bar in selected), type=pa.string()),
                        "open": pa.array((bar.open for bar in selected), type=pa.float64()),
                        "high": pa.array((bar.high for bar in selected), type=pa.float64()),
                        "low": pa.array((bar.low for bar in selected), type=pa.float64()),
                        "close": pa.array((bar.close for bar in selected), type=pa.float64()),
                        "volume": pa.array((bar.volume for bar in selected), type=pa.float64()),
                    }
                )
                pq.write_table(table, path, compression="zstd")
                checksums.append(
                    (str(path.relative_to(temporary)).replace("\\", "/"), _sha256(path))
                )
            fingerprint = hashlib.sha256(
                "".join(f"{name}:{checksum}\n" for name, checksum in checksums).encode("ascii")
            ).hexdigest()
            provenance = provenance_factory(tuple(checksums), f"sha256:{fingerprint}")
            manifest = temporary / "provenance.v1.json"
            with manifest.open("x", encoding="utf-8", newline="\n") as stream:
                stream.write(provenance.model_dump_json(indent=2))
                stream.write("\n")
                stream.flush()
                os.fsync(stream.fileno())
            for _, checksum in checksums:
                if len(checksum) != 71:
                    raise PublicationDataError("partition checksum is invalid")
            for name, checksum in checksums:
                if _sha256(temporary / Path(name)) != checksum:
                    raise PublicationDataError("partition checksum verification failed")
            os.replace(temporary, final)
            return final, provenance
        except Exception as error:
            if temporary.exists() and temporary.parent.resolve() == dataset_root.resolve():
                shutil.rmtree(temporary)
            if isinstance(error, PublicationDataError):
                raise
            raise PublicationDataError("catalog revision could not be published") from error

    @staticmethod
    def _read_table(path: Path, source_kind: SourceKind) -> pa.Table:
        try:
            if source_kind is SourceKind.CSV:
                return pacsv.read_csv(path)
            if source_kind is SourceKind.PARQUET:
                return pq.read_table(path)
        except (OSError, pa.ArrowException) as error:
            raise ValidationDataError("source file could not be parsed") from error
        raise ValidationDataError("unsupported file source kind")

    @staticmethod
    def _read_preview(
        path: Path,
        source_kind: SourceKind,
        max_rows: int,
        max_bytes: int,
    ) -> tuple[pa.Table, bool]:
        try:
            if source_kind is SourceKind.CSV:
                size = path.stat().st_size
                with path.open("rb") as stream:
                    payload = stream.read(max_bytes)
                truncated = size > len(payload)
                if truncated:
                    boundary = payload.rfind(b"\n")
                    if boundary <= 0:
                        raise ValidationDataError("CSV preview bound does not contain a row")
                    payload = payload[: boundary + 1]
                table = pacsv.read_csv(pa.py_buffer(payload)).slice(0, max_rows)
                return table, truncated or table.num_rows >= max_rows
            if source_kind is SourceKind.PARQUET:
                parquet = pq.ParquetFile(path)
                batches = parquet.iter_batches(batch_size=max_rows)
                first = next(batches, None)
                if first is None:
                    return pa.table({}), False
                table = pa.Table.from_batches((first,)).slice(0, max_rows)
                return table, parquet.metadata.num_rows > table.num_rows
        except ValidationDataError:
            raise
        except (OSError, pa.ArrowException) as error:
            raise ValidationDataError("source preview could not be parsed") from error
        raise ValidationDataError("unsupported file source kind")

    @staticmethod
    def _validated_path(value: str, source_kind: SourceKind) -> Path:
        path = Path(value).expanduser().resolve(strict=True)
        if not path.is_file() or path.suffix.casefold() != f".{source_kind.value}":
            raise ValidationDataError("source file kind does not match its path", field="path")
        return path

    @staticmethod
    def _detect_role(name: str) -> ColumnRole:
        normalized = name.casefold().replace("_", "").replace(" ", "")
        aliases: dict[str, ColumnRole] = {
            "date": "timestamp",
            "datetime": "timestamp",
            "time": "timestamp",
            "ticker": "symbol",
            "sym": "symbol",
            "vol": "volume",
        }
        if normalized in _ROLES:
            return normalized
        return aliases.get(normalized, "ignore")

    @staticmethod
    def _safe_preview_value(scalar: pa.Scalar) -> str | int | float | bool | None:
        value = scalar.as_py()
        if value is None or isinstance(value, str | int | float | bool):
            return value
        if isinstance(value, datetime):
            return value.isoformat()
        return str(value)

    @staticmethod
    def _timestamp(value: object, source_timezone: ZoneInfo) -> datetime:
        if isinstance(value, datetime):
            parsed = value
        elif isinstance(value, str):
            try:
                parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
            except ValueError as error:
                raise ValidationDataError(
                    "timestamp value is malformed", field="timestamp"
                ) from error
        elif isinstance(value, int | float) and not isinstance(value, bool):
            parsed = datetime.fromtimestamp(float(value), UTC)
        else:
            raise ValidationDataError("timestamp value is malformed", field="timestamp")
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=source_timezone)
        return parsed.astimezone(UTC)

    @staticmethod
    def _cancel(cancellation: CancellationSignalProtocol) -> None:
        if cancellation.is_set():
            raise CancelledDataError("file ingestion was cancelled")


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1_048_576), b""):
            digest.update(block)
    return f"sha256:{digest.hexdigest()}"


def source_checksum(path: Path) -> str:
    return _sha256(path)
