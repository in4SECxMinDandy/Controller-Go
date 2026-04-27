package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HandleScreenJPG: GET /agents/{id}/screen.jpg — frame mới nhất (1 ảnh).
func HandleScreenJPG(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pickAgentID(r.URL.Path, "/screen.jpg")
		ag := hub.Get(id)
		if ag == nil {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		_, frame := ag.LastFrame()
		if frame == nil {
			http.Error(w, "no frame yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(frame)
	}
}

// HandleStream: GET /agents/{id}/stream — MJPEG live stream (xem trong browser).
func HandleStream(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pickAgentID(r.URL.Path, "/stream")
		ag := hub.Get(id)
		if ag == nil {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		const boundary = "frameboundary"
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
		w.Header().Set("Cache-Control", "no-store")
		flusher, _ := w.(http.Flusher)

		ctx := r.Context()
		var lastSeq uint32
		ticker := time.NewTicker(2 * time.Second) // fallback nếu không có signal
		defer ticker.Stop()
		for {
			seq, frame := ag.LastFrame()
			if frame != nil && seq != lastSeq {
				lastSeq = seq
				fmt.Fprintf(w, "\r\n--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
				if _, err := w.Write(frame); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ag.FrameSignal():
			case <-ticker.C:
			}
		}
	}
}

// HandleViewer: GET /view/{id} — trang HTML tương tác:
//   - hiển thị MJPEG stream
//   - capture chuột/phím trên ảnh và đẩy qua WS /control/{id}
func HandleViewer(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/view/")
		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, viewerHTML, id, id, id, id)
	}
}

// %s xuất hiện 4 lần: title, header, stream src, ws path.
const viewerHTML = `<!doctype html><html><head><meta charset="utf-8"><title>Agent %s</title>
<style>
body{margin:0;background:#111;color:#ccc;font-family:sans-serif;overflow:hidden}
header{padding:6px 10px;background:#222;display:flex;gap:12px;align-items:center}
header b{color:#fff}
#status{margin-left:auto;font-size:12px}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%%;margin-right:4px;vertical-align:middle}
.on{background:#4ade80}.off{background:#ef4444}
#stage{position:relative;width:100vw;height:calc(100vh - 36px);display:flex;align-items:center;justify-content:center}
#screen{max-width:100%%;max-height:100%%;display:block;cursor:none;user-select:none;-webkit-user-drag:none}
#help{position:absolute;bottom:8px;left:8px;font-size:11px;color:#888;background:#0008;padding:4px 8px;border-radius:4px}
</style></head>
<body>
<header>
  Agent: <b>%s</b>
  <label><input type="checkbox" id="ctl" checked> điều khiển</label>
  <span id="status"><span class="dot off" id="dot"></span><span id="st">đang kết nối…</span></span>
</header>
<div id="stage"><img id="screen" src="/agents/%s/stream" draggable="false"/></div>
<div id="help">click vào hình để focus rồi gõ phím · cuộn để wheel · bỏ tích "điều khiển" để chỉ xem</div>
<script>
(function(){
  const id = %q;
  const img = document.getElementById('screen');
  const ctl = document.getElementById('ctl');
  const dot = document.getElementById('dot');
  const st  = document.getElementById('st');
  let ws=null, ready=false;
  function connect(){
    const proto = location.protocol==='https:'?'wss:':'ws:';
    ws = new WebSocket(proto+'//'+location.host+'/control/'+id);
    ws.onopen  = ()=>{ready=true; dot.className='dot on'; st.textContent='đã kết nối';};
    ws.onclose = ()=>{ready=false; dot.className='dot off'; st.textContent='mất kết nối, thử lại 2s'; setTimeout(connect,2000);};
    ws.onerror = ()=>{};
  }
  connect();
  function send(type,payload){
    if(!ready||!ctl.checked) return;
    ws.send(JSON.stringify({type:type,payload:payload}));
  }
  function norm(e){
    const r = img.getBoundingClientRect();
    return {nx:(e.clientX-r.left)/r.width, ny:(e.clientY-r.top)/r.height};
  }
  const BTN = {0:'left',1:'middle',2:'right'};
  let lastMove = 0;
  img.addEventListener('mousemove', e=>{
    const now = performance.now();
    if(now-lastMove < 16) return; // ~60Hz tối đa
    lastMove = now;
    const n = norm(e);
    send('mouse',{action:'move', nx:n.nx, ny:n.ny});
  });
  img.addEventListener('mousedown', e=>{
    e.preventDefault();
    const n = norm(e);
    send('mouse',{action:'down', button:BTN[e.button]||'left', nx:n.nx, ny:n.ny});
  });
  img.addEventListener('mouseup', e=>{
    e.preventDefault();
    const n = norm(e);
    send('mouse',{action:'up', button:BTN[e.button]||'left', nx:n.nx, ny:n.ny});
  });
  img.addEventListener('contextmenu', e=>e.preventDefault());
  img.addEventListener('wheel', e=>{
    e.preventDefault();
    // deltaY > 0 = cuộn xuống ⇒ Windows wheel = -120
    const delta = e.deltaY > 0 ? -120 : 120;
    send('mouse',{action:'wheel', delta:delta, nx:0, ny:0});
  }, {passive:false});
  // Keyboard: gắn lên window khi click vào img để có focus.
  let kbActive = false;
  img.addEventListener('click', ()=>{ kbActive = true; });
  document.addEventListener('keydown', e=>{
    if(!kbActive||!ctl.checked) return;
    e.preventDefault();
    send('key',{vk:e.keyCode, action:'down'});
  });
  document.addEventListener('keyup', e=>{
    if(!kbActive||!ctl.checked) return;
    e.preventDefault();
    send('key',{vk:e.keyCode, action:'up'});
  });
})();
</script>
</body></html>`

// pickAgentID: với path /agents/{id}/<suffix>, lấy {id}.
func pickAgentID(path, suffix string) string {
	p := strings.TrimPrefix(path, "/agents/")
	p = strings.TrimSuffix(p, suffix)
	return p
}
