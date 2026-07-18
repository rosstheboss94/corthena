from __future__ import annotations

from collections.abc import Callable
from datetime import UTC, datetime
from threading import Event
from typing import Protocol

from corthena.ui.data_experiments.deterministic import DataExperimentsDemo
from corthena.ui.data_experiments.models import Phase7Request, Phase7Workspace


class Benchmark(Protocol):
    def __call__[**Parameters, Result](
        self,
        target: Callable[Parameters, Result],
        *args: Parameters.args,
        **kwargs: Parameters.kwargs,
    ) -> Result: ...


def test_phase7_load_and_estimate_benchmark(benchmark: Benchmark) -> None:
    demo = DataExperimentsDemo(107, datetime(2026, 7, 10, 12, tzinfo=UTC))
    snapshot = benchmark(
        demo.load,
        Phase7Request("phase7-benchmark", 1, Phase7Workspace.EXPERIMENTS),
        Event(),
    )
    assert snapshot.evaluation.valid
    assert snapshot.evaluation.estimate.feature_bytes > 0
