// Package nativeui is the only application package permitted to hold Raylib
// and Raygui values. A Window is confined to its captured Windows UI thread.
package nativeui

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/rosstheboss94/corthena/internal/adapters/winthread"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
)

const (
	defaultFontSize = 48
	targetFPS       = 60
)

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type threadGuard interface {
	Check(operation string) error
}

type colorValue struct {
	red   uint8
	green uint8
	blue  uint8
	alpha uint8
}

type rectangle struct {
	x      float32
	y      float32
	width  float32
	height float32
}

type point struct {
	x float32
	y float32
}

type backend interface {
	setWindowFlags(hidden bool)
	initWindow(width int32, height int32, title string)
	isWindowReady() bool
	loadFont(data []byte, size int32) fontHandle
	isFontValid(font fontHandle) bool
	unloadFont(font fontHandle)
	loadImage(data []byte) imageHandle
	captureScreen() imageHandle
	isImageValid(image imageHandle) bool
	imageSize(image imageHandle) (int32, int32)
	imageColors(image imageHandle) []color.RGBA
	unloadImageColors(colors []color.RGBA)
	unloadImage(image imageHandle)
	loadTexture(image imageHandle) textureHandle
	isTextureValid(texture textureHandle) bool
	unloadTexture(texture textureHandle)
	setGUIFont(font fontHandle)
	setTargetFPS(fps int32)
	windowShouldClose() bool
	screenWidth() int32
	screenHeight() int32
	fps() int32
	windowScaleDPI() point
	mousePosition() point
	mouseDelta() point
	leftMousePressed() bool
	leftMouseDown() bool
	leftMouseReleased() bool
	mouseWheelMove() float32
	openCommandPalettePressed() bool
	openSettingsPressed() bool
	increaseUIScalePressed() bool
	decreaseUIScalePressed() bool
	resetUIScalePressed() bool
	escapePressed() bool
	enterPressed() bool
	tabPressed() bool
	shiftDown() bool
	upPressed() bool
	downPressed() bool
	beginDrawing()
	clearBackground(color colorValue)
	drawRectangle(bounds rectangle, color colorValue)
	drawRectangleLines(bounds rectangle, thickness float32, color colorValue)
	drawLine(start point, end point, thickness float32, color colorValue)
	drawCircle(center point, radius float32, color colorValue)
	drawTriangle(first point, second point, third point, color colorValue)
	drawTriangleFan(points []point, color colorValue)
	drawText(font fontHandle, text string, position point, fontSize float32, spacing float32, color colorValue)
	label(bounds rectangle, text string)
	beginScissor(bounds rectangle)
	endScissor()
	endDrawing()
	closeWindow()
}

// Config contains the native window settings admitted by the scaffold.
type Config struct {
	Width    int32
	Height   int32
	Title    string
	Hidden   bool
	FixedFPS int32
}

// Window owns all native window, font, image, texture, and Raygui resources.
// It must not be copied or called from any thread except its captured owner.
type Window struct {
	_             noCopy
	guard         threadGuard
	backend       backend
	windowOpen    bool
	interFont     fontHandle
	interLoaded   bool
	monoFont      fontHandle
	monoLoaded    bool
	iconTexture   textureHandle
	textureLoaded bool
	commandIndex  int
	dockUI        dockUIState
	researchUI    researchUIState
	fixedFPS      int32
	closed        bool
}

// Open captures the calling Windows thread and initializes the native window
// and all bundled resources on that thread.
func Open(config Config, assetSet assets.Set) (*Window, error) {
	owner, err := winthread.Capture()
	if err != nil {
		return nil, err
	}
	return openWithBackend(config, assetSet, owner, raylibBackend{})
}

func openWithBackend(
	config Config,
	assetSet assets.Set,
	guard threadGuard,
	nativeBackend backend,
) (*Window, error) {
	if config.Width <= 0 || config.Height <= 0 {
		return nil, errors.New("open native window: dimensions must be positive")
	}
	if config.Title == "" {
		return nil, errors.New("open native window: title is empty")
	}
	window := &Window{guard: guard, backend: nativeBackend, fixedFPS: config.FixedFPS}

	if err := window.check("configure Raylib window"); err != nil {
		return nil, err
	}
	window.backend.setWindowFlags(config.Hidden)
	if err := window.check("initialize Raylib window"); err != nil {
		return nil, err
	}
	window.backend.initWindow(config.Width, config.Height, config.Title)
	window.windowOpen = true
	if err := window.check("validate Raylib window"); err != nil {
		return nil, window.failOpen(err)
	}
	if !window.backend.isWindowReady() {
		return nil, window.failOpen(errors.New("initialize Raylib window: window is not ready"))
	}

	if err := window.loadFonts(assetSet); err != nil {
		return nil, window.failOpen(err)
	}
	if err := window.loadIconAtlas(assetSet); err != nil {
		return nil, window.failOpen(err)
	}
	if err := window.check("configure Raygui font"); err != nil {
		return nil, window.failOpen(err)
	}
	window.backend.setGUIFont(window.interFont)
	if err := window.check("configure Raylib frame rate"); err != nil {
		return nil, window.failOpen(err)
	}
	window.backend.setTargetFPS(targetFPS)
	return window, nil
}

func (window *Window) loadFonts(assetSet assets.Set) error {
	if err := window.check("load Inter font"); err != nil {
		return err
	}
	window.interFont = window.backend.loadFont(assetSet.InterFont(), defaultFontSize)
	if err := window.check("validate Inter font"); err != nil {
		return err
	}
	if !window.backend.isFontValid(window.interFont) {
		return errors.New("load Inter font: Raylib rejected the embedded font")
	}
	window.interLoaded = true

	if err := window.check("load JetBrains Mono font"); err != nil {
		return err
	}
	window.monoFont = window.backend.loadFont(assetSet.JetBrainsMonoFont(), defaultFontSize)
	if err := window.check("validate JetBrains Mono font"); err != nil {
		return err
	}
	if !window.backend.isFontValid(window.monoFont) {
		return errors.New("load JetBrains Mono font: Raylib rejected the embedded font")
	}
	window.monoLoaded = true
	return nil
}

func (window *Window) loadIconAtlas(assetSet assets.Set) error {
	if err := window.check("load Lucide icon atlas image"); err != nil {
		return err
	}
	image := window.backend.loadImage(assetSet.IconAtlas())
	if err := window.check("validate Lucide icon atlas image"); err != nil {
		return err
	}
	if !window.backend.isImageValid(image) {
		return errors.New("load Lucide icon atlas: Raylib rejected the embedded PNG")
	}
	if err := window.check("load Lucide icon atlas texture"); err != nil {
		return err
	}
	window.iconTexture = window.backend.loadTexture(image)
	if err := window.check("validate Lucide icon atlas texture"); err != nil {
		return err
	}
	textureValid := window.backend.isTextureValid(window.iconTexture)
	if err := window.check("release Lucide icon atlas image"); err != nil {
		return err
	}
	window.backend.unloadImage(image)
	if !textureValid {
		return errors.New("load Lucide icon atlas: Raylib rejected the texture")
	}
	window.textureLoaded = true
	return nil
}

// ShouldClose polls the native close request on the UI thread.
func (window *Window) ShouldClose() (bool, error) {
	if err := window.requireOpen("poll Raylib window"); err != nil {
		return false, err
	}
	if err := window.check("poll Raylib window"); err != nil {
		return false, err
	}
	return window.backend.windowShouldClose(), nil
}

// DrawEmptyFrame draws only the Phase 1 scaffold marker and one Raygui
// control. It intentionally contains no application-shell or domain behavior.
func (window *Window) DrawEmptyFrame() error {
	if err := window.requireOpen("draw empty frame"); err != nil {
		return err
	}
	if err := window.check("begin Raylib drawing"); err != nil {
		return err
	}
	window.backend.beginDrawing()
	if err := window.check("clear Raylib background"); err != nil {
		return err
	}
	window.backend.clearBackground(colorValue{red: 11, green: 13, blue: 16, alpha: 255})
	if err := window.check("draw Raygui scaffold label"); err != nil {
		return err
	}
	window.backend.label(
		rectangle{x: 24, y: 24, width: 280, height: 26},
		"Corthena frontend scaffold",
	)
	if err := window.check("end Raylib drawing"); err != nil {
		return err
	}
	window.backend.endDrawing()
	return nil
}

// CaptureRGBA captures the current framebuffer through Raylib on the owning
// UI thread and returns a native-value-free RGBA copy. File I/O and comparison
// remain the caller's background responsibility.
func (window *Window) CaptureRGBA() (*image.RGBA, error) {
	if err := window.requireOpen("capture Raylib frame"); err != nil {
		return nil, err
	}
	if err := window.check("capture Raylib frame"); err != nil {
		return nil, err
	}
	captured := window.backend.captureScreen()
	if !window.backend.isImageValid(captured) {
		return nil, errors.New("capture Raylib frame: invalid captured image")
	}
	defer window.backend.unloadImage(captured)
	width, height := window.backend.imageSize(captured)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("capture Raylib frame: invalid dimensions %dx%d", width, height)
	}
	colors := window.backend.imageColors(captured)
	defer window.backend.unloadImageColors(colors)
	if len(colors) != int(width*height) {
		return nil, fmt.Errorf("capture Raylib frame: got %d pixels, want %d", len(colors), width*height)
	}
	result := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for index, pixel := range colors {
		result.SetRGBA(index%int(width), index/int(width), pixel)
	}
	return result, nil
}

// Close releases all native resources on the owning UI thread. It is
// idempotent.
func (window *Window) Close() error {
	if window.closed {
		return nil
	}
	if err := window.check("close native window"); err != nil {
		return err
	}
	if err := window.releaseResources(); err != nil {
		return err
	}
	window.closed = true
	return nil
}

func (window *Window) releaseResources() error {
	if window.textureLoaded {
		if err := window.check("unload Lucide icon atlas texture"); err != nil {
			return err
		}
		window.backend.unloadTexture(window.iconTexture)
		window.textureLoaded = false
	}
	if window.monoLoaded {
		if err := window.check("unload JetBrains Mono font"); err != nil {
			return err
		}
		window.backend.unloadFont(window.monoFont)
		window.monoLoaded = false
	}
	if window.interLoaded {
		if err := window.check("unload Inter font"); err != nil {
			return err
		}
		window.backend.unloadFont(window.interFont)
		window.interLoaded = false
	}
	if window.windowOpen {
		if err := window.check("close Raylib window"); err != nil {
			return err
		}
		window.backend.closeWindow()
		window.windowOpen = false
	}
	return nil
}

func (window *Window) failOpen(openErr error) error {
	return errors.Join(openErr, window.releaseResources())
}

func (window *Window) requireOpen(operation string) error {
	if window.closed || !window.windowOpen {
		return fmt.Errorf("%s: native window is closed", operation)
	}
	return nil
}

func (window *Window) check(operation string) error {
	if err := window.guard.Check(operation); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}
