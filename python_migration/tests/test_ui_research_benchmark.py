from __future__ import annotations

from collections.abc import Callable
from datetime import UTC, datetime
from threading import Event
from typing import Protocol

from corthena.ui.research.deterministic import build_research_snapshot
from corthena.ui.research.models import default_research_query


class Benchmark(Protocol):
    def __call__[**Parameters, Result](
        self,
        target: Callable[Parameters, Result],
        *args: Parameters.args,
        **kwargs: Parameters.kwargs,
    ) -> Result: ...


def test_research_preparation_benchmark(benchmark: Benchmark) -> None:
    snapshot = benchmark(
        build_research_snapshot,
        42,
        datetime(2026, 7, 10, 12, tzinfo=UTC),
        default_research_query(),
        Event(),
    )
    assert len(snapshot.bars) > 2_000
    assert len(snapshot.rows.dataset.rows) <= snapshot.query.page_size
    assert snapshot.frame is not None
    primitive_count = sum(
        len(layer.segments) + len(layer.rects) + len(layer.markers)
        for layer in snapshot.frame.layers
    )
    assert primitive_count < len(snapshot.bars) * 12
