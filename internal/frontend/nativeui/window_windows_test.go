package nativeui

import (
	"errors"
	"image/color"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
)

var errRejectedThread = errors.New("rejected thread")

type allowGuard struct{}

func (allowGuard) Check(string) error {
	return nil
}

type rejectGuard struct{}

func (rejectGuard) Check(string) error {
	return errRejectedThread
}

type mutableGuard struct {
	err error
}

func (guard *mutableGuard) Check(string) error {
	return guard.err
}

type fakeBackend struct {
	calls        []string
	windowReady  bool
	fontValid    bool
	imageValid   bool
	textureValid bool
	shouldClose  bool
	width        int32
	height       int32
	fpsValue     int32
	dpi          point
	mouse        point
	delta        point
	leftPressed  bool
	leftDown     bool
	leftReleased bool
	wheel        float32
	openPalette  bool
	openSettings bool
	zoomIn       bool
	zoomOut      bool
	zoomReset    bool
	escape       bool
	enter        bool
	tab          bool
	shift        bool
	up           bool
	down         bool
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		windowReady:  true,
		fontValid:    true,
		imageValid:   true,
		textureValid: true,
		width:        1280,
		height:       720,
		fpsValue:     60,
		dpi:          point{x: 1, y: 1},
	}
}

func (backend *fakeBackend) record(call string) {
	backend.calls = append(backend.calls, call)
}

func (backend *fakeBackend) setWindowFlags(bool) {
	backend.record("setWindowFlags")
}

func (backend *fakeBackend) initWindow(int32, int32, string) {
	backend.record("initWindow")
}

func (backend *fakeBackend) isWindowReady() bool {
	backend.record("isWindowReady")
	return backend.windowReady
}

func (backend *fakeBackend) loadFont([]byte, int32) fontHandle {
	backend.record("loadFont")
	return fontHandle{}
}

func (backend *fakeBackend) isFontValid(fontHandle) bool {
	backend.record("isFontValid")
	return backend.fontValid
}

func (backend *fakeBackend) unloadFont(fontHandle) {
	backend.record("unloadFont")
}

func (backend *fakeBackend) loadImage([]byte) imageHandle {
	backend.record("loadImage")
	return imageHandle{}
}

func (backend *fakeBackend) captureScreen() imageHandle {
	backend.record("captureScreen")
	return imageHandle{}
}

func (backend *fakeBackend) isImageValid(imageHandle) bool {
	backend.record("isImageValid")
	return backend.imageValid
}

func (backend *fakeBackend) imageSize(imageHandle) (int32, int32) {
	backend.record("imageSize")
	return 1, 1
}

func (backend *fakeBackend) imageColors(imageHandle) []color.RGBA {
	backend.record("imageColors")
	return []color.RGBA{{R: 1, G: 2, B: 3, A: 255}}
}

func (backend *fakeBackend) unloadImageColors([]color.RGBA) {
	backend.record("unloadImageColors")
}

func (backend *fakeBackend) unloadImage(imageHandle) {
	backend.record("unloadImage")
}

func (backend *fakeBackend) loadTexture(imageHandle) textureHandle {
	backend.record("loadTexture")
	return textureHandle{}
}

func (backend *fakeBackend) isTextureValid(textureHandle) bool {
	backend.record("isTextureValid")
	return backend.textureValid
}

func (backend *fakeBackend) unloadTexture(textureHandle) {
	backend.record("unloadTexture")
}

func (backend *fakeBackend) setGUIFont(fontHandle) {
	backend.record("setGUIFont")
}

func (backend *fakeBackend) setTargetFPS(int32) {
	backend.record("setTargetFPS")
}

func (backend *fakeBackend) windowShouldClose() bool {
	backend.record("windowShouldClose")
	return backend.shouldClose
}

func (backend *fakeBackend) screenWidth() int32 {
	backend.record("screenWidth")
	return backend.width
}

func (backend *fakeBackend) screenHeight() int32 {
	backend.record("screenHeight")
	return backend.height
}

func (backend *fakeBackend) fps() int32 {
	backend.record("fps")
	return backend.fpsValue
}

func (backend *fakeBackend) windowScaleDPI() point {
	backend.record("windowScaleDPI")
	return backend.dpi
}

func (backend *fakeBackend) mousePosition() point {
	backend.record("mousePosition")
	return backend.mouse
}

func (backend *fakeBackend) mouseDelta() point {
	backend.record("mouseDelta")
	return backend.delta
}

func (backend *fakeBackend) leftMousePressed() bool {
	backend.record("leftMousePressed")
	return backend.leftPressed
}

func (backend *fakeBackend) leftMouseDown() bool {
	backend.record("leftMouseDown")
	return backend.leftDown
}

func (backend *fakeBackend) leftMouseReleased() bool {
	backend.record("leftMouseReleased")
	return backend.leftReleased
}

func (backend *fakeBackend) mouseWheelMove() float32 {
	backend.record("mouseWheelMove")
	return backend.wheel
}

func (backend *fakeBackend) openCommandPalettePressed() bool {
	backend.record("openCommandPalettePressed")
	return backend.openPalette
}

func (backend *fakeBackend) openSettingsPressed() bool {
	backend.record("openSettingsPressed")
	return backend.openSettings
}

func (backend *fakeBackend) increaseUIScalePressed() bool {
	backend.record("increaseUIScalePressed")
	return backend.zoomIn
}

func (backend *fakeBackend) decreaseUIScalePressed() bool {
	backend.record("decreaseUIScalePressed")
	return backend.zoomOut
}

func (backend *fakeBackend) resetUIScalePressed() bool {
	backend.record("resetUIScalePressed")
	return backend.zoomReset
}

func (backend *fakeBackend) escapePressed() bool {
	backend.record("escapePressed")
	return backend.escape
}

func (backend *fakeBackend) enterPressed() bool {
	backend.record("enterPressed")
	return backend.enter
}

func (backend *fakeBackend) tabPressed() bool {
	backend.record("tabPressed")
	return backend.tab
}

func (backend *fakeBackend) shiftDown() bool {
	backend.record("shiftDown")
	return backend.shift
}

func (backend *fakeBackend) upPressed() bool {
	backend.record("upPressed")
	return backend.up
}

func (backend *fakeBackend) downPressed() bool {
	backend.record("downPressed")
	return backend.down
}

func (backend *fakeBackend) beginDrawing() {
	backend.record("beginDrawing")
}

func (backend *fakeBackend) clearBackground(colorValue) {
	backend.record("clearBackground")
}

func (backend *fakeBackend) drawRectangle(rectangle, colorValue) {
	backend.record("drawRectangle")
}

func (backend *fakeBackend) drawRectangleLines(rectangle, float32, colorValue) {
	backend.record("drawRectangleLines")
}

func (backend *fakeBackend) drawLine(point, point, float32, colorValue) {
	backend.record("drawLine")
}

func (backend *fakeBackend) drawCircle(point, float32, colorValue) {
	backend.record("drawCircle")
}

func (backend *fakeBackend) drawTriangle(point, point, point, colorValue) {
	backend.record("drawTriangle")
}

func (backend *fakeBackend) drawTriangleFan([]point, colorValue) {
	backend.record("drawTriangleFan")
}

func (backend *fakeBackend) drawText(fontHandle, string, point, float32, float32, colorValue) {
	backend.record("drawText")
}

func (backend *fakeBackend) label(rectangle, string) {
	backend.record("label")
}

func (backend *fakeBackend) beginScissor(rectangle) {
	backend.record("beginScissor")
}

func (backend *fakeBackend) endScissor() {
	backend.record("endScissor")
}

func (backend *fakeBackend) endDrawing() {
	backend.record("endDrawing")
}

func (backend *fakeBackend) closeWindow() {
	backend.record("closeWindow")
}

func TestWrongThreadFailsBeforeNativeCall(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	_, err = openWithBackend(testConfig(), assetSet, rejectGuard{}, backend)
	if !errors.Is(err, errRejectedThread) {
		t.Fatalf("error = %v, want rejected thread", err)
	}
	if len(backend.calls) != 0 {
		t.Fatalf("native calls = %v, want none", backend.calls)
	}
}

func TestWrongThreadWindowCallsFailBeforeNativeCall(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	guard := &mutableGuard{}
	backend := newFakeBackend()
	window, err := openWithBackend(testConfig(), assetSet, guard, backend)
	if err != nil {
		t.Fatal(err)
	}

	guard.err = errRejectedThread
	nativeCallCount := len(backend.calls)
	if _, err := window.ShouldClose(); !errors.Is(err, errRejectedThread) {
		t.Fatalf("ShouldClose error = %v, want rejected thread", err)
	}
	if err := window.DrawEmptyFrame(); !errors.Is(err, errRejectedThread) {
		t.Fatalf("DrawEmptyFrame error = %v, want rejected thread", err)
	}
	if _, err := window.DrawShellFrame(testShellState(t)); !errors.Is(err, errRejectedThread) {
		t.Fatalf("DrawShellFrame error = %v, want rejected thread", err)
	}
	if err := window.Close(); !errors.Is(err, errRejectedThread) {
		t.Fatalf("Close error = %v, want rejected thread", err)
	}
	if got := len(backend.calls); got != nativeCallCount {
		t.Fatalf(
			"wrong-thread calls reached native backend: before %d calls, after %d; all calls: %v",
			nativeCallCount,
			got,
			backend.calls,
		)
	}

	guard.err = nil
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowLifecycleReleasesResources(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	shouldClose, err := window.ShouldClose()
	if err != nil {
		t.Fatal(err)
	}
	if shouldClose {
		t.Fatal("window unexpectedly requested close")
	}
	if err := window.DrawEmptyFrame(); err != nil {
		t.Fatal(err)
	}
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}

	wantCounts := map[string]int{
		"unloadTexture": 1,
		"unloadFont":    2,
		"closeWindow":   1,
		"label":         1,
	}
	for call, want := range wantCounts {
		if got := countCalls(backend.calls, call); got != want {
			t.Fatalf("%s calls = %d, want %d; all calls: %v", call, got, want, backend.calls)
		}
	}
}

func TestDrawShellFrameReturnsWorkspaceTabAction(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.mouse = point{x: 150, y: 20}
	backend.leftPressed = true
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	actions, err := window.DrawShellFrame(testShellState(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasWorkspaceAction(actions, appstate.WorkspaceResearch) {
		t.Fatalf("actions = %#v, want research workspace selection", actions)
	}
	if got := countCalls(backend.calls, "beginDrawing"); got != 1 {
		t.Fatalf("beginDrawing calls = %d, want 1", got)
	}
	if got := countCalls(backend.calls, "drawText"); got == 0 {
		t.Fatal("drawText was not called")
	}
}

func TestDrawShellFrameReturnsCommandPaletteActions(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.openPalette = true
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	actions, err := window.DrawShellFrame(testShellState(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasPaletteAction(actions, true) {
		t.Fatalf("actions = %#v, want command palette open action", actions)
	}
}

func TestShellScaleUsesDPIIndependentlyFromResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dpi  point
		want float32
	}{
		{dpi: point{x: 1, y: 1}, want: 1},
		{dpi: point{x: 1.5, y: 1.5}, want: 1.5},
		{dpi: point{x: 2, y: 2}, want: 2},
		{dpi: point{x: 0, y: 0}, want: 1},
		{dpi: point{x: 3, y: 3}, want: 2},
	}
	for _, test := range tests {
		got := shellScale(test.dpi, appstate.UIScale100)
		if got != test.want {
			t.Fatalf("shellScale(%v) = %.2f, want %.2f", test.dpi, got, test.want)
		}
	}
	if got := shellScale(point{x: 1, y: 1}, appstate.UIScale125); got != 1.25 {
		t.Fatalf("default UI scale = %.2f, want 1.25", got)
	}
	if got := shellScale(point{x: 1.5, y: 1.5}, appstate.UIScale150); got != 2 {
		t.Fatalf("clamped UI scale = %.2f, want 2", got)
	}
}

func TestSettingsAndUIScaleShortcutsReturnTypedActions(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.openSettings = true
	backend.zoomIn = true
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	actions, err := window.DrawShellFrame(testShellState(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasSettingsAction(actions, true) || !hasUIScaleAction(actions, appstate.UIScale150) {
		t.Fatalf("shortcut actions = %#v", actions)
	}
}

func TestSettingsModalPresetAndEscapeActions(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.mouse = point{x: 640, y: 370}
	backend.leftPressed = true
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	state := testShellState(t)
	state.Overlays.SettingsOpen = true
	actions, err := window.DrawShellFrame(state)
	if err != nil {
		t.Fatal(err)
	}
	if !hasUIScaleAction(actions, appstate.UIScale150) {
		t.Fatalf("preset actions = %#v", actions)
	}
	backend.leftPressed = false
	backend.escape = true
	actions, err = window.DrawShellFrame(state)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSettingsAction(actions, false) {
		t.Fatalf("escape actions = %#v", actions)
	}
}

func TestSettingsModalResolutionAndScaleMatrix(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	resolutions := []struct {
		width  int32
		height int32
	}{
		{width: 1280, height: 720},
		{width: 1920, height: 1080},
	}
	for _, resolution := range resolutions {
		for _, preset := range appstate.UIScalePresets() {
			backend := newFakeBackend()
			backend.width = resolution.width
			backend.height = resolution.height
			window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
			if err != nil {
				t.Fatal(err)
			}
			state := testShellState(t)
			state.Preferences.UIScale = preset
			state.Overlays.SettingsOpen = true
			if _, err := window.DrawShellFrame(state); err != nil {
				_ = window.Close()
				t.Fatalf("draw %dx%d at %d%%: %v", resolution.width, resolution.height, preset, err)
			}
			scale := shellScale(backend.dpi, preset)
			renderer := shellRenderer{scale: scale}
			bounds := renderer.modalBounds(float32(resolution.width), float32(resolution.height), 620, 350)
			if bounds.x < 0 || bounds.y < 0 || bounds.x+bounds.width > float32(resolution.width) || bounds.y+bounds.height > float32(resolution.height) {
				_ = window.Close()
				t.Fatalf("modal bounds at %dx%d %d%% = %+v", resolution.width, resolution.height, preset, bounds)
			}
			if err := window.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func hasSettingsAction(actions []appstate.UIAction, open bool) bool {
	for _, action := range actions {
		if action, ok := action.(appstate.SetSettingsOpenAction); ok && action.Open == open {
			return true
		}
	}
	return false
}

func hasUIScaleAction(actions []appstate.UIAction, scale appstate.UIScalePreset) bool {
	for _, action := range actions {
		if action, ok := action.(appstate.SetUIScaleAction); ok && action.Scale == scale {
			return true
		}
	}
	return false
}

func TestInitializationFailureClosesWindow(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.windowReady = false
	_, err = openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err == nil {
		t.Fatal("open succeeded with an unready window")
	}
	if got := countCalls(backend.calls, "closeWindow"); got != 1 {
		t.Fatalf("closeWindow calls = %d, want 1; all calls: %v", got, backend.calls)
	}
}

func TestCaptureRGBACopiesPixelsOnOwningThread(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	captured, err := window.CaptureRGBA()
	if err != nil {
		t.Fatal(err)
	}
	if captured.Bounds().Dx() != 1 || captured.Bounds().Dy() != 1 || captured.RGBAAt(0, 0) != (color.RGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatalf("captured = bounds %v pixel %+v", captured.Bounds(), captured.RGBAAt(0, 0))
	}
	if countCalls(backend.calls, "captureScreen") != 1 || countCalls(backend.calls, "unloadImageColors") != 1 {
		t.Fatalf("capture calls = %v", backend.calls)
	}
}

func testConfig() Config {
	return Config{Width: 320, Height: 120, Title: "test", Hidden: true}
}

func countCalls(calls []string, want string) int {
	count := 0
	for _, call := range calls {
		if call == want {
			count++
		}
	}
	return count
}

func testShellState(t *testing.T) appstate.AppState {
	t.Helper()

	state, _, err := appstate.NewInitialState(
		appstate.FixedClock{Time: time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)},
		appstate.NewSequentialIDSource("native"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func hasWorkspaceAction(actions []appstate.UIAction, workspace appstate.Workspace) bool {
	for _, action := range actions {
		selectAction, ok := action.(appstate.SelectWorkspaceAction)
		if ok && selectAction.Workspace == workspace {
			return true
		}
	}
	return false
}

func hasPaletteAction(actions []appstate.UIAction, open bool) bool {
	for _, action := range actions {
		paletteAction, ok := action.(appstate.SetCommandPaletteAction)
		if ok && paletteAction.Open == open {
			return true
		}
	}
	return false
}
