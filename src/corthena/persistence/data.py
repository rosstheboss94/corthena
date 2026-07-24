"""SQLite WAL repositories for ingestion metadata and schedules."""

from __future__ import annotations

import sqlite3
import threading
from importlib import resources
from pathlib import Path

from corthena.contracts.data import (
    CatalogRevisionDTO,
    OperationState,
    ProgressDTO,
    ScheduleDTO,
)
from corthena.data.errors import StaleRevisionError

_MIGRATION = (
    resources.files("corthena.persistence.migrations")
    .joinpath("0001_data.sql")
    .read_text(encoding="utf-8")
)


class DataRepository:
    """Single-process SQLite writer protected independently of the GIL."""

    def __init__(self, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        self._connection = sqlite3.connect(path, check_same_thread=False)
        self._connection.row_factory = sqlite3.Row
        self._lock = threading.RLock()
        self._closed = False
        with self._lock:
            mode = self._connection.execute("PRAGMA journal_mode=WAL").fetchone()
            if mode is None or str(mode[0]).casefold() != "wal":
                raise RuntimeError("SQLite WAL could not be enabled")
            self._connection.execute("PRAGMA foreign_keys=ON")
            self._connection.execute("PRAGMA synchronous=FULL")
            self._connection.executescript(_MIGRATION)
            self._connection.execute("PRAGMA user_version=1")
            self._connection.commit()

    def close(self) -> None:
        with self._lock:
            if not self._closed:
                self._connection.close()
                self._closed = True

    def current_revision(self, dataset_id: str) -> CatalogRevisionDTO | None:
        with self._lock:
            row = self._connection.execute(
                "SELECT document FROM catalog_revisions WHERE dataset_id=? AND current=1",
                (dataset_id,),
            ).fetchone()
        return None if row is None else CatalogRevisionDTO.model_validate_json(str(row[0]))

    def catalog(self) -> tuple[CatalogRevisionDTO, ...]:
        with self._lock:
            rows = self._connection.execute(
                "SELECT document FROM catalog_revisions WHERE current=1 ORDER BY dataset_id"
            ).fetchall()
        return tuple(CatalogRevisionDTO.model_validate_json(str(row[0])) for row in rows)

    def publish(self, revision: CatalogRevisionDTO, expected_revision: int) -> None:
        with self._lock, self._connection:
            current = self._connection.execute(
                "SELECT revision FROM catalog_revisions WHERE dataset_id=? AND current=1",
                (revision.dataset_id,),
            ).fetchone()
            observed = 0 if current is None else int(current[0])
            if observed != expected_revision or revision.revision != expected_revision + 1:
                raise StaleRevisionError("catalog revision changed")
            self._connection.execute(
                "UPDATE catalog_revisions SET current=0 WHERE dataset_id=? AND current=1",
                (revision.dataset_id,),
            )
            self._connection.execute(
                """INSERT INTO catalog_revisions(
                       dataset_id,revision,document,current
                   ) VALUES(?,?,?,1)""",
                (revision.dataset_id, revision.revision, revision.model_dump_json()),
            )

    def put_progress(self, progress: ProgressDTO, command_id: str) -> None:
        with self._lock, self._connection:
            self._connection.execute(
                """INSERT INTO imports(
                       operation_id,command_id,correlation_id,generation,state,document
                   )
                   VALUES(?,?,?,?,?,?)
                   ON CONFLICT(operation_id) DO UPDATE SET
                     state=excluded.state, document=excluded.document""",
                (
                    progress.operation_id,
                    command_id,
                    progress.correlation_id,
                    progress.generation,
                    progress.state.value,
                    progress.model_dump_json(),
                ),
            )

    def progress(self, operation_id: str) -> ProgressDTO | None:
        with self._lock:
            row = self._connection.execute(
                "SELECT document FROM imports WHERE operation_id=?", (operation_id,)
            ).fetchone()
        return None if row is None else ProgressDTO.model_validate_json(str(row[0]))

    def progress_by_command(self, command_id: str) -> ProgressDTO | None:
        with self._lock:
            row = self._connection.execute(
                "SELECT document FROM imports WHERE command_id=?", (command_id,)
            ).fetchone()
        return None if row is None else ProgressDTO.model_validate_json(str(row[0]))

    def claim_command(
        self,
        command_id: str,
        resource_kind: str,
        resource_id: str,
        request_fingerprint: str,
    ) -> bool:
        """Persist idempotency identity; reject reuse with different content."""
        with self._lock, self._connection:
            row = self._connection.execute(
                """SELECT resource_kind,resource_id,request_fingerprint
                   FROM accepted_commands WHERE command_id=?""",
                (command_id,),
            ).fetchone()
            if row is not None:
                if tuple(str(item) for item in row) != (
                    resource_kind,
                    resource_id,
                    request_fingerprint,
                ):
                    raise StaleRevisionError("command ID was reused with different content")
                return False
            self._connection.execute(
                """INSERT INTO accepted_commands(
                       command_id,resource_kind,resource_id,request_fingerprint
                   ) VALUES(?,?,?,?)""",
                (command_id, resource_kind, resource_id, request_fingerprint),
            )
            return True

    def recover_incomplete(self) -> tuple[str, ...]:
        recovered: list[str] = []
        with self._lock, self._connection:
            rows = self._connection.execute(
                """SELECT operation_id,document FROM imports
                   WHERE state IN ('queued','running','cancelling')
                   ORDER BY operation_id"""
            ).fetchall()
            for row in rows:
                progress = ProgressDTO.model_validate_json(str(row[1])).model_copy(
                    update={
                        "state": OperationState.CANCELLED,
                        "message": "Interrupted by coordinator restart",
                    }
                )
                self._connection.execute(
                    "UPDATE imports SET state=?,document=? WHERE operation_id=?",
                    (progress.state.value, progress.model_dump_json(), progress.operation_id),
                )
                recovered.append(progress.operation_id)
        return tuple(recovered)

    def schedules(self) -> tuple[ScheduleDTO, ...]:
        with self._lock:
            rows = self._connection.execute(
                "SELECT document FROM schedules ORDER BY schedule_id"
            ).fetchall()
        return tuple(ScheduleDTO.model_validate_json(str(row[0])) for row in rows)

    def schedule(self, schedule_id: str) -> ScheduleDTO | None:
        with self._lock:
            row = self._connection.execute(
                "SELECT document FROM schedules WHERE schedule_id=?", (schedule_id,)
            ).fetchone()
        return None if row is None else ScheduleDTO.model_validate_json(str(row[0]))

    def create_schedule(self, schedule: ScheduleDTO) -> ScheduleDTO:
        with self._lock, self._connection:
            try:
                self._connection.execute(
                    "INSERT INTO schedules(schedule_id,revision,document) VALUES(?,?,?)",
                    (schedule.schedule_id, schedule.revision, schedule.model_dump_json()),
                )
            except sqlite3.IntegrityError as error:
                raise StaleRevisionError("schedule already exists") from error
        return schedule

    def update_schedule(self, schedule: ScheduleDTO, expected_revision: int) -> ScheduleDTO:
        updated = schedule.model_copy(update={"revision": expected_revision + 1})
        with self._lock, self._connection:
            cursor = self._connection.execute(
                "UPDATE schedules SET revision=?,document=? WHERE schedule_id=? AND revision=?",
                (
                    updated.revision,
                    updated.model_dump_json(),
                    schedule.schedule_id,
                    expected_revision,
                ),
            )
            if cursor.rowcount != 1:
                raise StaleRevisionError("schedule revision changed")
        return updated

    def delete_schedule(self, schedule_id: str, expected_revision: int) -> None:
        with self._lock, self._connection:
            cursor = self._connection.execute(
                "DELETE FROM schedules WHERE schedule_id=? AND revision=?",
                (schedule_id, expected_revision),
            )
            if cursor.rowcount != 1:
                raise StaleRevisionError("schedule revision changed")

    def credential_test_status(self) -> tuple[str | None, bool | None, str | None]:
        with self._lock:
            row = self._connection.execute(
                """SELECT last_tested_at,last_test_succeeded,safe_detail
                   FROM credential_status WHERE provider='massive'"""
            ).fetchone()
        if row is None:
            return None, None, None
        succeeded = None if row[1] is None else bool(row[1])
        return (
            None if row[0] is None else str(row[0]),
            succeeded,
            None if row[2] is None else str(row[2]),
        )

    def set_credential_test_status(self, tested_at: str, succeeded: bool, detail: str) -> None:
        with self._lock, self._connection:
            self._connection.execute(
                """INSERT INTO credential_status(
                       provider,last_tested_at,last_test_succeeded,safe_detail
                   )
                   VALUES('massive',?,?,?)
                   ON CONFLICT(provider) DO UPDATE SET
                     last_tested_at=excluded.last_tested_at,
                     last_test_succeeded=excluded.last_test_succeeded,
                     safe_detail=excluded.safe_detail""",
                (tested_at, int(succeeded), detail),
            )

    def clear_credential_test_status(self) -> None:
        with self._lock, self._connection:
            self._connection.execute("DELETE FROM credential_status WHERE provider='massive'")
