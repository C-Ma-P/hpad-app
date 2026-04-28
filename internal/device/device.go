package device

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sstallion/go-hid"
)

const (
	vendorID          = 0xCAFE
	productID         = 0xB00B
	vendorUsagePage   = 0xFF00
	vendorUsage       = 0xFF01
	consumerUsagePage = 0x000C
)

type Info struct {
	Path            string `json:"path"`
	VendorID        uint16 `json:"vendorID"`
	ProductID       uint16 `json:"productID"`
	UsagePage       uint16 `json:"usagePage"`
	Usage           uint16 `json:"usage"`
	InterfaceNumber int    `json:"interfaceNumber"`
}

type Report struct {
	Connected      bool   `json:"connected"`
	Keys           uint8  `json:"keys"`
	EncoderDelta   int8   `json:"encoderDelta"`
	EncoderPressed bool   `json:"encoderPressed"`
	BatteryMV      uint16 `json:"batteryMV"`
	Charging       bool   `json:"charging"`
}

type Callbacks struct {
	Connected    func(Info)
	Disconnected func(string)
	Report       func(Report)
	Log          func(string, string)
}

type Manager struct {
	mu                   sync.Mutex
	callbacks            Callbacks
	running              bool
	cancel               context.CancelFunc
	done                 chan struct{}
	initOnce             sync.Once
	initErr              error
	lastScan             string
	lastOpen             string
	lastDisconnectReason string
}

// lastScanSentinel is an initial value for Manager.lastScan that can never
// be produced by summarizeCandidates, ensuring the first scan always logs.
const lastScanSentinel = "\x00"

func NewManager(callbacks Callbacks) *Manager {
	if callbacks.Connected == nil {
		callbacks.Connected = func(Info) {}
	}
	if callbacks.Disconnected == nil {
		callbacks.Disconnected = func(string) {}
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
		m.callbacks.Disconnected(err.Error())
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
			m.notifyDisconnected(err.Error())
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
				reason = "device visible in sysfs but not enumerable by hidapi (permission denied on hidraw – run: task linux:install-udev-rule, then replug)"
			}
			m.notifyDisconnected(reason)
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
			m.notifyDisconnected(err.Error())
			if !waitRetry(ctx) {
				return
			}
			continue
		}
		m.lastOpen = ""
		m.lastDisconnectReason = ""

		m.callbacks.Log("info", fmt.Sprintf("Opened HPad HID interface %s (usage 0x%04X/0x%04X, iface %d)", info.Path, info.UsagePage, info.Usage, info.InterfaceNumber))
		m.callbacks.Connected(info)
		readErr := m.readLoop(ctx, dev)
		_ = dev.Close()

		if ctx.Err() != nil {
			return
		}
		if readErr == nil {
			readErr = errors.New("device disconnected")
		}
		m.callbacks.Log("warn", fmt.Sprintf("HPad HID interface closed: %s", readErr.Error()))
		m.notifyDisconnected(readErr.Error())
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

func (m *Manager) notifyDisconnected(reason string) {
	if reason == m.lastDisconnectReason {
		return
	}
	m.lastDisconnectReason = reason
	m.callbacks.Disconnected(reason)
}

func (m *Manager) init() error {
	m.initOnce.Do(func() {
		m.initErr = hid.Init()
	})
	return m.initErr
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
			return err
		}
		if n == 0 {
			continue
		}

		report, ok := decodePayload(buf[:n])
		if !ok {
			continue
		}
		m.callbacks.Report(report)
	}
}

func enumerate() ([]Info, error) {
	var results []Info
	err := hid.Enumerate(vendorID, productID, func(info *hid.DeviceInfo) error {
		results = append(results, Info{
			Path:            info.Path,
			VendorID:        info.VendorID,
			ProductID:       info.ProductID,
			UsagePage:       info.UsagePage,
			Usage:           info.Usage,
			InterfaceNumber: info.InterfaceNbr,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		ri := candidateRank(results[i])
		rj := candidateRank(results[j])
		if ri != rj {
			return ri < rj
		}
		return results[i].Path < results[j].Path
	})
	return results, nil
}

func openCandidate(candidates []Info) (*hid.Device, Info, error) {
	var failures []string
	for _, candidate := range candidates {
		dev, err := hid.OpenPath(candidate.Path)
		if err == nil {
			return dev, candidate, nil
		}
		failures = append(failures, formatOpenError(candidate, err))
	}
	if len(failures) == 0 {
		return nil, Info{}, errors.New("device not found")
	}
	return nil, Info{}, errors.New(strings.Join(failures, "; "))
}

func formatOpenError(candidate Info, err error) string {
	message := err.Error()
	if isPermissionError(err) {
		message = fmt.Sprintf("%s (permission denied opening hidraw; add a udev rule for VID 0x%04X PID 0x%04X or grant access to %s)", message, candidate.VendorID, candidate.ProductID, candidate.Path)
	}
	return fmt.Sprintf("%s (usage 0x%04X/0x%04X, iface %d): %s", candidate.Path, candidate.UsagePage, candidate.Usage, candidate.InterfaceNumber, message)
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "permission denied")
}

func candidateRank(info Info) int {
	switch {
	case info.UsagePage == vendorUsagePage && info.Usage == vendorUsage:
		return 0
	case info.UsagePage == vendorUsagePage:
		return 1
	case info.UsagePage == consumerUsagePage:
		return 3
	default:
		return 2
	}
}

// sysfsHIDMatch reports whether any hidraw device in /sys/class/hidraw
// has a parent HID device matching vid and pid. On Linux, hidapi tries to
// open each hidraw node during enumeration; if the node is root-only the
// device is silently skipped and Enumerate returns an empty list even
// though the hardware is present. Cross-platform: returns false outside Linux.
func sysfsHIDMatch(vid, pid uint16) bool {
	entries, err := os.ReadDir("/sys/class/hidraw")
	if err != nil {
		return false
	}
	target := fmt.Sprintf("HID_ID=0003:%08X:%08X", vid, pid)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join("/sys/class/hidraw", e.Name(), "device", "uevent"))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), target) {
			return true
		}
	}
	return false
}

func summarizeCandidates(candidates []Info) string {
	if len(candidates) == 0 {
		return ""
	}
	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parts = append(parts, fmt.Sprintf("%s (usage 0x%04X/0x%04X, iface %d)", candidate.Path, candidate.UsagePage, candidate.Usage, candidate.InterfaceNumber))
	}
	return strings.Join(parts, ", ")
}

func decodePayload(raw []byte) (Report, bool) {
	payload := raw
	switch {
	case len(raw) >= 8 && raw[0] == 0:
		payload = raw[1:8]
	case len(raw) == 5 && raw[0] == 0:
		payload = raw[1:5]
	case len(raw) == 4 && raw[0] == 0 && !(raw[1] == 0 && raw[2] == 0 && raw[3] == 0):
		payload = raw[1:]
	}
	if len(payload) >= 7 {
		return Report{
			Connected:      payload[0] != 0,
			Keys:           payload[1],
			EncoderDelta:   int8(payload[2]),
			EncoderPressed: payload[3] != 0,
			BatteryMV:      uint16(payload[4]) | (uint16(payload[5]) << 8),
			Charging:       payload[6] != 0,
		}, true
	}
	if len(payload) >= 4 {
		return Report{
			Connected:      payload[0] != 0,
			Keys:           payload[1],
			EncoderDelta:   int8(payload[2]),
			EncoderPressed: payload[3] != 0,
		}, true
	}
	if len(payload) < 3 {
		return Report{}, false
	}
	return Report{
		Connected:      true,
		Keys:           payload[0],
		EncoderDelta:   int8(payload[1]),
		EncoderPressed: payload[2] != 0,
	}, true
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
