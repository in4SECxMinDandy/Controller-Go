# Thiết kế hệ thống — Remote Control Lab

## 1. Mục tiêu

- Agent **headless**, nhẹ (~5–10 MB), không UI, không icon tray.
- Controller có **GUI** (Wails) hiển thị danh sách agent + khung điều khiển.
- Giao thức **WSS port 443** — qua firewall, qua proxy doanh nghiệp.
- Agent **outbound only** (không mở port lắng nghe).

## 2. Thành phần

### 2.1 Agent (`agent/`)
- Ngôn ngữ: Go.
- Build: `go build -ldflags="-H windowsgui -s -w" -trimpath -o agent.exe`
- Modules:
  - `transport/`: WebSocket client, TLS, auto-reconnect, đọc `HTTPS_PROXY`.
  - `capture/`: chụp màn hình bằng `kbinani/screenshot`, encode JPEG (chất lượng 60).
  - `input/`: nhận lệnh, gọi `SendInput` qua `golang.org/x/sys/windows`.
  - `persistence/`: tự đăng ký autostart vào `HKCU\...\Run` ngay lần đầu chạy (không cần admin/UAC).
  - `inputlock/`: low-level keyboard/mouse hook chặn user thao tác local khi controller đang điều khiển; cho phép `SendInput` injected đi qua.
  - `main.go`: vòng đời tiến trình.

#### Persistence (autostart)
- Ghi `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`, value name `IoTLabAgent`, value data = đường dẫn `agent.exe` (có quote).
- Idempotent: chỉ ghi nếu chưa có hoặc đường dẫn khác.
- Không cần quyền admin → không có UAC popup.
- Tương ứng MITRE ATT&CK **T1547.001** — chủ đích để hệ thống phát hiện trong các paper PDF có thể nhận diện.

### 2.2 Controller (`controller/`)
- Ngôn ngữ: Go + Wails v2 (WebView2 frontend).
- Modules:
  - `server/`: WSS server (TLS self-signed cho lab), quản lý kết nối agent.
  - `frontend/`: HTML/CSS/JS — danh sách agent, canvas hiển thị màn hình, bắt input.
  - `main.go`: bootstrap Wails app + WSS server.

### 2.3 Shared (`shared/`)
- `protocol.go`: định nghĩa message JSON dùng chung 2 bên.

## 3. Protocol

Tất cả message dạng JSON, gói trong WebSocket text/binary frame.

### 3.1 Message envelope
```json
{ "type": "<kind>", "id": "<uuid optional>", "payload": { ... } }
```

### 3.2 Các loại message

| `type` | Hướng | Payload |
|---|---|---|
| `hello` | Agent → Controller | `{ "agent_id", "hostname", "os", "version" }` |
| `welcome` | Controller → Agent | `{ "session_id", "fps_target", "jpeg_quality" }` |
| `heartbeat` | Agent ↔ Controller | `{ "ts" }` mỗi 20s |
| `frame` | Agent → Controller | binary: `[8 byte header][JPEG bytes]` |
| `mouse` | Controller → Agent | `{ "x", "y", "button", "action" }` (action: move/down/up/scroll) |
| `key` | Controller → Agent | `{ "vk", "action" }` (action: down/up) |
| `shutdown` | Controller → Agent | `{}` — agent thoát |

Khung hình dùng **binary frame** để giảm overhead base64.

## 4. Bảo mật (lab)

- TLS 1.3 với self-signed cert; Agent **pin** SHA256 fingerprint của cert Controller.
- Token chia sẻ trước (`AGENT_TOKEN` env) gửi trong message `hello`.
- Mọi traffic mã hoá; không log payload nhạy cảm.

## 5. Vòng đời kết nối Agent

```
start
  └─▶ load config (controller URL, token, cert pin)
        └─▶ dial WSS (qua HTTPS_PROXY nếu có)
              ├─ fail ─▶ backoff 1→2→4→…→60s ─▶ retry
              └─ ok ─▶ send hello
                       └─▶ recv welcome
                             └─▶ start: capture loop + input loop + heartbeat
                                   └─ disconnect ─▶ backoff retry
```

## 6. Hiệu năng mục tiêu (MVP)

- FPS: 8–15 (tuỳ độ phân giải).
- Băng thông: 0.5–2 Mbps với JPEG q=60.
- Độ trễ click→hiển thị: < 200ms trên LAN.
- RAM agent idle: < 20 MB.

## 7. Roadmap

1. **MVP-1**: WSS + hello/heartbeat + auto-reconnect.
2. **MVP-2**: capture + frame stream → controller hiển thị.
3. **MVP-3**: mouse + keyboard injection.
4. **MVP-4**: Wails GUI hoàn chỉnh, multi-agent.
5. **v2**: WebRTC cho video low-latency, fallback WSS.
6. **v3**: file transfer, clipboard sync.

## 8. Cấu trúc thư mục

```
IoT/
├─ README.md
├─ design.md
├─ go.work                    (Go workspace cho 2 module)
├─ shared/
│  ├─ go.mod
│  └─ protocol.go
├─ agent/
│  ├─ go.mod
│  ├─ main.go
│  ├─ config.go
│  ├─ transport/ws.go
│  ├─ capture/screen.go
│  └─ input/sendinput_windows.go
└─ controller/
   ├─ go.mod
   ├─ main.go
   ├─ server/hub.go
   ├─ server/ws.go
   └─ frontend/
      ├─ index.html
      ├─ app.js
      └─ style.css
```
