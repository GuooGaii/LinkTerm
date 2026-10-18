# LinkTerm

在浏览器中连接 Linux 容器里**已经存在的 tmux 会话**。关闭网页只断开 tmux 客户端，AI CLI 继续运行。此版本使用单个预设会话、单个网页控制端；Agent 和 Worker 必须一起升级，旧版协议不兼容。

## AI CLI 重绘

终端不是逐行日志。浏览器使用 xterm.js 解释原始 PTY 字节中的光标移动、擦除、回车、备用屏幕等控制序列：

- PTY 数据以 base64 保留字节边界，浏览器写入 `Uint8Array`，避免中文/emoji 被跨包 UTF-8 解码破坏。
- 网络状态只显示在终端外；不向终端插入提示行，不做文本去重。
- 按网页实际行列数创建 tmux 客户端；只在尺寸改变时发送 resize。
- 重连时销毁旧模拟器并创建新的 tmux 客户端，由 tmux 重画当前屏幕；会话 epoch 丢弃旧连接的数据。
- 浏览器解析完一块数据后确认，Agent 最多保留 8 块在途输出，避免大量重绘无限堆积。

网页显示 tmux 当前屏幕，而不是回放从启动开始的全部日志。需要查看历史可使用 tmux 的复制模式。正常命令输出仍然会滚动。多个不同尺寸的 SSH/tmux 客户端同时连接时，tmux 的窗口尺寸策略仍然生效；建议网页作为唯一交互端。

## 部署

1. 创建 Cloudflare Access 自托管应用，绑定你的网页域名，策略只允许指定邮箱。记录应用 AUD 和团队域名。
2. 修改 `worker/wrangler.jsonc` 的 Worker 名称和 `vars`，为网页域名配置 Worker Custom Domain。
3. 为 Agent 生成随机密钥，保存为权限 `600` 的文件，并将相同值写入 Worker Secret `AGENT_TOKEN`。不要把密钥放进 URL、Git 或公开配置。
4. 构建和发布：

```sh
cd worker
npm ci
npm run build
npm test
npx wrangler secret put AGENT_TOKEN
npx wrangler deploy
```

网页和 `/ws` 都验证 Access JWT 的签名、issuer、AUD 和邮箱；直接访问 workers.dev 也不能绕过认证。Agent 使用 workers.dev 的 `/agent` 和独立 Bearer 密钥连接，不使用浏览器邮箱授权。WebSocket 限制同源访问；Access 会话过期会关闭连接。

5. 在容器内安装 tmux，启动或保留目标会话：

```sh
tmux new-session -s ai
# 在这个会话中运行 claude / codex；Ctrl+B D 脱离后继续运行
```

6. Go 1.25 构建 Agent（也可用 `agent/Dockerfile.build` 生成 Linux 二进制），在同一个 Linux 用户下启动：

```sh
cd agent
go build -o linkterm .
LINKTERM_TMUX_SESSION=ai LINKTERM_AGENT_TOKEN_FILE=/path/to/token ./linkterm \
  -server wss://YOUR-WORKER.YOUR-SUBDOMAIN.workers.dev \
  -web https://YOUR-ACCESS-DOMAIN -config /path/to/config.json
```

没有目标 tmux 会话时会显示错误，Agent 不会自行启动新的 AI CLI。容器重启会终止 tmux 内的进程，需要重新创建会话并使用 CLI 自己的恢复功能。

## 隔离

Agent 只能在你希望共享的容器内运行，以该容器用户身份访问终端。不要在 Mac 宿主机启动 Agent，也不要给容器挂载宿主机私有目录、Docker socket 或特权权限。LinkTerm 提供连接认证，文件和网络隔离由容器部署负责。

## 验证

`worker/npm test` 覆盖 100 次原地重绘、跨包中文/emoji、光标上移、擦行、缩放，以及实际 Workers 运行时中的认证和 WebSocket 角色限制。`agent/go test ./...` 在安装 tmux 时验证真实 PTY 连接、尺寸变化、重连及会话存活。Docker 构建包含 tmux 并执行这些测试。

测试使用终端协议样例，没有把每个版本的 Codex/Claude 都作为自动化测试依赖。

MIT
