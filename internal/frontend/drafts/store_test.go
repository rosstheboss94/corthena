package drafts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func validDraft(revision uint64) appstate.ExperimentDraft {
	return appstate.ExperimentDraft{
		Revision: revision, Name: "Autosaved baseline", DatasetID: "dataset-1", DatasetRevision: 7, DatasetFingerprint: "fingerprint-7",
		Features: []appstate.FeatureName{"ret_5"}, Target: appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Split:     appstate.SplitSpec{Kind: "walk_forward", TrainBars: 504, ValidationBars: 126, TestBars: 126, PurgeBars: 5},
		Model:     appstate.ModelSpec{Kind: appstate.ModelRandomForest, MaxDepth: 8, MinLeafSamples: 32, EstimatorCount: 100, HistogramBins: 64},
		Portfolio: appstate.PortfolioSpec{LongQuantile: 0.8, ShortQuantile: 0.2, CostBPS: 5}, RequestedCPU: 4,
	}
}

func TestEncodeDecodeStrictRoundTrip(t *testing.T) {
	snapshot := Snapshot{Revision: 3, Draft: validDraft(3)}
	document, err := Encode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(document)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Revision != snapshot.Revision || decoded.Draft.Name != snapshot.Draft.Name {
		t.Fatalf("decoded = %+v", decoded)
	}
	invalid := append([]byte(nil), document[:len(document)-2]...)
	invalid = append(invalid, []byte(",\"unknown\":true}\n")...)
	if !errors.Is(decodeError(invalid), ErrInvalidDocument) {
		t.Fatalf("unknown field error = %v", decodeError(invalid))
	}
}

func TestStoreSaveLoadRevisionAndInvalidRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.json")
	store, err := NewStore(path, appstate.ExperimentDraft{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Save(ctx, Snapshot{Revision: 1, Draft: validDraft(1)}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx)
	if err != nil || loaded.Source != LoadSaved || loaded.Snapshot.Revision != 1 {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	if err := store.Save(ctx, Snapshot{Revision: 0, Draft: validDraft(1)}); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("invalid revision save = %v", err)
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Load(ctx)
	if err != nil || recovered.Source != LoadDefaultInvalid || recovered.QuarantinePath == "" {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
	if _, err := os.Stat(recovered.QuarantinePath); err != nil {
		t.Fatal(err)
	}
}

func decodeError(document []byte) error {
	_, err := Decode(document)
	return err
}
