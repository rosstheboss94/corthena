from __future__ import annotations

import sys
from dataclasses import asdict

import pytest

from corthena.compatibility.runtime import regular_cpython as runtime
from corthena.compatibility.runtime.protocol import (
    NativeImportProbeProtocol,
    RuntimeProbeProtocol,
)

RUNTIME_PROBE: RuntimeProbeProtocol = runtime.RegularCpythonRuntimeProbe()
NATIVE_IMPORT_PROBE: NativeImportProbeProtocol = runtime.RegularCpythonRuntimeProbe()


def _configure_supported_runtime(monkeypatch: pytest.MonkeyPatch) -> None:
    values: dict[str, object] = {"Py_GIL_DISABLED": 0, "SOABI": runtime.EXPECTED_ABI}
    monkeypatch.setattr(runtime.sysconfig, "get_config_var", values.get)
    monkeypatch.setattr(runtime.sysconfig, "get_platform", lambda: "win-amd64")
    monkeypatch.setattr(runtime.platform, "system", lambda: "Windows")
    monkeypatch.setattr(runtime.platform, "machine", lambda: "AMD64")
    monkeypatch.setattr(runtime.platform, "python_implementation", lambda: "CPython")
    monkeypatch.setattr(runtime.platform, "python_version", lambda: "3.14.2")
    monkeypatch.setattr(runtime.sys, "version_info", runtime.EXPECTED_VERSION)


def test_free_threaded_cpython_is_rejected(monkeypatch: pytest.MonkeyPatch) -> None:
    _configure_supported_runtime(monkeypatch)
    values: dict[str, object] = {"Py_GIL_DISABLED": 1, "SOABI": runtime.EXPECTED_ABI}
    monkeypatch.setattr(runtime.sysconfig, "get_config_var", values.get)
    with pytest.raises(
        runtime.UnsupportedRuntimeError, match="free-threaded CPython is unsupported"
    ):
        RUNTIME_PROBE.probe("test")


def test_supported_runtime_is_healthy(monkeypatch: pytest.MonkeyPatch) -> None:
    _configure_supported_runtime(monkeypatch)
    capabilities = RUNTIME_PROBE.probe("test", process_count=2)
    assert capabilities.status == "healthy"
    assert capabilities.process_count == 2
    assert capabilities.python_abi == runtime.EXPECTED_ABI
    assert "gil_enabled" not in asdict(capabilities)
    assert "free_threaded_build" not in asdict(capabilities)


@pytest.mark.skipif(
    sys.version_info[:3] != runtime.EXPECTED_VERSION,
    reason="exact runtime assertion belongs to the CPython 3.14.2 gate",
)
def test_configured_runtime_is_regular_cpython() -> None:
    capabilities = RUNTIME_PROBE.probe("test")
    assert capabilities.python_abi == runtime.EXPECTED_ABI
