package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"hpad-app/internal/actions"
	"hpad-app/internal/config"
	"hpad-app/internal/device"
)

type DeviceConnectionStatus struct {
	State  string `json:"state"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Path   string `json:"path,omitempty"`
}

type BatteryStatus struct {
	State           string `json:"state"`
	Label           string `json:"label"`
	Detail          string `json:"detail"`
	BatteryMV       int    `json:"batteryMV"`
	USBPowerPresent bool   `json:"usbPowerPresent"`
}

type DashboardState struct {
	Profile        string                 `json:"profile"`
	DongleStatus   DeviceConnectionStatus `json:"dongleStatus"`
	MacropadStatus DeviceConnectionStatus `json:"macropadStatus"`
	BatteryStatus  BatteryStatus          `json:"batteryStatus"`
	KeyAssignments []config.KeyAssignment `json:"keyAssignments"`
	Dirty          bool                   `json:"dirty"`
	Saved          bool                   `json:"saved"`
}

type RuntimeStatus struct {
	Dashboard    DashboardState `json:"dashboard"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
}

type keyLEDSyncer interface {
	SyncKeyLEDSettings(device.KeyLEDSettings) error
}

type Core struct {
	mu             sync.RWMutex
	store          *config.Store
	device         *device.Manager
	runner         *actions.Runner
	cancel         context.CancelFunc
	config         config.Config
	persisted      config.Config
	state          DashboardState
	prevKeys       uint8
	started        bool
	runtimeError   string
	statusObserver func(RuntimeStatus)
}

func NewCore() (*Core, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	defaultConfig := config.Default()
	return &Core{
		store:     store,
		config:    defaultConfig,
		persisted: defaultConfig,
		state:     newDashboardState(defaultConfig),
	}, nil
}

func (c *Core) Start(parent context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}

	cfg, err := c.store.Load()
	if err != nil {
		c.mu.Unlock()
		return err
	}
	cfg = config.Normalize(cfg)

	ctx, cancel := context.WithCancel(parent)
	runner := actions.NewRunner(ctx)
	manager := device.NewManager(device.Callbacks{
		Connected:    c.handleConnected,
		Disconnected: c.handleDisconnected,
		Report:       c.handleReport,
		Log:          c.handleDeviceLog,
	})

	c.cancel = cancel
	c.runner = runner
	c.device = manager
	c.config = config.Clone(cfg)
	c.persisted = config.Clone(cfg)
	c.state = newDashboardState(cfg)
	c.started = true
	c.prevKeys = 0
	c.mu.Unlock()

	if err := manager.Start(ctx); err != nil {
		cancel()
		c.mu.Lock()
		c.runner = nil
		c.device = nil
		c.cancel = nil
		c.started = false
		c.mu.Unlock()
		return err
	}
	return nil
}

func (c *Core) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	deviceManager := c.device
	cancel := c.cancel
	c.started = false
	c.runner = nil
	c.device = nil
	c.cancel = nil
	c.prevKeys = 0
	c.runtimeError = ""
	c.state = newDashboardState(c.config)
	c.state.Dirty = !reflect.DeepEqual(c.config, c.persisted)
	c.state.Saved = !c.state.Dirty
	observer, status := c.runtimeStatusLocked()
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if deviceManager != nil {
		deviceManager.Stop()
	}
	notifyRuntimeStatus(observer, status)
	return nil
}

func (c *Core) SetRuntimeStatusObserver(observer func(RuntimeStatus)) {
	c.mu.Lock()
	c.statusObserver = observer
	_, status := c.runtimeStatusLocked()
	c.mu.Unlock()
	notifyRuntimeStatus(observer, status)
}

func (c *Core) GetDashboardState() DashboardState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshotLocked()
}

func (c *Core) ApplyKeyAction(index int, action config.KeyAction) (DashboardState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.config.KeyAssignments) {
		return DashboardState{}, newIndexError(index)
	}
	action = config.NormalizeAction(action)
	if err := config.ValidateAction(action); err != nil {
		return DashboardState{}, err
	}
	assignment := c.config.KeyAssignments[index]
	assignment.Action = action
	c.config.KeyAssignments[index] = assignment
	c.updateConfigStateLocked()
	return c.snapshotLocked(), nil
}

func (c *Core) ApplyKeySettings(index int, action config.KeyAction, color string, brightness uint8) (DashboardState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.config.KeyAssignments) {
		return DashboardState{}, newIndexError(index)
	}
	action = config.NormalizeAction(action)
	if err := config.ValidateAction(action); err != nil {
		return DashboardState{}, err
	}
	color = config.NormalizeColor(color)
	if err := config.ValidateColor(color); err != nil {
		return DashboardState{}, err
	}
	assignment := c.config.KeyAssignments[index]
	assignment.Action = action
	assignment.Color = color
	assignment.Brightness = &brightness
	c.config.KeyAssignments[index] = assignment
	c.updateConfigStateLocked()
	return c.snapshotLocked(), nil
}

func (c *Core) ClearKeyAction(index int) (DashboardState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.config.KeyAssignments) {
		return DashboardState{}, newIndexError(index)
	}
	assignment := c.config.KeyAssignments[index]
	assignment.Action = config.ClearAction()
	c.config.KeyAssignments[index] = assignment
	c.updateConfigStateLocked()
	return c.snapshotLocked(), nil
}

func (c *Core) SaveToDevice() (DashboardState, error) {
	c.mu.Lock()
	cfg := config.Clone(c.config)
	manager := c.device
	c.mu.Unlock()

	if err := c.store.Save(cfg); err != nil {
		return DashboardState{}, err
	}
	if err := syncKeyLEDConfig(manager, cfg); err != nil {
		return DashboardState{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.persisted = config.Clone(cfg)
	c.updateConfigStateLocked()
	return c.snapshotLocked(), nil
}

func (c *Core) handleConnected(info device.Info) {
	log.Printf("[core] dongle connected: %s", info.Path)
	c.mu.Lock()
	cfg := config.Clone(c.config)
	manager := c.device
	c.state.DongleStatus = DeviceConnectionStatus{
		State:  "connected",
		Label:  "Connected",
		Detail: "USB HID interface available",
		Path:   info.Path,
	}
	c.runtimeError = ""
	c.state.MacropadStatus = disconnectedMacropadStatus()
	observer, status := c.runtimeStatusLocked()
	c.mu.Unlock()
	notifyRuntimeStatus(observer, status)

	if err := syncKeyLEDConfig(manager, cfg); err != nil {
		log.Printf("[core] failed to sync LED config: %v", err)
	}
}

func (c *Core) handleDisconnected(info device.DisconnectInfo) {
	log.Printf("[core] dongle disconnected: %s", info.Reason)
	c.mu.Lock()
	c.state.DongleStatus = disconnectedDongleStatus(info.Reason)
	c.state.MacropadStatus = unknownMacropadStatus()
	c.state.BatteryStatus = waitingBatteryStatus()
	c.prevKeys = 0
	if info.IsError {
		c.runtimeError = info.Reason
	} else {
		c.runtimeError = ""
	}
	observer, status := c.runtimeStatusLocked()
	c.mu.Unlock()
	notifyRuntimeStatus(observer, status)
}

func (c *Core) handleDeviceLog(level, message string) {
	if level == "debug" {
		return
	}
	log.Printf("[device] %s: %s", level, message)
}

func (c *Core) handleReport(report device.Report) {
	c.mu.Lock()
	if !report.Connected {
		c.runtimeError = ""
		c.state.BatteryStatus = waitingBatteryStatus()
		c.prevKeys = 0
		c.state.MacropadStatus = disconnectedMacropadStatus()
		observer, status := c.runtimeStatusLocked()
		c.mu.Unlock()
		notifyRuntimeStatus(observer, status)
		return
	}

	report.Keys &= 0x3F
	previous := c.prevKeys
	rising := report.Keys &^ previous
	c.prevKeys = report.Keys
	c.runtimeError = ""
	c.state.MacropadStatus = connectedMacropadStatus()
	c.state.BatteryStatus = batteryStatusFromReport(report)
	assignments := config.Clone(c.config).KeyAssignments
	runner := c.runner
	observer, status := c.runtimeStatusLocked()
	c.mu.Unlock()
	notifyRuntimeStatus(observer, status)

	if runner == nil {
		return
	}

	for i := 0; i < len(assignments); i++ {
		mask := uint8(1 << i)
		if rising&mask == 0 {
			continue
		}
		assignment := assignments[i]
		if !runner.CanRun(assignment.Action) {
			continue
		}
		if err := runner.Run(i, assignment); err != nil {
			log.Printf("[core] failed to execute %s: %v", assignment.Label, err)
		}
	}
}

func (c *Core) updateConfigStateLocked() {
	c.state.Profile = c.config.Profile
	c.state.KeyAssignments = append([]config.KeyAssignment(nil), c.config.KeyAssignments...)
	c.state.Dirty = !reflect.DeepEqual(c.config, c.persisted)
	c.state.Saved = !c.state.Dirty
}

func (c *Core) snapshotLocked() DashboardState {
	state := c.state
	state.KeyAssignments = append([]config.KeyAssignment(nil), c.state.KeyAssignments...)
	return state
}

func (c *Core) runtimeStatusLocked() (func(RuntimeStatus), RuntimeStatus) {
	return c.statusObserver, RuntimeStatus{
		Dashboard:    c.snapshotLocked(),
		ErrorMessage: c.runtimeError,
	}
}

func notifyRuntimeStatus(observer func(RuntimeStatus), status RuntimeStatus) {
	if observer != nil {
		observer(status)
	}
}

func newDashboardState(cfg config.Config) DashboardState {
	cfg = config.Normalize(cfg)
	return DashboardState{
		Profile:        cfg.Profile,
		DongleStatus:   disconnectedDongleStatus(""),
		MacropadStatus: unknownMacropadStatus(),
		BatteryStatus:  waitingBatteryStatus(),
		KeyAssignments: append([]config.KeyAssignment(nil), cfg.KeyAssignments...),
		Dirty:          false,
		Saved:          true,
	}
}

func syncKeyLEDConfig(syncer keyLEDSyncer, cfg config.Config) error {
	if syncer == nil {
		return errors.New("dongle not connected")
	}

	settings, err := keyLEDSettingsFromConfig(cfg)
	if err != nil {
		return err
	}

	return syncer.SyncKeyLEDSettings(settings)
}

func keyLEDSettingsFromConfig(cfg config.Config) (device.KeyLEDSettings, error) {
	cfg = config.Normalize(cfg)
	var settings device.KeyLEDSettings

	for index, assignment := range cfg.KeyAssignments {
		color, err := parseColorBytes(assignment.Color)
		if err != nil {
			return device.KeyLEDSettings{}, fmt.Errorf("key %d: %w", index+1, err)
		}

		settings[index] = device.KeyLEDSetting{
			Color:      color,
			Brightness: config.NormalizeBrightness(assignment.Brightness),
		}
	}

	return settings, nil
}

func parseColorBytes(color string) ([3]byte, error) {
	var rgb [3]byte

	color = strings.TrimSpace(color)
	color = strings.TrimPrefix(color, "#")
	if len(color) != 6 {
		return rgb, fmt.Errorf("invalid color %q", color)
	}

	for index := range rgb {
		value, err := strconv.ParseUint(color[index*2:index*2+2], 16, 8)
		if err != nil {
			return rgb, fmt.Errorf("invalid color %q", color)
		}
		rgb[index] = byte(value)
	}

	return rgb, nil
}

func newIndexError(index int) error {
	return fmt.Errorf("invalid key index: %d", index)
}
