"""Data-only contract for storage compatibility evidence."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class StorageEvidence:
    """Observed storage round-trip properties."""

    rows: int
    arrow_bytes: int
    matrix_sum: float
