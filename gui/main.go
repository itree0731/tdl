package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "TDL · Telegram Media Transfer",
		Width:  1440,
		Height: 900,
		MinWidth: 980,
		MinHeight: 640,
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 16, A: 1},
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup: app.startup,
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
