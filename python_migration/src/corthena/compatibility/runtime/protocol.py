from typing import Protocol

from corthena.compatibility.runtime.models import NativeImportEvidence, RuntimeCapabilities


class RuntimeProbeProtocol(Protocol):
    def probe(
        self,
        process_role: str,
        *,
        task_count: int = 0,
        process_count: int = 1,
        cpu_lease: int = 1,
        library_pool_limit: int = 1,
    ) -> RuntimeCapabilities: ...


class NativeImportProbeProtocol(Protocol):
    def audit_native_imports(self) -> tuple[NativeImportEvidence, ...]: ...
