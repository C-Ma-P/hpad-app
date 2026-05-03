package device

import "testing"

func TestDecodeVendorInputReport(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want Report
		ok   bool
	}{
		{
			name: "legacy report without connection byte",
			raw:  []byte{0x15, 0xFF, 0x01},
			want: Report{Connected: true, Keys: 0x15, EncoderDelta: -1, EncoderPressed: true},
			ok:   true,
		},
		{
			name: "firmware host report vector",
			raw:  []byte{0x00, 0x01, 0x15, 0xFF, 0x01, 0x74, 0x10, 0x01},
			want: Report{Connected: true, Keys: 0x15, EncoderDelta: -1, EncoderPressed: true, BatteryMV: 4212, USBPowerPresent: true},
			ok:   true,
		},
		{
			name: "host report without report id",
			raw:  []byte{0x01, 0x15, 0xFF, 0x01, 0x74, 0x10, 0x01},
			want: Report{Connected: true, Keys: 0x15, EncoderDelta: -1, EncoderPressed: true, BatteryMV: 4212, USBPowerPresent: true},
			ok:   true,
		},
		{
			name: "explicit disconnect with report id",
			raw:  []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			want: Report{},
			ok:   true,
		},
		{
			name: "explicit disconnect without report id",
			raw:  []byte{0x00, 0x00, 0x00, 0x00},
			want: Report{},
			ok:   true,
		},
		{
			name: "too short",
			raw:  []byte{0x01, 0x02},
			want: Report{},
			ok:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := decodeVendorInputReport(test.raw)
			if ok != test.ok {
				t.Fatalf("decodeVendorInputReport(% X) ok = %v, want %v", test.raw, ok, test.ok)
			}
			if ok && got != test.want {
				t.Fatalf("decodeVendorInputReport(% X) = %+v, want %+v", test.raw, got, test.want)
			}
		})
	}
}

func TestEncodeKeyLEDConfigReportMatchesFirmwareLayout(t *testing.T) {
	settings := KeyLEDSettings{
		{Color: [3]byte{0x11, 0x22, 0x33}, Brightness: 0x40},
		{Color: [3]byte{0x44, 0x55, 0x66}, Brightness: 0x50},
		{Color: [3]byte{0x77, 0x88, 0x99}, Brightness: 0x60},
		{Color: [3]byte{0xAA, 0xBB, 0xCC}, Brightness: 0x70},
		{Color: [3]byte{0xDD, 0xEE, 0xFF}, Brightness: 0x80},
		{Color: [3]byte{0x10, 0x20, 0x30}, Brightness: 0x90},
	}

	want := [usbVendorOutputTransferSize]byte{
		0x00,
		configKindKeyColors,
		0x11, 0x22, 0x33, 0x40,
		0x44, 0x55, 0x66, 0x50,
		0x77, 0x88, 0x99, 0x60,
		0xAA, 0xBB, 0xCC, 0x70,
		0xDD, 0xEE, 0xFF, 0x80,
		0x10, 0x20, 0x30, 0x90,
	}

	got := encodeKeyLEDConfigReport(settings)
	if got != want {
		t.Fatalf("encodeKeyLEDConfigReport() = % X, want % X", got[:], want[:])
	}
}
