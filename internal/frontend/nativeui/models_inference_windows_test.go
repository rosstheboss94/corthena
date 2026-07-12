package nativeui

import (
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestModelsAndInferencePanelsUseTypedWorkspaceState(t *testing.T) {
	t.Parallel()
	models := []appstate.PanelType{appstate.PanelModelRegistry, appstate.PanelAliasHistory, appstate.PanelArtifactMetadata, appstate.PanelFeatureImportance, appstate.PanelTreeInspector}
	for _, panel := range models {
		if !isModelsPanel(panel) || isInferencePanel(panel) {
			t.Fatalf("models panel classification for %q", panel)
		}
	}
	inference := []appstate.PanelType{appstate.PanelModelSelector, appstate.PanelInferenceDataset, appstate.PanelRankedScores, appstate.PanelScoreDistribution, appstate.PanelPredictionHistory, appstate.PanelExportStatus}
	for _, panel := range inference {
		if !isInferencePanel(panel) || isModelsPanel(panel) {
			t.Fatalf("inference panel classification for %q", panel)
		}
	}
	if isModelsPanel(appstate.PanelRunBrowser) || isInferencePanel(appstate.PanelRunBrowser) {
		t.Fatal("unrelated panel classified as Phase 9")
	}
}
