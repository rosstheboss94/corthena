package docking

import (
	"fmt"
	"math"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	// DefaultSplitterThickness is the logical splitter width at 100% scale.
	DefaultSplitterThickness = 4.0
	// DefaultDropTargetFraction is the share of a host edge used for drop zones.
	DefaultDropTargetFraction = 0.25
	// MinimumSplitRatio prevents a mutation from storing an unusable zero-sized side.
	MinimumSplitRatio = 0.05
	// MaximumSplitRatio prevents a mutation from storing an unusable zero-sized side.
	MaximumSplitRatio = 0.95
)

// Point is a logical or physical two-dimensional position.
type Point struct {
	X float64
	Y float64
}

// Size is a double-precision logical or physical size.
type Size struct {
	Width  float64
	Height float64
}

// Rect is a double-precision axis-aligned rectangle.
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Contains reports whether point is inside the rectangle's half-open bounds.
func (rect Rect) Contains(point Point) bool {
	if !rect.valid() || !finite(point.X) || !finite(point.Y) || rect.Width == 0 || rect.Height == 0 {
		return false
	}
	return point.X >= rect.X && point.X < rect.X+rect.Width &&
		point.Y >= rect.Y && point.Y < rect.Y+rect.Height
}

func (rect Rect) valid() bool {
	return finite(rect.X) && finite(rect.Y) && finite(rect.Width) && finite(rect.Height) &&
		finite(rect.X+rect.Width) && finite(rect.Y+rect.Height) &&
		rect.Width >= 0 && rect.Height >= 0
}

// Bounds contains the same rectangle in logical units and physical pixels.
type Bounds struct {
	Logical  Rect
	Physical Rect
}

// GeometryOptions controls logical layout and DPI conversion.
type GeometryOptions struct {
	DPIScale          float64
	SplitterThickness float64
}

// DefaultGeometryOptions returns the 100% scale geometry configuration.
func DefaultGeometryOptions() GeometryOptions {
	return GeometryOptions{
		DPIScale:          1,
		SplitterThickness: DefaultSplitterThickness,
	}
}

// NodeKind identifies a calculated dock-node variant.
type NodeKind string

const (
	// NodeSplit identifies an internal binary split.
	NodeSplit NodeKind = "split"
	// NodeTabStack identifies a leaf tab stack.
	NodeTabStack NodeKind = "tab_stack"
)

// NodeGeometry contains the bounds and logical minimum of one dock node.
type NodeGeometry struct {
	ID      appstate.DockNodeID
	Kind    NodeKind
	Bounds  Bounds
	Minimum Size
}

// SplitterGeometry contains one split's draggable divider geometry.
type SplitterGeometry struct {
	SplitID          appstate.DockNodeID
	Orientation      appstate.SplitOrientation
	Bounds           Bounds
	EffectiveRatio   float64
	LogicalThickness float64
}

// Geometry is a deterministic pre-order calculation of a dock tree.
type Geometry struct {
	Host      Bounds
	Minimum   Size
	Nodes     []NodeGeometry
	Splitters []SplitterGeometry
	DPIScale  float64
}

// Node returns calculated geometry for an ID.
func (geometry Geometry) Node(id appstate.DockNodeID) (NodeGeometry, bool) {
	for _, node := range geometry.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return NodeGeometry{}, false
}

// Splitter returns calculated splitter geometry for a split ID.
func (geometry Geometry) Splitter(id appstate.DockNodeID) (SplitterGeometry, bool) {
	for _, splitter := range geometry.Splitters {
		if splitter.SplitID == id {
			return splitter, true
		}
	}
	return SplitterGeometry{}, false
}

// ClampSplitRatio converts non-finite ratios to a stable default and clamps
// finite ratios to the mutation-safe range.
func ClampSplitRatio(ratio float64) float64 {
	switch {
	case math.IsNaN(ratio):
		return 0.5
	case math.IsInf(ratio, -1):
		return MinimumSplitRatio
	case math.IsInf(ratio, 1):
		return MaximumSplitRatio
	case ratio < MinimumSplitRatio:
		return MinimumSplitRatio
	case ratio > MaximumSplitRatio:
		return MaximumSplitRatio
	default:
		return ratio
	}
}

// MinimumSize returns the recursive logical minimum for a dock tree. A
// horizontal split arranges children left-to-right; a vertical split arranges
// them top-to-bottom.
func MinimumSize(root appstate.DockNode, splitterThickness float64) (Size, error) {
	if !finite(splitterThickness) || splitterThickness < 0 {
		return Size{}, fmt.Errorf("%w: splitter thickness must be finite and non-negative", ErrInvalidGeometry)
	}
	return minimumSize(root, splitterThickness)
}

// CalculateGeometry recursively assigns logical and physical bounds. When a
// host cannot satisfy both child minima, the remaining axis is divided in
// proportion to those minima; this keeps undersized layouts deterministic and
// prevents negative rectangles.
func CalculateGeometry(root appstate.DockNode, logicalHost Rect, options GeometryOptions) (Geometry, error) {
	if root == nil {
		return Geometry{}, fmt.Errorf("%w: dock root is nil", ErrInvalidGeometry)
	}
	if !logicalHost.valid() {
		return Geometry{}, fmt.Errorf("%w: host bounds must be finite and non-negative", ErrInvalidGeometry)
	}
	if !finite(options.DPIScale) || options.DPIScale <= 0 {
		return Geometry{}, fmt.Errorf("%w: DPI scale must be finite and positive", ErrInvalidGeometry)
	}
	if !finite(options.SplitterThickness) || options.SplitterThickness < 0 {
		return Geometry{}, fmt.Errorf("%w: splitter thickness must be finite and non-negative", ErrInvalidGeometry)
	}
	hostBounds := boundsFor(logicalHost, options.DPIScale)
	if !hostBounds.Physical.valid() {
		return Geometry{}, fmt.Errorf("%w: physical host bounds overflow float64", ErrInvalidGeometry)
	}
	minimum, err := minimumSize(root, options.SplitterThickness)
	if err != nil {
		return Geometry{}, err
	}
	geometry := Geometry{
		Host:     hostBounds,
		Minimum:  minimum,
		DPIScale: options.DPIScale,
	}
	seen := make(map[appstate.DockNodeID]struct{})
	if err := appendGeometry(&geometry, root, logicalHost, options, seen); err != nil {
		return Geometry{}, err
	}
	return geometry, nil
}

func minimumSize(root appstate.DockNode, splitterThickness float64) (Size, error) {
	return minimumSizeAtDepth(root, splitterThickness, 0)
}

func minimumSizeAtDepth(root appstate.DockNode, splitterThickness float64, depth int) (Size, error) {
	if depth > MaximumDockDepth {
		return Size{}, fmt.Errorf("%w: dock tree exceeds depth %d", ErrInvalidGeometry, MaximumDockDepth)
	}
	if root == nil {
		return Size{}, nil
	}
	switch node := root.(type) {
	case appstate.TabStackNode:
		var result Size
		for _, panel := range node.Panels {
			descriptor, err := appstate.PanelDescriptorFor(panel.Type)
			if err != nil {
				return Size{}, fmt.Errorf("%w: panel %q: %v", ErrInvalidGeometry, panel.ID, err)
			}
			result.Width = math.Max(result.Width, float64(descriptor.MinimumSize.Width))
			result.Height = math.Max(result.Height, float64(descriptor.MinimumSize.Height))
		}
		return result, nil
	case appstate.SplitNode:
		if node.First == nil || node.Second == nil {
			return Size{}, fmt.Errorf("%w: split %q has a nil child", ErrInvalidGeometry, node.ID)
		}
		first, err := minimumSizeAtDepth(node.First, splitterThickness, depth+1)
		if err != nil {
			return Size{}, err
		}
		second, err := minimumSizeAtDepth(node.Second, splitterThickness, depth+1)
		if err != nil {
			return Size{}, err
		}
		switch node.Orientation {
		case appstate.SplitHorizontal:
			return Size{Width: first.Width + splitterThickness + second.Width, Height: math.Max(first.Height, second.Height)}, nil
		case appstate.SplitVertical:
			return Size{Width: math.Max(first.Width, second.Width), Height: first.Height + splitterThickness + second.Height}, nil
		default:
			return Size{}, fmt.Errorf("%w: split %q has orientation %q", ErrInvalidGeometry, node.ID, node.Orientation)
		}
	default:
		return Size{}, fmt.Errorf("%w: unsupported dock node %T", ErrInvalidGeometry, root)
	}
}

func appendGeometry(
	geometry *Geometry,
	root appstate.DockNode,
	logical Rect,
	options GeometryOptions,
	seen map[appstate.DockNodeID]struct{},
) error {
	if root == nil {
		return nil
	}
	var id appstate.DockNodeID
	var kind NodeKind
	switch node := root.(type) {
	case appstate.TabStackNode:
		id = node.ID
		kind = NodeTabStack
	case appstate.SplitNode:
		id = node.ID
		kind = NodeSplit
	default:
		return fmt.Errorf("%w: unsupported dock node %T", ErrInvalidGeometry, root)
	}
	if id == "" {
		return fmt.Errorf("%w: dock node ID is empty", ErrInvalidGeometry)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("%w: %w: dock node ID %q", ErrInvalidGeometry, ErrDuplicateID, id)
	}
	seen[id] = struct{}{}
	minimum, err := minimumSize(root, options.SplitterThickness)
	if err != nil {
		return err
	}
	geometry.Nodes = append(geometry.Nodes, NodeGeometry{
		ID:      id,
		Kind:    kind,
		Bounds:  boundsFor(logical, options.DPIScale),
		Minimum: minimum,
	})

	node, split := root.(appstate.SplitNode)
	if !split {
		return nil
	}
	firstMinimum, err := minimumSize(node.First, options.SplitterThickness)
	if err != nil {
		return err
	}
	secondMinimum, err := minimumSize(node.Second, options.SplitterThickness)
	if err != nil {
		return err
	}
	firstRect, splitterRect, secondRect, effective, err := splitRects(
		logical,
		node.Orientation,
		node.Ratio,
		options.SplitterThickness,
		firstMinimum,
		secondMinimum,
	)
	if err != nil {
		return fmt.Errorf("%w: split %q: %v", ErrInvalidGeometry, node.ID, err)
	}
	geometry.Splitters = append(geometry.Splitters, SplitterGeometry{
		SplitID:          node.ID,
		Orientation:      node.Orientation,
		Bounds:           boundsFor(splitterRect, options.DPIScale),
		EffectiveRatio:   effective,
		LogicalThickness: axisLength(splitterRect, node.Orientation),
	})
	if err := appendGeometry(geometry, node.First, firstRect, options, seen); err != nil {
		return err
	}
	return appendGeometry(geometry, node.Second, secondRect, options, seen)
}

func splitRects(
	host Rect,
	orientation appstate.SplitOrientation,
	ratio float64,
	splitterThickness float64,
	firstMinimum Size,
	secondMinimum Size,
) (Rect, Rect, Rect, float64, error) {
	var total, firstMin, secondMin float64
	switch orientation {
	case appstate.SplitHorizontal:
		total = host.Width
		firstMin = firstMinimum.Width
		secondMin = secondMinimum.Width
	case appstate.SplitVertical:
		total = host.Height
		firstMin = firstMinimum.Height
		secondMin = secondMinimum.Height
	default:
		return Rect{}, Rect{}, Rect{}, 0, fmt.Errorf("unknown orientation %q", orientation)
	}
	actualSplitter := math.Min(splitterThickness, total)
	available := math.Max(total-actualSplitter, 0)
	firstLength := allocateFirst(available, firstMin, secondMin, ClampSplitRatio(ratio))
	secondLength := math.Max(available-firstLength, 0)
	effective := ClampSplitRatio(ratio)
	if available > 0 {
		effective = firstLength / available
	}

	switch orientation {
	case appstate.SplitHorizontal:
		first := Rect{X: host.X, Y: host.Y, Width: firstLength, Height: host.Height}
		splitter := Rect{X: host.X + firstLength, Y: host.Y, Width: actualSplitter, Height: host.Height}
		second := Rect{X: splitter.X + actualSplitter, Y: host.Y, Width: secondLength, Height: host.Height}
		return first, splitter, second, effective, nil
	case appstate.SplitVertical:
		first := Rect{X: host.X, Y: host.Y, Width: host.Width, Height: firstLength}
		splitter := Rect{X: host.X, Y: host.Y + firstLength, Width: host.Width, Height: actualSplitter}
		second := Rect{X: host.X, Y: splitter.Y + actualSplitter, Width: host.Width, Height: secondLength}
		return first, splitter, second, effective, nil
	default:
		return Rect{}, Rect{}, Rect{}, 0, fmt.Errorf("unknown orientation %q", orientation)
	}
}

func allocateFirst(available float64, firstMinimum float64, secondMinimum float64, ratio float64) float64 {
	minimumTotal := firstMinimum + secondMinimum
	if available <= 0 {
		return 0
	}
	if minimumTotal <= available {
		return clamp(available*ratio, firstMinimum, available-secondMinimum)
	}
	if minimumTotal > 0 {
		return available * firstMinimum / minimumTotal
	}
	return available * ratio
}

func axisLength(rect Rect, orientation appstate.SplitOrientation) float64 {
	if orientation == appstate.SplitHorizontal {
		return rect.Width
	}
	return rect.Height
}

func boundsFor(logical Rect, scale float64) Bounds {
	return Bounds{
		Logical: logical,
		Physical: Rect{
			X:      logical.X * scale,
			Y:      logical.Y * scale,
			Width:  logical.Width * scale,
			Height: logical.Height * scale,
		},
	}
}

// DockPosition identifies a tab or directional split drop target.
type DockPosition string

const (
	DockLeft   DockPosition = "left"
	DockRight  DockPosition = "right"
	DockTop    DockPosition = "top"
	DockBottom DockPosition = "bottom"
	DockCenter DockPosition = "center"
)

// Valid reports whether the drop position is supported.
func (position DockPosition) Valid() bool {
	switch position {
	case DockLeft, DockRight, DockTop, DockBottom, DockCenter:
		return true
	default:
		return false
	}
}

// DropTargetRect associates a logical rectangle with a dock position.
type DropTargetRect struct {
	Position DockPosition
	Rect     Rect
}

// DropTargetRects returns non-overlapping left, right, top, bottom, and center
// targets using DefaultDropTargetFraction.
func DropTargetRects(host Rect) ([]DropTargetRect, error) {
	return DropTargetRectsWithFraction(host, DefaultDropTargetFraction)
}

// DropTargetRectsWithFraction returns non-overlapping drop targets. Corners
// belong to left or right; top and bottom occupy the middle column.
func DropTargetRectsWithFraction(host Rect, edgeFraction float64) ([]DropTargetRect, error) {
	if !host.valid() {
		return nil, fmt.Errorf("%w: drop host must be finite and non-negative", ErrInvalidGeometry)
	}
	if !finite(edgeFraction) || edgeFraction <= 0 || edgeFraction >= 0.5 {
		return nil, fmt.Errorf("%w: drop edge fraction must be between zero and one half", ErrInvalidGeometry)
	}
	edgeWidth := host.Width * edgeFraction
	edgeHeight := host.Height * edgeFraction
	middleX := host.X + edgeWidth
	middleWidth := math.Max(host.Width-2*edgeWidth, 0)
	middleY := host.Y + edgeHeight
	middleHeight := math.Max(host.Height-2*edgeHeight, 0)
	return []DropTargetRect{
		{Position: DockLeft, Rect: Rect{X: host.X, Y: host.Y, Width: edgeWidth, Height: host.Height}},
		{Position: DockRight, Rect: Rect{X: host.X + host.Width - edgeWidth, Y: host.Y, Width: edgeWidth, Height: host.Height}},
		{Position: DockTop, Rect: Rect{X: middleX, Y: host.Y, Width: middleWidth, Height: edgeHeight}},
		{Position: DockBottom, Rect: Rect{X: middleX, Y: host.Y + host.Height - edgeHeight, Width: middleWidth, Height: edgeHeight}},
		{Position: DockCenter, Rect: Rect{X: middleX, Y: middleY, Width: middleWidth, Height: middleHeight}},
	}, nil
}

// HitTestDropTargets returns the first target containing point. Callers should
// pass the stable order returned by DropTargetRects.
func HitTestDropTargets(targets []DropTargetRect, point Point) (DockPosition, bool) {
	for _, target := range targets {
		if target.Position.Valid() && target.Rect.Contains(point) {
			return target.Position, true
		}
	}
	return "", false
}

// HitTestDropTarget calculates default targets and hit-tests a logical point.
func HitTestDropTarget(host Rect, point Point) (DockPosition, bool, error) {
	targets, err := DropTargetRects(host)
	if err != nil {
		return "", false, err
	}
	position, found := HitTestDropTargets(targets, point)
	return position, found, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func clamp(value float64, minimum float64, maximum float64) float64 {
	return math.Max(minimum, math.Min(value, maximum))
}
