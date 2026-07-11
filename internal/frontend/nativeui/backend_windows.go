package nativeui

import (
	"image/color"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type fontHandle struct {
	value rl.Font
}

type imageHandle struct {
	value *rl.Image
}

type textureHandle struct {
	value rl.Texture2D
}

type raylibBackend struct{}

func (raylibBackend) setWindowFlags(hidden bool) {
	flags := uint32(rl.FlagWindowResizable | rl.FlagVsyncHint)
	if hidden {
		flags |= rl.FlagWindowHidden
	}
	rl.SetConfigFlags(flags)
}

func (raylibBackend) initWindow(width int32, height int32, title string) {
	rl.InitWindow(width, height, title)
}

func (raylibBackend) isWindowReady() bool {
	return rl.IsWindowReady()
}

func (raylibBackend) loadFont(data []byte, size int32) fontHandle {
	return fontHandle{value: rl.LoadFontFromMemory(".ttf", data, size, nil)}
}

func (raylibBackend) isFontValid(font fontHandle) bool {
	return rl.IsFontValid(font.value)
}

func (raylibBackend) unloadFont(font fontHandle) {
	rl.UnloadFont(font.value)
}

func (raylibBackend) loadImage(data []byte) imageHandle {
	return imageHandle{value: rl.LoadImageFromMemory(".png", data, int32(len(data)))}
}

func (raylibBackend) captureScreen() imageHandle {
	return imageHandle{value: rl.LoadImageFromScreen()}
}

func (raylibBackend) isImageValid(image imageHandle) bool {
	return image.value != nil && rl.IsImageValid(image.value)
}

func (raylibBackend) imageSize(image imageHandle) (int32, int32) {
	if image.value == nil {
		return 0, 0
	}
	return image.value.Width, image.value.Height
}

func (raylibBackend) imageColors(image imageHandle) []color.RGBA {
	return rl.LoadImageColors(image.value)
}

func (raylibBackend) unloadImageColors(colors []color.RGBA) {
	rl.UnloadImageColors(colors)
}

func (raylibBackend) unloadImage(image imageHandle) {
	rl.UnloadImage(image.value)
}

func (raylibBackend) loadTexture(image imageHandle) textureHandle {
	return textureHandle{value: rl.LoadTextureFromImage(image.value)}
}

func (raylibBackend) isTextureValid(texture textureHandle) bool {
	return rl.IsTextureValid(texture.value)
}

func (raylibBackend) unloadTexture(texture textureHandle) {
	rl.UnloadTexture(texture.value)
}

func (raylibBackend) setGUIFont(font fontHandle) {
	gui.SetFont(font.value)
}

func (raylibBackend) setTargetFPS(fps int32) {
	rl.SetTargetFPS(fps)
}

func (raylibBackend) windowShouldClose() bool {
	return rl.WindowShouldClose()
}

func (raylibBackend) screenWidth() int32 {
	return int32(rl.GetScreenWidth())
}

func (raylibBackend) screenHeight() int32 {
	return int32(rl.GetScreenHeight())
}

func (raylibBackend) fps() int32 {
	return int32(rl.GetFPS())
}

func (raylibBackend) windowScaleDPI() point {
	scale := rl.GetWindowScaleDPI()
	return point{x: scale.X, y: scale.Y}
}

func (raylibBackend) mousePosition() point {
	position := rl.GetMousePosition()
	return point{x: position.X, y: position.Y}
}

func (raylibBackend) mouseDelta() point {
	delta := rl.GetMouseDelta()
	return point{x: delta.X, y: delta.Y}
}

func (raylibBackend) leftMousePressed() bool {
	return rl.IsMouseButtonPressed(rl.MouseLeftButton)
}

func (raylibBackend) leftMouseDown() bool {
	return rl.IsMouseButtonDown(rl.MouseLeftButton)
}

func (raylibBackend) leftMouseReleased() bool {
	return rl.IsMouseButtonReleased(rl.MouseLeftButton)
}

func (raylibBackend) mouseWheelMove() float32 {
	return rl.GetMouseWheelMove()
}

func (raylibBackend) openCommandPalettePressed() bool {
	return controlKeyDown() && (rl.IsKeyPressed(rl.KeyK) || rl.IsKeyPressed(rl.KeyP))
}

func (raylibBackend) openSettingsPressed() bool {
	return controlKeyDown() && rl.IsKeyPressed(rl.KeyComma)
}

func (raylibBackend) increaseUIScalePressed() bool {
	return controlKeyDown() && (rl.IsKeyPressed(rl.KeyEqual) || rl.IsKeyPressed(rl.KeyKpAdd))
}

func (raylibBackend) decreaseUIScalePressed() bool {
	return controlKeyDown() && (rl.IsKeyPressed(rl.KeyMinus) || rl.IsKeyPressed(rl.KeyKpSubtract))
}

func (raylibBackend) resetUIScalePressed() bool {
	return controlKeyDown() && (rl.IsKeyPressed(rl.KeyZero) || rl.IsKeyPressed(rl.KeyKp0))
}

func (raylibBackend) escapePressed() bool {
	return rl.IsKeyPressed(rl.KeyEscape)
}

func (raylibBackend) enterPressed() bool {
	return rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter)
}

func (raylibBackend) tabPressed() bool {
	return rl.IsKeyPressed(rl.KeyTab)
}

func (raylibBackend) shiftDown() bool {
	return rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
}

func (raylibBackend) upPressed() bool {
	return rl.IsKeyPressed(rl.KeyUp)
}

func (raylibBackend) downPressed() bool {
	return rl.IsKeyPressed(rl.KeyDown)
}

func controlKeyDown() bool {
	return rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
}

func (raylibBackend) beginDrawing() {
	rl.BeginDrawing()
}

func (raylibBackend) clearBackground(value colorValue) {
	rl.ClearBackground(toNativeColor(value))
}

func (raylibBackend) drawRectangle(bounds rectangle, value colorValue) {
	rl.DrawRectangleRec(toNativeRectangle(bounds), toNativeColor(value))
}

func (raylibBackend) drawRectangleLines(bounds rectangle, thickness float32, value colorValue) {
	rl.DrawRectangleLinesEx(toNativeRectangle(bounds), thickness, toNativeColor(value))
}

func (raylibBackend) drawLine(start point, end point, thickness float32, value colorValue) {
	rl.DrawLineEx(toNativeVector(start), toNativeVector(end), thickness, toNativeColor(value))
}

func (raylibBackend) drawCircle(center point, radius float32, value colorValue) {
	rl.DrawCircleV(toNativeVector(center), radius, toNativeColor(value))
}

func (raylibBackend) drawTriangle(first point, second point, third point, value colorValue) {
	rl.DrawTriangle(toNativeVector(first), toNativeVector(second), toNativeVector(third), toNativeColor(value))
}

func (raylibBackend) drawTriangleFan(points []point, value colorValue) {
	native := make([]rl.Vector2, len(points))
	for index, item := range points {
		native[index] = toNativeVector(item)
	}
	rl.DrawTriangleFan(native, toNativeColor(value))
}

func (raylibBackend) drawText(
	font fontHandle,
	text string,
	position point,
	fontSize float32,
	spacing float32,
	value colorValue,
) {
	rl.DrawTextEx(font.value, text, toNativeVector(position), fontSize, spacing, toNativeColor(value))
}

func (raylibBackend) label(bounds rectangle, text string) {
	gui.Label(toNativeRectangle(bounds), text)
}

func (raylibBackend) beginScissor(bounds rectangle) {
	rl.BeginScissorMode(
		int32(bounds.x),
		int32(bounds.y),
		int32(bounds.width),
		int32(bounds.height),
	)
}

func (raylibBackend) endScissor() {
	rl.EndScissorMode()
}

func (raylibBackend) endDrawing() {
	rl.EndDrawing()
}

func (raylibBackend) closeWindow() {
	rl.CloseWindow()
}

func toNativeColor(value colorValue) color.RGBA {
	return color.RGBA{
		R: value.red,
		G: value.green,
		B: value.blue,
		A: value.alpha,
	}
}

func toNativeRectangle(bounds rectangle) rl.Rectangle {
	return rl.Rectangle{
		X:      bounds.x,
		Y:      bounds.y,
		Width:  bounds.width,
		Height: bounds.height,
	}
}

func toNativeVector(value point) rl.Vector2 {
	return rl.Vector2{X: value.x, Y: value.y}
}
