package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
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
	State     string `json:"state"`
	Label     string `json:"label"`
	Detail    string `json:"detail"`
	BatteryMV int    `json:"batteryMV"`
	Charging  bool   `json:"charging"`
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

type Core struct {
	mu        sync.RWMutex
	store     *config.Store
	device    *device.Manager
	runner    *actions.Runner
	cancel    context.CancelFunc
	config    config.Config
	persisted config.Config
	state     DashboardState
	prevKeys  uint8
	started   bool
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
	c.state = newDashboardState(c.config)
	c.state.Dirty = !reflect.DeepEqual(c.config, c.persisted)
	c.state.Saved = !c.state.Dirty
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if deviceManager != nil {
		deviceManager.Stop()
	}
	return nil
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
	defer c.mu.Unlock()
	if err := c.store.Save(c.config); err != nil {
		return DashboardState{}, err
	}
	c.persisted = config.Clone(c.config)
	c.updateConfigStateLocked()
	return c.snapshotLocked(), nil
}

func (c *Core) handleConnected(info device.Info) {
	log.Printf("[core] dongle connected: %s", info.Path)
	c.mu.Lock()
	c.state.DongleStatus = DeviceConnectionStatus{
		State:  "connected",
		Label:  "Connected",
		Detail: "USB HID interface available",
		Path:   info.Path,
	}
	c.state.MacropadStatus = disconnectedMacropadStatus()
	c.mu.Unlock()
}

func (c *Core) handleDisconnected(reason string) {
	log.Printf("[core] dongle disconnected: %s", reason)
	c.mu.Lock()
	c.state.DongleStatus = disconnectedDongleStatus(reason)
	c.state.MacropadStatus = unknownMacropadStatus()
	c.state.BatteryStatus = waitingBatteryStatus()
	c.prevKeys = 0
	c.mu.Unlock()
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
		c.state.BatteryStatus = waitingBatteryStatus()
		c.prevKeys = 0
		c.state.MacropadStatus = disconnectedMacropadStatus()
		c.mu.Unlock()
		return
	}

	report.Keys &= 0x3F
	previous := c.prevKeys
	rising := report.Keys &^ previous
	c.prevKeys = report.Keys
	c.state.MacropadStatus = connectedMacropadStatus()
	c.state.BatteryStatus = batteryStatusFromReport(report)
	assignments := config.Clone(c.config).KeyAssignments
	runner := c.runner
	c.mu.Unlock()

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

func disconnectedDongleStatus(reason string) DeviceConnectionStatus {
	detail := "USB HID dongle not detected"
	if reason != "" {
		detail = reason
	}
	return DeviceConnectionStatus{
		State:  "not_detected",
		Label:  "Not Detected",
		Detail: detail,
	}
}

func unknownMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "unknown",
		Label:  "Unknown",
		Detail: "Waiting for dongle",
	}
}

func disconnectedMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "disconnected",
		Label:  "Disconnected",
		Detail: "Dongle is online, waiting for the wireless macropad",
	}
}

func connectedMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "connected",
		Label:  "Connected",
		Detail: "Wireless macropad is reporting through the dongle",
	}
}

func waitingBatteryStatus() BatteryStatus {
	return BatteryStatus{
		State:     "waiting",
		Label:     "--.- V",
		Detail:    "Waiting for device report",
		BatteryMV: 0,
		Charging:  false,
	}
}

func batteryStatusFromReport(report device.Report) BatteryStatus {
	if report.BatteryMV == 0 {
		return waitingBatteryStatus()
	}

	detail := "Running on battery power"
	state := "connected"
	if report.Charging {
		detail = "Charging over USB"
		state = "charging"
	}

	return BatteryStatus{
		State:     state,
		Label:     formatBatteryMV(report.BatteryMV),
		Detail:    detail,
		BatteryMV: int(report.BatteryMV),
		Charging:  report.Charging,
	}
}

func formatBatteryMV(batteryMV uint16) string {
	whole := batteryMV / 1000
	fraction := (batteryMV % 1000) / 10
	return fmt.Sprintf("%d.%02d V", whole, fraction)
}

func newIndexError(index int) error {
	return errors.New("invalid key index")
}
