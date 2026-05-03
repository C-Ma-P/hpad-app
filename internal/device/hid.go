package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/sstallion/go-hid"
)

func isTransientReadError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, syscall.EINTR.Error()) || strings.Contains(message, "eintr")
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
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join("/sys/class/hidraw", entry.Name(), "device", "uevent"))
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
