"""Pure validation and deterministic preview building for dataset recipes."""

from __future__ import annotations

import hashlib
from dataclasses import replace
from datetime import datetime

from corthena.ui.datasets.models import (
    ColumnType,
    CrossSectionalStep,
    DatasetBuild,
    DatasetBuildRequest,
    DatasetBuildState,
    DatasetDiagnostic,
    DatasetValidation,
    DatasetVersion,
    EngineeredColumn,
    FeatureCategory,
    FeatureStep,
    LaggedReturnStep,
    PriceVolumeRatioStep,
    RollingRangeStep,
    RollingStatisticStep,
    RollingVolatilityStep,
    SourceDefinition,
    SourceFamily,
)

REGISTRY_VERSION = "builtin-market-bars-v1"
ENGINE_FINGERPRINT = "sha256:dataset-engine-v1"


def validate_dataset_version(
    version: DatasetVersion, source: SourceDefinition
) -> DatasetValidation:
    diagnostics: list[DatasetDiagnostic] = []
    if source.family is not SourceFamily.MARKET_BARS:
        diagnostics.append(
            DatasetDiagnostic(
                "unsupported_source_family", "Only market bars are supported", "source"
            )
        )
    if version.source_id != source.source_id or version.source_revision != source.revision:
        diagnostics.append(
            DatasetDiagnostic("source_revision", "Recipe source revision is stale", "source")
        )
    if version.registry_version != REGISTRY_VERSION:
        diagnostics.append(
            DatasetDiagnostic(
                "registry_version", "Feature registry version is unsupported", "registry_version"
            )
        )
    if version.implementation_fingerprint != ENGINE_FINGERPRINT:
        diagnostics.append(
            DatasetDiagnostic(
                "implementation_fingerprint",
                "Feature implementation fingerprint is unsupported",
                "implementation_fingerprint",
            )
        )

    available: dict[str, tuple[ColumnType, int]] = {
        column.name: (column.type, 0) for column in source.schema
    }
    columns: list[EngineeredColumn] = []
    for index, step in enumerate(version.steps):
        output = step.output_name
        if not output or output.strip() != output:
            diagnostics.append(
                DatasetDiagnostic(
                    "output_name", "Output name must be normalized", "output_name", index
                )
            )
            continue
        if output in available:
            diagnostics.append(
                DatasetDiagnostic(
                    "output_collision", f"Output {output!r} already exists", "output_name", index
                )
            )
            continue
        inputs, category, own_lookback = _step_contract(step, index, diagnostics)
        input_lookback = 0
        for name in inputs:
            value = available.get(name)
            if value is None:
                diagnostics.append(
                    DatasetDiagnostic(
                        "dependency_order",
                        f"Input {name!r} is not available from the source or an earlier step",
                        "inputs",
                        index,
                    )
                )
            elif value[0] is not ColumnType.FLOAT64 and name not in {"timestamp", "symbol"}:
                diagnostics.append(
                    DatasetDiagnostic(
                        "input_type", f"Input {name!r} must be numeric", "inputs", index
                    )
                )
            else:
                input_lookback = max(input_lookback, value[1])
        if isinstance(step, CrossSectionalStep) and (
            available.get(step.timestamp_column, (None, 0))[0] is not ColumnType.TIMESTAMP
            or available.get(step.symbol_column, (None, 0))[0] is not ColumnType.SYMBOL
        ):
            diagnostics.append(
                DatasetDiagnostic(
                    "cross_section_keys",
                    "Cross-sectional transforms require timestamp and symbol keys",
                    "inputs",
                    index,
                )
            )
        lookback = input_lookback + own_lookback
        fingerprint = _fingerprint(f"{version.implementation_fingerprint}|{index}|{step!r}")
        available[output] = (ColumnType.FLOAT64, lookback)
        columns.append(
            EngineeredColumn(output, ColumnType.FLOAT64, category, lookback, fingerprint, inputs)
        )
    return DatasetValidation(
        version,
        tuple(columns),
        tuple(diagnostics),
        max((column.lookback for column in columns), default=0),
    )


def build_dataset_preview(
    request: DatasetBuildRequest,
    source: SourceDefinition,
    completed_at: datetime,
) -> DatasetBuild:
    validation = validate_dataset_version(request.version, source)
    state = DatasetBuildState.READY if validation.valid else DatasetBuildState.FAILED
    fingerprint = _fingerprint(
        "|".join(
            (
                request.version.recipe_fingerprint,
                request.source_snapshot.content_fingerprint,
                request.version.registry_version,
                request.version.implementation_fingerprint,
            )
        )
    )
    return DatasetBuild(
        f"build-{fingerprint[7:23]}",
        request.command_id,
        request.correlation_id,
        request.generation,
        request.version,
        request.source_snapshot,
        state,
        validation,
        fingerprint,
        min(request.preview_limit, request.source_snapshot.row_count),
        completed_at,
    )


def mark_build_stale(build: DatasetBuild, latest_snapshot_id: str) -> DatasetBuild:
    if (
        build.source_snapshot.snapshot_id == latest_snapshot_id
        or build.state is not DatasetBuildState.READY
    ):
        return build
    return replace(build, state=DatasetBuildState.STALE)


def _step_contract(
    step: FeatureStep,
    index: int,
    diagnostics: list[DatasetDiagnostic],
) -> tuple[tuple[str, ...], FeatureCategory, int]:
    if isinstance(step, LaggedReturnStep):
        if not 1 <= step.periods <= 4096:
            diagnostics.append(
                DatasetDiagnostic(
                    "parameter_bounds", "Return periods must be 1-4096", "periods", index
                )
            )
        return (step.input_column,), FeatureCategory.RETURNS, max(0, step.periods)
    if isinstance(step, RollingStatisticStep):
        _window(step.window, index, diagnostics)
        return (step.input_column,), FeatureCategory.ROLLING, max(0, step.window - 1)
    if isinstance(step, PriceVolumeRatioStep):
        return (step.price_column, step.volume_column), FeatureCategory.RATIO, 0
    if isinstance(step, RollingVolatilityStep):
        _window(step.window, index, diagnostics)
        return (step.input_column,), FeatureCategory.VOLATILITY, max(0, step.window)
    if isinstance(step, RollingRangeStep):
        _window(step.window, index, diagnostics)
        return (
            (step.high_column, step.low_column),
            FeatureCategory.VOLATILITY,
            max(0, step.window - 1),
        )
    return (
        (step.input_column, step.timestamp_column, step.symbol_column),
        FeatureCategory.CROSS_SECTIONAL,
        0,
    )


def _window(value: int, index: int, diagnostics: list[DatasetDiagnostic]) -> None:
    if not 2 <= value <= 4096:
        diagnostics.append(
            DatasetDiagnostic("parameter_bounds", "Rolling window must be 2-4096", "window", index)
        )


def _fingerprint(value: str) -> str:
    return f"sha256:{hashlib.sha256(value.encode()).hexdigest()}"
