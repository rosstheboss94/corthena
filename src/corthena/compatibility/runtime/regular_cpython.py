"""Exact regular-CPython runtime and native-import probe implementation."""

from __future__ import annotations

import importlib
import importlib.metadata
import platform
import sys
import sysconfig
import threading

from corthena.compatibility.runtime.models import NativeImportEvidence, RuntimeCapabilities

EXPECTED_VERSION = (3, 14, 2)
EXPECTED_ABI = "cp314-win_amd64"


class UnsupportedRuntimeError(RuntimeError):
    """Raised when startup is attempted with an unsupported interpreter."""


class RegularCpythonRuntimeProbe:
    """Validate and describe the one supported Corthena interpreter."""

    def probe(
        self,
        process_role: str,
        *,
        task_count: int = 0,
        process_count: int = 1,
        cpu_lease: int = 1,
        library_pool_limit: int = 1,
    ) -> RuntimeCapabilities:
        """Validate the exact interpreter and return process capabilities."""
        version = sys.version_info[:3]
        free_threaded = sysconfig.get_config_var("Py_GIL_DISABLED") == 1
        abi = str(sysconfig.get_config_var("SOABI") or "")
        platform_tag = sysconfig.get_platform().replace("-", "_")
        if platform.python_implementation() != "CPython" or version != EXPECTED_VERSION:
            raise UnsupportedRuntimeError(
                "Corthena requires exactly regular CPython 3.14.2 for Windows AMD64"
            )
        if free_threaded:
            raise UnsupportedRuntimeError(
                "free-threaded CPython is unsupported; use the regular build"
            )
        if platform.system() != "Windows" or platform.machine().upper() not in {
            "AMD64",
            "X86_64",
        }:
            raise UnsupportedRuntimeError("Corthena requires the Windows AMD64 runtime")
        if abi != EXPECTED_ABI:
            raise UnsupportedRuntimeError(
                f"expected Python ABI {EXPECTED_ABI}, observed {abi or 'unknown'}"
            )

        return RuntimeCapabilities(
            python_version=platform.python_version(),
            python_abi=abi,
            platform_tag=platform_tag,
            process_role=process_role,
            thread_count=threading.active_count(),
            process_count=process_count,
            task_count=task_count,
            cpu_lease=cpu_lease,
            library_pool_limit=library_pool_limit,
            status="healthy",
        )

    def audit_native_imports(self) -> tuple[NativeImportEvidence, ...]:
        """Import approved native dependencies incrementally and record versions."""
        dependencies = (
            ("cffi", "cffi"),
            ("numpy", "numpy"),
            ("pyarrow", "pyarrow"),
            ("cython", "Cython"),
            ("pydantic-core", "pydantic_core"),
            ("platformdirs", "platformdirs"),
            ("exchange-calendars", "exchange_calendars"),
            ("raylib", "pyray"),
            ("corthena", "corthena.cython_ext._compat"),
        )
        evidence: list[NativeImportEvidence] = []
        for distribution, module in dependencies:
            importlib.import_module(module)
            version = (
                "local" if distribution == "corthena" else importlib.metadata.version(distribution)
            )
            evidence.append(NativeImportEvidence(distribution, module, version))
        return tuple(evidence)
