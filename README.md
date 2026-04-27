# IoT Remote Control (Lab project)

Hệ thống điều khiển máy tính từ xa cho mục đích **nghiên cứu phát hiện spyware/RAT ở tầng firmware** (xem các paper PDF trong workspace).

> ⚠️ Chỉ dùng trong môi trường lab có sự đồng ý. Không dùng để truy cập máy tính của người khác trái phép.

## Kiến trúc

```
┌─────────────────────────┐         wss://:443          ┌──────────────────────────┐
│  Agent (headless .exe)  │ ─── lệnh + heartbeat ─────▶ │  Controller (Wails GUI)  │
│  - Không UI             │ ◀── khung hình JPEG ────── │  - Danh sách agent       │
│  - Auto-reconnect       │ ─── chuột/phím ──────────── │  - Khung xem màn hình   │
│  - Outbound TLS:443     │                              │                          │
└─────────────────────────┘                              └──────────────────────────┘
```

## Đặc điểm

- **Giao thức**: WebSocket Secure (WSS) qua TCP/443 — đi qua mọi firewall/proxy thông thường.
- **Agent**: Go, headless (`-H windowsgui`), một file `.exe` ~5–10 MB, RAM < 20 MB.
- **Controller**: Go + Wails (WebView2) — GUI hiện đại, đóng gói thành 1 file `.exe`.
- **Outbound only**: Agent chủ động kết nối, không cần mở port → vượt NAT.
- **Auto-reconnect** với exponential backoff, đọc `HTTPS_PROXY` env.
- **Bảo mật**: TLS 1.3 + cert pinning + token chia sẻ trước.

## Cấu trúc thư mục

```
IoT/
├─ README.md
├─ design.md          (tài liệu thiết kế chi tiết)
├─ go.work            (Go workspace)
├─ shared/            (protocol message dùng chung)
├─ agent/             (headless agent)
└─ controller/        (Wails GUI controller)
```

## Cách build (sau khi code xong)

### Agent (headless, không console)
```bash
cd agent
go build -ldflags="-H windowsgui -s -w" -trimpath -o agent.exe
```

### Controller (Wails GUI)
```bash
cd controller
wails build
```

## Roadmap

Xem `design.md` mục **Roadmap**. Hiện tại đang ở giai đoạn **MVP-1** (kết nối WSS + heartbeat).

## Tham khảo

- `A spyware detection method for firmware based cyber attack.pdf`
- `UEFI Firmware Runtime Observability Layer for Detection.pdf`

# Controller-Go
