# Wails v3 API 重构记录

核对日期：2026-09-10。Go 模块与前端 `@wailsio/runtime` 统一为 `v3.0.0-beta.19`，前端锁文件同步更新。本记录覆盖桌面系统能力、前后端绑定和事件交互；不代表对游戏安装、下载器或所有业务代码完成了全面审计。

## 核对方式

通过本机 `wails3 mcp` 的 Streamable HTTP 服务执行 `wails_project_inspect`、`wails_project_doctor` 和 `wails_project_task_run`。项目被识别为 Wails 项目，环境诊断 `ready=true`。项目 MCP 没有文档搜索工具，API 说明来自 Wails 官方文档，并用本机 Go module cache 中的 `beta.19` 源码及 npm runtime 的实际声明核对。

项目 MCP 用于构建和诊断；带 `mcp` build tag 的应用 MCP 才能检查运行中窗口和操作界面，两者职责不同。参见 [Wails MCP 文档](https://v3.wails.io/guides/mcp-service/)。

## 已实施

| 位置 | 原实现 | 改动与作用 |
| --- | --- | --- |
| `internal/explorer/explorer.go` | 拼接 PowerShell 命令后启动 Explorer | 使用 `app.Browser.OpenFile` 打开目录，保留缺失目录的创建和失败返回；路径不再交给 PowerShell 解释。`MkdirAll` 同时拒绝普通文件，避免目录入口调用文件默认处理器。 |
| `InstallPage`、`DownloadPage`、`LipTaskConsoleContext` | `navigator.clipboard.writeText` | 统一使用 `Clipboard.SetText`；保留原有提示，并为镜像链接复制增加成功和失败反馈。 |
| `useInstanceSettings` | 拼接 `main.VersionService.*`、`unknown[]` 参数及泛型结果断言 | 5 个备份/恢复接口改用自动生成的 service bindings，参数及返回值由绑定声明检查。原有输入归一化继续负责业务默认值。 |
| `LIPPackagePage` | 拼接 `main.Minecraft.*` 并强制断言返回类型 | README 获取和 LIP 安装直接调用生成的 `GetLIPPackageReadme`、`InstallLIPPackage`。 |
| `useModsPage`、`useSettings`、`ModIntelligenceContext` | 4 次 LIP 安装/卸载、缓存清理和安装状态查询使用按名称调用 | 改用生成的 `InstallLIPPackage`、`UninstallLIPPackage`、`CacheClean`、`GetLIPPackageInstallStates`。前端应用源码共 13 个调用点完成迁移，不再自行拼接 `Call.ByName`。 |
| `CurseForgeModPage` | 用 `Events.Off(name)` 删除事件名下全部订阅；启动下载后才注册回调 | 保存并调用 `Events.On` 返回的注销函数；先订阅再启动下载，完成路径取自 `done` 事件。取消、卸载和后续操作使旧异步处理失效。 |
| `WindowControls` | 同时订阅 Windows 与 Common 事件，点击后额外查询状态 | 只订阅 `Events.Types.Common` 常量。Wails 已映射对应 Windows 事件；使用查询序号避免旧响应覆盖新状态。 |
| `main.go` 的拖拽回调 | 直接读取 `DropTargetDetails().ElementID` | 处理 `DropTargetDetails()` 为 `nil` 的情况，目标为空时仍传递文件列表。 |

Wails 的 [Manager API](https://v3.wails.io/concepts/manager-api/) 提供目录/文件处理器入口；[Frontend Runtime](https://v3.wails.io/reference/frontend-runtime/) 提供剪贴板和订阅注销接口；[窗口事件文档](https://v3.wails.io/features/windows/events/) 说明 Common 事件及其生命周期。生成的绑定是业务服务调用的首选入口，不需要额外维护按名称拼接的 RPC 封装。

## 保留的实现及原因

| 位置/能力 | Wails 能力 | 当前决定 |
| --- | --- | --- |
| 浏览器外链、文件选择对话框 | `Browser.OpenURL`、`Dialogs.OpenFile` | 项目已经采用这些 API，继续沿用。 |
| Explorer 选中文件 | `app.Browser.OpenFile` | “打开文件”与“在目录中选中文件”语义不同。保留 `SelectFile` 的 Explorer `/select` 调用。 |
| 文件拖拽到前端的转发 | `WindowFilesDropped`、`DroppedFiles()` | 保留。`beta.19` 的 `handleDragAndDropMessage` 把文件路径交给 Go 监听器；普通前端窗口事件没有等价的文件路径载荷。官方[文件拖拽示例](https://v3.wails.io/features/drag-and-drop/files/) 同样由 Go 转发自定义事件。 |
| 单实例互斥和命名管道 | `SingleInstanceOptions`、`OnSecondInstanceLaunch` | 后续可迁移，但必须同时保留 `--launch` 无窗口启动、现有实例接收启动请求、自更新专用窗口和更新后重启等待。当前检查早于 Wails 启动；仅替换互斥体会改变启动时序。参见[单实例文档](https://v3.wails.io/guides/single-instance/)。 |
| `syncWindowResizeHandles` | `SetResizable` | 保留现有兼容处理。`SetResizable` 还会修改 `DisableResize` 选项和原生窗口行为；现有代码只控制最大化后的前端缩放手柄。两者不能仅凭命名视为等价。 |
| `frontend/src/polyfills/wails.ts` | `System.Environment()` 等 | 保留，需在桌面首次导航、热更新和浏览器预览三种环境下验证初始化时序后再移除。不要把异步环境查询直接放进无边框拖拽的同步初始化路径。 |
| 启动失败的原生 MessageBox、调试控制台 | `app.Dialog` | 保留。这些代码必须在应用或 WebView2 尚未创建成功时仍可工作。 |
| 本地图片 token 注册和 middleware | `AssetOptions.Middleware` | 已通过 Wails 资源服务器扩展点提供服务；随机 token 和路径注册是应用访问规则，不能用通用文件服务器直接覆盖。 |
| 游戏下载、安装、备份、LIP 和 GDK/注册表操作 | Wails bindings/events | 属于应用业务或 Windows/GDK 能力，Wails 负责通信与生命周期，不能替代这些业务实现。 |
| 应用自更新 | Wails Updater | 值得单独迁移。需要把当前 release 来源、镜像策略、校验、提权和重启流程映射到 provider 与更新生命周期；涉及发布协议和已有用户升级路径。参见 [Updater 文档](https://v3.wails.io/guides/updater/)。 |

## 目录打开接口的错误传播

目录打开链路已进一步收敛：

```text
openDirectory(path) → Minecraft.OpenPathDir(path) → explorer.OpenPath(path) → app.Browser.OpenFile(path)
openModsDirectory(name) → ModsService.OpenModsExplorer(name) → explorer.OpenMods(name) → explorer.OpenPath(path)
```

移除 `mcservice.OpenPathDir`、`mcservice.OpenModsExplorer` 两个纯转发函数，以及没有调用方的 `OpenWorlds`、`OpenInstallers`、`OpenVersionsDir` 辅助函数。世界目录的玩家选择和兼容路径解析继续留在 `mcservice`。

`OpenPathDir`、`OpenModsExplorer`、`OpenWorldsExplorer`、`OpenGameDataExplorer` 均返回标准 Go `error`；空路径、目录准备失败、应用尚未就绪和系统处理器启动失败沿绑定传递为 Promise rejection。错误使用 `%w` 保留底层原因，并附带操作与目录信息。成功意味着系统打开请求已提交，Wails 不会等待文件管理器窗口出现。

前端统一由 `frontend/src/utils/explorer.ts` 接收错误并显示本地化提示及具体原因。界面调用 `openDirectory` 或 `openModsDirectory`，成功返回 `true`，失败在显示提示后返回 `false`；这样不等待结果的按钮也不会留下未处理的 rejection。路径查询本身抛出的错误使用相同的提示函数。目录错误标题覆盖全部 11 种现有语言。

`npm run test:explorer` 使用 Node 内置测试运行器测试实际前端工具模块，替换系统绑定和提示投递，共覆盖 6 个成功、等待、失败、重试及路径解析场景；不依赖新增测试框架，也不会打开用户目录。后端测试同时检查原始文件系统错误和处理器错误没有被丢弃。

## 后续优先级

1. 为文件下载事件统一增加任务标识。当前 `file.download.error` 只有错误文本，多个任务并行时缺少精确关联依据；本次局部注销解决订阅所有权，不构成完整的多任务事件协议。
2. 把单实例迁移作为一个完整启动流程处理，覆盖双击启动、快捷方式 `--launch`、主窗口未就绪和更新后重启。
3. 将长耗时备份、恢复和 LIP 操作逐步改为接收 `context.Context` 的服务方法，再考虑使用 Wails 生成绑定的取消能力。必须先让后端操作真正响应取消，不能只取消前端等待。
4. 自更新迁移应在 provider、校验、提权和回滚流程明确后实施，并单独验证发布端兼容性。

## 验证与边界

- 目录打开单元测试覆盖中文/空格/特殊字符路径、递归创建、普通文件拒绝、非法子路径和系统处理器失败；测试注入处理器，不实际打开用户目录。
- 使用 `go test ./...` 验证后端现有测试及新增测试。
- 使用 Wails MCP 执行生产前端任务，重新生成 bindings，并运行样式检查、TypeScript 检查、Vite 构建及 bundle budget 检查。
- 编译 Windows production 可执行文件以检查前后端资源集成。
- 本次未进行桌面界面的交互验收；仍需在实际 WebView2 中检查最大化/还原、剪贴板占用、原生拖拽和系统文件管理器打开行为。
- 构建中的 Vite `__dirname` 配置兼容性警告属于现有构建配置问题，本次不影响构建通过。
