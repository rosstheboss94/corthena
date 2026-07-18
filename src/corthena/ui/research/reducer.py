"""Pure reducer for independently generation-ordered Research link groups."""

from dataclasses import replace
from typing import assert_never

from corthena.ui.research.actions import (
    CancelResearch,
    LoadResearch,
    RequestResearch,
    ResearchAction,
    ResearchCancelled,
    ResearchCompleted,
    ResearchEffect,
    ResearchFailed,
    SelectResearchRow,
    SetResearchFeature,
    SetResearchRange,
    SetResearchScenario,
    SetResearchVisibility,
)
from corthena.ui.research.models import (
    ResearchGroupState,
    ResearchLoadState,
    ResearchQuery,
    ResearchScenario,
    ResearchWorkspaceState,
)


def reduce_research(
    state: ResearchWorkspaceState, action: ResearchAction
) -> tuple[ResearchWorkspaceState, tuple[ResearchEffect, ...]]:
    match action:
        case RequestResearch(query=query):
            current = state.group(query.group_id)
            if current is not None and query.generation <= current.generation:
                raise ValueError("Research generation must advance monotonically")
            group = ResearchGroupState(query.group_id) if current is None else current
            effects: tuple[ResearchEffect, ...] = (LoadResearch(query),)
            if group.query is not None and group.state is ResearchLoadState.LOADING:
                effects = (
                    CancelResearch(
                        group.query.request_id,
                        group.group_id,
                        group.generation,
                    ),
                    *effects,
                )
            updated = replace(
                group,
                generation=query.generation,
                selected_feature=query.selected_features[0],
                scenario=query.scenario,
                state=ResearchLoadState.LOADING,
                stale=group.snapshot is not None,
                query=query,
                error=None,
            )
            return _replace_group(state, updated), effects
        case ResearchCompleted(snapshot=snapshot):
            group = state.group(snapshot.query.group_id)
            if (
                group is None
                or group.query is None
                or snapshot.query.generation != group.generation
                or snapshot.query.correlation_id != group.query.correlation_id
            ):
                return state, ()
            status = (
                ResearchLoadState.DEGRADED
                if snapshot.degraded
                else ResearchLoadState.RECOVERED
                if snapshot.query.scenario is ResearchScenario.RECOVERED
                else ResearchLoadState.EMPTY
                if not snapshot.bars and snapshot.rows.total_rows == 0
                else ResearchLoadState.READY
            )
            return (
                _replace_group(
                    state,
                    replace(
                        group,
                        query=snapshot.query,
                        snapshot=snapshot,
                        state=status,
                        stale=False,
                        error=None,
                    ),
                ),
                (),
            )
        case ResearchFailed(group_id=group_id, generation=generation, message=message, busy=busy):
            group = state.group(group_id)
            if group is None or group.generation != generation:
                return state, ()
            return (
                _replace_group(
                    state,
                    replace(
                        group,
                        state=ResearchLoadState.BUSY if busy else ResearchLoadState.FAILED,
                        stale=group.snapshot is not None,
                        error=message,
                    ),
                ),
                (),
            )
        case ResearchCancelled(group_id=group_id, generation=generation):
            group = state.group(group_id)
            if group is None or group.generation != generation:
                return state, ()
            return (
                _replace_group(
                    state,
                    replace(
                        group,
                        state=ResearchLoadState.CANCELLED,
                        stale=group.snapshot is not None,
                        error="Research request cancelled",
                    ),
                ),
                (),
            )
        case SetResearchFeature(group_id=group_id, feature=feature):
            if not feature or feature.strip() != feature:
                raise ValueError("Research feature is invalid")
            group = _required_group(state, group_id)
            query = _next_query(group, selected_features=(feature,), cursor="")
            return reduce_research(state, RequestResearch(query))
        case SetResearchScenario(group_id=group_id, scenario=scenario):
            group = _required_group(state, group_id)
            query = _next_query(group, scenario=scenario, cursor="")
            return reduce_research(state, RequestResearch(query))
        case SetResearchVisibility(
            group_id=group_id,
            show_ohlcv=show_ohlcv,
            show_feature=show_feature,
            show_target=show_target,
        ):
            group = _required_group(state, group_id)
            return (
                _replace_group(
                    state,
                    replace(
                        group,
                        show_ohlcv=show_ohlcv,
                        show_feature=show_feature,
                        show_target=show_target,
                    ),
                ),
                (),
            )
        case SelectResearchRow(group_id=group_id, row_id=row_id, toggle=toggle):
            if not row_id:
                raise ValueError("Research row ID is required")
            group = _required_group(state, group_id)
            selected = list(group.selected_rows)
            if toggle and row_id in selected:
                selected.remove(row_id)
            elif toggle:
                selected.append(row_id)
            else:
                selected = [row_id]
            return _replace_group(state, replace(group, selected_rows=tuple(selected))), ()
        case SetResearchRange(group_id=group_id, time_range=time_range):
            group = _required_group(state, group_id)
            query = _next_query(group, time_range=time_range, cursor="")
            return reduce_research(state, RequestResearch(query))
        case _ as unreachable:
            assert_never(unreachable)


def _required_group(state: ResearchWorkspaceState, group_id: str) -> ResearchGroupState:
    group = state.group(group_id)
    if group is None or group.query is None:
        raise ValueError(f"Research group {group_id!r} is not initialized")
    return group


def _next_query(group: ResearchGroupState, **changes: object) -> ResearchQuery:
    if group.query is None:
        raise ValueError("Research group has no query")
    generation = group.generation + 1
    return replace(
        group.query,
        generation=generation,
        correlation_id=f"research-{group.group_id}-{generation:020d}",
        **changes,
    )


def _replace_group(
    state: ResearchWorkspaceState, updated: ResearchGroupState
) -> ResearchWorkspaceState:
    found = False
    groups: list[ResearchGroupState] = []
    for group in state.groups:
        if group.group_id == updated.group_id:
            groups.append(updated)
            found = True
        else:
            groups.append(group)
    if not found:
        groups.append(updated)
    groups.sort(key=lambda value: value.group_id)
    return replace(state, groups=tuple(groups), selected_feature=updated.selected_feature)
