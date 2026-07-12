"""Windows storage and immutable-buffer compatibility probes."""

# pyright: reportMissingTypeStubs=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false, reportUnknownVariableType=false

from __future__ import annotations

import csv
import sqlite3
import tempfile
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pyarrow as pa
import pyarrow.csv as pacsv
import pyarrow.ipc as ipc
import pyarrow.parquet as pq


@dataclass(frozen=True, slots=True)
class StorageEvidence:
    """Observed storage round-trip properties."""

    rows: int
    arrow_bytes: int
    matrix_sum: float


def run_storage_probe() -> StorageEvidence:
    """Prove Arrow, Parquet, SQLite WAL, and read-only mapped-array behavior."""
    with tempfile.TemporaryDirectory(prefix="corthena-phase0-storage-") as directory:
        root = Path(directory)
        csv_path = root / "bars.csv"
        with csv_path.open("w", newline="", encoding="utf-8") as stream:
            writer = csv.writer(stream)
            writer.writerow(("timestamp", "close"))
            writer.writerow(("2026-01-01T00:00:00Z", "100.25"))
        table = pacsv.read_csv(
            csv_path,
            convert_options=pacsv.ConvertOptions(
                column_types={"timestamp": pa.timestamp("ms", tz="UTC"), "close": pa.float64()}
            ),
        )
        parquet_path = root / "bars.parquet"
        pq.write_table(table, parquet_path)
        if not table.equals(pq.read_table(parquet_path)):
            raise RuntimeError("Parquet round trip changed the table")
        sink = pa.BufferOutputStream()
        with ipc.new_stream(sink, table.schema) as writer:
            writer.write_table(table)
        buffer = sink.getvalue()
        if not table.equals(ipc.open_stream(buffer).read_all()):
            raise RuntimeError("Arrow IPC round trip changed the table")

        database = root / "phase0.sqlite3"
        writer_connection = sqlite3.connect(database)
        reader_connection: sqlite3.Connection | None = None
        try:
            mode = writer_connection.execute("PRAGMA journal_mode=WAL").fetchone()
            if mode is None or str(mode[0]).lower() != "wal":
                raise RuntimeError("SQLite WAL was unavailable")
            writer_connection.execute("CREATE TABLE evidence (name TEXT PRIMARY KEY)")
            writer_connection.execute("INSERT INTO evidence VALUES ('phase0')")
            writer_connection.commit()
            reader_connection = sqlite3.connect(f"file:{database}?mode=ro", uri=True)
            if reader_connection.execute("SELECT name FROM evidence").fetchone() != ("phase0",):
                raise RuntimeError("SQLite reader observed unexpected data")
        finally:
            if reader_connection is not None:
                reader_connection.close()
            writer_connection.close()

        matrix_path = root / "matrix.f64"
        mutable = np.memmap(matrix_path, dtype="<f8", mode="w+", shape=(2, 2))
        mutable[:] = ((1.0, 2.0), (3.0, 4.0))
        mutable.flush()
        del mutable
        published = np.memmap(matrix_path, dtype="<f8", mode="r", shape=(2, 2))
        matrix_sum = float(published.sum())
        if published.flags.writeable or matrix_sum != 10.0:
            raise RuntimeError("published mapped matrix was not read-only and stable")
        del published
        return StorageEvidence(table.num_rows, buffer.size, matrix_sum)
