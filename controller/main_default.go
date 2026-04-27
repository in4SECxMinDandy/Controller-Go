//go:build !wails

// Default build (không build tag): chạy server thuần, không có GUI.
package main

import "log"

func main() {
	app := SetupApp()
	if err := app.ListenBlocking(); err != nil {
		log.Fatal(err)
	}
}
