package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/iot-lab/controller/server"
)

// AppContext gói toàn bộ phụ thuộc dựng được từ env, tái dùng cho cả 2 entry point.
type AppContext struct {
	Addr     string
	CertFile string
	KeyFile  string
	UseTLS   bool
	Hub      *server.Hub
	Mux      *http.ServeMux
}

// SetupApp chuẩn bị routes + hub + cert. Trả về AppContext sẵn sàng listen.
func SetupApp() *AppContext {
	addr := getenv("LISTEN_ADDR", ":8443")
	certFile := getenv("TLS_CERT", "cert.pem")
	keyFile := getenv("TLS_KEY", "key.pem")
	useTLS := os.Getenv("TLS_DISABLE") != "1"

	hub := server.NewHub()
	go hub.Run()

	scheme := "https"
	if !useTLS {
		scheme = "http"
	}
	host := strings.TrimPrefix(addr, ":")
	if host == addr {
		host = strings.Replace(host, "0.0.0.0", "127.0.0.1", 1)
	} else {
		host = "127.0.0.1" + addr
	}
	hub.SetLocalURL(scheme + "://" + host)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWS(hub))
	mux.HandleFunc("/agents", server.HandleListAgents(hub))
	mux.HandleFunc("/agents/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/screen.jpg"):
			server.HandleScreenJPG(hub)(w, r)
		case strings.HasSuffix(r.URL.Path, "/stream"):
			server.HandleStream(hub)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/view/", server.HandleViewer(hub))
	mux.HandleFunc("/control/", server.HandleControlWS(hub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>IoT Lab Controller</title>
<h1>IoT Lab Controller</h1>
<p><a href="/agents">Danh sách agent (JSON)</a></p>
<p>Mở <code>/view/{agent_id}</code> để xem màn hình live.</p>`))
	})

	if useTLS {
		if err := server.EnsureSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatalf("ensure cert: %v", err)
		}
	}

	return &AppContext{
		Addr:     addr,
		CertFile: certFile,
		KeyFile:  keyFile,
		UseTLS:   useTLS,
		Hub:      hub,
		Mux:      mux,
	}
}

// ListenBlocking chạy HTTP server cho tới khi lỗi.
func (a *AppContext) ListenBlocking() error {
	if a.UseTLS {
		log.Printf("controller listening on %s (wss)  cert=%s", a.Addr, a.CertFile)
		return http.ListenAndServeTLS(a.Addr, a.CertFile, a.KeyFile, a.Mux)
	}
	log.Printf("controller listening on %s (HTTP — no TLS)", a.Addr)
	return http.ListenAndServe(a.Addr, a.Mux)
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
