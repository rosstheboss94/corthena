"""Data-only contracts for runtime compatibility evidence."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class RuntimeCapabilities:
    """Validated process runtime identity and bounded-resource state."""

    python_version: str
    python_abi: str
    platform_tag: str
    process_role: str
    thread_count: int
    process_count: int
    task_count: int
    cpu_lease: int
    library_pool_limit: int
    status: str


@dataclass(frozen=True, slots=True)
class NativeImportEvidence:
    """Version evidence observed after one native dependency import."""

    distribution: str
    module: str
    version: str
