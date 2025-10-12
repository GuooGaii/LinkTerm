import { DurableObject } from "cloudflare:workers";

class SessionState {
    constructor(id) {
        this.id = id;
        this.connections = new Map(); // role -> WebSocket
    }

    setConnection(role, ws) {
        this.connections.set(role, ws);
    }

    getConnection(role) {
        return this.connections.get(role);
    }

    removeConnection(role, ws) {
        const current = this.connections.get(role);
        if (!current) {
            return false;
        }
        if (ws && current !== ws) {
            return false;
        }
        this.connections.delete(role);
        return true;
    }

    listRoles() {
        return Array.from(this.connections.keys());
    }

    hasOpenConnection(role) {
        const ws = this.connections.get(role);
        return Boolean(ws && ws.readyState === WebSocket.READY_STATE_OPEN);
    }

    isEmpty() {
        return this.connections.size === 0;
    }

    forEachConnection(callback) {
        for (const entry of this.connections.entries()) {
            callback(entry[0], entry[1]);
        }
    }
}

class SessionStore {
    constructor() {
        this.sessions = new Map(); // sessionId -> SessionState
    }

    ensure(sessionId) {
        let session = this.sessions.get(sessionId);
        if (!session) {
            session = new SessionState(sessionId);
            this.sessions.set(sessionId, session);
        }
        return session;
    }

    get(sessionId) {
        return this.sessions.get(sessionId) || null;
    }

    delete(sessionId) {
        this.sessions.delete(sessionId);
    }
}

class PluginRegistry {
    constructor(host) {
        this.host = host;
        this.plugins = [];
    }

    register(plugin) {
        if (!plugin || !plugin.name) {
            throw new Error("Plugins must provide a name");
        }
        this.plugins.push(plugin);
        if (typeof plugin.onRegister === "function") {
            try {
                plugin.onRegister({ host: this.host, registry: this });
            } catch (error) {
                console.error(`Plugin "${plugin.name}" onRegister error:`, error);
            }
        }
    }

    invoke(method, ctx) {
        for (const plugin of this.plugins) {
            const handler = plugin[method];
            if (typeof handler !== "function") {
                continue;
            }
            try {
                handler.call(plugin, ctx);
            } catch (error) {
                console.error(`Plugin "${plugin.name}" ${method} error:`, error);
            }
        }
    }

    dispatchMessage(ctx) {
        for (const plugin of this.plugins) {
            if (typeof plugin.onMessage !== "function") {
                continue;
            }
            try {
                const handled = plugin.onMessage(ctx);
                if (handled) {
                    return true;
                }
            } catch (error) {
                console.error(`Plugin "${plugin.name}" onMessage error:`, error);
            }
        }
        return false;
    }
}

const formatRoleForLog = (role) => {
    if (!role) return "Unknown device";
    if (role === "desktop") return "Desktop";
    if (role === "web") return "Web";
    return role.charAt(0).toUpperCase() + role.slice(1);
};

class SessionHost {
    constructor(store) {
        this.store = store;
        this.registry = null;
        this.connectionMeta = new WeakMap(); // ws -> { sessionId, role }
    }

    setRegistry(registry) {
        this.registry = registry;
    }

    registerConnection(sessionId, ws) {
        const id = sessionId || "default";
        this.connectionMeta.set(ws, { sessionId: id, role: null });
        this.store.ensure(id);
        const ctx = this.createContext(ws);
        if (ctx && this.registry) {
            this.registry.invoke("onConnect", ctx);
        }
    }

    handleMessage(ws, event) {
        const ctx = this.createContext(ws, { rawEvent: event });
        if (!ctx) return;

        const rawData = typeof event.data === "string" ? event.data : null;
        if (rawData === null) {
            console.warn("Received non-text WebSocket message; ignoring");
            return;
        }

        try {
            ctx.message = JSON.parse(rawData);
        } catch (error) {
            console.error("WebSocket message parse error:", error);
            return;
        }

        const handled = this.registry ? this.registry.dispatchMessage(ctx) : false;
        if (!handled) {
            console.warn(
                `Unhandled message type "${ctx.message?.type}" in session ${ctx.sessionId}`
            );
        }
    }

    handleClose(ws, event) {
        const meta = this.connectionMeta.get(ws);
        if (!meta) return;

        const ctx = this.createContext(ws, { closeEvent: event });
        if (ctx && this.registry) {
            this.registry.invoke("onDisconnect", ctx);
        }

        if (meta.role) {
            this.detachConnection(meta.sessionId, meta.role, ws);
            this.broadcastStatus(meta.sessionId);
        }

        this.connectionMeta.delete(ws);
        this.cleanup(meta.sessionId);
    }

    attachConnection(sessionId, role, ws) {
        if (!role) return;
        const session = this.store.ensure(sessionId);
        session.setConnection(role, ws);

        const meta = this.connectionMeta.get(ws);
        if (meta) {
            meta.role = role;
        }

        console.log(`${formatRoleForLog(role)} connected to session: ${sessionId}`);
    }

    detachConnection(sessionId, role, ws) {
        const session = this.store.get(sessionId);
        if (!session) return false;

        const removed = session.removeConnection(role, ws);
        if (removed) {
            console.log(`${formatRoleForLog(role)} disconnected from session: ${sessionId}`);
        }
        return removed;
    }

    send(ws, payload) {
        if (!ws || ws.readyState !== WebSocket.READY_STATE_OPEN) {
            return false;
        }
        const data = typeof payload === "string" ? payload : JSON.stringify(payload);
        ws.send(data);
        return true;
    }

    sendTo(sessionId, role, payload) {
        const session = this.store.get(sessionId);
        if (!session) return false;

        const target = session.getConnection(role);
        if (!target || target.readyState !== WebSocket.READY_STATE_OPEN) {
            return false;
        }
        return this.send(target, payload);
    }

    broadcast(sessionId, payload, predicate) {
        const session = this.store.get(sessionId);
        if (!session) return;

        session.forEachConnection((role, ws) => {
            if (typeof predicate === "function" && !predicate(role, ws)) {
                return;
            }
            this.send(ws, payload);
        });
    }

    broadcastStatus(sessionId) {
        const session = this.store.get(sessionId);
        if (!session) return;

        const devices = {};
        const defaults = new Set(["desktop", "web"]);
        session.listRoles().forEach((role) => defaults.add(role));

        for (const role of defaults) {
            devices[role] = session.hasOpenConnection(role) ? "connected" : "disconnected";
        }

        const statusMessage = {
            type: "connection.devices",
            to: "all",
            data: { devices },
        };

        this.broadcast(sessionId, statusMessage);
    }

    cleanup(sessionId) {
        const session = this.store.get(sessionId);
        if (session && session.isEmpty()) {
            this.store.delete(sessionId);
            console.log(`Session deleted: ${sessionId}`);
        }
    }

    createContext(ws, extras = {}) {
        const meta = this.connectionMeta.get(ws);
        if (!meta) return null;

        const session = this.store.ensure(meta.sessionId);
        const connection = {
            role: meta.role || null,
            setRole: (role) => {
                meta.role = role || null;
                connection.role = meta.role;
            },
        };

        return {
            ...extras,
            sessionId: session.id,
            session,
            ws,
            host: this,
            connection,
            reply: (payload) => this.send(ws, payload),
            sendTo: (role, payload) => this.sendTo(session.id, role, payload),
            broadcast: (payload, predicate) => this.broadcast(session.id, payload, predicate),
        };
    }
}

class DeviceBridgePlugin {
    constructor() {
        this.name = "device-bridge";
    }

    onMessage(ctx) {
        const { message, host, sessionId, ws } = ctx;
        if (!message || typeof message !== "object") {
            return false;
        }

        switch (message.type) {
            case "connection.devices": {
                const role = message.data?.deviceType;
                const status = message.data?.status;
                if (!role || !status) return false;

                if (status === "connected") {
                    host.attachConnection(sessionId, role, ws);
                    ctx.connection.setRole(role);
                    host.broadcastStatus(sessionId);
                } else if (status === "disconnected") {
                    host.detachConnection(sessionId, role, ws);
                    if (ctx.connection.role === role) {
                        ctx.connection.setRole(null);
                    }
                    host.broadcastStatus(sessionId);
                }
                return true;
            }

            case "data.input":
            case "data.output":
            case "system.init":
            case "system.resize":
            case "system.command":
            case "system.config": {
                if (!message.to) return false;
                host.sendTo(sessionId, message.to, message);
                return true;
            }

            default:
                return false;
        }
    }
}

class PingPongPlugin {
    constructor() {
        this.name = "ping-pong";
    }

    onMessage(ctx) {
        if (!ctx.message || ctx.message.type !== "connection.ping") {
            return false;
        }
        const target =
            ctx.message.data?.deviceType ||
            ctx.connection.role ||
            "web";

        ctx.reply({
            type: "connection.pong",
            to: target,
            data: {},
        });
        return true;
    }
}

const createDefaultPluginRegistry = (host) => {
    const registry = new PluginRegistry(host);
    registry.register(new DeviceBridgePlugin());
    registry.register(new PingPongPlugin());
    return registry;
};

export class TerminalSessionManager extends DurableObject {
    constructor(ctx, env) {
        super(ctx, env);
        this.store = new SessionStore();
        this.host = new SessionHost(this.store);
        this.registry = createDefaultPluginRegistry(this.host);
        this.host.setRegistry(this.registry);
    }

    async fetch(request) {
        const url = new URL(request.url);

        if (url.pathname === "/ws") {
            const upgradeHeader = request.headers.get("Upgrade");
            if (upgradeHeader !== "websocket") {
                return new Response("Expected Upgrade: websocket", { status: 426 });
            }

            const sessionId = url.searchParams.get("session") || "default";
            const webSocketPair = new WebSocketPair();
            const [client, server] = Object.values(webSocketPair);

            this.host.registerConnection(sessionId, server);
            server.accept();

            server.addEventListener("message", (event) => {
                this.host.handleMessage(server, event);
            });

            server.addEventListener("close", (event) => {
                this.host.handleClose(server, event);
            });

            return new Response(null, {
                status: 101,
                webSocket: client,
            });
        }

        return new Response("Not Found", { status: 404 });
    }
}

export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);

        if (url.pathname === "/ws") {
            const sessionId = url.searchParams.get("session") || "default";
            const id = env.TERMINAL_SESSION_MANAGER.idFromName(sessionId);
            const stub = env.TERMINAL_SESSION_MANAGER.get(id);
            return stub.fetch(request);
        }

        return env.ASSETS.fetch(request);
    },
};
