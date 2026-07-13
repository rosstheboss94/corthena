"""Bounded deterministic cache and worker ownership for Phase 5 preparation."""

from __future__ import annotations

import queue
import threading
import time
from collections import OrderedDict
from collections.abc import Callable
from contextlib import suppress
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class RenderFrame:
    """Immutable generation-bound render payload with complete byte accounting."""

    key: str
    generation: int
    payload: bytes

    def __post_init__(self) -> None:
        if not self.key or self.generation < 1:
            raise ValueError("frame identity is invalid")

    @property
    def byte_size(self) -> int:
        return len(self.key.encode()) + len(self.payload) + 8


class ByteLRU:
    """Explicitly locked, byte-bounded deterministic least-recently-used cache."""

    def __init__(self, capacity_bytes: int) -> None:
        if capacity_bytes <= 0:
            raise ValueError("cache capacity must be positive")
        self._capacity = capacity_bytes
        self._bytes = 0
        self._entries: OrderedDict[str, RenderFrame] = OrderedDict()
        self._lock = threading.Lock()

    @property
    def used_bytes(self) -> int:
        with self._lock:
            return self._bytes

    def get(self, key: str) -> RenderFrame | None:
        with self._lock:
            frame = self._entries.get(key)
            if frame is None:
                return None
            self._entries.move_to_end(key)
            return frame

    def put(self, frame: RenderFrame) -> bool:
        """Insert atomically; reject oversized frames without changing the cache."""
        size = frame.byte_size
        if size > self._capacity:
            return False
        with self._lock:
            previous = self._entries.pop(frame.key, None)
            if previous is not None:
                self._bytes -= previous.byte_size
            self._entries[frame.key] = frame
            self._bytes += size
            while self._bytes > self._capacity:
                _, removed = self._entries.popitem(last=False)
                self._bytes -= removed.byte_size
        return True


@dataclass(frozen=True, slots=True)
class PrepareRequest:
    """One scope-owned cancellable preparation request."""

    scope: str
    key: str
    generation: int

    def __post_init__(self) -> None:
        if not self.scope or not self.key or self.generation < 1:
            raise ValueError("request identity is invalid")


@dataclass(frozen=True, slots=True)
class PrepareResult:
    """A current immutable frame or typed failure."""

    scope: str
    generation: int
    frame: RenderFrame | None = None
    error: str | None = None


Prepare = Callable[[PrepareRequest, threading.Event], bytes]


class VisualizationWorkers:
    """Fixed workers with bounded queues, deduplication, cancellation, and stale rejection."""

    _STOP = object()

    def __init__(self, prepare: Prepare, *, workers: int, capacity: int, cache_bytes: int) -> None:
        if workers <= 0 or capacity <= 0:
            raise ValueError("workers and capacity must be positive")
        self._prepare = prepare
        self._requests: queue.Queue[PrepareRequest | object] = queue.Queue(capacity)
        self._results: queue.Queue[PrepareResult] = queue.Queue(capacity)
        self._cache = ByteLRU(cache_bytes)
        self._lock = threading.Lock()
        self._latest: dict[str, int] = {}
        self._active: dict[tuple[str, int], threading.Event] = {}
        self._closed = False
        self._threads = tuple(
            threading.Thread(target=self._run, name=f"corthena-viz-{index}")
            for index in range(workers)
        )
        for thread in self._threads:
            thread.start()

    @property
    def cache(self) -> ByteLRU:
        return self._cache

    def submit(self, request: PrepareRequest) -> bool:
        """Nonblockingly accept only a newer generation for a scope."""
        with self._lock:
            if self._closed or request.generation <= self._latest.get(request.scope, 0):
                return False
            for (scope, _), cancelled in self._active.items():
                if scope == request.scope:
                    cancelled.set()
            self._latest[request.scope] = request.generation
            cancelled = threading.Event()
            self._active[(request.scope, request.generation)] = cancelled
        cached = self._cache.get(request.key)
        if cached is not None:
            self._publish(
                request, RenderFrame(request.key, request.generation, cached.payload), None
            )
            return True
        try:
            self._requests.put_nowait(request)
            return True
        except queue.Full:
            with self._lock:
                self._active.pop((request.scope, request.generation), None)
            return False

    def cancel(self, scope: str) -> None:
        with self._lock:
            for (active_scope, _), cancelled in self._active.items():
                if active_scope == scope:
                    cancelled.set()

    def get_nowait(self) -> PrepareResult | None:
        try:
            return self._results.get_nowait()
        except queue.Empty:
            return None

    def close(self, timeout: float = 2.0) -> None:
        """Idempotently cancel and join every owned non-daemon worker."""
        if timeout <= 0:
            raise ValueError("shutdown timeout must be positive")
        deadline = time.monotonic() + timeout
        with self._lock:
            if self._closed:
                return
            self._closed = True
            for cancelled in self._active.values():
                cancelled.set()
        for _ in self._threads:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("visualization worker shutdown deadline exceeded")
            try:
                self._requests.put(self._STOP, timeout=remaining)
            except queue.Full as error:
                raise TimeoutError("visualization worker shutdown deadline exceeded") from error
        for thread in self._threads:
            thread.join(max(0.0, deadline - time.monotonic()))
        if any(thread.is_alive() for thread in self._threads):
            raise TimeoutError("visualization worker shutdown deadline exceeded")

    def __enter__(self) -> VisualizationWorkers:
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.close()

    def _run(self) -> None:
        while True:
            item = self._requests.get()
            if item is self._STOP:
                return
            if not isinstance(item, PrepareRequest):
                continue
            with self._lock:
                cancelled = self._active.get((item.scope, item.generation))
            if cancelled is None or cancelled.is_set():
                continue
            try:
                payload = self._prepare(item, cancelled)
                if cancelled.is_set():
                    continue
                frame = RenderFrame(item.key, item.generation, bytes(payload))
                self._cache.put(frame)
                self._publish(item, frame, None)
            except Exception as error:  # contained at the worker boundary
                self._publish(item, None, f"{type(error).__name__}: {error}")

    def _publish(
        self, request: PrepareRequest, frame: RenderFrame | None, error: str | None
    ) -> None:
        with self._lock:
            self._active.pop((request.scope, request.generation), None)
            current = not self._closed and self._latest.get(request.scope) == request.generation
        if not current:
            return
        with suppress(queue.Full):
            self._results.put_nowait(PrepareResult(request.scope, request.generation, frame, error))


__all__ = ["ByteLRU", "PrepareRequest", "PrepareResult", "RenderFrame", "VisualizationWorkers"]
