package main

import (
	"io/fs"
	"log"
	"log/slog"
	"sync/atomic"

	trayctrl "hpad-app/internal/tray"
	ui "hpad-app/internal/ui"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func main() {
	frontendAssets, err := fs.Sub(ui.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}

	service, err := NewAgentService()
	if err != nil {
		log.Fatal(err)
	}

	var allowQuit atomic.Bool

	app := application.New(application.Options{
		Name:        "HPAD",
		Description: "HPAD desktop configurator",
		LogLevel:    slog.LevelInfo,
		ShouldQuit: func() bool {
			return allowQuit.Load()
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
	})
	app.SetIcon(trayctrl.DefaultAppIcon())

	tray := app.SystemTray.New()
	trayController := trayctrl.NewTrayController(tray)

	trayMenu := app.NewMenu()
	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		allowQuit.Store(true)
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	service.observeRuntimeStatus(trayController.Update)

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "HPAD",
		Width:            1280,
		Height:           820,
		MinWidth:         1120,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(16, 18, 23),
		URL:              "/",
		Hidden:           true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	})

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if allowQuit.Load() {
			return
		}
		window.Hide()
		event.Cancel()
	})

	tray.OnClick(func() {
		window.UnMinimise()
		window.Show().Focus()
	})
	tray.OnRightClick(tray.OpenMenu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
