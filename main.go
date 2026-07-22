package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Single instance: if another Quokka is running, activate it and exit.
	if !ensureSingleInstance() {
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Quokka",
		Width:            680,
		Height:           120,
		MinWidth:         450,
		MinHeight:        90,
		DisableResize:    false,
		AlwaysOnTop:      true,
		Frameless:        true,
		StartHidden:      false,
		HideWindowOnClose: true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		
		// Windows-specific advanced options
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			DisableWindowIcon:    true,
			BackdropType:         windows.Acrylic, // Mica or Acrylic for modern Windows look
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
