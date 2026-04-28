package main

import (
	"embed"
	"io/fs"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var embeddedAssets embed.FS

func main() {
	frontendAssets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	service, err := NewAgentService()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "HPAD",
		Description: "HPAD desktop configurator",
		LogLevel:    slog.LevelInfo,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "HPAD",
		Width:            1280,
		Height:           820,
		MinWidth:         1120,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(16, 18, 23),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
