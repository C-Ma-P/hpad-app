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
	device                 *hid.Device
	initOnce               sync.Once
	initErr                error
	lastScan               string
	lastOpen               string
	lastDisconnectReason   string
	lastDisconnectWasError bool
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
	return &Manager{callbacks: callbacks, lastScan: lastScanSentinel}
}

func (m *Manager) Start(parent context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	m.running = true
	m.cancel = cancel
	m.done = make(chan struct{})
	go m.loop(ctx)
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
	if done != nil {
		<-done
	}
}

func (m *Manager) loop(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		close(m.done)
		m.mu.Unlock()
	}()

	if err := m.init(); err != nil {
		log.Printf("[device] hid.Init failed: %v", err)
		m.callbacks.Disconnected(DisconnectInfo{Reason: err.Error(), IsError: true})
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
			m.notifyDisconnected(DisconnectInfo{Reason: err.Error(), IsError: true})
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
			m.notifyDisconnected(DisconnectInfo{Reason: reason})
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
			m.notifyDisconnected(DisconnectInfo{Reason: err.Error(), IsError: true})
			if !waitRetry(ctx) {
				return
			}
			continue
		}
		m.lastOpen = ""
		m.lastDisconnectReason = ""
		m.setDevice(dev)

		m.callbacks.Log("info", fmt.Sprintf("Opened HPad HID interface %s (usage 0x%04X/0x%04X, iface %d)", info.Path, info.UsagePage, info.Usage, info.InterfaceNumber))
		m.callbacks.Connected(info)
		readErr := m.readLoop(ctx, dev)
		m.setDevice(nil)
		_ = dev.Close()

		if ctx.Err() != nil {
			return
		}
		if readErr == nil {
			readErr = errors.New("device disconnected")
		}
		m.callbacks.Log("warn", fmt.Sprintf("HPad HID interface closed: %s", readErr.Error()))
		m.notifyDisconnected(DisconnectInfo{Reason: readErr.Error()})
		if !waitRetry(ctx) {
			return
		}
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
	if info.Reason == m.lastDisconnectReason && info.IsError == m.lastDisconnectWasError {
		return
	}
	m.lastDisconnectReason = info.Reason
	m.lastDisconnectWasError = info.IsError
	m.callbacks.Disconnected(info)
}

func (m *Manager) init() error {
	m.initOnce.Do(func() {
		m.initErr = hid.Init()
	})
	return m.initErr
}

func (m *Manager) SyncKeyLEDSettings(settings KeyLEDSettings) error {
	report := encodeKeyLEDConfigReport(settings)

	m.mu.Lock()
	dev := m.device
	m.mu.Unlock()
	if dev == nil {
		return errors.New("dongle not connected")
	}

	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	n, err := dev.SendOutputReport(report[:])
	if err != nil {
		return err
	}
	if n != len(report) {
		return fmt.Errorf("short HID output report write: %d/%d", n, len(report))
	}

	return nil
}

func (m *Manager) setDevice(dev *hid.Device) {
	m.mu.Lock()
	m.device = dev
	m.mu.Unlock()
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
		m.callbacks.Report(report)
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
