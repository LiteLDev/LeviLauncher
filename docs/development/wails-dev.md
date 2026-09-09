# Wails 开发模式启动

在仓库根目录运行 `wails3 dev`。CLI 会把开发服务器端口写入
`WAILS_VITE_PORT`，并通过 `FRONTEND_DEVSERVER_URL` 告知原生程序。
也可以使用 `wails3 dev -port 1145` 指定端口。

## 启动链

1. 阻塞构建安装依赖、生成绑定、构建前端和 Go 开发版二进制。
2. Vite 在 `127.0.0.1` 和 CLI 指定的端口启动。
3. Windows 启动任务通过 HTTP 等待 Vite 页面就绪，再运行原生程序。

`build/Taskfile.yml` 的依赖安装任务使用时间戳比较 `package.json`、
`package-lock.json` 与 `node_modules/.package-lock.json`。已完成的安装可在
`windows:common:` 和 `common:` 两种任务入口间复用；输入文件更新或安装标记缺失时仍执行 `npm ci`。

就绪检查由 `build/wait-for-dev-server.mjs` 实现。它统一使用 IPv4，并确认
响应中存在 `/@vite/client`，避免把普通静态构建或尚未就绪的端口判为成功。
普通 `wails3 task run` 未设置开发服务器地址时会直接启动已有程序。

## 2026-09-09 修复验证

修复前实测：构建后启动 Vite 时再次执行 `npm ci`，耗时约 34 秒，超过原有
30 秒等待上限；Vite 默认仅监听 `::1`，也与当前 Go 开发资源代理的 IPv4
连接方式不一致。原有 PowerShell TCP 检查在服务器已通过 IPv4 响应时仍报超时。

修复后实测：

- `wails3 dev` 完整启动成功，两个入口均复用已安装依赖，Vite 在 563 毫秒内启动。
- 原生日志包含 `Connected to frontend dev server!` 和 WebView 导航完成记录。
- 原生 WebView 对 `/@vite/client`、`/src/main.jsx`、`/src/style.css` 的请求均返回 200，确认加载开发资源。
- `node --test build/wait-for-dev-server.test.mjs`：4 项通过。
- 任务 dry run 核验：依赖新鲜时跳过；清单/锁文件变新或安装标记缺失时计划重新安装。
- `git diff --check` 通过。

如需区分开发服务器与静态构建，检查启动日志中的服务器连接记录和
`/@vite/client` 请求；仅看到一次 `build:dev` 输出并不意味着后续 Vite 服务和原生程序已经启动。
