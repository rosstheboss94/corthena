from __future__ import annotations

import sys

import pytest

from corthena.compatibility import runtime


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
        runtime.probe_runtime("test")


def test_supported_runtime_is_healthy(monkeypatch: pytest.MonkeyPatch) -> None:
    _configure_supported_runtime(monkeypatch)
    capabilities = runtime.probe_runtime("test", process_count=2)
    assert capabilities.status == "healthy"
    assert capabilities.process_count == 2
    assert capabilities.python_abi == runtime.EXPECTED_ABI
    assert "gil_enabled" not in capabilities.health_payload()
    assert "free_threaded_build" not in capabilities.health_payload()


@pytest.mark.skipif(
    sys.version_info[:3] != runtime.EXPECTED_VERSION,
    reason="exact runtime assertion belongs to the CPython 3.14.2 gate",
)
def test_configured_runtime_is_regular_cpython() -> None:
    capabilities = runtime.probe_runtime("test")
    assert capabilities.python_abi == runtime.EXPECTED_ABI
