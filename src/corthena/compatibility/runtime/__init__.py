"""Regular-CPython runtime compatibility capability."""

from corthena.compatibility.runtime.models import NativeImportEvidence, RuntimeCapabilities
from corthena.compatibility.runtime.protocol import NativeImportProbeProtocol, RuntimeProbeProtocol
from corthena.compatibility.runtime.regular_cpython import (
    EXPECTED_ABI,
    EXPECTED_VERSION,
    RegularCpythonRuntimeProbe,
    UnsupportedRuntimeError,
)


def probe_runtime(
    process_role: str,
    *,
    task_count: int = 0,
    process_count: int = 1,
    cpu_lease: int = 1,
    library_pool_limit: int = 1,
) -> RuntimeCapabilities:
    """Validate the process with the default regular-CPython adapter."""
    return RegularCpythonRuntimeProbe().probe(
        process_role,
        task_count=task_count,
        process_count=process_count,
        cpu_lease=cpu_lease,
        library_pool_limit=library_pool_limit,
    )


def audit_native_imports() -> tuple[NativeImportEvidence, ...]:
    """Audit approved native imports with the default runtime adapter."""
    return RegularCpythonRuntimeProbe().audit_native_imports()


__all__ = (
    "EXPECTED_ABI",
    "EXPECTED_VERSION",
    "NativeImportEvidence",
    "NativeImportProbeProtocol",
    "RegularCpythonRuntimeProbe",
    "RuntimeCapabilities",
    "RuntimeProbeProtocol",
    "UnsupportedRuntimeError",
    "audit_native_imports",
    "probe_runtime",
)
