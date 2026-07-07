package desktopapp

import (
	"io/fs"
	"log"
	"log/slog"
	"sync/atomic"

	agentservice "hpad-app/internal/agentservice"
	"hpad-app/internal/appdirs"
	"hpad-app/internal/singleinstance"
	trayctrl "hpad-app/internal/tray"
	ui "hpad-app/internal/ui"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func Run() error {
	paths, err := appdirs.Resolve()
	if err != nil {
		return err
	}

	log.Printf("[startup] HPAD starting")
	log.Printf("[startup] config dir: %s", paths.ConfigDir)
	log.Printf("[startup] state dir: %s", paths.StateDir)
	log.Printf("[startup] runtime dir: %s", paths.RuntimeDir)
	log.Printf("[startup] lock file: %s", paths.LockFile)

	if err := appdirs.Ensure(paths); err != nil {
		return err
	}

	lock, acquired, err := singleinstance.Acquire(paths.LockFile)
	if err != nil {
		return err
	}
	if !acquired {
		log.Printf("[startup] HPAD is already running")
		return nil
	}
	defer func() {
		if err := lock.Release(); err != nil {
			log.Printf("[startup] failed to release lock: %v", err)
		}
	}()
	defer log.Printf("[startup] HPAD shutting down")

	frontendAssets, err := fs.Sub(ui.Assets, "dist")
	if err != nil {
		return err
	}

	service, err := agentservice.NewAgentService()
	if err != nil {
		return err
	}

	var allowQuit atomic.Bool

	app := application.New(application.Options{
		Name:        "HPAD",
		Description: "HPAD desktop configurator",
		Icon:        trayctrl.DefaultAppIcon(),
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

	tray := app.SystemTray.New()
	trayController := trayctrl.NewTrayController(tray)

	trayMenu := app.NewMenu()
	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		allowQuit.Store(true)
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	agentservice.ObserveRuntimeStatus(service, trayController.Update)

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "HPAD",
		Width:            424,
		Height:           860,
		MinWidth:         392,
		MinHeight:        760,
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

	trayctrl.BindWindowToTrayClicks(tray, window)

	return app.Run()
}
