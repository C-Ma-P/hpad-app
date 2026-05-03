package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testBrightnessPtr(value uint8) *uint8 {
	brightness := value
	return &brightness
}

func TestNormalizePadsAssignments(t *testing.T) {
	cfg := Normalize(Config{KeyAssignments: []KeyAssignment{{Label: "One"}}})
	if len(cfg.KeyAssignments) != 6 {
		t.Fatalf("expected 6 keys, got %d", len(cfg.KeyAssignments))
	}
	if cfg.KeyAssignments[0].Label != "One" {
		t.Fatalf("expected first key label to be preserved")
	}
	if cfg.KeyAssignments[5].Label != "K6" {
		t.Fatalf("expected missing keys to get defaults")
	}
	if cfg.KeyAssignments[0].Color != "#000000" {
		t.Fatalf("expected missing color to default to #000000, got %q", cfg.KeyAssignments[0].Color)
	}
	if got := NormalizeBrightness(cfg.KeyAssignments[0].Brightness); got != 0xFF {
		t.Fatalf("expected missing brightness to default to 255, got %d", got)
	}
}

func TestNormalizeMigratesLegacyKeys(t *testing.T) {
	cfg := Normalize(Config{LegacyKeys: []legacyKey{{Label: "Legacy", Command: "echo", Args: "hello"}}})
	if cfg.KeyAssignments[0].Action.Type != ActionTypeRunCommand {
		t.Fatalf("expected legacy command to migrate to run_command")
	}
	if cfg.KeyAssignments[0].Action.Command != "echo" {
		t.Fatalf("expected legacy command to be preserved")
	}
	if cfg.KeyAssignments[0].Action.Arguments != "hello" {
		t.Fatalf("expected legacy args to be preserved")
	}
	if cfg.KeyAssignments[0].Label != "Legacy" {
		t.Fatalf("expected legacy label to be preserved")
	}
	if cfg.Profile != "Default" {
		t.Fatalf("expected default profile to be applied")
	}
	if len(cfg.KeyAssignments) != 6 {
		t.Fatalf("expected 6 keys after migration")
	}
	if cfg.KeyAssignments[5].ID != "K6" {
		t.Fatalf("expected default key ids to be applied")
	}
	if got := NormalizeBrightness(cfg.KeyAssignments[0].Brightness); got != 0xFF {
		t.Fatalf("expected migrated key brightness to default to 255, got %d", got)
	}
	if len(cfg.LegacyKeys) != 0 {
		t.Fatalf("expected legacy keys to be removed from normalized config")
	}
}

func TestNormalizePreservesExplicitBrightness(t *testing.T) {
	cfg := Normalize(Config{KeyAssignments: []KeyAssignment{{Brightness: testBrightnessPtr(0)}}})
	if got := NormalizeBrightness(cfg.KeyAssignments[0].Brightness); got != 0 {
		t.Fatalf("expected explicit zero brightness to be preserved, got %d", got)
	}
}

func TestValidateRequiresCommandForRunCommand(t *testing.T) {
	cfg := Default()
	cfg.KeyAssignments[0].Action = KeyAction{Type: ActionTypeRunCommand}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for run_command without command")
	}
}

func TestNormalizeColorFormatsHexValues(t *testing.T) {
	assignment := Normalize(Config{KeyAssignments: []KeyAssignment{{Color: "ff00aa"}}}).KeyAssignments[0]
	if assignment.Color != "#FF00AA" {
		t.Fatalf("expected uppercase hex color, got %q", assignment.Color)
	}
}

func TestValidateRejectsInvalidColor(t *testing.T) {
	cfg := Default()
	cfg.KeyAssignments[0].Color = "#12"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid color")
	}
}

func TestNewStoreUsesHpadConfigDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if want := filepath.Join(homeDir, ".config", "hpad", "config.json"); store.Path() != want {
		t.Fatalf("Path() = %q, want %q", store.Path(), want)
	}
}

func TestLoadMigratesLegacyConfigOnFirstRead(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	legacyPath := filepath.Join(homeDir, ".config", "hpad-agent", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacyConfig := Default()
	legacyConfig.Profile = "Migrated"
	if err := os.WriteFile(legacyPath, []byte("{\n  \"profile\": \"Migrated\",\n  \"keyAssignments\": [\n    {\n      \"id\": \"K1\",\n      \"label\": \"K1\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    },\n    {\n      \"id\": \"K2\",\n      \"label\": \"K2\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    },\n    {\n      \"id\": \"K3\",\n      \"label\": \"K3\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    },\n    {\n      \"id\": \"K4\",\n      \"label\": \"K4\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    },\n    {\n      \"id\": \"K5\",\n      \"label\": \"K5\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    },\n    {\n      \"id\": \"K6\",\n      \"label\": \"K6\",\n      \"color\": \"#000000\",\n      \"action\": {\n        \"type\": \"unassigned\"\n      }\n    }\n  ]\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "Migrated" {
		t.Fatalf("Profile = %q, want %q", cfg.Profile, "Migrated")
	}
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("expected migrated config at %q: %v", store.Path(), err)
	}
}
