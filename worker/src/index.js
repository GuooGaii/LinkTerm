import { DurableObject } from 'cloudflare:workers';
import { authenticate } from './security.js';
import { validMessage, MAX_MESSAGE } from './protocol.js';
import page from '../dist/page.html';

export class SessionManager extends DurableObject {
  constructor(ctx, env) {
    super(ctx, env);
    ctx.setWebSocketAutoResponse(new WebSocketRequestResponsePair('ping', 'pong'));
  }
  peers(role) { return this.ctx.getWebSockets(role).filter(s => s.readyState === WebSocket.OPEN && s.deserializeAttachment()?.active); }
  send(role, message) { for (const s of this.peers(role)) s.send(JSON.stringify(message)); }
  async fetch(request) {
    const role = request.headers.get('X-LinkTerm-Role');
    if (!['web', 'agent'].includes(role)) return new Response('Forbidden', { status: 403 });
    const old = this.peers(role);
    const url = new URL(request.url);
    const clientId = url.searchParams.get('client') || null;
    const replace = role === 'web' && (url.searchParams.get('takeover') === '1' || (clientId && old.every(s => s.deserializeAttachment().clientId === clientId)));
    if (old.length && !replace) return new Response('A connection is already active', { status: 409 });
    for (const previous of old) {
      this.disconnected(previous, 4002, 'Control transferred to another connection');
    }
    const [client, server] = Object.values(new WebSocketPair());
    const expires = Number(request.headers.get('X-LinkTerm-Expires')) || 0;
    this.ctx.acceptWebSocket(server, [role]);
    server.serializeAttachment({ role, active: true, expires, epoch: null, clientId });
    if (role === 'web') server.send(JSON.stringify({ type: 'system', data: { cmd: this.peers('agent').length ? 'connected' : 'disconnected' } }));
    else this.send('web', { type: 'system', data: { cmd: 'connected' } });
    if (expires) {
      const alarm = await this.ctx.storage.getAlarm();
      if (!alarm || alarm > expires * 1000) await this.ctx.storage.setAlarm(expires * 1000);
    }
    return new Response(null, { status: 101, webSocket: client });
  }
  webSocketMessage(ws, raw) {
    const a = ws.deserializeAttachment();
    if (!a?.active) return;
    if (a.expires && Date.now() >= a.expires * 1000) { ws.close(4001, 'Login expired'); return; }
    if (typeof raw !== 'string' || raw.length > MAX_MESSAGE) { ws.close(1009, 'Message too large'); return; }
    let msg;
    try { msg = JSON.parse(raw); } catch { ws.close(1008, 'Invalid JSON'); return; }
    if (!validMessage(a.role, msg)) { ws.close(1008, 'Invalid message for role'); return; }
    if (a.role === 'web') {
      if (msg.type === 'system' && msg.data.cmd === 'attach') {
        a.epoch = msg.epoch; ws.serializeAttachment(a);
      } else if (a.epoch !== msg.epoch) return;
      this.send('agent', msg);
    } else {
      for (const web of this.peers('web')) if (web.deserializeAttachment().epoch === msg.epoch) web.send(raw);
    }
  }
  webSocketClose(ws) { this.disconnected(ws); }
  webSocketError(ws) { this.disconnected(ws); }
  disconnected(ws, code = 1000, reason = 'Disconnected') {
    const a = ws.deserializeAttachment();
    if (!a?.active) return;
    ws.serializeAttachment({ ...a, active: false });
    if (a.role === 'agent') this.send('web', { type: 'system', data: { cmd: 'disconnected' } });
    else this.send('agent', { type: 'system', epoch: a.epoch, data: { cmd: 'detach' } });
    try { ws.close(code, reason); } catch {}
  }
  async alarm() {
    let next = Infinity;
    for (const ws of this.peers('web')) {
      const a = ws.deserializeAttachment();
      if (!a.expires) continue;
      if (a.expires * 1000 <= Date.now()) { ws.close(4001, 'Login expired'); this.disconnected(ws); }
      else next = Math.min(next, a.expires * 1000);
    }
    if (Number.isFinite(next)) await this.ctx.storage.setAlarm(next);
  }
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method !== 'GET') return new Response('Method not allowed', { status: 405 });
    if (!['/', '/ws', '/agent'].includes(url.pathname)) return new Response('Not found', { status: 404 });
    const auth = await authenticate(request, env);
    if (!auth) return new Response('Cloudflare Access login required', { status: 403, headers: { 'Cache-Control': 'no-store' } });
    if (url.pathname === '/') return new Response(page, { headers: {
      'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store',
      'X-Content-Type-Options': 'nosniff', 'Referrer-Policy': 'no-referrer',
      'Content-Security-Policy': "default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; font-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
    } });
    if (request.headers.get('Upgrade')?.toLowerCase() !== 'websocket') return new Response('Expected WebSocket', { status: 426 });
    if (auth.role === 'web' && request.headers.get('Origin') !== url.origin) return new Response('Forbidden origin', { status: 403 });
    const headers = new Headers(request.headers);
    // Overwrite caller-supplied identity headers after authentication.
    headers.set('X-LinkTerm-Role', auth.role);
    headers.set('X-LinkTerm-Expires', String(auth.expires || 0));
    return env.SESSIONS.getByName(env.SESSION_ID).fetch(new Request(request, { headers }));
  }
};
