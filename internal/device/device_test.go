package device

import (
	"os"
	"strings"
	"testing"
)

func TestDecodePayload(t *testing.T) {
	report, ok := decodePayload([]byte{0x15, 0xFF, 0x01})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if !report.Connected || report.Keys != 0x15 || report.EncoderDelta != -1 || !report.EncoderPressed || report.BatteryMV != 0 || report.Charging {
		t.Fatal("unexpected decoded report")
	}
}

func TestDecodePayloadWithBatteryAndCharging(t *testing.T) {
	report, ok := decodePayload([]byte{0x01, 0x15, 0xFF, 0x01, 0x74, 0x10, 0x01})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if !report.Connected || report.Keys != 0x15 || report.EncoderDelta != -1 || !report.EncoderPressed {
		t.Fatal("unexpected decoded input fields")
	}
	if report.BatteryMV != 4212 {
		t.Fatalf("unexpected battery mv: %d", report.BatteryMV)
	}
	if !report.Charging {
		t.Fatal("expected charging flag")
	}
}

func TestDecodePayloadWithReportID(t *testing.T) {
	report, ok := decodePayload([]byte{0x00, 0x03, 0x02, 0x00})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if !report.Connected || report.Keys != 0x03 || report.EncoderDelta != 2 || report.EncoderPressed || report.BatteryMV != 0 || report.Charging {
		t.Fatal("unexpected decoded report with report id")
	}
}

func TestDecodePayloadWithBatteryAndChargingAndReportID(t *testing.T) {
	report, ok := decodePayload([]byte{0x00, 0x01, 0x03, 0x02, 0x00, 0x10, 0x0F, 0x00})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if !report.Connected || report.Keys != 0x03 || report.EncoderDelta != 2 || report.EncoderPressed {
		t.Fatal("unexpected decoded input fields with report id")
	}
	if report.BatteryMV != 3856 {
		t.Fatalf("unexpected battery mv with report id: %d", report.BatteryMV)
	}
	if report.Charging {
		t.Fatal("expected charging flag to be false")
	}
}

func TestDecodePayloadWithExplicitConnectionState(t *testing.T) {
	report, ok := decodePayload([]byte{0x01, 0x15, 0xFF, 0x01})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if !report.Connected || report.Keys != 0x15 || report.EncoderDelta != -1 || !report.EncoderPressed || report.BatteryMV != 0 || report.Charging {
		t.Fatal("unexpected decoded report with explicit connection state")
	}
}

func TestDecodePayloadWithExplicitDisconnectStateAndReportID(t *testing.T) {
	report, ok := decodePayload([]byte{0x00, 0x00, 0x00, 0x00, 0x00})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if report.Connected || report.Keys != 0 || report.EncoderDelta != 0 || report.EncoderPressed || report.BatteryMV != 0 || report.Charging {
		t.Fatal("unexpected decoded report with explicit disconnect state")
	}
}

func TestDecodePayloadWithExplicitDisconnectState(t *testing.T) {
	report, ok := decodePayload([]byte{0x00, 0x00, 0x00, 0x00})
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if report.Connected || report.Keys != 0 || report.EncoderDelta != 0 || report.EncoderPressed || report.BatteryMV != 0 || report.Charging {
		t.Fatal("unexpected decoded report with explicit disconnect state")
	}
}

func TestFormatOpenErrorIncludesPermissionHint(t *testing.T) {
	candidate := Info{
		Path:            "/dev/hidraw0",
		VendorID:        vendorID,
		ProductID:       productID,
		UsagePage:       vendorUsagePage,
		Usage:           vendorUsage,
		InterfaceNumber: 1,
	}

	message := formatOpenError(candidate, os.ErrPermission)

	for _, want := range []string{
		"/dev/hidraw0",
		"permission denied opening hidraw",
		"udev rule",
		"0xCAFE",
		"0xB00B",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in %q", want, message)
		}
	}
}
