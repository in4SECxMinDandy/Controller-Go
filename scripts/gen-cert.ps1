# Tạo self-signed cert cho Controller (chỉ dùng trong lab).
# Yêu cầu: OpenSSL có trong PATH (Git Bash, Chocolatey openssl, hoặc WSL).
# Chạy:  powershell -ExecutionPolicy Bypass -File scripts\gen-cert.ps1

$ErrorActionPreference = "Stop"
$out = "controller"
if (-not (Test-Path $out)) { New-Item -ItemType Directory $out | Out-Null }

openssl req -x509 -newkey rsa:2048 -nodes -days 365 `
    -keyout "$out/key.pem" -out "$out/cert.pem" `
    -subj "/CN=localhost" `
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

Write-Host "OK -> $out/cert.pem & $out/key.pem"
