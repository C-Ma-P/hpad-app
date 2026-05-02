package tray

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sync"

	backend "hpad-app/internal/app"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed icons/hpad-connected.png
var iconConnectedSource []byte

//go:embed icons/hpad-disconnected.png
var iconDisconnectedSource []byte

//go:embed icons/hpad-charging.png
var iconChargingSource []byte

//go:embed icons/hpad-low-battery.png
var iconLowBatterySource []byte

//go:embed icons/hpad-error.png
var iconErrorSource []byte

var (
	iconConnected    = mustSanitizeIcon(iconConnectedSource)
	iconDisconnected = mustSanitizeIcon(iconDisconnectedSource)
	iconCharging     = mustSanitizeIcon(iconChargingSource)
	iconLowBattery   = mustSanitizeIcon(iconLowBatterySource)
	iconError        = mustSanitizeIcon(iconErrorSource)
)

const (
	batteryMinMV         = 3000
	batteryMaxMV         = 4200
	backgroundMinChannel = 220
)

type trayVisualState string

const (
	trayStateConnected    trayVisualState = "connected"
	trayStateDisconnected trayVisualState = "disconnected"
	trayStateCharging     trayVisualState = "charging"
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
	return iconConnected
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
	case trayStateCharging:
		return iconCharging
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
	if status.Dashboard.BatteryStatus.Charging {
		return trayStateCharging
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
	case trayStateCharging:
		return fmt.Sprintf("HPAD: Charging (%s)", status.Dashboard.BatteryStatus.Label)
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

func sanitizeIcon(icon []byte) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	result := image.NewNRGBA(bounds)
	draw.Draw(result, bounds, decoded, bounds.Min, draw.Src)
	clearBackground(result)

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

func isBackgroundPixel(pixel color.NRGBA) bool {
	if pixel.A == 0 {
		return true
	}
	return pixel.R >= backgroundMinChannel && pixel.G >= backgroundMinChannel && pixel.B >= backgroundMinChannel
}
