package device

import (
	"context"
	"testing"
)

type fakeBLETransport struct {
	settings KeyLEDSettings
	synced   bool
	err      error
}

func (f *fakeBLETransport) Start(context.Context) error { return nil }
func (f *fakeBLETransport) Stop()                       {}
func (f *fakeBLETransport) SyncKeyLEDSettings(settings KeyLEDSettings) error {
	f.synced = true
	f.settings = settings
	return f.err
}

func TestManagerActiveSourceArbitration(t *testing.T) {
	var reports []Report
	manager := NewManager(Callbacks{
		Report: func(report Report) {
			reports = append(reports, report)
		},
	})

	manager.emitReport(SourceUSB, Report{Connected: true, Keys: 0x01})
	manager.emitReport(SourceBLE, Report{Connected: true, Keys: 0x02})
	manager.emitReport(SourceUSB, Report{Connected: true, Keys: 0x03})
	manager.emitReport(SourceUSB, Report{Connected: false})
	manager.emitReport(SourceBLE, Report{Connected: true, Keys: 0x04})

	if len(reports) != 4 {
		t.Fatalf("got %d reports, want 4: %+v", len(reports), reports)
	}
	if reports[0].Source != SourceUSB || reports[0].Keys != 0x01 {
		t.Fatalf("first report = %+v, want USB keys 0x01", reports[0])
	}
	if reports[1].Source != SourceUSB || reports[1].Keys != 0x03 {
		t.Fatalf("second accepted report = %+v, want USB keys 0x03", reports[1])
	}
	if reports[2].Source != SourceUSB || reports[2].Connected {
		t.Fatalf("third accepted report = %+v, want USB disconnect", reports[2])
	}
	if reports[3].Source != SourceBLE || reports[3].Keys != 0x04 {
		t.Fatalf("fourth accepted report = %+v, want BLE keys 0x04", reports[3])
	}
}

func TestManagerSyncKeyLEDSettingsUsesActiveBLETransport(t *testing.T) {
	fakeBLE := &fakeBLETransport{}
	manager := NewManager(Callbacks{})
	manager.bleClient = fakeBLE
	manager.emitReport(SourceBLE, Report{Connected: true})

	settings := KeyLEDSettings{{Color: [3]byte{1, 2, 3}, Brightness: 4}}
	if err := manager.SyncKeyLEDSettings(settings); err != nil {
		t.Fatalf("SyncKeyLEDSettings returned error: %v", err)
	}
	if !fakeBLE.synced {
		t.Fatal("expected BLE sync to be used")
	}
	if fakeBLE.settings != settings {
		t.Fatalf("synced settings = %+v, want %+v", fakeBLE.settings, settings)
	}
}

func TestManagerSyncKeyLEDSettingsRequiresActiveSource(t *testing.T) {
	manager := NewManager(Callbacks{})
	if err := manager.SyncKeyLEDSettings(KeyLEDSettings{}); err == nil {
		t.Fatal("expected active source error")
	}
}
