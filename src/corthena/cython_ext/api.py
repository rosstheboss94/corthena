"""Typed adapter for the Phase 0 native-extension probe."""

from __future__ import annotations

from collections.abc import Callable

_native_add: Callable[[int, int], int] | None

try:
    from corthena.cython_ext._compat import add_checked as _native_add
except ImportError:
    _native_add = None


def native_available() -> bool:
    """Return whether the compiled compatibility extension imported."""
    return _native_add is not None


def add_checked(left: int, right: int) -> int:
    """Add two integers through Cython, with a deterministic Python fallback."""
    if _native_add is None:
        return left + right
    return _native_add(left, right)
