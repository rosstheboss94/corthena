"""Closed Research actions and effects."""

from dataclasses import dataclass

from corthena.ui.research.models import (
    ResearchQuery,
    ResearchScenario,
    ResearchSnapshot,
    TimeRange,
)


@dataclass(frozen=True, slots=True)
class RequestResearch:
    query: ResearchQuery


@dataclass(frozen=True, slots=True)
class ResearchCompleted:
    snapshot: ResearchSnapshot


@dataclass(frozen=True, slots=True)
class ResearchFailed:
    group_id: str
    generation: int
    message: str
    busy: bool = False


@dataclass(frozen=True, slots=True)
class ResearchCancelled:
    group_id: str
    generation: int


@dataclass(frozen=True, slots=True)
class SetResearchFeature:
    group_id: str
    feature: str


@dataclass(frozen=True, slots=True)
class SetResearchScenario:
    group_id: str
    scenario: ResearchScenario


@dataclass(frozen=True, slots=True)
class SetResearchVisibility:
    group_id: str
    show_ohlcv: bool
    show_feature: bool
    show_target: bool


@dataclass(frozen=True, slots=True)
class SelectResearchRow:
    group_id: str
    row_id: str
    toggle: bool = False


@dataclass(frozen=True, slots=True)
class SetResearchRange:
    group_id: str
    source_panel_id: str
    time_range: TimeRange


ResearchAction = (
    RequestResearch
    | ResearchCompleted
    | ResearchFailed
    | ResearchCancelled
    | SetResearchFeature
    | SetResearchScenario
    | SetResearchVisibility
    | SelectResearchRow
    | SetResearchRange
)
RESEARCH_ACTION_TYPES = (
    RequestResearch,
    ResearchCompleted,
    ResearchFailed,
    ResearchCancelled,
    SetResearchFeature,
    SetResearchScenario,
    SetResearchVisibility,
    SelectResearchRow,
    SetResearchRange,
)


@dataclass(frozen=True, slots=True)
class LoadResearch:
    query: ResearchQuery

    @property
    def request_id(self) -> str:
        return self.query.request_id

    @property
    def generation(self) -> int:
        return self.query.generation


@dataclass(frozen=True, slots=True)
class CancelResearch:
    request_id: str
    group_id: str
    generation: int


ResearchEffect = LoadResearch | CancelResearch
