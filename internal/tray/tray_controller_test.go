package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	backend "hpad-app/internal/app"
)

func TestClassifyTrayState(t *testing.T) {
	tests := []struct {
		name   string
		status backend.RuntimeStatus
		want   trayVisualState
	}{
		{
			name: "error wins over connection state",
			status: backend.RuntimeStatus{
				Dashboard:    connectedDashboardState(3900, false),
				ErrorMessage: "HID enumerate failed",
			},
			want: trayStateError,
		},
		{
			name:   "usb power wins over low battery",
			status: backend.RuntimeStatus{Dashboard: connectedDashboardState(3200, true)},
			want:   trayStateUSBPower,
		},
		{
			name:   "low battery when connected at threshold",
			status: backend.RuntimeStatus{Dashboard: connectedDashboardState(3240, false)},
			want:   trayStateLowBattery,
		},
		{
			name:   "connected when both devices are online and battery is healthy",
			status: backend.RuntimeStatus{Dashboard: connectedDashboardState(3900, false)},
			want:   trayStateConnected,
		},
		{
			name:   "disconnected when only the dongle is connected",
			status: backend.RuntimeStatus{Dashboard: dongleOnlyDashboardState()},
			want:   trayStateDisconnected,
		},
		{
			name:   "disconnected when nothing is connected",
			status: backend.RuntimeStatus{Dashboard: disconnectedDashboardState()},
			want:   trayStateDisconnected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyTrayState(test.status); got != test.want {
				t.Fatalf("classifyTrayState() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBatteryPercentageFromMV(t *testing.T) {
	tests := []struct {
		mv   int
		want int
	}{
		{mv: 2800, want: 0},
		{mv: 3000, want: 0},
		{mv: 3240, want: 20},
		{mv: 3600, want: 50},
		{mv: 4200, want: 100},
		{mv: 4350, want: 100},
	}

	for _, test := range tests {
		if got := batteryPercentageFromMV(test.mv); got != test.want {
			t.Fatalf("batteryPercentageFromMV(%d) = %d, want %d", test.mv, got, test.want)
		}
	}
}

func TestSanitizeIconClearsOnlyBackground(t *testing.T) {
	sanitized, err := sanitizeIcon(iconConnectedSource)
	if err != nil {
		t.Fatalf("sanitizeIcon() error = %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}

	backgroundPixel := decoded.At(0, 0)
	_, _, _, backgroundAlpha := backgroundPixel.RGBA()
	if backgroundAlpha != 0 {
		t.Fatalf("background alpha = %d, want 0", backgroundAlpha)
	}

	contentBounds, ok := nonTransparentBounds(decoded)
	if !ok {
		t.Fatal("expected sanitized icon to retain visible content")
	}

	widthFill := float64(contentBounds.Dx()) / float64(decoded.Bounds().Dx())
	heightFill := float64(contentBounds.Dy()) / float64(decoded.Bounds().Dy())
	dominantFill := widthFill
	if heightFill > dominantFill {
		dominantFill = heightFill
	}
	if dominantFill < trayIconTargetFill-0.02 || widthFill < 0.90 || heightFill < 0.90 {
		t.Fatalf("sanitized icon still leaves too much padding: width fill=%.2f height fill=%.2f", widthFill, heightFill)
	}

	interiorPixel := decoded.At(450, 450)
	_, _, _, interiorAlpha := interiorPixel.RGBA()
	if interiorAlpha != 0xffff {
		t.Fatalf("interior alpha = %d, want %d", interiorAlpha, 0xffff)
	}
	red, green, blue, _ := interiorPixel.RGBA()
	if red <= green/2 || red <= blue/2 {
		t.Fatalf("interior pixel was unexpectedly altered: got (%d,%d,%d)", red, green, blue)
	}
	if red < 0xd000 || green < 0xd000 || blue < 0xd000 {
		t.Fatalf("interior pixel lost its light fill: got (%d,%d,%d)", red, green, blue)
	}
}

func TestDefaultAppIconKeepsTransparentBackground(t *testing.T) {
	decoded, err := png.Decode(bytes.NewReader(DefaultAppIcon()))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}

	backgroundPixel := decoded.At(0, 0)
	_, _, _, backgroundAlpha := backgroundPixel.RGBA()
	if backgroundAlpha != 0 {
		t.Fatalf("background alpha = %d, want 0", backgroundAlpha)
	}

	contentBounds, ok := nonTransparentBoundsAtAlpha(decoded, appIconAlphaThreshold)
	if !ok {
		t.Fatal("expected app icon to retain visible content")
	}

	widthFill := float64(contentBounds.Dx()) / float64(decoded.Bounds().Dx())
	heightFill := float64(contentBounds.Dy()) / float64(decoded.Bounds().Dy())
	dominantFill := widthFill
	if heightFill > dominantFill {
		dominantFill = heightFill
	}
	if dominantFill < trayIconTargetFill-0.02 || widthFill < 0.89 || heightFill < 0.90 {
		t.Fatalf("app icon still leaves too much padding: width fill=%.2f height fill=%.2f", widthFill, heightFill)
	}

	leftMargin := contentBounds.Min.X - decoded.Bounds().Min.X
	rightMargin := decoded.Bounds().Max.X - contentBounds.Max.X
	topMargin := contentBounds.Min.Y - decoded.Bounds().Min.Y
	bottomMargin := decoded.Bounds().Max.Y - contentBounds.Max.Y
	if absInt(leftMargin-rightMargin) > 3 || absInt(topMargin-bottomMargin) > 3 {
		t.Fatalf("app icon visual bounds are off-center: left=%d right=%d top=%d bottom=%d", leftMargin, rightMargin, topMargin, bottomMargin)
	}
}

func TestRemoveDisconnectedArtifactsKeepsLargestComponent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 2; y <= 5; y++ {
		for x := 2; x <= 5; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 20, G: 20, B: 20, A: 0xff})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 0xff})
	img.SetNRGBA(7, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 0xff})

	removeDisconnectedArtifacts(img)

	if img.NRGBAAt(0, 0).A != 0 || img.NRGBAAt(7, 1).A != 0 {
		t.Fatal("expected disconnected artifacts to be cleared")
	}

	for y := 2; y <= 5; y++ {
		for x := 2; x <= 5; x++ {
			if img.NRGBAAt(x, y).A == 0 {
				t.Fatalf("expected largest component pixel (%d,%d) to remain", x, y)
			}
		}
	}
}

func TestTrayControllerRecordPresentationDedupesRenderedState(t *testing.T) {
	controller := &TrayController{}

	connected := buildTrayPresentation(backend.RuntimeStatus{
		Dashboard: connectedDashboardState(3900, false),
	})
	if iconChanged, tooltipChanged := controller.recordPresentation(connected); !iconChanged || !tooltipChanged {
		t.Fatalf("first presentation should update icon and tooltip, got iconChanged=%v tooltipChanged=%v", iconChanged, tooltipChanged)
	}

	if iconChanged, tooltipChanged := controller.recordPresentation(connected); iconChanged || tooltipChanged {
		t.Fatalf("duplicate presentation should be ignored, got iconChanged=%v tooltipChanged=%v", iconChanged, tooltipChanged)
	}

	updatedTooltipStatus := backend.RuntimeStatus{
		Dashboard: connectedDashboardState(3900, false),
	}
	updatedTooltipStatus.Dashboard.BatteryStatus.Label = "4.01 V"
	updatedTooltip := buildTrayPresentation(updatedTooltipStatus)
	if iconChanged, tooltipChanged := controller.recordPresentation(updatedTooltip); iconChanged || !tooltipChanged {
		t.Fatalf("battery-only change should only update tooltip, got iconChanged=%v tooltipChanged=%v", iconChanged, tooltipChanged)
	}

	usbPower := buildTrayPresentation(backend.RuntimeStatus{
		Dashboard: connectedDashboardState(4010, true),
	})
	if iconChanged, tooltipChanged := controller.recordPresentation(usbPower); !iconChanged || !tooltipChanged {
		t.Fatalf("usb power change should update icon and tooltip, got iconChanged=%v tooltipChanged=%v", iconChanged, tooltipChanged)
	}
}

func connectedDashboardState(batteryMV int, usbPowerPresent bool) backend.DashboardState {
	return backend.DashboardState{
		DongleStatus:   backend.DeviceConnectionStatus{State: "connected"},
		MacropadStatus: backend.DeviceConnectionStatus{State: "connected"},
		BatteryStatus: backend.BatteryStatus{
			BatteryMV:       batteryMV,
			USBPowerPresent: usbPowerPresent,
			Label:           "battery",
		},
	}
}

func dongleOnlyDashboardState() backend.DashboardState {
	return backend.DashboardState{
		DongleStatus:   backend.DeviceConnectionStatus{State: "connected"},
		MacropadStatus: backend.DeviceConnectionStatus{State: "disconnected"},
	}
}

func disconnectedDashboardState() backend.DashboardState {
	return backend.DashboardState{
		DongleStatus:   backend.DeviceConnectionStatus{State: "not_detected"},
		MacropadStatus: backend.DeviceConnectionStatus{State: "unknown"},
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
