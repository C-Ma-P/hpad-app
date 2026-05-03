package tray

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const linuxTrayPrimaryClickDelay = 200 * time.Millisecond

type trayPrimaryClickGate struct {
	linux        bool
	delay        time.Duration
	after        func(time.Duration, func())
	invoke       func(func())
	hasMenuHooks bool

	generation        atomic.Uint64
	menuOpen          atomic.Bool
	suppressNextClick atomic.Bool
}

func newTrayPrimaryClickGate() *trayPrimaryClickGate {
	return &trayPrimaryClickGate{
		linux: runtime.GOOS == "linux",
		delay: linuxTrayPrimaryClickDelay,
		after: func(delay time.Duration, fn func()) {
			time.AfterFunc(delay, fn)
		},
		invoke: application.InvokeAsync,
	}
}

func BindWindowToTrayClicks(tray *application.SystemTray, window application.Window) {
	gate := newTrayPrimaryClickGate()
	if gate.linux {
		gate.hasMenuHooks = setSystemTrayMenuHooks(tray, gate.handleMenuOpen, gate.handleMenuClose)
	}

	tray.OnClick(func() {
		gate.handleClick(func() {
			window.UnMinimise()
			window.Show().Focus()
		})
	})

	tray.OnRightClick(func() {
		gate.handleRightClick(tray.OpenMenu)
	})
}

func (g *trayPrimaryClickGate) handleClick(action func()) {
	if !g.linux {
		action()
		return
	}

	if g.suppressNextClick.Swap(false) {
		return
	}

	if g.menuOpen.Load() {
		return
	}

	generation := g.generation.Load()
	g.after(g.delay, func() {
		if g.generation.Load() != generation {
			return
		}
		if g.menuOpen.Load() || g.suppressNextClick.Load() {
			return
		}
		g.invoke(action)
	})
}

func (g *trayPrimaryClickGate) handleRightClick(action func()) {
	if g.linux {
		generation := g.generation.Add(1)
		g.suppressNextClick.Store(true)
		if !g.hasMenuHooks {
			g.after(g.delay, func() {
				if g.generation.Load() != generation || g.menuOpen.Load() {
					return
				}
				g.suppressNextClick.Store(false)
			})
		}
	}
	action()
}

func (g *trayPrimaryClickGate) handleMenuOpen() {
	if !g.linux {
		return
	}
	g.menuOpen.Store(true)
}

func (g *trayPrimaryClickGate) handleMenuClose() {
	if !g.linux {
		return
	}
	g.menuOpen.Store(false)
	g.suppressNextClick.Store(false)
}
