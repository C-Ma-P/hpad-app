package device

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestFormatOpenErrorIncludesPermissionHint(t *testing.T) {
	candidate := Info{
		Path:            "/dev/hidraw0",
		VendorID:        vendorID,
		ProductID:       productID,
		UsagePage:       vendorUsagePage,
		Usage:           vendorUsage,
		InterfaceNumber: 1,
	}

	message := formatOpenError(candidate, os.ErrPermission)

	for _, want := range []string{
		"/dev/hidraw0",
		"permission denied opening hidraw",
		"udev rule",
		"0xCAFE",
		"0xB00B",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in %q", want, message)
		}
	}
}

func TestIsTransientReadError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eintr string", err: errors.New("Interrupted system call"), want: true},
		{name: "eintr token", err: errors.New("poll failed: EINTR"), want: true},
		{name: "syscall eintr", err: syscall.EINTR, want: true},
		{name: "disconnect", err: errors.New("device disconnected"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isTransientReadError(test.err)
			if got != test.want {
				t.Fatalf("isTransientReadError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
