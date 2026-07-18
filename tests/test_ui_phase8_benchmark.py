from __future__ import annotations

from collections.abc import Callable
from datetime import UTC, datetime
from threading import Event
from typing import Protocol

from corthena.ui.jobs_results.deterministic import JobsResultsDemo
from corthena.ui.jobs_results.models import (
    ComparisonQuery,
    Phase8Request,
    Phase8Scenario,
    Phase8Workspace,
)


class Benchmark(Protocol):
    def __call__[**Parameters, Result](
        self,
        target: Callable[Parameters, Result],
        *args: Parameters.args,
        **kwargs: Parameters.kwargs,
    ) -> Result: ...


def test_phase8_comparison_preparation_benchmark(benchmark: Benchmark) -> None:
    demo = JobsResultsDemo(208, datetime(2026, 7, 10, 12, tzinfo=UTC))
    snapshot = demo.load(
        Phase8Request(
            "phase8-benchmark-load",
            1,
            Phase8Workspace.RESULTS,
            Phase8Scenario.RESULTS_NORMAL,
        ),
        Event(),
    )
    comparison = benchmark(
        demo.compare,
        ComparisonQuery(
            "phase8-benchmark-comparison",
            1,
            tuple(item.run_id for item in snapshot.runs),
        ),
        Event(),
    )
    assert comparison.runs
