package actions

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"regexp"
	"strings"

	"github.com/google/shlex"

	"hpad-app/internal/config"
)

type Runner struct {
	ctx context.Context
}

func NewRunner(ctx context.Context) *Runner {
	return &Runner{ctx: ctx}
}

func (r *Runner) Run(index int, assignment config.KeyAssignment) error {
	command, args, workingDirectory, err := prepareCommand(assignment.Action)
	if err != nil {
		return err
	}

	keyLabel := assignment.Label
	if keyLabel == "" {
		keyLabel = assignment.ID
	}
	go func() {
		cmd := exec.CommandContext(r.ctx, command, args...)
		if workingDirectory != "" {
			cmd.Dir = workingDirectory
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[actions] %s execution failed: %v (%s)", keyLabel, err, strings.TrimSpace(string(output)))
			return
		}
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			log.Printf("[actions] %s executed: %s", keyLabel, trimmedOutput)
		}
	}()

	return nil
}

func (r *Runner) CanRun(action config.KeyAction) bool {
	switch config.NormalizeAction(action).Type {
	case config.ActionTypeKeyboardShortcut, config.ActionTypeRunCommand,
		config.ActionTypeOpenApplication, config.ActionTypeMediaControl:
		return true
	default:
		return false
	}
}

var functionKeyPattern = regexp.MustCompile(`(?i)^f([1-9]|1[0-9]|2[0-4])$`)

func parseArgs(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	args, err := shlex.Split(input)
	if err != nil {
		return nil, errors.New("invalid command arguments")
	}
	return args, nil
}

func prepareCommand(action config.KeyAction) (string, []string, string, error) {
	action = config.NormalizeAction(action)
	switch action.Type {
	case config.ActionTypeKeyboardShortcut:
		shortcut, err := normalizeShortcut(action.Shortcut)
		if err != nil {
			return "", nil, "", err
		}
		return "xdotool", []string{"key", "--clearmodifiers", shortcut}, "", nil
	case config.ActionTypeRunCommand:
		if strings.TrimSpace(action.Command) == "" {
			return "", nil, "", errors.New("command is required")
		}
		args, err := parseArgs(action.Arguments)
		if err != nil {
			return "", nil, "", err
		}
		return action.Command, args, action.WorkingDirectory, nil
	case config.ActionTypeOpenApplication:
		if strings.TrimSpace(action.Application) == "" {
			return "", nil, "", errors.New("application path is required")
		}
		return action.Application, nil, "", nil
	case config.ActionTypeMediaControl:
		mediaKey, err := mediaControlKey(action.MediaControl)
		if err != nil {
			return "", nil, "", err
		}
		return "xdotool", []string{"key", "--clearmodifiers", mediaKey}, "", nil
	default:
		return "", nil, "", errors.New("action cannot be executed")
	}
}

func normalizeShortcut(shortcut string) (string, error) {
	parts := strings.Split(shortcut, "+")
	if len(parts) == 0 {
		return "", errors.New("keyboard shortcut is required")
	}

	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		key, err := normalizeShortcutPart(part)
		if err != nil {
			return "", err
		}
		normalized = append(normalized, key)
	}

	return strings.Join(normalized, "+"), nil
}

func normalizeShortcutPart(part string) (string, error) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", errors.New("keyboard shortcut contains an empty key")
	}

	switch strings.ToLower(part) {
	case "ctrl", "control":
		return "ctrl", nil
	case "shift":
		return "shift", nil
	case "alt", "option":
		return "alt", nil
	case "cmd", "command", "meta", "super", "win", "windows":
		return "super", nil
	case "enter", "return":
		return "Return", nil
	case "esc", "escape":
		return "Escape", nil
	case "space", "spacebar":
		return "space", nil
	case "tab":
		return "Tab", nil
	case "backspace":
		return "BackSpace", nil
	case "delete", "del":
		return "Delete", nil
	case "insert", "ins":
		return "Insert", nil
	case "home":
		return "Home", nil
	case "end":
		return "End", nil
	case "pageup", "page_up", "pgup":
		return "Page_Up", nil
	case "pagedown", "page_down", "pgdn":
		return "Page_Down", nil
	case "up", "arrowup":
		return "Up", nil
	case "down", "arrowdown":
		return "Down", nil
	case "left", "arrowleft":
		return "Left", nil
	case "right", "arrowright":
		return "Right", nil
	}

	if len(part) == 1 {
		return strings.ToLower(part), nil
	}
	if functionKeyPattern.MatchString(part) {
		return strings.ToUpper(part), nil
	}

	return part, nil
}

func mediaControlKey(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "play_pause":
		return "XF86AudioPlay", nil
	case "next_track":
		return "XF86AudioNext", nil
	case "previous_track":
		return "XF86AudioPrev", nil
	case "stop":
		return "XF86AudioStop", nil
	case "volume_up":
		return "XF86AudioRaiseVolume", nil
	case "volume_down":
		return "XF86AudioLowerVolume", nil
	case "mute":
		return "XF86AudioMute", nil
	default:
		return "", errors.New("unsupported media control")
	}
}
