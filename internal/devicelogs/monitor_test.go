package devicelogs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyBasename(t *testing.T) {
	tests := []struct {
		name       string
		basename   string
		wantResult classification
	}{
		{name: "dongle", basename: "usb-HPad_Dongle-if00-port0", wantResult: classificationDongle},
		{name: "pad", basename: "usb-HPAD_Pad-if00-port0", wantResult: classificationPad},
		{name: "macropad", basename: "usb-HPAD_Macropad-if00-port0", wantResult: classificationPad},
		{name: "unclassified hpad", basename: "usb-HPAD-if00-port0", wantResult: classificationUnknownHPAD},
		{name: "ambiguous", basename: "usb-HPAD_Pad_Dongle-if00-port0", wantResult: classificationAmbiguous},
		{name: "ignored", basename: "usb-Generic_CDC-if00-port0", wantResult: classificationIgnored},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := classifyBasename(filepath.Join("/dev/serial/by-id", test.basename), test.basename)
			if entry.Result != test.wantResult {
				t.Fatalf("got result %q, want %q", entry.Result, test.wantResult)
			}
		})
	}
}

func TestSelectCandidatePrefersSortedStablePath(t *testing.T) {
	root := t.TempDir()
	padA := filepath.Join(root, "usb-HPAD_Pad-A-if00-port0")
	padB := filepath.Join(root, "usb-HPAD_Pad-B-if00-port0")
	if err := os.Symlink("/dev/null", padB); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := os.Symlink("/dev/null", padA); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	snap := scanDirectory(root)
	entry, ok, note := snap.selectCandidate(KindPad)
	if !ok {
		t.Fatal("expected a pad candidate")
	}
	if entry.StablePath != padA {
		t.Fatalf("got %s, want %s", entry.StablePath, padA)
	}
	if !strings.Contains(note, "multiple matching serial entries") {
		t.Fatalf("expected multiple-candidate note, got %q", note)
	}
	if !strings.Contains(note, padB) {
		t.Fatalf("expected note to mention ignored candidate, got %q", note)
	}
	if _, ok, _ := snap.selectCandidate(KindDongle); ok {
		t.Fatal("did not expect a dongle candidate")
	}
}

func TestSelectCandidateFallsBackToGenericZephyrPad(t *testing.T) {
	root := t.TempDir()
	dongle := filepath.Join(root, "usb-HPad_Dongle-if02")
	pad := filepath.Join(root, "usb-Zephyr_Project_CDC_ACM_serial_backend_9041E4756A34FE2E-if00")
	if err := os.Symlink("/dev/null", dongle); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := os.Symlink("/dev/null", pad); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	snap := scanDirectory(root)
	entry, ok, note := snap.selectCandidate(KindPad)
	if !ok {
		t.Fatal("expected fallback pad candidate")
	}
	if entry.StablePath != pad {
		t.Fatalf("got %s, want %s", entry.StablePath, pad)
	}
	if !strings.Contains(note, "generic Zephyr CDC ACM fallback") {
		t.Fatalf("expected fallback note, got %q", note)
	}

	dongleEntry, ok, _ := snap.selectCandidate(KindDongle)
	if !ok {
		t.Fatal("expected dongle candidate")
	}
	if dongleEntry.StablePath != dongle {
		t.Fatalf("got %s, want %s", dongleEntry.StablePath, dongle)
	}
}

func TestSelectCandidateDoesNotGuessBetweenMultipleGenericZephyrPads(t *testing.T) {
	root := t.TempDir()
	padA := filepath.Join(root, "usb-Zephyr_Project_CDC_ACM_serial_backend_A-if00")
	padB := filepath.Join(root, "usb-Zephyr_Project_CDC_ACM_serial_backend_B-if00")
	if err := os.Symlink("/dev/null", padA); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := os.Symlink("/dev/null", padB); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	snap := scanDirectory(root)
	if _, ok, note := snap.selectCandidate(KindPad); ok {
		t.Fatal("did not expect fallback to choose between multiple generic Zephyr pad candidates")
	} else if !strings.Contains(note, "multiple generic Zephyr CDC ACM candidates") {
		t.Fatalf("expected ambiguity note, got %q", note)
	}
}

func TestRoleStateFormatLogLineSuppressesBlankAndIdlePadNoise(t *testing.T) {
	state := &roleState{kind: KindPad}

	if _, ok := state.formatLogLine(""); ok {
		t.Fatal("expected empty line to be suppressed")
	}
	if _, ok := state.formatLogLine("   \r"); ok {
		t.Fatal("expected whitespace-only line to be suppressed")
	}
	if _, ok := state.formatLogLine("[00:00:00.000,000] <inf> radio_esb: Queued report kind=0 keys=0x00 enc_delta=0 enc_pressed=0"); ok {
		t.Fatal("expected idle input report to be suppressed")
	}
	if _, ok := state.formatLogLine("[00:00:00.000,001] <inf> radio_esb: Report delivery acknowledged for kind=0"); ok {
		t.Fatal("expected idle delivery acknowledgement to be suppressed")
	}

	line, ok := state.formatLogLine("[00:00:00.100,000] <inf> radio_esb: Queued report kind=0 keys=0x04 enc_delta=0 enc_pressed=0")
	if !ok {
		t.Fatal("expected active key report to be shown")
	}
	if line != "input: key 3 down" {
		t.Fatalf("got %q, want %q", line, "input: key 3 down")
	}

	line, ok = state.formatLogLine("[00:00:00.200,000] <inf> radio_esb: Queued report kind=0 keys=0x04 enc_delta=1 enc_pressed=1")
	if !ok {
		t.Fatal("expected encoder activity to be shown")
	}
	if line != "input: encoder +1, encoder button down" {
		t.Fatalf("got %q, want %q", line, "input: encoder +1, encoder button down")
	}

	line, ok = state.formatLogLine("[00:00:00.300,000] <inf> radio_esb: Queued report kind=0 keys=0x00 enc_delta=0 enc_pressed=1")
	if !ok {
		t.Fatal("expected key release to be shown")
	}
	if line != "input: key 3 up" {
		t.Fatalf("got %q, want %q", line, "input: key 3 up")
	}

	line, ok = state.formatLogLine("[00:00:00.400,000] <inf> radio_esb: Queued report kind=0 keys=0x00 enc_delta=0 enc_pressed=0")
	if !ok {
		t.Fatal("expected encoder button release to be shown")
	}
	if line != "input: encoder button up" {
		t.Fatalf("got %q, want %q", line, "input: encoder button up")
	}
}

func TestRoleStateFormatLogLineLeavesOtherLogsAlone(t *testing.T) {
	state := &roleState{kind: KindDongle}
	line, ok := state.formatLogLine("[00:00:00.000,000] <inf> main: Dongle boot start")
	if !ok {
		t.Fatal("expected non-pad line to pass through")
	}
	if line != "[00:00:00.000,000] <inf> main: Dongle boot start" {
		t.Fatalf("got %q", line)
	}
}
