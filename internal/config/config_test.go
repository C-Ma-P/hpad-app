package config

import "testing"

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
	if len(cfg.LegacyKeys) != 0 {
		t.Fatalf("expected legacy keys to be removed from normalized config")
	}
}

func TestValidateRequiresCommandForRunCommand(t *testing.T) {
	cfg := Default()
	cfg.KeyAssignments[0].Action = KeyAction{Type: ActionTypeRunCommand}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for run_command without command")
	}
}
