package app

import (
	"errors"
	"testing"

	"hpad-app/internal/config"
	"hpad-app/internal/device"
)

type fakeKeyLEDSyncer struct {
	settings device.KeyLEDSettings
	err      error
	called   bool
}

func testBrightness(value uint8) *uint8 {
	brightness := value
	return &brightness
}

func (f *fakeKeyLEDSyncer) SyncKeyLEDSettings(settings device.KeyLEDSettings) error {
	f.called = true
	f.settings = settings
	return f.err
}

func TestKeyLEDSettingsFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.KeyAssignments[0].Color = "#112233"
	cfg.KeyAssignments[0].Brightness = testBrightness(0x40)
	cfg.KeyAssignments[1].Color = "445566"
	cfg.KeyAssignments[1].Brightness = testBrightness(0x00)
	cfg.KeyAssignments[2].Color = "#000000"

	settings, err := keyLEDSettingsFromConfig(cfg)
	if err != nil {
		t.Fatalf("keyLEDSettingsFromConfig returned error: %v", err)
	}

	if settings[0].Color != [3]byte{0x11, 0x22, 0x33} {
		t.Fatalf("unexpected key 1 color: %#v", settings[0].Color)
	}
	if settings[1].Color != [3]byte{0x44, 0x55, 0x66} {
		t.Fatalf("unexpected key 2 color: %#v", settings[1].Color)
	}
	if settings[2].Color != [3]byte{0x00, 0x00, 0x00} {
		t.Fatalf("unexpected key 3 color: %#v", settings[2].Color)
	}

	if settings[0].Brightness != 0x40 {
		t.Fatalf("unexpected key 1 brightness: %d", settings[0].Brightness)
	}
	if settings[1].Brightness != 0x00 {
		t.Fatalf("unexpected key 2 brightness: %d", settings[1].Brightness)
	}
	if settings[2].Brightness != 0xFF {
		t.Fatalf("unexpected key 3 brightness: %d", settings[2].Brightness)
	}
}

func TestKeyLEDSettingsFromConfigNormalizesInvalidColorToOff(t *testing.T) {
	cfg := config.Default()
	cfg.KeyAssignments[0].Color = "#12"

	settings, err := keyLEDSettingsFromConfig(cfg)
	if err != nil {
		t.Fatalf("keyLEDSettingsFromConfig returned error: %v", err)
	}
	if settings[0].Color != [3]byte{0x00, 0x00, 0x00} {
		t.Fatalf("unexpected normalized color: %#v", settings[0].Color)
	}
}

func TestSyncKeyLEDConfig(t *testing.T) {
	cfg := config.Default()
	cfg.KeyAssignments[0].Color = "#ABCDEF"
	cfg.KeyAssignments[0].Brightness = testBrightness(0x7F)
	syncer := &fakeKeyLEDSyncer{}

	if err := syncKeyLEDConfig(syncer, cfg); err != nil {
		t.Fatalf("syncKeyLEDConfig returned error: %v", err)
	}
	if !syncer.called {
		t.Fatal("expected SyncKeyLEDSettings to be called")
	}
	if syncer.settings[0].Color != [3]byte{0xAB, 0xCD, 0xEF} {
		t.Fatalf("unexpected synced color: %#v", syncer.settings[0].Color)
	}
	if syncer.settings[0].Brightness != 0x7F {
		t.Fatalf("unexpected synced brightness: %d", syncer.settings[0].Brightness)
	}
}

func TestSyncKeyLEDConfigPropagatesSyncError(t *testing.T) {
	wantErr := errors.New("write failed")
	syncer := &fakeKeyLEDSyncer{err: wantErr}

	err := syncKeyLEDConfig(syncer, config.Default())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSyncKeyLEDConfigRequiresSyncer(t *testing.T) {
	err := syncKeyLEDConfig(nil, config.Default())
	if err == nil {
		t.Fatal("expected dongle not connected error")
	}
}
