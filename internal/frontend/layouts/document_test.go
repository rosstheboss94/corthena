package layouts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestEncodeDecodeCanonicalRoundTrip(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t, 17)
	snapshot.Layouts[0].LinkGroups[0].Context = appstate.LinkContext{
		DatasetID: "dataset-17",
		Symbols:   []appstate.Symbol{"MSFT", "AAPL"},
		Interval:  appstate.IntervalHourly,
		TimeRange: appstate.TimeRange{
			Start: time.Date(2026, 1, 2, 9, 30, 0, 0, time.FixedZone("test", -5*60*60)),
			End:   time.Date(2026, 1, 2, 16, 0, 0, 0, time.FixedZone("test", -5*60*60)),
		},
	}
	// Research is already a multi-split chart-first layout; split another
	// workspace to keep this fixture exercising arbitrary split trees.
	snapshot.Layouts[2] = splitFirstLayout(t, snapshot.Layouts[2])

	encoded, err := Encode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatalf("canonical round trip changed bytes\nfirst:  %s\nsecond: %s", encoded, reencoded)
	}
	if decoded.Revision != snapshot.Revision {
		t.Fatalf("revision = %d, want %d", decoded.Revision, snapshot.Revision)
	}

	reversed := snapshot.Clone()
	for left, right := 0, len(reversed.Layouts)-1; left < right; left, right = left+1, right-1 {
		reversed.Layouts[left], reversed.Layouts[right] = reversed.Layouts[right], reversed.Layouts[left]
	}
	reversedBytes, err := Encode(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reversedBytes, encoded) {
		t.Fatal("layout input order changed canonical encoding")
	}
}

func TestDecodeRejectsUnknownFieldsTrailingDataAndBadDiscriminators(t *testing.T) {
	t.Parallel()

	encoded, err := Encode(testSnapshot(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	var envelope envelopeWire
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}

	unknownEnvelope := append(bytes.TrimSpace(encoded), []byte("{}")...)
	if _, err := Decode(unknownEnvelope); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("trailing data error = %v, want ErrInvalidDocument", err)
	}

	unknownPayload := append([]byte{}, envelope.Payload[:len(envelope.Payload)-1]...)
	unknownPayload = append(unknownPayload, []byte(`,"unexpected":true}`)...)
	if _, err := Decode(wrapPayload(t, unknownPayload)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unknown payload field error = %v, want ErrInvalidDocument", err)
	}

	var payload snapshotPayloadWire
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	root := payload.Layouts[0].Root
	rootWithUnknown := append([]byte{}, root[:len(root)-1]...)
	rootWithUnknown = append(rootWithUnknown, []byte(`,"unexpected":true}`)...)
	payload.Layouts[0].Root = rootWithUnknown
	unknownNodePayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(wrapPayload(t, unknownNodePayload)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unknown node field error = %v, want ErrInvalidDocument", err)
	}

	payload.Layouts[0].Root = bytes.Replace(root, []byte(`"tab_stack"`), []byte(`"unknown"`), 1)
	badDiscriminatorPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(wrapPayload(t, badDiscriminatorPayload)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("bad discriminator error = %v, want ErrInvalidDocument", err)
	}

	unknownEnvelopeField := append([]byte{}, bytes.TrimSpace(encoded)[:len(bytes.TrimSpace(encoded))-1]...)
	unknownEnvelopeField = append(unknownEnvelopeField, []byte(`,"unexpected":true}`)...)
	if _, err := Decode(unknownEnvelopeField); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unknown envelope field error = %v, want ErrInvalidDocument", err)
	}

	duplicateField := bytes.Replace(bytes.TrimSpace(encoded), []byte(`{"format":`), []byte(`{"format":"duplicate","format":`), 1)
	if _, err := Decode(duplicateField); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("duplicate field error = %v, want ErrInvalidDocument", err)
	}
}

func TestDecodeVerifiesChecksum(t *testing.T) {
	t.Parallel()

	encoded, err := Encode(testSnapshot(t, 29))
	if err != nil {
		t.Fatal(err)
	}
	var envelope envelopeWire
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Payload = bytes.Replace(envelope.Payload, []byte(`"revision":29`), []byte(`"revision":30`), 1)
	tampered, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(tampered); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("tampered error = %v, want ErrChecksumMismatch", err)
	}
}

func TestDecodeMigratesLegacySchemaOne(t *testing.T) {
	t.Parallel()

	original := testSnapshot(t, 41)
	legacyPayload := legacyPayload(t, original)
	for name, document := range map[string][]byte{
		"direct":      legacyPayload,
		"checksummed": wrapPayload(t, legacyPayload),
	} {
		t.Run(name, func(t *testing.T) {
			migrated, err := Decode(document)
			if err != nil {
				t.Fatal(err)
			}
			if migrated.Revision != original.Revision {
				t.Fatalf("revision = %d, want %d", migrated.Revision, original.Revision)
			}
			for _, layout := range migrated.Layouts {
				if layout.SchemaVersion != appstate.LayoutSchemaVersion {
					t.Fatalf("schema = %d, want %d", layout.SchemaVersion, appstate.LayoutSchemaVersion)
				}
				if len(layout.LinkGroups) != 1 {
					t.Fatalf("workspace %q groups = %d, want 1", layout.Workspace, len(layout.LinkGroups))
				}
				group := layout.LinkGroups[0]
				if group.Name != "Default" || group.Color != appstate.LinkGroupCyan || group.Context.Interval != appstate.IntervalDaily {
					t.Fatalf("workspace %q migrated group = %+v", layout.Workspace, group)
				}
				assertPanelGroups(t, layout.Root, group.ID)
				for _, panel := range layout.HiddenPanels {
					if panel.LinkGroup != group.ID {
						t.Fatalf("hidden panel %q group = %q, want %q", panel.ID, panel.LinkGroup, group.ID)
					}
				}
			}
			if _, err := Encode(migrated); err != nil {
				t.Fatalf("encode migrated snapshot: %v", err)
			}
		})
	}
}

func TestValidateRejectsBrokenReferencesAndGeometry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{
			name: "duplicate workspace",
			mutate: func(snapshot *Snapshot) {
				snapshot.Layouts = append(snapshot.Layouts, snapshot.Layouts[0].Clone())
			},
		},
		{
			name: "missing link group",
			mutate: func(snapshot *Snapshot) {
				stack := requireTabStack(t, snapshot.Layouts[0].Root)
				stack.Panels[0].LinkGroup = "absent"
				snapshot.Layouts[0].Root = stack
			},
		},
		{
			name: "active panel absent",
			mutate: func(snapshot *Snapshot) {
				stack := requireTabStack(t, snapshot.Layouts[0].Root)
				stack.Active = "absent"
				snapshot.Layouts[0].Root = stack
			},
		},
		{
			name: "invalid ratio",
			mutate: func(snapshot *Snapshot) {
				snapshot.Layouts[0] = splitFirstLayout(t, snapshot.Layouts[0])
				split := requireSplit(t, snapshot.Layouts[0].Root)
				split.Ratio = 1
				snapshot.Layouts[0].Root = split
			},
		},
		{
			name: "unknown color",
			mutate: func(snapshot *Snapshot) {
				snapshot.Layouts[0].LinkGroups[0].Color = "infrared"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := testSnapshot(t, 1)
			test.mutate(&snapshot)
			if err := Validate(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("Validate error = %v, want ErrInvalidSnapshot", err)
			}
		})
	}
}

func TestSnapshotCloneIsIndependent(t *testing.T) {
	t.Parallel()

	original := testSnapshot(t, 2)
	clone := original.Clone()
	clone.Layouts[0].LinkGroups[0].Name = "Changed"
	stack := requireTabStack(t, clone.Layouts[0].Root)
	stack.Panels[0].Title = "Changed"
	clone.Layouts[0].Root = stack
	if original.Layouts[0].LinkGroups[0].Name == "Changed" {
		t.Fatal("link groups alias after Clone")
	}
	if requireTabStack(t, original.Layouts[0].Root).Panels[0].Title == "Changed" {
		t.Fatal("dock panels alias after Clone")
	}
}

func TestValidateAllowsOnlyAnEmptyRootStack(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t, 9)
	stack := requireTabStack(t, snapshot.Layouts[0].Root)
	snapshot.Layouts[0].HiddenPanels = append(snapshot.Layouts[0].HiddenPanels, stack.Panels...)
	snapshot.Layouts[0].Root = appstate.TabStackNode{ID: stack.ID}
	if err := Validate(snapshot); err != nil {
		t.Fatalf("Validate empty root: %v", err)
	}

	snapshot.Layouts[0].Root = appstate.SplitNode{
		ID:          "split-empty-child",
		Orientation: appstate.SplitHorizontal,
		Ratio:       minimumSplitRatio,
		First:       appstate.TabStackNode{ID: "empty-child"},
		Second: appstate.TabStackNode{
			ID:     "nonempty-child",
			Active: stack.Panels[0].ID,
			Panels: []appstate.PanelInstanceState{stack.Panels[0]},
		},
	}
	snapshot.Layouts[0].HiddenPanels = append([]appstate.PanelInstanceState(nil), stack.Panels[1:]...)
	if err := Validate(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Validate empty child error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestValidateAcceptsSplitRatioBounds(t *testing.T) {
	t.Parallel()

	for _, ratio := range []float64{minimumSplitRatio, maximumSplitRatio} {
		snapshot := testSnapshot(t, 10)
		snapshot.Layouts[0] = splitFirstLayout(t, snapshot.Layouts[0])
		split := requireSplit(t, snapshot.Layouts[0].Root)
		split.Ratio = ratio
		snapshot.Layouts[0].Root = split
		if err := Validate(snapshot); err != nil {
			t.Fatalf("Validate ratio %v: %v", ratio, err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	snapshot := fuzzSnapshot()
	encoded, err := Encode(snapshot)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add(legacyPayloadForFuzz(snapshot))
	f.Add([]byte(`{"schema_version":99}`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, document []byte) {
		decoded, err := Decode(document)
		if err != nil {
			return
		}
		canonical, err := Encode(decoded)
		if err != nil {
			t.Fatalf("Encode accepted Decode result: %v", err)
		}
		roundTrip, err := Decode(canonical)
		if err != nil {
			t.Fatalf("Decode canonical result: %v", err)
		}
		again, err := Encode(roundTrip)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(canonical, again) {
			t.Fatal("canonical encoding is not stable")
		}
	})
}

func testSnapshot(t testing.TB, revision uint64) Snapshot {
	t.Helper()
	ids := appstate.NewSequentialIDSource("layout-test")
	layouts := make([]appstate.WorkspaceLayout, 0, len(appstate.Workspaces()))
	for _, workspace := range appstate.Workspaces() {
		layout, err := appstate.DefaultWorkspaceLayout(workspace, ids)
		if err != nil {
			t.Fatal(err)
		}
		layouts = append(layouts, layout)
	}
	return Snapshot{Revision: revision, Layouts: layouts}
}

func fuzzSnapshot() Snapshot {
	ids := appstate.NewSequentialIDSource("layout-fuzz")
	layout, err := appstate.DefaultWorkspaceLayout(appstate.WorkspaceResearch, ids)
	if err != nil {
		panic(err)
	}
	return Snapshot{Revision: 7, Layouts: []appstate.WorkspaceLayout{layout}}
}

func splitFirstLayout(t testing.TB, layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
	t.Helper()
	stack := requireTabStack(t, layout.Root)
	if len(stack.Panels) < 2 {
		t.Fatal("test layout needs at least two panels")
	}
	firstPanels := append([]appstate.PanelInstanceState(nil), stack.Panels[:1]...)
	secondPanels := append([]appstate.PanelInstanceState(nil), stack.Panels[1:]...)
	layout.Root = appstate.SplitNode{
		ID:          appstate.DockNodeID("split-" + string(layout.Workspace)),
		Orientation: appstate.SplitHorizontal,
		Ratio:       0.375,
		First: appstate.TabStackNode{
			ID:     appstate.DockNodeID("stack-first-" + string(layout.Workspace)),
			Active: firstPanels[0].ID,
			Panels: firstPanels,
		},
		Second: appstate.TabStackNode{
			ID:     appstate.DockNodeID("stack-second-" + string(layout.Workspace)),
			Active: secondPanels[0].ID,
			Panels: secondPanels,
		},
	}
	return layout
}

func requireTabStack(t testing.TB, node appstate.DockNode) appstate.TabStackNode {
	t.Helper()
	switch node := node.(type) {
	case appstate.TabStackNode:
		return node
	case *appstate.TabStackNode:
		if node != nil {
			return *node
		}
	}
	t.Fatalf("node = %T, want TabStackNode", node)
	return appstate.TabStackNode{}
}

func requireSplit(t testing.TB, node appstate.DockNode) appstate.SplitNode {
	t.Helper()
	switch node := node.(type) {
	case appstate.SplitNode:
		return node
	case *appstate.SplitNode:
		if node != nil {
			return *node
		}
	}
	t.Fatalf("node = %T, want SplitNode", node)
	return appstate.SplitNode{}
}

func wrapPayload(t testing.TB, payload []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(payload)
	envelope := envelopeWire{
		Format: DocumentFormat,
		Checksum: checksumWire{
			Algorithm: checksumSHA256,
			Digest:    hex.EncodeToString(digest[:]),
		},
		Payload: payload,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func legacyPayload(t testing.TB, snapshot Snapshot) []byte {
	t.Helper()
	encoded, err := encodeLegacyPayload(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func legacyPayloadForFuzz(snapshot Snapshot) []byte {
	encoded, err := encodeLegacyPayload(snapshot)
	if err != nil {
		panic(err)
	}
	return encoded
}

func encodeLegacyPayload(snapshot Snapshot) ([]byte, error) {
	wire := legacySnapshotPayloadWire{
		SchemaVersion: legacySchema,
		Revision:      snapshot.Revision,
		Layouts:       make([]legacyLayoutWire, len(snapshot.Layouts)),
	}
	for index, layout := range snapshot.Layouts {
		root, err := encodeLegacyNode(layout.Root)
		if err != nil {
			return nil, err
		}
		wire.Layouts[index] = legacyLayoutWire{
			Workspace:    layout.Workspace,
			Root:         root,
			HiddenPanels: make([]legacyPanelWire, len(layout.HiddenPanels)),
			Maximized:    layout.Maximized,
		}
		for panelIndex, panel := range layout.HiddenPanels {
			wire.Layouts[index].HiddenPanels[panelIndex] = legacyPanelFromState(panel)
		}
	}
	return json.Marshal(wire)
}

func encodeLegacyNode(node appstate.DockNode) (json.RawMessage, error) {
	switch node := node.(type) {
	case appstate.SplitNode:
		first, err := encodeLegacyNode(node.First)
		if err != nil {
			return nil, err
		}
		second, err := encodeLegacyNode(node.Second)
		if err != nil {
			return nil, err
		}
		return json.Marshal(splitNodeWire{Type: nodeSplit, ID: node.ID, Orientation: node.Orientation, Ratio: node.Ratio, First: first, Second: second})
	case appstate.TabStackNode:
		wire := legacyTabStackNodeWire{Type: nodeTabStack, ID: node.ID, Active: node.Active, Panels: make([]legacyPanelWire, len(node.Panels))}
		for index, panel := range node.Panels {
			wire.Panels[index] = legacyPanelFromState(panel)
		}
		return json.Marshal(wire)
	case *appstate.SplitNode:
		if node == nil {
			return nil, errors.New("nil split")
		}
		return encodeLegacyNode(*node)
	case *appstate.TabStackNode:
		if node == nil {
			return nil, errors.New("nil tab stack")
		}
		return encodeLegacyNode(*node)
	default:
		return nil, errors.New("unknown node")
	}
}

func legacyPanelFromState(panel appstate.PanelInstanceState) legacyPanelWire {
	return legacyPanelWire{
		ID:       panel.ID,
		Type:     panel.Type,
		Title:    panel.Title,
		Settings: panelToWire(panel).Settings,
	}
}

func assertPanelGroups(t testing.TB, node appstate.DockNode, groupID appstate.LinkGroupID) {
	t.Helper()
	switch node := node.(type) {
	case appstate.SplitNode:
		assertPanelGroups(t, node.First, groupID)
		assertPanelGroups(t, node.Second, groupID)
	case appstate.TabStackNode:
		for _, panel := range node.Panels {
			if panel.LinkGroup != groupID {
				t.Fatalf("panel %q group = %q, want %q", panel.ID, panel.LinkGroup, groupID)
			}
		}
	case *appstate.SplitNode:
		if node == nil {
			t.Fatal("nil split")
			return
		}
		assertPanelGroups(t, *node, groupID)
	case *appstate.TabStackNode:
		if node == nil {
			t.Fatal("nil tab stack")
			return
		}
		assertPanelGroups(t, *node, groupID)
	default:
		t.Fatalf("unknown node %T", node)
	}
}
