package device

import (
	"errors"
	"os"
	"strings"
	"syscall"
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

func TestIsTransientReadError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eintr string", err: errors.New("Interrupted system call"), want: true},
		{name: "eintr token", err: errors.New("poll failed: EINTR"), want: true},
		{name: "syscall eintr", err: syscall.EINTR, want: true},
		{name: "disconnect", err: errors.New("device disconnected"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isTransientReadError(test.err)
			if got != test.want {
				t.Fatalf("isTransientReadError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestEncodeKeyLEDReport(t *testing.T) {
	settings := KeyLEDSettings{
		{Color: [3]byte{0x11, 0x22, 0x33}, Brightness: 0x40},
		{Color: [3]byte{0x44, 0x55, 0x66}, Brightness: 0x50},
		{Color: [3]byte{0x77, 0x88, 0x99}, Brightness: 0x60},
		{Color: [3]byte{0xAA, 0xBB, 0xCC}, Brightness: 0x70},
		{Color: [3]byte{0xDD, 0xEE, 0xFF}, Brightness: 0x80},
		{Color: [3]byte{0x10, 0x20, 0x30}, Brightness: 0x90},
	}

	report := encodeKeyLEDReport(settings)
	if report[0] != 0 {
		t.Fatalf("unexpected report id byte: %d", report[0])
	}
	if report[1] != vendorConfigCmd {
		t.Fatalf("unexpected command byte: %d", report[1])
	}
	want := []byte{
		0x11, 0x22, 0x33, 0x40,
		0x44, 0x55, 0x66, 0x50,
		0x77, 0x88, 0x99, 0x60,
		0xAA, 0xBB, 0xCC, 0x70,
		0xDD, 0xEE, 0xFF, 0x80,
		0x10, 0x20, 0x30, 0x90,
	}
	for index, value := range want {
		if report[index+2] != value {
			t.Fatalf("unexpected payload byte %d: got 0x%02X want 0x%02X", index, report[index+2], value)
		}
	}
}
