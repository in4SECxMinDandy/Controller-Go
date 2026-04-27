# Controller-Go — IoT Remote Control (Lab Project)

Hệ thống điều khiển máy tính từ xa, viết bằng **Go** (agent headless) + **Wails v2** (controller GUI), phục vụ mục đích **nghiên cứu phát hiện spyware/RAT ở tầng firmware (UEFI)**.

> ⚠️ **Tuyên bố sử dụng**: Dự án chỉ dùng trong môi trường **lab có sự đồng ý** của người sở hữu máy. Tuyệt đối **không** dùng để truy cập / giám sát máy tính của người khác trái phép. Tác giả không chịu trách nhiệm cho mọi hành vi sử dụng sai mục đích.

---

## 1. Bối cảnh & Mục tiêu

Đề tài lab tham khảo 2 paper về phát hiện spyware ở tầng firmware:

- *A spyware detection method for firmware based cyber attack*
- *UEFI Firmware Runtime Observability Layer for Detection*

Để kiểm thử các giải pháp phát hiện đó, cần một **mẫu RAT/remote-control thật** chạy được trên Windows với hành vi tiêu biểu của spyware (autostart, outbound C2, screen capture, input injection). Repo này hiện thực hoá mẫu đó.

**Mục tiêu kỹ thuật:**

| Yêu cầu | Hiện trạng |
|---|---|
| Agent headless (không UI, không tray) | ✅ `-H windowsgui` |
| Agent một file `.exe`, kích thước nhỏ | ✅ ~5–10 MB (`-s -w -trimpath`) |
| Outbound-only (không mở port) | ✅ Dial WSS ra `:8443` / `:443` |
| Vượt firewall / NAT / proxy | ✅ Đọc `HTTPS_PROXY` env |
| Auto-reconnect | ✅ Exponential backoff 1→2→4→…→60s |
| TLS + cert pinning | ✅ TLS 1.3, SHA256 pin cert controller |
| Persistence (autostart) | ✅ `HKCU\…\Run`, không cần admin |
| Khoá input người dùng khi đang điều khiển từ xa | ✅ Low-level keyboard/mouse hook |
| Multi-agent management | ✅ Hub quản lý đồng thời nhiều agent |
| Controller có GUI hiện đại | ✅ Wails v2 (WebView2) |

---

## 2. Kiến trúc tổng quan

```
                      Outbound TLS 1.3 (WSS / port 8443)
┌────────────────────────────┐                              ┌──────────────────────────────┐
│        AGENT (.exe)        │ ─── hello + heartbeat ─────▶ │     CONTROLLER (Wails GUI)   │
│  ─ headless, không icon     │ ◀── frame JPEG (binary) ─── │  ─ Hub: quản lý agents       │
│  ─ Auto-reconnect           │ ─── khung hình live ─────── │  ─ Wails GUI: list + viewer  │
│  ─ Persistence HKCU\…\Run   │ ◀── mouse / key injection ─ │  ─ HTTP fallback /view/{id}  │
│  ─ Input lock khi remote    │                              │  ─ Self-signed cert auto-gen │
└────────────────────────────┘                              └──────────────────────────────┘
        │                                                               │
        ├─ capture/  (kbinani/screenshot → JPEG q60)                    ├─ server/  (WSS hub, TLS)
        ├─ input/    (Win32 SendInput injection)                        ├─ frontend/ (HTML/JS/CSS)
        ├─ inputlock/(low-level hook chặn user local)                   └─ control/  (controller WS)
        ├─ persistence/ (HKCU autostart — MITRE T1547.001)
        └─ transport/ (WSS client + cert pin)
```

Toàn bộ **3 module Go** dùng chung qua `go.work`:

```
IoT/
├─ shared/        protocol message dùng chung (JSON envelope)
├─ agent/         headless agent (Windows)
└─ controller/    Wails GUI controller + WSS server
```

---

## 3. Cấu trúc thư mục chi tiết

```
IoT/
├─ README.md                      ← file này
├─ design.md                      ← thiết kế chi tiết (protocol, lifecycle, bảo mật)
├─ go.work                        ← Go workspace (3 modules)
│
├─ shared/
│  ├─ go.mod
│  └─ protocol.go                 ← định nghĩa Envelope, message types
│
├─ agent/
│  ├─ go.mod / go.sum
│  ├─ main.go                     ← entry point, vòng đời tiến trình
│  ├─ config.go                   ← load env: CONTROLLER_URL, AGENT_TOKEN, AGENT_ID
│  ├─ session.go                  ← orchestrate capture + input + heartbeat
│  ├─ transport/
│  │  └─ ws.go                    ← WSS dial, TLS, cert pin, HTTPS_PROXY
│  ├─ capture/
│  │  └─ screen.go                ← kbinani/screenshot → JPEG
│  ├─ input/
│  │  ├─ sendinput_windows.go     ← Win32 SendInput (mouse/keyboard)
│  │  └─ sendinput_other.go       ← stub cho non-Windows (build tag)
│  ├─ inputlock/
│  │  ├─ lock_windows.go          ← LowLevelKeyboardProc / MouseProc hook
│  │  └─ lock_other.go            ← stub cho non-Windows
│  └─ persistence/
│     ├─ autostart_windows.go     ← ghi HKCU\Software\Microsoft\Windows\CurrentVersion\Run
│     └─ autostart_other.go       ← stub
│
├─ controller/
│  ├─ go.mod / go.sum
│  ├─ main.go                     ← entry common (build mặc định)
│  ├─ main_default.go             ← `//go:build !wails` — chạy server thuần CLI
│  ├─ main_wails.go               ← `//go:build wails`  — chạy GUI Wails + server
│  ├─ app.go                      ← Wails App struct, bindings
│  ├─ setup.go                    ← AppContext: hub + routes + cert
│  ├─ wails.json                  ← Wails project config
│  ├─ server/
│  │  ├─ hub.go                   ← quản lý agents, broadcast
│  │  ├─ ws.go                    ← WSS handler agent ↔ hub
│  │  ├─ control.go               ← WS handler controller ↔ agent (mouse/key)
│  │  ├─ stream.go                ← MJPEG / single JPEG endpoint cho browser
│  │  └─ cert.go                  ← auto-generate self-signed cert
│  └─ frontend/
│     ├─ dist/                    ← static HTML/JS/CSS (build sẵn)
│     └─ wailsjs/                 ← Wails generated bindings (Go ↔ JS)
│
└─ scripts/
   ├─ gen-cert.ps1                ← tạo self-signed cert bằng OpenSSL
   ├─ run-agent.bat               ← chạy agent với env mặc định (dev)
   └─ run-controller.bat          ← chạy controller (dev)
```

---

## 4. Protocol

Tất cả message dạng **JSON envelope** trong WebSocket frame; riêng frame ảnh dùng **binary frame** để giảm overhead base64.

### 4.1 Envelope

```json
{ "type": "<kind>", "id": "<uuid optional>", "payload": { ... } }
```

### 4.2 Message types

| `type` | Hướng | Payload |
|---|---|---|
| `hello` | Agent → Controller | `{ agent_id, hostname, os, version }` |
| `welcome` | Controller → Agent | `{ session_id, fps_target, jpeg_quality }` |
| `heartbeat` | hai chiều, mỗi 20s | `{ ts }` |
| `frame` | Agent → Controller | binary: `[8 byte header][JPEG bytes]` |
| `mouse` | Controller → Agent | `{ x, y, button, action }` (move/down/up/scroll) |
| `key` | Controller → Agent | `{ vk, action }` (down/up) |
| `shutdown` | Controller → Agent | `{}` |

Chi tiết xem <ref_file file="C:\Users\haqua\Documents\GitHub\IoT\design.md" /> mục **3. Protocol**.

---

## 5. Bảo mật (lab-grade)

- **TLS 1.3** với self-signed cert auto-generate (`controller/server/cert.go`).
- **Certificate pinning**: agent pin SHA256 fingerprint của cert controller.
- **Pre-shared token**: agent gửi `AGENT_TOKEN` env trong message `hello`; controller xác thực trước khi đưa vào hub.
- **Không log payload nhạy cảm** (chỉ log type + size).
- **Không yêu cầu admin/UAC** ở agent → cố tình để hệ thống phát hiện firmware-level (đối tượng nghiên cứu) có cơ hội nhận diện qua hành vi `HKCU\…\Run`, tương ứng MITRE ATT&CK **T1547.001**.

---

## 6. Yêu cầu hệ thống

| Thành phần | Yêu cầu |
|---|---|
| OS phát triển | Windows 10/11 (agent dùng Win32 API) |
| Go | **1.25+** (xem `go.work`) |
| Wails CLI | v2 — `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| WebView2 Runtime | Có sẵn trên Windows 11; Win10 cài Edge Updated |
| OpenSSL (tuỳ chọn) | Chỉ cần nếu chạy `scripts/gen-cert.ps1` (controller cũng tự gen cert được) |

---

## 7. Build & Run

### 7.1 Clone

```bash
git clone https://github.com/in4SECxMinDandy/Controller-Go.git
cd Controller-Go
```

### 7.2 Build Agent (headless, không console)

```bash
cd agent
go build -ldflags="-H windowsgui -s -w" -trimpath -o agent.exe
```

> `-H windowsgui` ẩn console window. `-s -w` strip debug info. `-trimpath` xoá path tuyệt đối khỏi binary.

### 7.3 Build Controller (chọn 1 trong 2)

**Mode A — CLI thuần (không GUI), chỉ chạy WSS server:**

```bash
cd controller
go build -o controller.exe .
```

**Mode B — Wails GUI (kèm WSS server chạy nền):**

```bash
cd controller
wails build           # output: build/bin/controller-gui.exe
```

### 7.4 Chạy thử (dev)

Mặc định controller listen tại `:8443` với TLS, agent dial `wss://127.0.0.1:8443/ws`.

```bash
# Terminal 1 — controller
scripts\run-controller.bat

# Terminal 2 — agent
scripts\run-agent.bat
```

Mở trình duyệt → `https://127.0.0.1:8443/` (chấp nhận self-signed warning) → `/view/test-machine-1` để xem màn hình live qua MJPEG.

Hoặc chạy file Wails GUI để có giao diện danh sách agent + viewer tích hợp.

---

## 8. Cấu hình (Environment Variables)

### 8.1 Agent (`agent/agent.exe`)

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `CONTROLLER_URL` | `wss://127.0.0.1:8443/ws` | URL WSS của controller |
| `AGENT_TOKEN` | `dev-token` | Token chia sẻ trước |
| `AGENT_ID` | `<hostname>` | ID cố định của agent |
| `HTTPS_PROXY` | (không) | Proxy HTTPS (nếu cần qua doanh nghiệp) |

### 8.2 Controller

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `LISTEN_ADDR` | `:8443` | Port listen WSS |
| `TLS_CERT` | `cert.pem` | Đường dẫn cert |
| `TLS_KEY` | `key.pem` | Đường dẫn private key |
| `TLS_DISABLE` | (không) | `1` để tắt TLS (chỉ HTTP — chỉ cho debug) |

---

## 9. Endpoints (Controller HTTP/WSS)

| Path | Mô tả |
|---|---|
| `GET /` | Trang chủ (HTML đơn giản) |
| `GET /agents` | Danh sách agent đang online (JSON) |
| `GET /agents/{id}/screen.jpg` | Snapshot JPEG khung hình mới nhất |
| `GET /agents/{id}/stream` | MJPEG stream (cho `<img>` browser) |
| `GET /view/{id}` | Trang HTML xem live + điều khiển |
| `WS  /ws` | Endpoint WSS cho agent kết nối |
| `WS  /control/{id}` | Endpoint WS cho controller (browser/Wails) gửi mouse/key |

---

## 10. Build với build tags (controller)

Controller có 2 entry-point để cùng codebase chạy được cả CLI lẫn Wails GUI:

| Tag | File active | Mục đích |
|---|---|---|
| (mặc định) | `main_default.go` | Server thuần, không GUI |
| `-tags=wails` | `main_wails.go` | Wails GUI + server chạy nền |

> Lưu ý gopls/VS Code: thư mục `.vscode/settings.json` đã được gitignore. Khi mở dự án local, nếu muốn IDE phân tích `main_wails.go`, hãy thêm:
> ```json
> { "gopls": { "build.buildFlags": ["-tags=wails"] } }
> ```

---

## 11. Roadmap

- [x] **MVP-1**: WSS connect + hello/heartbeat + auto-reconnect.
- [x] **MVP-2**: Capture + frame stream → controller hiển thị (MJPEG fallback).
- [x] **MVP-3**: Mouse + keyboard injection.
- [x] **MVP-4**: Wails GUI hoàn chỉnh, multi-agent. ← **hiện tại (v0.4.0)**
- [ ] **v2**: WebRTC video low-latency, fallback WSS.
- [ ] **v3**: File transfer, clipboard sync, audio.

---

## 12. Đặc điểm phục vụ nghiên cứu firmware detection

Agent cố ý hiện thực một số hành vi điển hình của spyware/RAT để các giải pháp phát hiện ở tầng firmware (đối tượng nghiên cứu trong các paper) có dấu hiệu để bám:

| Hành vi | MITRE ATT&CK | Vị trí code |
|---|---|---|
| Persistence qua HKCU Run key | T1547.001 | `agent/persistence/autostart_windows.go` |
| Screen capture | T1113 | `agent/capture/screen.go` |
| Input capture / injection | T1056.001 / T1059 | `agent/input/`, `agent/inputlock/` |
| C2 over WSS (port 443) | T1071.001 / T1573 | `agent/transport/ws.go` |
| Application-layer protocol | T1071 | `shared/protocol.go` |

---

## 13. Tham khảo

- *A spyware detection method for firmware based cyber attack* (paper, không kèm trong repo)
- *UEFI Firmware Runtime Observability Layer for Detection* (paper, không kèm trong repo)
- [Wails v2 Documentation](https://wails.io)
- [MITRE ATT&CK Matrix](https://attack.mitre.org)

---

## 14. Giấy phép

Mã nguồn dùng nội bộ cho mục đích nghiên cứu lab. Không phân phối lại; không dùng cho mục đích thương mại hay tấn công.
