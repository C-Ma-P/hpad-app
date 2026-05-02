package actions

import (
	"testing"

	"hpad-app/internal/config"
)

func TestPrepareCommandKeyboardShortcut(t *testing.T) {
	command, args, workingDirectory, err := prepareCommand(config.KeyAction{
		Type:     config.ActionTypeKeyboardShortcut,
		Shortcut: "Ctrl+Shift+P",
	})
	if err != nil {
		t.Fatalf("prepareCommand returned error: %v", err)
	}
	if command != "xdotool" {
		t.Fatalf("unexpected command: %q", command)
	}
	wantArgs := []string{"key", "--clearmodifiers", "ctrl+shift+p"}
	if len(args) != len(wantArgs) {
		t.Fatalf("unexpected arg count: %v", args)
	}
	for i := range args {
		if args[i] != wantArgs[i] {
			t.Fatalf("unexpected arg %d: got %q want %q", i, args[i], wantArgs[i])
		}
	}
	if workingDirectory != "" {
		t.Fatalf("unexpected working directory: %q", workingDirectory)
	}
}

func TestPrepareCommandMediaControl(t *testing.T) {
	command, args, _, err := prepareCommand(config.KeyAction{
		Type:         config.ActionTypeMediaControl,
		MediaControl: "next_track",
	})
	if err != nil {
		t.Fatalf("prepareCommand returned error: %v", err)
	}
	if command != "xdotool" {
		t.Fatalf("unexpected command: %q", command)
	}
	wantArgs := []string{"key", "--clearmodifiers", "XF86AudioNext"}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("unexpected arg %d: got %q want %q", i, args[i], wantArgs[i])
		}
	}
}

func TestPrepareCommandRejectsInvalidShortcut(t *testing.T) {
	_, _, _, err := prepareCommand(config.KeyAction{
		Type:     config.ActionTypeKeyboardShortcut,
		Shortcut: "Ctrl++P",
	})
	if err == nil {
		t.Fatal("expected invalid shortcut error")
	}
}
