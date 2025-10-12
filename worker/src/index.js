import { DurableObject } from "cloudflare:workers";

// 简化的会话管理器
class SessionManager extends DurableObject {
    constructor(ctx, env) {
        super(ctx, env);
        this.sessions = new Map(); // sessionId -> { desktop: ws, web: ws }
    }

    async fetch(request) {
        if (request.headers.get("Upgrade") !== "websocket") {
            return new Response("Expected WebSocket", { status: 426 });
        }

        const url = new URL(request.url);
        const sessionId = url.searchParams.get("session") || "default";

        const [client, server] = new WebSocketPair();
        server.accept();

        this.handleConnection(server, sessionId);

        return new Response(null, { status: 101, webSocket: client });
    }

    handleConnection(ws, sessionId) {
        // 获取或创建会话
        if (!this.sessions.has(sessionId)) {
            this.sessions.set(sessionId, { desktop: null, web: null });
        }
        const session = this.sessions.get(sessionId);

        // 设置消息处理
        ws.addEventListener("message", async (event) => {
            try {
                const msg = JSON.parse(event.data);
                await this.handleMessage(ws, sessionId, msg);
            } catch (error) {
                console.error("处理消息错误:", error);
            }
        });

        // 设置关闭处理
        ws.addEventListener("close", () => {
            if (session.desktop === ws) {
                session.desktop = null;
                console.log(`Desktop 断开: ${sessionId}`);
                // 通知 Web 端
                if (session.web && session.web.readyState === WebSocket.READY_STATE_OPEN) {
                    session.web.send(JSON.stringify({
                        type: "system",
                        data: { cmd: "disconnected", args: { deviceType: "desktop" } }
                    }));
                }
            } else if (session.web === ws) {
                session.web = null;
                console.log(`Web 断开: ${sessionId}`);
                // 通知 Desktop 端
                if (session.desktop && session.desktop.readyState === WebSocket.READY_STATE_OPEN) {
                    session.desktop.send(JSON.stringify({
                        type: "system",
                        data: { cmd: "disconnected", args: { deviceType: "web" } }
                    }));
                }
            }

            // 如果会话为空，清理
            if (!session.desktop && !session.web) {
                this.sessions.delete(sessionId);
            }
        });
    }

    async handleMessage(ws, sessionId, msg) {
        const session = this.sessions.get(sessionId);
        if (!session) return;

        // 处理3种消息类型
        switch (msg.type) {
            case "input":
                // Web -> Desktop: 用户输入
                if (session.desktop && session.desktop.readyState === WebSocket.READY_STATE_OPEN) {
                    session.desktop.send(JSON.stringify(msg));
                }
                break;

            case "output":
                // Desktop -> Web: 终端输出
                if (session.web && session.web.readyState === WebSocket.READY_STATE_OPEN) {
                    session.web.send(JSON.stringify(msg));
                }
                break;

            case "system":
                // 系统消息处理
                await this.handleSystemMessage(ws, sessionId, msg);
                break;
        }
    }

    async handleSystemMessage(ws, sessionId, msg) {
        const session = this.sessions.get(sessionId);
        if (!session) return;

        const sysData = msg.data;
        const cmd = typeof sysData === 'string' ? sysData : sysData.cmd;

        switch (cmd) {
            case "register":
                // 注册设备
                const args = sysData.args || {};
                if (args.deviceType === "desktop") {
                    session.desktop = ws;
                    console.log(`Desktop 注册: ${sessionId}`);

                    // 通知 Web 端（如果已连接）
                    if (session.web && session.web.readyState === WebSocket.READY_STATE_OPEN) {
                        session.web.send(JSON.stringify({
                            type: "system",
                            data: { cmd: "connected", args: { deviceType: "desktop" } }
                        }));
                    }
                } else if (args.deviceType === "web") {
                    session.web = ws;
                    console.log(`Web 注册: ${sessionId}`);

                    // 通知 Desktop 端（如果已连接）
                    if (session.desktop && session.desktop.readyState === WebSocket.READY_STATE_OPEN) {
                        session.desktop.send(JSON.stringify({
                            type: "system",
                            data: { cmd: "connected", args: { deviceType: "web" } }
                        }));

                        // 同时通知 Web 端，桌面端已经连接
                        session.web.send(JSON.stringify({
                            type: "system",
                            data: { cmd: "connected", args: { deviceType: "desktop" } }
                        }));
                    }
                }
                break;

            case "resize":
            case "clear":
            case "restart":
            case "config":
            case "init":
                // 转发系统命令
                if (ws === session.web && session.desktop) {
                    // Web -> Desktop
                    session.desktop.send(JSON.stringify(msg));
                } else if (ws === session.desktop && session.web) {
                    // Desktop -> Web
                    session.web.send(JSON.stringify(msg));
                }
                break;

            case "ping":
                // 响应 ping
                ws.send(JSON.stringify({
                    type: "system",
                    data: { cmd: "pong" }
                }));
                break;
        }
    }
}

// 主入口
export default {
    async fetch(request, env) {
        const url = new URL(request.url);

        // WebSocket 路由
        if (url.pathname === "/ws") {
            const sessionId = url.searchParams.get("session") || "default";

            // 使用 Durable Object 处理会话
            const id = env.SESSIONS.idFromName(sessionId);
            const stub = env.SESSIONS.get(id);

            return stub.fetch(request);
        }

        // 返回前端文件
        return env.ASSETS.fetch(request);
    }
};

// 导出 Durable Object
export { SessionManager };