package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const keyCount = 6

const (
	ActionTypeUnassigned       = "unassigned"
	ActionTypeKeyboardShortcut = "keyboard_shortcut"
	ActionTypeRunCommand       = "run_command"
	ActionTypeOpenApplication  = "open_application"
	ActionTypeMediaControl     = "media_control"
)

const defaultProfileName = "Default"
const defaultKeyColor = "#000000"
const defaultKeyBrightness = uint8(0xFF)

type KeyAction struct {
	Type             string `json:"type"`
	Shortcut         string `json:"shortcut,omitempty"`
	Command          string `json:"command,omitempty"`
	Arguments        string `json:"arguments,omitempty"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	Application      string `json:"application,omitempty"`
	MediaControl     string `json:"mediaControl,omitempty"`
}

type KeyAssignment struct {
	ID     string    `json:"id"`
	Label  string    `json:"label"`
	Color  string    `json:"color"`
	Brightness *uint8 `json:"brightness,omitempty"`
	Action KeyAction `json:"action"`
}

type Config struct {
	Profile        string          `json:"profile"`
	KeyAssignments []KeyAssignment `json:"keyAssignments"`
	LegacyKeys     []legacyKey     `json:"keys,omitempty"`
}

type legacyKey struct {
	Label      string `json:"label"`
	Command    string `json:"command"`
	Args       string `json:"args"`
	WorkingDir string `json:"workingDir"`
	Enabled    bool   `json:"enabled"`
}

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(base, "hpad-agent", "config.json")}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (Config, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := s.Save(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg = Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	cfg = Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.path, data, 0o644)
}

func Default() Config {
	assignments := make([]KeyAssignment, keyCount)
	for i := range assignments {
		assignments[i] = defaultAssignment(i)
	}
	return Config{
		Profile:        defaultProfileName,
		KeyAssignments: assignments,
	}
}

func Normalize(cfg Config) Config {
	if strings.TrimSpace(cfg.Profile) == "" {
		cfg.Profile = defaultProfileName
	}

	assignments := make([]KeyAssignment, keyCount)
	legacyAssignments := normalizeLegacyAssignments(cfg.LegacyKeys)
	for i := 0; i < keyCount; i++ {
		assignments[i] = defaultAssignment(i)
		if i < len(cfg.KeyAssignments) {
			assignments[i] = normalizeAssignment(i, cfg.KeyAssignments[i])
			continue
		}
		if i < len(legacyAssignments) {
			assignments[i] = normalizeAssignment(i, legacyAssignments[i])
		}
	}

	return Config{
		Profile:        cfg.Profile,
		KeyAssignments: assignments,
	}
}

func Validate(cfg Config) error {
	if len(cfg.KeyAssignments) != keyCount {
		return fmt.Errorf("config must contain %d keys", keyCount)
	}
	for index, key := range cfg.KeyAssignments {
		if err := ValidateColor(key.Color); err != nil {
			return fmt.Errorf("key %d: %w", index+1, err)
		}
		if err := ValidateAction(key.Action); err != nil {
			return fmt.Errorf("key %d: %w", index+1, err)
		}
	}
	return nil
}

func ValidateColor(color string) error {
	if _, err := normalizeColor(color); err != nil {
		return err
	}
	return nil
}

func ValidateAction(action KeyAction) error {
	switch normalizeActionType(action.Type) {
	case ActionTypeUnassigned:
		return nil
	case ActionTypeKeyboardShortcut:
		if strings.TrimSpace(action.Shortcut) == "" {
			return errors.New("keyboard shortcut is required")
		}
		return nil
	case ActionTypeRunCommand:
		if strings.TrimSpace(action.Command) == "" {
			return errors.New("command is required")
		}
		return nil
	case ActionTypeOpenApplication:
		if strings.TrimSpace(action.Application) == "" {
			return errors.New("application path is required")
		}
		return nil
	case ActionTypeMediaControl:
		if strings.TrimSpace(action.MediaControl) == "" {
			return errors.New("media control is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

func Clone(cfg Config) Config {
	assignments := make([]KeyAssignment, len(cfg.KeyAssignments))
	copy(assignments, cfg.KeyAssignments)
	return Config{
		Profile:        cfg.Profile,
		KeyAssignments: assignments,
	}
}

func DefaultAssignment(index int) KeyAssignment {
	return defaultAssignment(index)
}

func NormalizeBrightness(brightness *uint8) uint8 {
	if brightness == nil {
		return defaultKeyBrightness
	}
	return *brightness
}

func ClearAction() KeyAction {
	return KeyAction{Type: ActionTypeUnassigned}
}

func NormalizeAction(action KeyAction) KeyAction {
	action.Type = normalizeActionType(action.Type)
	action.Shortcut = strings.TrimSpace(action.Shortcut)
	action.Command = strings.TrimSpace(action.Command)
	action.Arguments = strings.TrimSpace(action.Arguments)
	action.WorkingDirectory = strings.TrimSpace(action.WorkingDirectory)
	action.Application = strings.TrimSpace(action.Application)
	action.MediaControl = strings.TrimSpace(action.MediaControl)

	switch action.Type {
	case ActionTypeKeyboardShortcut:
		action.Command = ""
		action.Arguments = ""
		action.WorkingDirectory = ""
		action.Application = ""
		action.MediaControl = ""
	case ActionTypeRunCommand:
		action.Shortcut = ""
		action.Application = ""
		action.MediaControl = ""
	case ActionTypeOpenApplication:
		action.Shortcut = ""
		action.Command = ""
		action.Arguments = ""
		action.WorkingDirectory = ""
		action.MediaControl = ""
	case ActionTypeMediaControl:
		action.Shortcut = ""
		action.Command = ""
		action.Arguments = ""
		action.WorkingDirectory = ""
		action.Application = ""
	default:
		action = ClearAction()
	}

	return action
}

func defaultAssignment(index int) KeyAssignment {
	name := fmt.Sprintf("K%d", index+1)
	return KeyAssignment{
		ID:     name,
		Label:  name,
		Color:  defaultKeyColor,
		Brightness: brightnessPtr(defaultKeyBrightness),
		Action: ClearAction(),
	}
}

func normalizeAssignment(index int, assignment KeyAssignment) KeyAssignment {
	defaultKey := defaultAssignment(index)
	assignment.ID = strings.TrimSpace(assignment.ID)
	if assignment.ID == "" {
		assignment.ID = defaultKey.ID
	}
	assignment.Label = strings.TrimSpace(assignment.Label)
	if assignment.Label == "" {
		assignment.Label = defaultKey.Label
	}
	assignment.Color = NormalizeColor(assignment.Color)
	assignment.Brightness = brightnessPtr(NormalizeBrightness(assignment.Brightness))
	assignment.Action = NormalizeAction(assignment.Action)
	return assignment
}

func normalizeLegacyAssignments(keys []legacyKey) []KeyAssignment {
	assignments := make([]KeyAssignment, 0, len(keys))
	for index, key := range keys {
		action := ClearAction()
		if strings.TrimSpace(key.Command) != "" {
			action = KeyAction{
				Type:             ActionTypeRunCommand,
				Command:          strings.TrimSpace(key.Command),
				Arguments:        strings.TrimSpace(key.Args),
				WorkingDirectory: strings.TrimSpace(key.WorkingDir),
			}
		}
		assignments = append(assignments, KeyAssignment{
			ID:     fmt.Sprintf("K%d", index+1),
			Label:  strings.TrimSpace(key.Label),
			Color:  defaultKeyColor,
			Brightness: brightnessPtr(defaultKeyBrightness),
			Action: NormalizeAction(action),
		})
	}
	return assignments
}

func brightnessPtr(brightness uint8) *uint8 {
	value := brightness
	return &value
}

func NormalizeColor(color string) string {
	normalized, err := normalizeColor(color)
	if err != nil {
		return defaultKeyColor
	}
	return normalized
}

func normalizeColor(color string) (string, error) {
	color = strings.TrimSpace(strings.ToUpper(color))
	if color == "" {
		return defaultKeyColor, nil
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	if len(color) != 7 {
		return "", fmt.Errorf("color must be a 6-digit hex value")
	}
	if _, err := strconv.ParseUint(color[1:], 16, 24); err != nil {
		return "", fmt.Errorf("color must be a 6-digit hex value")
	}
	return color, nil
}

func normalizeActionType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ActionTypeUnassigned
	}
	return value
}
