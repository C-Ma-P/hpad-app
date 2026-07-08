package device

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sstallion/go-hid"
)

type Manager struct {
	mu                     sync.Mutex
	writeMu                sync.Mutex
	callbacks              Callbacks
	running                bool
	cancel                 context.CancelFunc
	done                   chan struct{}
	usbDevice              *hid.Device
	bleClient              desktopBLETransport
	activeSource           Source
	initOnce               sync.Once
	initErr                error
	lastScan               string
	lastOpen               string
	lastDisconnectReason   map[Source]string
	lastDisconnectWasError map[Source]bool
}

type desktopBLETransport interface {
	Start(context.Context) error
	Stop()
	SyncKeyLEDSettings(KeyLEDSettings) error
}

// lastScanSentinel is an initial value for Manager.lastScan that can never
// be produced by summarizeCandidates, ensuring the first scan always logs.
const lastScanSentinel = "\x00"

func NewManager(callbacks Callbacks) *Manager {
	if callbacks.Connected == nil {
		callbacks.Connected = func(Info) {}
	}
	if callbacks.Disconnected == nil {
		callbacks.Disconnected = func(DisconnectInfo) {}
	}
	if callbacks.Report == nil {
		callbacks.Report = func(Report) {}
	}
	if callbacks.Log == nil {
		callbacks.Log = func(string, string) {}
	}
	manager := &Manager{
		callbacks:              callbacks,
		lastScan:               lastScanSentinel,
		lastDisconnectReason:   map[Source]string{},
		lastDisconnectWasError: map[Source]bool{},
	}
	manager.bleClient = newDesktopBLEClient(callbacks, manager.emitReport)
	return manager
}

func (m *Manager) Start(parent context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup

	m.running = true
	m.cancel = cancel
	m.done = make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.usbLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		m.bleLoop(ctx)
	}()
	go func() {
		wg.Wait()
		m.mu.Lock()
		m.running = false
		close(m.done)
		m.mu.Unlock()
	}()
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	done := m.done
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if m.bleClient != nil {
		m.bleClient.Stop()
	}
	if done != nil {
		<-done
	}
}

func (m *Manager) usbLoop(ctx context.Context) {
	if err := m.init(); err != nil {
		log.Printf("[device] hid.Init failed: %v", err)
		m.sourceDisconnected(DisconnectInfo{Source: SourceUSB, Reason: err.Error(), IsError: true})
		return
	}
	m.callbacks.Log("info", "HID manager initialized")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[device] context cancelled")
			return
		default:
		}

		candidates, err := enumerate()
		if err != nil {
			m.callbacks.Log("error", fmt.Sprintf("HID enumerate failed: %v", err))
			m.sourceDisconnected(DisconnectInfo{Source: SourceUSB, Reason: err.Error(), IsError: true})
			if !waitRetry(ctx) {
				return
			}
			continue
		}

		m.logScan(candidates)
		if len(candidates) == 0 {
			sysfsMatch := sysfsHIDMatch(vendorID, productID)
			reason := "device not found"
			if sysfsMatch {
				reason = "device visible in sysfs but not enumerable by hidapi (permission denied on hidraw - run: task linux:install-udev-rule, then replug)"
			}
			m.sourceDisconnected(DisconnectInfo{Source: SourceUSB, Reason: reason})
			if !waitRetry(ctx) {
				return
			}
			continue
		}

		dev, info, err := openCandidate(candidates)
		if err != nil {
			if err.Error() != m.lastOpen {
				m.callbacks.Log("error", fmt.Sprintf("Failed to open HPad HID interface(s): %s", err.Error()))
				m.lastOpen = err.Error()
			}
			m.sourceDisconnected(DisconnectInfo{Source: SourceUSB, Reason: err.Error(), IsError: true})
			if !waitRetry(ctx) {
				return
			}
			continue
		}
		m.lastOpen = ""
		m.lastDisconnectReason[SourceUSB] = ""
		info.Source = SourceUSB
		m.setUSBDevice(dev)

		m.callbacks.Log("info", fmt.Sprintf("Opened HPad HID interface %s (usage 0x%04X/0x%04X, iface %d)", info.Path, info.UsagePage, info.Usage, info.InterfaceNumber))
		m.callbacks.Connected(info)
		readErr := m.readLoop(ctx, dev)
		m.setUSBDevice(nil)
		_ = dev.Close()

		if ctx.Err() != nil {
			return
		}
		if readErr == nil {
			readErr = errors.New("device disconnected")
		}
		m.callbacks.Log("warn", fmt.Sprintf("HPad HID interface closed: %s", readErr.Error()))
		m.sourceDisconnected(DisconnectInfo{Source: SourceUSB, Reason: readErr.Error()})
		if !waitRetry(ctx) {
			return
		}
	}
}

func (m *Manager) bleLoop(ctx context.Context) {
	if m.bleClient == nil {
		return
	}
	if err := m.bleClient.Start(ctx); err != nil && ctx.Err() == nil {
		m.sourceDisconnected(DisconnectInfo{Source: SourceBLE, Reason: err.Error(), IsError: true})
	}
}

func (m *Manager) logScan(candidates []Info) {
	summary := summarizeCandidates(candidates)
	if summary == m.lastScan {
		return
	}
	m.lastScan = summary
	if summary == "" {
		m.callbacks.Log("debug", fmt.Sprintf("No HPad HID interfaces found for VID/PID 0x%04X/0x%04X", vendorID, productID))
		return
	}
	m.callbacks.Log("debug", fmt.Sprintf("Discovered %d HPad HID interface(s): %s", len(candidates), summary))
}

func (m *Manager) notifyDisconnected(info DisconnectInfo) {
	if info.Source == "" {
		info.Source = SourceUSB
	}
	if info.Reason == m.lastDisconnectReason[info.Source] &&
		info.IsError == m.lastDisconnectWasError[info.Source] {
		return
	}
	m.lastDisconnectReason[info.Source] = info.Reason
	m.lastDisconnectWasError[info.Source] = info.IsError
	m.callbacks.Disconnected(info)
}

func (m *Manager) init() error {
	m.initOnce.Do(func() {
		m.initErr = hid.Init()
	})
	return m.initErr
}

func (m *Manager) SyncKeyLEDSettings(settings KeyLEDSettings) error {
	m.mu.Lock()
	source := m.activeSource
	dev := m.usbDevice
	bleClient := m.bleClient
	m.mu.Unlock()

	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	switch source {
	case SourceUSB:
		if dev == nil {
			return errors.New("USB dongle is not connected")
		}
		return syncUSBKeyLEDSettings(dev, settings)
	case SourceBLE:
		if bleClient == nil {
			return errors.New("Desktop BLE transport is unavailable")
		}
		return bleClient.SyncKeyLEDSettings(settings)
	default:
		return errors.New("no active macropad transport")
	}
}

func syncUSBKeyLEDSettings(dev *hid.Device, settings KeyLEDSettings) error {
	report := encodeKeyLEDConfigReport(settings)
	n, err := dev.SendOutputReport(report[:])
	if err != nil {
		return err
	}
	if n != len(report) {
		return fmt.Errorf("short HID output report write: %d/%d", n, len(report))
	}

	return nil
}

func (m *Manager) setUSBDevice(dev *hid.Device) {
	m.mu.Lock()
	m.usbDevice = dev
	m.mu.Unlock()
}

func (m *Manager) emitReport(source Source, report Report) {
	report.Source = source
	if !m.claimSource(source, report.Connected) {
		m.callbacks.Log("debug", fmt.Sprintf("Ignoring %s report while %s is active", source, m.currentActiveSource()))
		return
	}
	m.callbacks.Report(report)
}

func (m *Manager) claimSource(source Source, connected bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !connected {
		if m.activeSource == source {
			m.activeSource = ""
			m.callbacks.Log("info", fmt.Sprintf("Released active HPAD source %s", source))
			return true
		}
		return false
	}
	if m.activeSource == "" {
		m.activeSource = source
		m.callbacks.Log("info", fmt.Sprintf("Latched active HPAD source %s", source))
		return true
	}

	return m.activeSource == source
}

func (m *Manager) sourceDisconnected(info DisconnectInfo) {
	if info.Source == "" {
		info.Source = SourceUSB
	}
	m.mu.Lock()
	wasActive := m.activeSource == info.Source
	if wasActive {
		m.activeSource = ""
	}
	m.mu.Unlock()

	if wasActive {
		m.callbacks.Log("info", fmt.Sprintf("Released active HPAD source %s after disconnect", info.Source))
		m.callbacks.Report(Report{Source: info.Source, Connected: false})
	}
	m.notifyDisconnected(info)
}

func (m *Manager) currentActiveSource() Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeSource
}

func (m *Manager) readLoop(ctx context.Context, dev *hid.Device) error {
	buf := make([]byte, 64)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := dev.ReadWithTimeout(buf, 500*time.Millisecond)
		if errors.Is(err, hid.ErrTimeout) {
			continue
		}
		if err != nil {
			if isTransientReadError(err) {
				continue
			}
			return err
		}
		if n == 0 {
			continue
		}

		report, ok := decodeVendorInputReport(buf[:n])
		if !ok {
			continue
		}
		m.emitReport(SourceUSB, report)
	}
}

func waitRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
