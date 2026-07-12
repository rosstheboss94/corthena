from concurrent.futures import ThreadPoolExecutor

import pytest

from corthena.cython_ext import add_checked, api, native_available


def test_compiled_extension_imports() -> None:
    assert native_available()
    assert add_checked(2, 3) == 5


def test_python_fallback_is_deterministic(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(api, "_native_add", None)
    assert not api.native_available()
    assert api.add_checked(2, 3) == 5


def test_compiled_extension_is_safe_for_concurrent_calls() -> None:
    def increment(value: int) -> int:
        return add_checked(value, 1)

    with ThreadPoolExecutor(max_workers=4) as pool:
        results = tuple(pool.map(increment, range(1_000)))
    assert results == tuple(range(1, 1_001))
