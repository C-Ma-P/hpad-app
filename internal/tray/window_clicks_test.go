package tray

import (
	"testing"
	"time"
)

func TestTrayPrimaryClickGateAllowsPlainLinuxClick(t *testing.T) {
	var scheduled func()

	gate := &trayPrimaryClickGate{
		linux: true,
		delay: linuxTrayPrimaryClickDelay,
		after: func(delay time.Duration, fn func()) {
			if delay != linuxTrayPrimaryClickDelay {
				t.Fatalf("after() delay = %v, want %v", delay, linuxTrayPrimaryClickDelay)
			}
			scheduled = fn
		},
		invoke: func(fn func()) {
			fn()
		},
	}

	opened := 0
	gate.handleClick(func() {
		opened++
	})

	if opened != 0 {
		t.Fatalf("click should wait for debounce, got %d opens", opened)
	}
	if scheduled == nil {
		t.Fatal("click should schedule a delayed open")
	}

	scheduled()

	if opened != 1 {
		t.Fatalf("scheduled click should open once, got %d", opened)
	}
}

func TestTrayPrimaryClickGateSuppressesMenuOpenClick(t *testing.T) {
	scheduled := false

	gate := &trayPrimaryClickGate{
		linux:        true,
		delay:        linuxTrayPrimaryClickDelay,
		hasMenuHooks: true,
		after: func(time.Duration, func()) {
			scheduled = true
		},
		invoke: func(fn func()) {
			fn()
		},
	}

	menuOpened := 0
	gate.handleRightClick(func() {
		menuOpened++
	})
	gate.handleMenuOpen()

	gate.handleClick(func() {
		t.Fatal("menu-open click after right click should be suppressed")
	})
	gate.handleMenuClose()

	if menuOpened != 1 {
		t.Fatalf("right click should still open the menu once, got %d", menuOpened)
	}
	if scheduled {
		t.Fatal("suppressed click should not schedule an open")
	}
}

func TestTrayPrimaryClickGateSuppressesPendingClickWhenRightClickFollows(t *testing.T) {
	var scheduled func()

	gate := &trayPrimaryClickGate{
		linux: true,
		delay: linuxTrayPrimaryClickDelay,
		after: func(_ time.Duration, fn func()) {
			scheduled = fn
		},
		invoke: func(fn func()) {
			fn()
		},
	}

	opened := 0
	gate.handleClick(func() {
		opened++
	})
	if scheduled == nil {
		t.Fatal("click should schedule a delayed open")
	}

	menuOpened := 0
	gate.handleRightClick(func() {
		menuOpened++
	})

	scheduled()

	if menuOpened != 1 {
		t.Fatalf("right click should open the menu once, got %d", menuOpened)
	}
	if opened != 0 {
		t.Fatalf("pending click should have been suppressed, got %d opens", opened)
	}
}

func TestTrayPrimaryClickGateAllowsClickAfterMenuCloseWithoutSyntheticClick(t *testing.T) {
	var scheduled func()

	gate := &trayPrimaryClickGate{
		linux:        true,
		delay:        linuxTrayPrimaryClickDelay,
		hasMenuHooks: true,
		after: func(_ time.Duration, fn func()) {
			scheduled = fn
		},
		invoke: func(fn func()) {
			fn()
		},
	}

	gate.handleRightClick(func() {})
	gate.handleMenuOpen()
	gate.handleMenuClose()

	opened := 0
	gate.handleClick(func() {
		opened++
	})
	if scheduled == nil {
		t.Fatal("click after menu close should schedule an open")
	}

	scheduled()

	if opened != 1 {
		t.Fatalf("click after menu close should open once, got %d", opened)
	}
}
