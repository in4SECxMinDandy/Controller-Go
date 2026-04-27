module github.com/iot-lab/agent

go 1.25.0

require (
	github.com/gorilla/websocket v1.5.3
	github.com/iot-lab/shared v0.0.0
	github.com/kbinani/screenshot v0.0.0-20240820160931-a8a2c5d0e191
	golang.org/x/image v0.39.0
	golang.org/x/sys v0.24.0
)

require (
	github.com/gen2brain/shm v0.1.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/lxn/win v0.0.0-20210218163916-a377121e959e // indirect
)

replace github.com/iot-lab/shared => ../shared
