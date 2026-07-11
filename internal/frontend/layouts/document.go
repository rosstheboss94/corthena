// Package layouts owns the typed, versioned persistence boundary for frontend
// workspace layouts. It does not perform rendering or mutate application state.
package layouts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	// DocumentFormat identifies Corthena layout snapshot envelopes.
	DocumentFormat = "corthena.layout-snapshot"
	legacySchema   = 1
)

var (
	// ErrInvalidDocument identifies malformed or semantically invalid layout JSON.
	ErrInvalidDocument = errors.New("invalid layout document")
	// ErrChecksumMismatch identifies a document whose payload was changed after writing.
	ErrChecksumMismatch = errors.New("layout checksum mismatch")
	// ErrUnsupportedVersion identifies a layout schema without a registered migration.
	ErrUnsupportedVersion = errors.New("unsupported layout schema version")
)

// Snapshot is one immutable, revisioned set of workspace layouts.
type Snapshot struct {
	Revision uint64
	Layouts  []appstate.WorkspaceLayout
}

// Clone returns an independent copy of the snapshot.
func (snapshot Snapshot) Clone() Snapshot {
	clone := Snapshot{Revision: snapshot.Revision}
	if snapshot.Layouts != nil {
		clone.Layouts = make([]appstate.WorkspaceLayout, len(snapshot.Layouts))
		for index, layout := range snapshot.Layouts {
			clone.Layouts[index] = layout.Clone()
		}
	}
	return clone
}

type checksumAlgorithm string

const checksumSHA256 checksumAlgorithm = "sha256"

type checksumWire struct {
	Algorithm checksumAlgorithm `json:"algorithm"`
	Digest    string            `json:"digest"`
}

type envelopeWire struct {
	Format   string          `json:"format"`
	Checksum checksumWire    `json:"checksum"`
	Payload  json.RawMessage `json:"payload"`
}

type payloadProbe struct {
	SchemaVersion int                `json:"schema_version"`
	Workspace     appstate.Workspace `json:"workspace"`
	Layouts       []json.RawMessage  `json:"layouts"`
}

type snapshotPayloadWire struct {
	SchemaVersion int          `json:"schema_version"`
	Revision      uint64       `json:"revision"`
	Layouts       []layoutWire `json:"layouts"`
}

type layoutWire struct {
	Workspace    appstate.Workspace `json:"workspace"`
	Root         json.RawMessage    `json:"root"`
	HiddenPanels []panelWire        `json:"hidden_panels"`
	Maximized    appstate.PanelID   `json:"maximized"`
	LinkGroups   []linkGroupWire    `json:"link_groups"`
}

type nodeKind string

const (
	nodeSplit    nodeKind = "split"
	nodeTabStack nodeKind = "tab_stack"
)

type nodeProbe struct {
	Type nodeKind `json:"type"`
}

type splitNodeWire struct {
	Type        nodeKind                  `json:"type"`
	ID          appstate.DockNodeID       `json:"id"`
	Orientation appstate.SplitOrientation `json:"orientation"`
	Ratio       float64                   `json:"ratio"`
	First       json.RawMessage           `json:"first"`
	Second      json.RawMessage           `json:"second"`
}

type tabStackNodeWire struct {
	Type   nodeKind            `json:"type"`
	ID     appstate.DockNodeID `json:"id"`
	Active appstate.PanelID    `json:"active"`
	Panels []panelWire         `json:"panels"`
}

type panelWire struct {
	ID        appstate.PanelID     `json:"id"`
	Type      appstate.PanelType   `json:"panel_type"`
	Title     string               `json:"title"`
	LinkGroup appstate.LinkGroupID `json:"link_group"`
	Settings  panelSettingsWire    `json:"settings"`
}

type panelSettingsWire struct {
	Pinned  bool          `json:"pinned"`
	Compact bool          `json:"compact"`
	View    panelViewWire `json:"view"`
}

type panelViewWire struct {
	Version         int           `json:"version"`
	CursorRow       int           `json:"cursor_row"`
	SortKey         string        `json:"sort_key"`
	Filter          string        `json:"filter"`
	TimeRange       timeRangeWire `json:"time_range"`
	SelectedColumns []string      `json:"selected_columns"`
}

type linkGroupWire struct {
	ID      appstate.LinkGroupID    `json:"id"`
	Name    string                  `json:"name"`
	Color   appstate.LinkGroupColor `json:"color"`
	Context linkContextWire         `json:"context"`
}

type linkContextWire struct {
	DatasetID    appstate.DatasetID    `json:"dataset_id"`
	Symbols      []appstate.Symbol     `json:"symbols"`
	Interval     appstate.BarInterval  `json:"interval"`
	TimeRange    timeRangeWire         `json:"time_range"`
	ExperimentID appstate.ExperimentID `json:"experiment_id"`
	RunID        appstate.RunID        `json:"run_id"`
	ModelID      appstate.ModelID      `json:"model_id"`
}

type timeRangeWire struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type legacySnapshotPayloadWire struct {
	SchemaVersion int                `json:"schema_version"`
	Revision      uint64             `json:"revision"`
	Layouts       []legacyLayoutWire `json:"layouts"`
}

type legacySinglePayloadWire struct {
	SchemaVersion int                `json:"schema_version"`
	Revision      uint64             `json:"revision"`
	Workspace     appstate.Workspace `json:"workspace"`
	Root          json.RawMessage    `json:"root"`
	HiddenPanels  []legacyPanelWire  `json:"hidden_panels"`
	Maximized     appstate.PanelID   `json:"maximized"`
}

type legacyLayoutWire struct {
	Workspace    appstate.Workspace `json:"workspace"`
	Root         json.RawMessage    `json:"root"`
	HiddenPanels []legacyPanelWire  `json:"hidden_panels"`
	Maximized    appstate.PanelID   `json:"maximized"`
}

type legacyTabStackNodeWire struct {
	Type   nodeKind            `json:"type"`
	ID     appstate.DockNodeID `json:"id"`
	Active appstate.PanelID    `json:"active"`
	Panels []legacyPanelWire   `json:"panels"`
}

type legacyPanelWire struct {
	ID       appstate.PanelID   `json:"id"`
	Type     appstate.PanelType `json:"panel_type"`
	Title    string             `json:"title"`
	Settings panelSettingsWire  `json:"settings"`
}

type decodedDocument struct {
	Snapshot Snapshot
	Migrated bool
}

// Encode validates a snapshot and returns its canonical checksummed JSON form.
func Encode(snapshot Snapshot) ([]byte, error) {
	payload, err := encodePayload(snapshot)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(payload)
	envelope := envelopeWire{
		Format: DocumentFormat,
		Checksum: checksumWire{
			Algorithm: checksumSHA256,
			Digest:    hex.EncodeToString(digest[:]),
		},
		Payload: payload,
	}
	document, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode layout envelope: %w", err)
	}
	return append(document, '\n'), nil
}

// Decode verifies, migrates, and validates a layout snapshot document.
func Decode(document []byte) (Snapshot, error) {
	decoded, err := decodeDocument(document)
	if err != nil {
		return Snapshot{}, err
	}
	return decoded.Snapshot, nil
}

func decodeDocument(document []byte) (decodedDocument, error) {
	if len(document) > maximumDocumentBytes {
		return decodedDocument{}, ErrDocumentTooLarge
	}
	if len(bytes.TrimSpace(document)) == 0 {
		return decodedDocument{}, fmt.Errorf("%w: document is empty", ErrInvalidDocument)
	}
	if err := rejectDuplicateFields(document); err != nil {
		return decodedDocument{}, err
	}
	var probe struct {
		Format        string `json:"format"`
		SchemaVersion int    `json:"schema_version"`
	}
	if err := json.Unmarshal(document, &probe); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: decode top level: %w", ErrInvalidDocument, err)
	}
	if probe.Format != "" {
		return decodeEnvelope(document)
	}
	if probe.SchemaVersion == legacySchema {
		return decodeLegacyPayload(document)
	}
	if probe.SchemaVersion != 0 {
		return decodedDocument{}, fmt.Errorf("%w: schema %d", ErrUnsupportedVersion, probe.SchemaVersion)
	}
	return decodedDocument{}, fmt.Errorf("%w: missing format envelope", ErrInvalidDocument)
}

func decodeEnvelope(document []byte) (decodedDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var envelope envelopeWire
	if err := decoder.Decode(&envelope); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: decode envelope: %w", ErrInvalidDocument, err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return decodedDocument{}, err
	}
	if envelope.Format != DocumentFormat {
		return decodedDocument{}, fmt.Errorf("%w: unknown format %q", ErrInvalidDocument, envelope.Format)
	}
	if envelope.Checksum.Algorithm != checksumSHA256 {
		return decodedDocument{}, fmt.Errorf("%w: unsupported checksum algorithm %q", ErrInvalidDocument, envelope.Checksum.Algorithm)
	}
	want, err := hex.DecodeString(envelope.Checksum.Digest)
	if err != nil || len(want) != sha256.Size {
		return decodedDocument{}, fmt.Errorf("%w: malformed checksum digest", ErrInvalidDocument)
	}
	got := sha256.Sum256(envelope.Payload)
	if !bytes.Equal(got[:], want) {
		return decodedDocument{}, ErrChecksumMismatch
	}
	return decodePayload(envelope.Payload)
}

func decodePayload(payload []byte) (decodedDocument, error) {
	var probe payloadProbe
	if err := json.Unmarshal(payload, &probe); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: decode payload version: %w", ErrInvalidDocument, err)
	}
	switch probe.SchemaVersion {
	case appstate.LayoutSchemaVersion:
		return decodeCurrentPayload(payload)
	case legacySchema:
		return decodeLegacyPayload(payload)
	default:
		return decodedDocument{}, fmt.Errorf("%w: schema %d", ErrUnsupportedVersion, probe.SchemaVersion)
	}
}

func decodeCurrentPayload(payload []byte) (decodedDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var wire snapshotPayloadWire
	if err := decoder.Decode(&wire); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: decode schema %d payload: %w", ErrInvalidDocument, appstate.LayoutSchemaVersion, err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return decodedDocument{}, err
	}
	if wire.SchemaVersion != appstate.LayoutSchemaVersion {
		return decodedDocument{}, fmt.Errorf("%w: schema %d", ErrUnsupportedVersion, wire.SchemaVersion)
	}
	snapshot, err := snapshotFromWire(wire)
	if err != nil {
		return decodedDocument{}, err
	}
	return decodedDocument{Snapshot: snapshot}, nil
}

func decodeLegacyPayload(payload []byte) (decodedDocument, error) {
	var probe payloadProbe
	if err := json.Unmarshal(payload, &probe); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: decode legacy payload: %w", ErrInvalidDocument, err)
	}
	var snapshot Snapshot
	if probe.Workspace != "" {
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		var wire legacySinglePayloadWire
		if err := decoder.Decode(&wire); err != nil {
			return decodedDocument{}, fmt.Errorf("%w: decode legacy single layout: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return decodedDocument{}, err
		}
		if wire.SchemaVersion != legacySchema {
			return decodedDocument{}, fmt.Errorf("%w: schema %d", ErrUnsupportedVersion, wire.SchemaVersion)
		}
		layout, err := migrateLegacyLayout(legacyLayoutWire{
			Workspace:    wire.Workspace,
			Root:         wire.Root,
			HiddenPanels: wire.HiddenPanels,
			Maximized:    wire.Maximized,
		})
		if err != nil {
			return decodedDocument{}, err
		}
		snapshot = Snapshot{Revision: wire.Revision, Layouts: []appstate.WorkspaceLayout{layout}}
	} else {
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		var wire legacySnapshotPayloadWire
		if err := decoder.Decode(&wire); err != nil {
			return decodedDocument{}, fmt.Errorf("%w: decode legacy snapshot: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return decodedDocument{}, err
		}
		if wire.SchemaVersion != legacySchema {
			return decodedDocument{}, fmt.Errorf("%w: schema %d", ErrUnsupportedVersion, wire.SchemaVersion)
		}
		snapshot.Revision = wire.Revision
		snapshot.Layouts = make([]appstate.WorkspaceLayout, len(wire.Layouts))
		for index, layoutWire := range wire.Layouts {
			layout, err := migrateLegacyLayout(layoutWire)
			if err != nil {
				return decodedDocument{}, err
			}
			snapshot.Layouts[index] = layout
		}
	}
	if err := Validate(snapshot); err != nil {
		return decodedDocument{}, fmt.Errorf("%w: migrated schema %d: %w", ErrInvalidDocument, legacySchema, err)
	}
	return decodedDocument{Snapshot: snapshot, Migrated: true}, nil
}

func encodePayload(snapshot Snapshot) ([]byte, error) {
	if err := Validate(snapshot); err != nil {
		return nil, fmt.Errorf("encode layout snapshot: %w", err)
	}
	canonical := snapshot.Clone()
	sort.Slice(canonical.Layouts, func(first, second int) bool {
		return workspaceOrder(canonical.Layouts[first].Workspace) < workspaceOrder(canonical.Layouts[second].Workspace)
	})
	wire := snapshotPayloadWire{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Revision:      canonical.Revision,
		Layouts:       make([]layoutWire, len(canonical.Layouts)),
	}
	for index, layout := range canonical.Layouts {
		encoded, err := layoutToWire(layout)
		if err != nil {
			return nil, err
		}
		wire.Layouts[index] = encoded
	}
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode layout payload: %w", err)
	}
	return payload, nil
}

func snapshotFromWire(wire snapshotPayloadWire) (Snapshot, error) {
	snapshot := Snapshot{
		Revision: wire.Revision,
		Layouts:  make([]appstate.WorkspaceLayout, len(wire.Layouts)),
	}
	for index, encoded := range wire.Layouts {
		layout, err := layoutFromWire(encoded)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Layouts[index] = layout
	}
	if err := Validate(snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate schema %d payload: %w", ErrInvalidDocument, appstate.LayoutSchemaVersion, err)
	}
	return snapshot, nil
}

func layoutToWire(layout appstate.WorkspaceLayout) (layoutWire, error) {
	root, err := encodeNode(layout.Root)
	if err != nil {
		return layoutWire{}, err
	}
	wire := layoutWire{
		Workspace:    layout.Workspace,
		Root:         root,
		HiddenPanels: make([]panelWire, len(layout.HiddenPanels)),
		Maximized:    layout.Maximized,
		LinkGroups:   make([]linkGroupWire, len(layout.LinkGroups)),
	}
	for index, panel := range layout.HiddenPanels {
		wire.HiddenPanels[index] = panelToWire(panel)
	}
	for index, group := range layout.LinkGroups {
		wire.LinkGroups[index] = linkGroupToWire(group)
	}
	return wire, nil
}

func layoutFromWire(wire layoutWire) (appstate.WorkspaceLayout, error) {
	root, err := decodeNode(wire.Root, 0)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	layout := appstate.WorkspaceLayout{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Workspace:     wire.Workspace,
		Root:          root,
		HiddenPanels:  make([]appstate.PanelInstanceState, len(wire.HiddenPanels)),
		Maximized:     wire.Maximized,
		LinkGroups:    make([]appstate.LinkGroup, len(wire.LinkGroups)),
	}
	for index, panel := range wire.HiddenPanels {
		layout.HiddenPanels[index] = panelFromWire(panel)
	}
	for index, group := range wire.LinkGroups {
		layout.LinkGroups[index] = linkGroupFromWire(group)
	}
	return layout, nil
}

func encodeNode(node appstate.DockNode) (json.RawMessage, error) {
	switch node := node.(type) {
	case appstate.SplitNode:
		return encodeSplitNode(node)
	case *appstate.SplitNode:
		if node == nil {
			return nil, fmt.Errorf("%w: nil split node", ErrInvalidDocument)
		}
		return encodeSplitNode(*node)
	case appstate.TabStackNode:
		return encodeTabStackNode(node)
	case *appstate.TabStackNode:
		if node == nil {
			return nil, fmt.Errorf("%w: nil tab-stack node", ErrInvalidDocument)
		}
		return encodeTabStackNode(*node)
	default:
		return nil, fmt.Errorf("%w: unknown dock node variant %T", ErrInvalidDocument, node)
	}
}

func encodeSplitNode(node appstate.SplitNode) (json.RawMessage, error) {
	first, err := encodeNode(node.First)
	if err != nil {
		return nil, err
	}
	second, err := encodeNode(node.Second)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(splitNodeWire{
		Type:        nodeSplit,
		ID:          node.ID,
		Orientation: node.Orientation,
		Ratio:       node.Ratio,
		First:       first,
		Second:      second,
	})
	if err != nil {
		return nil, fmt.Errorf("encode split node %q: %w", node.ID, err)
	}
	return encoded, nil
}

func encodeTabStackNode(node appstate.TabStackNode) (json.RawMessage, error) {
	wire := tabStackNodeWire{
		Type:   nodeTabStack,
		ID:     node.ID,
		Active: node.Active,
		Panels: make([]panelWire, len(node.Panels)),
	}
	for index, panel := range node.Panels {
		wire.Panels[index] = panelToWire(panel)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode tab stack %q: %w", node.ID, err)
	}
	return encoded, nil
}

func decodeNode(encoded []byte, depth int) (appstate.DockNode, error) {
	if depth > maximumDockDepth {
		return nil, fmt.Errorf("%w: dock tree exceeds depth %d", ErrInvalidDocument, maximumDockDepth)
	}
	var probe nodeProbe
	if err := json.Unmarshal(encoded, &probe); err != nil {
		return nil, fmt.Errorf("%w: decode dock node discriminator: %w", ErrInvalidDocument, err)
	}
	switch probe.Type {
	case nodeSplit:
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var wire splitNodeWire
		if err := decoder.Decode(&wire); err != nil {
			return nil, fmt.Errorf("%w: decode split node: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return nil, err
		}
		first, err := decodeNode(wire.First, depth+1)
		if err != nil {
			return nil, err
		}
		second, err := decodeNode(wire.Second, depth+1)
		if err != nil {
			return nil, err
		}
		return appstate.SplitNode{
			ID:          wire.ID,
			Orientation: wire.Orientation,
			Ratio:       wire.Ratio,
			First:       first,
			Second:      second,
		}, nil
	case nodeTabStack:
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var wire tabStackNodeWire
		if err := decoder.Decode(&wire); err != nil {
			return nil, fmt.Errorf("%w: decode tab-stack node: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return nil, err
		}
		node := appstate.TabStackNode{
			ID:     wire.ID,
			Active: wire.Active,
			Panels: make([]appstate.PanelInstanceState, len(wire.Panels)),
		}
		for index, panel := range wire.Panels {
			node.Panels[index] = panelFromWire(panel)
		}
		return node, nil
	default:
		return nil, fmt.Errorf("%w: unknown dock node discriminator %q", ErrInvalidDocument, probe.Type)
	}
}

func migrateLegacyLayout(wire legacyLayoutWire) (appstate.WorkspaceLayout, error) {
	groupID := appstate.LinkGroupID("link-default-" + string(wire.Workspace))
	root, err := decodeLegacyNode(wire.Root, groupID, 0)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	layout := appstate.WorkspaceLayout{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Workspace:     wire.Workspace,
		Root:          root,
		HiddenPanels:  make([]appstate.PanelInstanceState, len(wire.HiddenPanels)),
		Maximized:     wire.Maximized,
		LinkGroups: []appstate.LinkGroup{{
			ID:    groupID,
			Name:  "Default",
			Color: appstate.LinkGroupCyan,
			Context: appstate.LinkContext{
				Interval: appstate.IntervalDaily,
			},
		}},
	}
	for index, panel := range wire.HiddenPanels {
		layout.HiddenPanels[index] = legacyPanelFromWire(panel, groupID)
	}
	return layout, nil
}

func decodeLegacyNode(encoded []byte, groupID appstate.LinkGroupID, depth int) (appstate.DockNode, error) {
	if depth > maximumDockDepth {
		return nil, fmt.Errorf("%w: legacy dock tree exceeds depth %d", ErrInvalidDocument, maximumDockDepth)
	}
	var probe nodeProbe
	if err := json.Unmarshal(encoded, &probe); err != nil {
		return nil, fmt.Errorf("%w: decode legacy dock node discriminator: %w", ErrInvalidDocument, err)
	}
	switch probe.Type {
	case nodeSplit:
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var wire splitNodeWire
		if err := decoder.Decode(&wire); err != nil {
			return nil, fmt.Errorf("%w: decode legacy split node: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return nil, err
		}
		first, err := decodeLegacyNode(wire.First, groupID, depth+1)
		if err != nil {
			return nil, err
		}
		second, err := decodeLegacyNode(wire.Second, groupID, depth+1)
		if err != nil {
			return nil, err
		}
		return appstate.SplitNode{ID: wire.ID, Orientation: wire.Orientation, Ratio: wire.Ratio, First: first, Second: second}, nil
	case nodeTabStack:
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var wire legacyTabStackNodeWire
		if err := decoder.Decode(&wire); err != nil {
			return nil, fmt.Errorf("%w: decode legacy tab stack: %w", ErrInvalidDocument, err)
		}
		if err := requireJSONEnd(decoder); err != nil {
			return nil, err
		}
		active := wire.Active
		if active == "" && len(wire.Panels) > 0 {
			active = wire.Panels[0].ID
		}
		node := appstate.TabStackNode{ID: wire.ID, Active: active, Panels: make([]appstate.PanelInstanceState, len(wire.Panels))}
		for index, panel := range wire.Panels {
			node.Panels[index] = legacyPanelFromWire(panel, groupID)
		}
		return node, nil
	default:
		return nil, fmt.Errorf("%w: unknown legacy dock node discriminator %q", ErrInvalidDocument, probe.Type)
	}
}

func panelToWire(panel appstate.PanelInstanceState) panelWire {
	return panelWire{
		ID:        panel.ID,
		Type:      panel.Type,
		Title:     panel.Title,
		LinkGroup: panel.LinkGroup,
		Settings: panelSettingsWire{
			Pinned:  panel.Settings.Pinned,
			Compact: panel.Settings.Compact,
			View: panelViewWire{
				Version:         panel.Settings.View.Version,
				CursorRow:       panel.Settings.View.CursorRow,
				SortKey:         panel.Settings.View.SortKey,
				Filter:          panel.Settings.View.Filter,
				TimeRange:       timeRangeToWire(panel.Settings.View.TimeRange),
				SelectedColumns: cloneStringsForWire(panel.Settings.View.SelectedColumns),
			},
		},
	}
}

func panelFromWire(wire panelWire) appstate.PanelInstanceState {
	return appstate.PanelInstanceState{
		ID:        wire.ID,
		Type:      wire.Type,
		Title:     wire.Title,
		LinkGroup: wire.LinkGroup,
		Settings:  panelSettingsFromWire(wire.Settings),
	}
}

func legacyPanelFromWire(wire legacyPanelWire, groupID appstate.LinkGroupID) appstate.PanelInstanceState {
	title := wire.Title
	if title == "" {
		if descriptor, err := appstate.PanelDescriptorFor(wire.Type); err == nil {
			title = descriptor.Title
		}
	}
	settings := panelSettingsFromWire(wire.Settings)
	if settings.View.Version == 0 {
		settings.View.Version = 1
	}
	return appstate.PanelInstanceState{
		ID:        wire.ID,
		Type:      wire.Type,
		Title:     title,
		LinkGroup: groupID,
		Settings:  settings,
	}
}

func panelSettingsFromWire(wire panelSettingsWire) appstate.PanelSettings {
	return appstate.PanelSettings{
		Pinned:  wire.Pinned,
		Compact: wire.Compact,
		View: appstate.PanelViewState{
			Version:         wire.View.Version,
			CursorRow:       wire.View.CursorRow,
			SortKey:         wire.View.SortKey,
			Filter:          wire.View.Filter,
			TimeRange:       timeRangeFromWire(wire.View.TimeRange),
			SelectedColumns: cloneStringsForWire(wire.View.SelectedColumns),
		},
	}
}

func linkGroupToWire(group appstate.LinkGroup) linkGroupWire {
	return linkGroupWire{
		ID:    group.ID,
		Name:  group.Name,
		Color: group.Color,
		Context: linkContextWire{
			DatasetID:    group.Context.DatasetID,
			Symbols:      cloneSymbolsForWire(group.Context.Symbols),
			Interval:     group.Context.Interval,
			TimeRange:    timeRangeToWire(group.Context.TimeRange),
			ExperimentID: group.Context.ExperimentID,
			RunID:        group.Context.RunID,
			ModelID:      group.Context.ModelID,
		},
	}
}

func linkGroupFromWire(wire linkGroupWire) appstate.LinkGroup {
	return appstate.LinkGroup{
		ID:    wire.ID,
		Name:  wire.Name,
		Color: wire.Color,
		Context: appstate.LinkContext{
			DatasetID:    wire.Context.DatasetID,
			Symbols:      cloneSymbolsForWire(wire.Context.Symbols),
			Interval:     wire.Context.Interval,
			TimeRange:    timeRangeFromWire(wire.Context.TimeRange),
			ExperimentID: wire.Context.ExperimentID,
			RunID:        wire.Context.RunID,
			ModelID:      wire.Context.ModelID,
		},
	}
}

func timeRangeToWire(timeRange appstate.TimeRange) timeRangeWire {
	return timeRangeWire{Start: timeRange.Start.UTC(), End: timeRange.End.UTC()}
}

func timeRangeFromWire(wire timeRangeWire) appstate.TimeRange {
	return appstate.TimeRange{Start: wire.Start.UTC(), End: wire.End.UTC()}
}

func cloneStringsForWire(input []string) []string {
	output := make([]string, len(input))
	copy(output, input)
	return output
}

func cloneSymbolsForWire(input []appstate.Symbol) []appstate.Symbol {
	output := make([]appstate.Symbol, len(input))
	copy(output, input)
	return output
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: decode trailing data: %w", ErrInvalidDocument, err)
	}
	return fmt.Errorf("%w: trailing JSON value", ErrInvalidDocument)
}

func rejectDuplicateFields(document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	if err := scanJSONValue(decoder); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDocument, err)
	}
	return requireJSONEnd(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		fields := make(map[string]struct{})
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return errors.New("object field name is not a string")
			}
			if _, exists := fields[name]; exists {
				return fmt.Errorf("duplicate object field %q", name)
			}
			fields[name] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("object has invalid closing delimiter")
		}
		return nil
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("array has invalid closing delimiter")
		}
		return nil
	default:
		return fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

func workspaceOrder(workspace appstate.Workspace) int {
	for index, candidate := range appstate.Workspaces() {
		if candidate == workspace {
			return index
		}
	}
	return len(appstate.Workspaces())
}
