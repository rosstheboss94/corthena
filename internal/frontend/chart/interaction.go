package chart

import (
	"fmt"
	"math"
)

// DragMode identifies the active pointer gesture.
type DragMode uint8

const (
	DragNone DragMode = iota
	DragPan
	DragSelect
)

// SeriesVisibility stores stable series visibility without map iteration.
type SeriesVisibility struct {
	SeriesID string
	Visible  bool
}

// InteractionState is pure panel-local chart interaction state.
type InteractionState struct {
	View        Rect
	Fit         Rect
	Crosshair   Point
	CrosshairOn bool
	Selection   Rect
	Selected    bool
	Drag        DragMode
	DragStart   Point
	DragView    Rect
	Series      []SeriesVisibility
}

// Clone returns an independent state value.
func (state InteractionState) Clone() InteractionState {
	state.Series = append([]SeriesVisibility(nil), state.Series...)
	return state
}

// InteractionInput is one complete routed chart-input frame.
type InteractionInput struct {
	Pointer         Point
	PointerPressed  bool
	PointerDown     bool
	PointerReleased bool
	WheelY          float64
	SelectModifier  bool
	Reset           bool
	ToggleSeriesID  string
}

// InteractionEventKind identifies a deterministic chart transition.
type InteractionEventKind uint8

const (
	EventViewChanged InteractionEventKind = iota + 1
	EventRangeSelected
	EventCrosshairChanged
	EventSeriesVisibilityChanged
)

// InteractionEvent is emitted in stable reducer order.
type InteractionEvent struct {
	Kind      InteractionEventKind
	View      Rect
	Selection Rect
	Point     Point
	SeriesID  string
	Visible   bool
}

// ReduceInteraction applies pan, zoom, box selection, crosshair, visibility,
// and reset-to-fit without native UI dependencies.
func ReduceInteraction(state InteractionState, viewport Rect, input InteractionInput) (InteractionState, []InteractionEvent, error) {
	if !viewport.Valid() || !state.View.Valid() || !state.Fit.Valid() || !finitePoint(input.Pointer) || !finite(input.WheelY) {
		return state, nil, fmt.Errorf("%w: invalid interaction state, viewport, or input", ErrInvalidGeometry)
	}
	next := state.Clone()
	events := make([]InteractionEvent, 0, 3)
	pointerInside := viewport.Contains(input.Pointer)
	if pointerInside {
		transform, err := NewTransform(next.View, viewport)
		if err != nil {
			return state, nil, err
		}
		dataPoint, err := transform.Inverse(input.Pointer)
		if err != nil {
			return state, nil, err
		}
		next.Crosshair = dataPoint
		next.CrosshairOn = true
		events = append(events, InteractionEvent{Kind: EventCrosshairChanged, Point: dataPoint})
	} else if next.CrosshairOn {
		next.CrosshairOn = false
	}

	if input.ToggleSeriesID != "" {
		visible := toggleSeries(&next.Series, input.ToggleSeriesID)
		events = append(events, InteractionEvent{Kind: EventSeriesVisibilityChanged, SeriesID: input.ToggleSeriesID, Visible: visible})
	}
	if input.Reset {
		next.View = next.Fit
		next.Selected = false
		next.Drag = DragNone
		events = append(events, InteractionEvent{Kind: EventViewChanged, View: next.View})
	}

	if pointerInside && input.WheelY != 0 {
		transform, _ := NewTransform(next.View, viewport)
		anchor, _ := transform.Inverse(input.Pointer)
		factor := math.Pow(1.18, -input.WheelY)
		factor = min(max(factor, 0.05), 20)
		next.View = zoomRect(next.View, anchor, factor)
		events = append(events, InteractionEvent{Kind: EventViewChanged, View: next.View})
	}

	if input.PointerPressed && pointerInside {
		next.Drag = DragPan
		if input.SelectModifier {
			next.Drag = DragSelect
		}
		next.DragStart = input.Pointer
		next.DragView = next.View
		next.Selected = false
	}
	if input.PointerDown && next.Drag == DragPan {
		dx := input.Pointer.X - next.DragStart.X
		dy := input.Pointer.Y - next.DragStart.Y
		next.View = Rect{
			MinX: next.DragView.MinX - dx*next.DragView.Width()/viewport.Width(),
			MaxX: next.DragView.MaxX - dx*next.DragView.Width()/viewport.Width(),
			MinY: next.DragView.MinY + dy*next.DragView.Height()/viewport.Height(),
			MaxY: next.DragView.MaxY + dy*next.DragView.Height()/viewport.Height(),
		}
		events = append(events, InteractionEvent{Kind: EventViewChanged, View: next.View})
	}
	if input.PointerReleased {
		if next.Drag == DragSelect {
			selectionScreen := normalizedRect(next.DragStart, input.Pointer)
			selectionScreen = intersectRect(selectionScreen, viewport)
			if selectionScreen.Valid() && selectionScreen.Width() >= 2 && selectionScreen.Height() >= 2 {
				transform, _ := NewTransform(next.View, viewport)
				leftBottom, _ := transform.Inverse(Point{X: selectionScreen.MinX, Y: selectionScreen.MaxY})
				rightTop, _ := transform.Inverse(Point{X: selectionScreen.MaxX, Y: selectionScreen.MinY})
				next.Selection = Rect{MinX: leftBottom.X, MinY: leftBottom.Y, MaxX: rightTop.X, MaxY: rightTop.Y}
				next.Selected = next.Selection.Valid()
				if next.Selected {
					events = append(events, InteractionEvent{Kind: EventRangeSelected, Selection: next.Selection})
				}
			}
		}
		next.Drag = DragNone
	}
	if !input.PointerDown && !input.PointerPressed && !input.PointerReleased && next.Drag != DragNone {
		next.Drag = DragNone
	}
	return next, events, nil
}

// TooltipValueKind identifies one typed tooltip value.
type TooltipValueKind uint8

const (
	TooltipNumber TooltipValueKind = iota + 1
	TooltipText
	TooltipTimeUnixNano
)

// TooltipEntry is one explicitly typed tooltip row.
type TooltipEntry struct {
	Label    string
	Kind     TooltipValueKind
	Number   float64
	Text     string
	UnixNano int64
	Missing  bool
}

// Tooltip is an immutable typed crosshair payload.
type Tooltip struct {
	Title   string
	Entries []TooltipEntry
}

// Clone returns an independent tooltip.
func (tooltip Tooltip) Clone() Tooltip {
	tooltip.Entries = append([]TooltipEntry(nil), tooltip.Entries...)
	return tooltip
}

func toggleSeries(series *[]SeriesVisibility, id string) bool {
	for index := range *series {
		if (*series)[index].SeriesID == id {
			(*series)[index].Visible = !(*series)[index].Visible
			return (*series)[index].Visible
		}
	}
	*series = append(*series, SeriesVisibility{SeriesID: id, Visible: false})
	return false
}

func zoomRect(rect Rect, anchor Point, factor float64) Rect {
	return Rect{
		MinX: anchor.X + (rect.MinX-anchor.X)*factor,
		MaxX: anchor.X + (rect.MaxX-anchor.X)*factor,
		MinY: anchor.Y + (rect.MinY-anchor.Y)*factor,
		MaxY: anchor.Y + (rect.MaxY-anchor.Y)*factor,
	}
}

func normalizedRect(first Point, second Point) Rect {
	return Rect{MinX: min(first.X, second.X), MinY: min(first.Y, second.Y), MaxX: max(first.X, second.X), MaxY: max(first.Y, second.Y)}
}

func intersectRect(left Rect, right Rect) Rect {
	return Rect{MinX: max(left.MinX, right.MinX), MinY: max(left.MinY, right.MinY), MaxX: min(left.MaxX, right.MaxX), MaxY: min(left.MaxY, right.MaxY)}
}
