package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/png"
	"math"
	"sync"

	backend "hpad-app/internal/app"
	res "hpad-app/internal/res"

	"github.com/wailsapp/wails/v3/pkg/application"
	xdraw "golang.org/x/image/draw"
)

var (
	iconConnectedSource    = res.TrayConnectedIcon
	iconDisconnectedSource = res.TrayDisconnectedIcon
	iconUSBPowerSource     = res.TrayUSBPowerIcon
	iconLowBatterySource   = res.TrayLowBatteryIcon
	iconErrorSource        = res.TrayErrorIcon
)

var (
	appIcon = mustSanitizeAppIcon(res.AppIcon)

	iconConnected    = mustSanitizeIcon(iconConnectedSource)
	iconDisconnected = mustSanitizeIcon(iconDisconnectedSource)
	iconUSBPower     = mustSanitizeIcon(iconUSBPowerSource)
	iconLowBattery   = mustSanitizeIcon(iconLowBatterySource)
	iconError        = mustSanitizeIcon(iconErrorSource)
)

const (
	batteryMinMV          = 3000
	batteryMaxMV          = 4200
	backgroundMinChannel  = 220
	trayIconTargetFill    = 0.96
	appIconAlphaThreshold = 0x60
)

type trayVisualState string

const (
	trayStateConnected    trayVisualState = "connected"
	trayStateDisconnected trayVisualState = "disconnected"
	trayStateUSBPower     trayVisualState = "usb_power"
	trayStateLowBattery   trayVisualState = "lowbattery"
	trayStateError        trayVisualState = "error"
)

type TrayController struct {
	mu              sync.Mutex
	tray            *application.SystemTray
	presentation    trayPresentation
	hasPresentation bool
}

type trayPresentation struct {
	state   trayVisualState
	tooltip string
}

func NewTrayController(tray *application.SystemTray) *TrayController {
	return &TrayController{tray: tray}
}

func DefaultAppIcon() []byte {
	return appIcon
}

func (c *TrayController) Update(status backend.RuntimeStatus) {
	presentation := buildTrayPresentation(status)
	iconChanged, tooltipChanged := c.recordPresentation(presentation)
	if iconChanged {
		c.tray.SetIcon(iconForState(presentation.state))
	}
	if tooltipChanged {
		c.tray.SetTooltip(presentation.tooltip)
	}
}

func (c *TrayController) recordPresentation(next trayPresentation) (iconChanged, tooltipChanged bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasPresentation && c.presentation == next {
		return false, false
	}

	iconChanged = !c.hasPresentation || c.presentation.state != next.state
	tooltipChanged = !c.hasPresentation || c.presentation.tooltip != next.tooltip
	c.presentation = next
	c.hasPresentation = true
	return iconChanged, tooltipChanged
}

func buildTrayPresentation(status backend.RuntimeStatus) trayPresentation {
	state := classifyTrayState(status)
	return trayPresentation{
		state:   state,
		tooltip: buildTrayTooltip(status, state),
	}
}

func iconForState(state trayVisualState) []byte {
	switch state {
	case trayStateConnected:
		return iconConnected
	case trayStateUSBPower:
		return iconUSBPower
	case trayStateLowBattery:
		return iconLowBattery
	case trayStateError:
		return iconError
	default:
		return iconDisconnected
	}
}

func classifyTrayState(status backend.RuntimeStatus) trayVisualState {
	if status.ErrorMessage != "" {
		return trayStateError
	}
	if !isFullyConnected(status.Dashboard) {
		return trayStateDisconnected
	}
	if status.Dashboard.BatteryStatus.USBPowerPresent {
		return trayStateUSBPower
	}
	if isLowBattery(status.Dashboard.BatteryStatus.BatteryMV) {
		return trayStateLowBattery
	}
	return trayStateConnected
}

func buildTrayTooltip(status backend.RuntimeStatus, state trayVisualState) string {
	switch state {
	case trayStateError:
		return "HPAD: Error detected. Check logs in a dev run for details."
	case trayStateUSBPower:
		return fmt.Sprintf("HPAD: USB power present (%s)", status.Dashboard.BatteryStatus.Label)
	case trayStateLowBattery:
		return fmt.Sprintf("HPAD: Low battery (%d%%)", batteryPercentageFromMV(status.Dashboard.BatteryStatus.BatteryMV))
	case trayStateConnected:
		return fmt.Sprintf("HPAD: Connected (%s)", status.Dashboard.BatteryStatus.Label)
	default:
		if status.Dashboard.DongleStatus.State == "connected" {
			return "HPAD: Waiting for macropad"
		}
		return "HPAD: Disconnected"
	}
}

func isFullyConnected(state backend.DashboardState) bool {
	return state.DongleStatus.State == "connected" && state.MacropadStatus.State == "connected"
}

func isLowBattery(batteryMV int) bool {
	if batteryMV <= 0 {
		return false
	}
	return batteryPercentageFromMV(batteryMV) <= 20
}

func batteryPercentageFromMV(batteryMV int) int {
	if batteryMV <= batteryMinMV {
		return 0
	}
	if batteryMV >= batteryMaxMV {
		return 100
	}
	raw := float64(batteryMV-batteryMinMV) * 100 / float64(batteryMaxMV-batteryMinMV)
	return int(math.Round(raw))
}

func mustSanitizeIcon(icon []byte) []byte {
	result, err := sanitizeIcon(icon)
	if err != nil {
		panic(err)
	}
	return result
}

func mustSanitizeAppIcon(icon []byte) []byte {
	result, err := sanitizeAppIcon(icon)
	if err != nil {
		panic(err)
	}
	return result
}

func sanitizeIcon(icon []byte) ([]byte, error) {
	return transformIcon(icon, func(img *image.NRGBA) *image.NRGBA {
		clearBackground(img)
		removeDisconnectedArtifacts(img)
		return fitIconContent(img)
	})
}

func sanitizeAppIcon(icon []byte) ([]byte, error) {
	return transformIcon(icon, func(img *image.NRGBA) *image.NRGBA {
		clearBackground(img)
		removeDisconnectedArtifacts(img)
		return fitAppIconContent(img)
	})
}

func transformIcon(icon []byte, transform func(*image.NRGBA) *image.NRGBA) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	result := image.NewNRGBA(bounds)
	imagedraw.Draw(result, bounds, decoded, bounds.Min, imagedraw.Src)
	if transform != nil {
		if transformed := transform(result); transformed != nil {
			result = transformed
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, result); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func clearBackground(img *image.NRGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	visited := make([]bool, width*height)
	type point struct{ x, y int }
	queue := make([]point, 0, 2*(width+height))

	push := func(x, y int) {
		if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
			return
		}
		index := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
		if visited[index] {
			return
		}
		visited[index] = true
		queue = append(queue, point{x: x, y: y})
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		push(x, bounds.Min.Y)
		push(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		push(bounds.Min.X, y)
		push(bounds.Max.X-1, y)
	}

	for head := 0; head < len(queue); head++ {
		current := queue[head]
		pixel := img.NRGBAAt(current.x, current.y)
		if !isBackgroundPixel(pixel) {
			continue
		}

		img.SetNRGBA(current.x, current.y, color.NRGBA{})
		push(current.x+1, current.y)
		push(current.x-1, current.y)
		push(current.x, current.y+1)
		push(current.x, current.y-1)
	}
}

func removeDisconnectedArtifacts(img *image.NRGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return
	}

	visited := make([]bool, width*height)
	largestComponent := []int(nil)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			index := pixelOffset(bounds, width, x, y)
			if visited[index] || img.NRGBAAt(x, y).A == 0 {
				continue
			}

			component := collectOpaqueComponent(img, bounds, width, height, x, y, visited)
			if len(component) > len(largestComponent) {
				clearPixels(img, largestComponent)
				largestComponent = component
				continue
			}

			clearPixels(img, component)
		}
	}
}

func collectOpaqueComponent(img *image.NRGBA, bounds image.Rectangle, width, height, startX, startY int, visited []bool) []int {
	queue := []int{pixelOffset(bounds, width, startX, startY)}
	visited[queue[0]] = true
	component := make([]int, 0, 64)

	for head := 0; head < len(queue); head++ {
		index := queue[head]
		component = append(component, index)
		x, y := pixelCoordinates(bounds, width, index)

		for deltaY := -1; deltaY <= 1; deltaY++ {
			for deltaX := -1; deltaX <= 1; deltaX++ {
				if deltaX == 0 && deltaY == 0 {
					continue
				}

				nextX := x + deltaX
				nextY := y + deltaY
				if nextX < bounds.Min.X || nextX >= bounds.Max.X || nextY < bounds.Min.Y || nextY >= bounds.Max.Y {
					continue
				}

				nextIndex := pixelOffset(bounds, width, nextX, nextY)
				if visited[nextIndex] || img.NRGBAAt(nextX, nextY).A == 0 {
					continue
				}

				visited[nextIndex] = true
				queue = append(queue, nextIndex)
			}
		}
	}

	return component
}

func clearPixels(img *image.NRGBA, indices []int) {
	if len(indices) == 0 {
		return
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	for _, index := range indices {
		x, y := pixelCoordinates(bounds, width, index)
		img.SetNRGBA(x, y, color.NRGBA{})
	}
}

func pixelOffset(bounds image.Rectangle, width, x, y int) int {
	return (y-bounds.Min.Y)*width + (x - bounds.Min.X)
}

func pixelCoordinates(bounds image.Rectangle, width, index int) (int, int) {
	x := index % width
	y := index / width
	return bounds.Min.X + x, bounds.Min.Y + y
}

func fitIconContent(img *image.NRGBA) *image.NRGBA {
	contentBounds, ok := nonTransparentBounds(img)
	if !ok {
		return img
	}
	return fitIconContentToBounds(img, contentBounds, contentBounds)
}

func fitAppIconContent(img *image.NRGBA) *image.NRGBA {
	sourceBounds, ok := nonTransparentBounds(img)
	if !ok {
		return img
	}

	visualBounds, ok := nonTransparentBoundsAtAlpha(img, appIconAlphaThreshold)
	if !ok {
		return fitIconContentToBounds(img, sourceBounds, sourceBounds)
	}

	return fitIconContentToBounds(img, sourceBounds, visualBounds)
}

func fitIconContentToBounds(img *image.NRGBA, sourceBounds, visualBounds image.Rectangle) *image.NRGBA {
	if sourceBounds.Empty() || visualBounds.Empty() {
		return img
	}

	canvas := img.Bounds()
	scale := math.Min(
		trayIconTargetFill*float64(canvas.Dx())/float64(visualBounds.Dx()),
		trayIconTargetFill*float64(canvas.Dy())/float64(visualBounds.Dy()),
	)
	if scale <= 1 {
		return img
	}

	dstWidth := int(math.Round(float64(sourceBounds.Dx()) * scale))
	dstHeight := int(math.Round(float64(sourceBounds.Dy()) * scale))
	if dstWidth < 1 {
		dstWidth = 1
	}
	if dstHeight < 1 {
		dstHeight = 1
	}

	sourceCenterX, sourceCenterY := rectCenter(sourceBounds)
	visualCenterX, visualCenterY := rectCenter(visualBounds)
	canvasCenterX, canvasCenterY := rectCenter(canvas)

	dstMinX := int(math.Round(canvasCenterX - float64(dstWidth)/2 - (visualCenterX-sourceCenterX)*scale))
	dstMinY := int(math.Round(canvasCenterY - float64(dstHeight)/2 - (visualCenterY-sourceCenterY)*scale))
	fitted := image.NewNRGBA(canvas)
	xdraw.CatmullRom.Scale(
		fitted,
		image.Rect(dstMinX, dstMinY, dstMinX+dstWidth, dstMinY+dstHeight),
		img,
		sourceBounds,
		imagedraw.Over,
		nil,
	)
	return fitted
}

func nonTransparentBounds(img image.Image) (image.Rectangle, bool) {
	return nonTransparentBoundsAtAlpha(img, 1)
}

func nonTransparentBoundsAtAlpha(img image.Image, minAlpha uint8) (image.Rectangle, bool) {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	minAlpha16 := uint32(minAlpha) * 0x101

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < minAlpha16 {
				continue
			}

			if !found {
				minX, minY = x, y
				maxX, maxY = x+1, y+1
				found = true
				continue
			}

			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}

	if !found {
		return image.Rectangle{}, false
	}

	return image.Rect(minX, minY, maxX, maxY), true
}

func rectCenter(rect image.Rectangle) (float64, float64) {
	return (float64(rect.Min.X) + float64(rect.Max.X)) / 2, (float64(rect.Min.Y) + float64(rect.Max.Y)) / 2
}

func isBackgroundPixel(pixel color.NRGBA) bool {
	if pixel.A == 0 {
		return true
	}
	return pixel.R >= backgroundMinChannel && pixel.G >= backgroundMinChannel && pixel.B >= backgroundMinChannel
}
