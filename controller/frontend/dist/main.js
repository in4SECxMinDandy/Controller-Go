// Frontend cho Wails app — gọi Go bindings: window.go.main.App.*
// Stream + control vẫn dùng HTTP server local.

const $ = (sel) => document.querySelector(sel);

let serverURL = '';   // được lấy từ App.LocalServerURL()
let agents = [];
let activeID = null;
let ws = null;        // control websocket cho agent đang chọn

async function loadServer() {
  serverURL = await window.go.main.App.LocalServerURL();
  $('#server-info').textContent = `server: ${serverURL}`;
}

async function refreshAgents() {
  agents = await window.go.main.App.ListAgents();
  const list = $('#list');
  if (!agents.length) {
    list.innerHTML = '<li class="empty">chưa có agent nào</li>';
    return;
  }
  list.innerHTML = agents.map(a => `
    <li data-id="${a.id}" class="${a.id === activeID ? 'active' : ''}">
      <div class="id">${a.id}</div>
      <div class="meta">${a.os} · v${a.version} · ${new Date(a.since).toLocaleTimeString()}</div>
    </li>
  `).join('');
  list.querySelectorAll('li').forEach(li => {
    li.addEventListener('click', () => selectAgent(li.dataset.id));
  });
}

function selectAgent(id) {
  if (activeID === id) return;
  activeID = id;
  closeWS();
  const a = agents.find(x => x.id === id);
  if (!a) return;

  $('#welcome').hidden = true;
  $('#viewer').hidden = false;
  $('#cur-id').textContent = a.id;
  $('#cur-os').textContent = `${a.os} · v${a.version}`;

  // Stream MJPEG từ HTTP server local.
  $('#screen').src = `${serverURL}/agents/${encodeURIComponent(id)}/stream`;

  // Mở control WS.
  openWS(id);

  // Cập nhật sidebar active.
  document.querySelectorAll('#list li').forEach(li => {
    li.classList.toggle('active', li.dataset.id === id);
  });
}

function closeView() {
  activeID = null;
  closeWS();
  $('#screen').src = '';
  $('#viewer').hidden = true;
  $('#welcome').hidden = false;
  document.querySelectorAll('#list li').forEach(li => li.classList.remove('active'));
}

function closeWS() {
  if (ws) {
    try { ws.close(); } catch {}
    ws = null;
  }
}

function openWS(id) {
  const wsScheme = serverURL.startsWith('https') ? 'wss' : 'ws';
  const url = `${wsScheme}://${serverURL.replace(/^https?:\/\//, '')}/control/${encodeURIComponent(id)}`;
  ws = new WebSocket(url);
  const dot = document.querySelector('#ws-status .dot');
  const text = $('#ws-text');
  ws.onopen  = () => { dot.className = 'dot on';  text.textContent = 'connected'; };
  ws.onclose = () => { dot.className = 'dot off'; text.textContent = 'disconnected'; };
  ws.onerror = () => { dot.className = 'dot off'; text.textContent = 'error'; };
}

function send(type, payload) {
  if (!ws || ws.readyState !== WebSocket.OPEN || !$('#ctl-toggle').checked) return;
  ws.send(JSON.stringify({ type, payload }));
}

function norm(e) {
  const r = $('#screen').getBoundingClientRect();
  return { nx: (e.clientX - r.left) / r.width, ny: (e.clientY - r.top) / r.height };
}

function setupInput() {
  const img = $('#screen');
  const BTN = { 0: 'left', 1: 'middle', 2: 'right' };
  let lastMove = 0;

  img.addEventListener('mousemove', e => {
    const now = performance.now();
    if (now - lastMove < 16) return;
    lastMove = now;
    const n = norm(e);
    send('mouse', { action: 'move', nx: n.nx, ny: n.ny });
  });
  img.addEventListener('mousedown', e => {
    e.preventDefault();
    const n = norm(e);
    send('mouse', { action: 'down', button: BTN[e.button] || 'left', nx: n.nx, ny: n.ny });
  });
  img.addEventListener('mouseup', e => {
    e.preventDefault();
    const n = norm(e);
    send('mouse', { action: 'up', button: BTN[e.button] || 'left', nx: n.nx, ny: n.ny });
  });
  img.addEventListener('contextmenu', e => e.preventDefault());
  img.addEventListener('wheel', e => {
    e.preventDefault();
    send('mouse', { action: 'wheel', delta: e.deltaY > 0 ? -120 : 120, nx: 0, ny: 0 });
  }, { passive: false });

  let kbActive = false;
  img.addEventListener('click', () => { kbActive = true; });
  document.addEventListener('keydown', e => {
    if (!kbActive || $('#viewer').hidden) return;
    e.preventDefault();
    send('key', { vk: e.keyCode, action: 'down' });
  });
  document.addEventListener('keyup', e => {
    if (!kbActive || $('#viewer').hidden) return;
    e.preventDefault();
    send('key', { vk: e.keyCode, action: 'up' });
  });
}

window.addEventListener('DOMContentLoaded', async () => {
  await loadServer();
  await refreshAgents();
  setInterval(refreshAgents, 3000); // poll danh sách
  $('#refresh').addEventListener('click', refreshAgents);
  $('#close-btn').addEventListener('click', closeView);
  setupInput();
});
