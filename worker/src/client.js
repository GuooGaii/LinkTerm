import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { decodeOutput, sameSize } from './terminal-stream.js';
import '@xterm/xterm/css/xterm.css';
import './style.css';
const el = document.querySelector('#terminal');
const status = document.querySelector('#status');
const reconnect = document.querySelector('#reconnect');
const clientId = sessionStorage.getItem('linkterm-client') || crypto.randomUUID();
sessionStorage.setItem('linkterm-client', clientId);
let terminal, fit, ws, epoch, ready = false, lastSize, retry, heartbeat, lastPong, generation = 0;
function show(text) { status.textContent = text; }
function send(message) { if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ ...message, epoch })); }
function dimensions() { fit.fit(); return { cols: terminal.cols, rows: terminal.rows }; }
function attach() {
  ready = false;
  // Dispose pending parser writes as well as screen state. New tmux client paints a full screen.
  terminal?.dispose(); el.replaceChildren();
  terminal = new Terminal({ cursorBlink: true, fontFamily: 'Menlo, Monaco, monospace', fontSize: 14,
    theme: { background: '#101318', foreground: '#e5e7eb' }, scrollback: 3000, allowProposedApi: true });
  fit = new FitAddon(); terminal.loadAddon(fit);
  const unicode = new Unicode11Addon(); terminal.loadAddon(unicode); terminal.unicode.activeVersion = '11';
  terminal.open(el);
  epoch = crypto.randomUUID();
  lastSize = dimensions();
  const ownEpoch = epoch;
  terminal.onData(data => { if (ready && ownEpoch === epoch) send({ type: 'input', data }); });
  terminal.onBinary(data => { if (ready && ownEpoch === epoch) send({ type: 'input', data }); });
  send({ type: 'system', data: { cmd: 'attach', args: lastSize } });
  show('正在恢复 tmux 会话…');
}
function connect(takeover = false) {
  clearTimeout(retry); clearInterval(heartbeat);
  const gen = ++generation;
  ws?.close(); ready = false;
  const socket = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/ws?client=${encodeURIComponent(clientId)}${takeover ? '&takeover=1' : ''}`);
  ws = socket;
  show('连接中…');
  socket.onopen = () => {
    if (gen !== generation) return;
    lastPong = Date.now();
    heartbeat = setInterval(() => {
      if (Date.now() - lastPong > 45000) { socket.close(); return; }
      if (socket.readyState === WebSocket.OPEN) socket.send('ping');
    }, 15000);
  };
  socket.onmessage = event => {
    if (gen !== generation) return;
    if (event.data === 'pong') { lastPong = Date.now(); return; }
    let msg; try { msg = JSON.parse(event.data); } catch { return; }
    if (msg.type === 'system') {
      const cmd = msg.data?.cmd;
      if (cmd === 'connected') { attach(); return; }
      if (cmd === 'disconnected') { ready = false; show('容器暂时离线；等待恢复…'); return; }
      if (msg.epoch !== epoch) return;
      if (cmd === 'ready') { ready = true; show('已连接 · 原会话继续运行'); terminal.focus(); }
      if (cmd === 'error') { ready = false; show(msg.data.args?.message || '会话不可用'); }
    } else if (msg.type === 'output' && msg.epoch === epoch) {
      // No line splitting, deduplication, println, or HTML rendering in this stream.
      const outputEpoch = epoch;
      terminal.write(decodeOutput(msg.data), () => { if (gen === generation && outputEpoch === epoch) send({ type: 'ack' }); });
    }
  };
  socket.onclose = e => {
    if (gen !== generation) return;
    ready = false; clearInterval(heartbeat);
    if (e.code === 4001) { show('登录已过期，请刷新页面重新登录'); return; }
    if (e.code === 4002) { show('控制权已转移；点击重新连接可接管'); return; }
    show('连接中断或被占用；点击重新连接可接管');
    retry = setTimeout(() => connect(), 3000);
  };
  socket.onerror = () => { if (gen === generation) show('连接失败；请确认已登录且没有其他控制窗口'); };
}
let resizeTimer;
new ResizeObserver(() => {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(() => {
    if (!terminal || !ready) return;
    const size = dimensions();
    if (!sameSize(lastSize, size)) { lastSize = size; send({ type: 'system', data: { cmd: 'resize', args: size } }); }
  }, 120);
}).observe(el);
reconnect.onclick = () => connect(true);
window.addEventListener('beforeunload', () => { ++generation; ws?.close(); });
await document.fonts.ready;
connect();
