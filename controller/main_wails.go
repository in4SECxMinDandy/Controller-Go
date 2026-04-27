//go:build wails

// Build với tag `wails` (vd: `go build -tags wails ...` hoặc `wails build`):
// chạy app dưới dạng GUI Wails kèm HTTP/WSS server chạy nền.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Khi Wails CLI generate bindings, nó chạy binary với flag -wailsbindings.
	// Trong chế độ đó: KHÔNG start HTTP server (sẽ đụng port), chỉ để Wails
	// reflect ra binding rồi exit.
	bindingsMode := false
	for _, a := range os.Args[1:] {
		if a == "-wailsbindings" || a == "--wailsbindings" {
			bindingsMode = true
			break
		}
	}

	// Cho phép WebView2 chấp nhận self-signed cert của controller.
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
		"--ignore-certificate-errors --allow-insecure-localhost")

	appCtx := SetupApp()

	if !bindingsMode {
		// HTTP/WSS server chạy nền — agent dial vào, browser xem fallback.
		// KHÔNG fatal khi listen lỗi: trong chế độ Wails bindings generation,
		// Wails có thể đã chạy trước đó để dump bindings và port có thể bận.
		go func() {
			if err := appCtx.ListenBlocking(); err != nil {
				log.Printf("HTTP server stopped: %v", err)
			}
		}()
	}

	// Wails GUI là main thread.
	app := NewApp(appCtx.Hub)
	err := wails.Run(&options.App{
		Title:  "IoT Lab Controller",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 17, B: 17, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
