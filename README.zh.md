# LeviLauncher

一个面向 **Windows** 的 **Minecraft 基岩版（GDK / UWP）** 桌面启动器。

## 背景

LeviLauncher 用于帮助你安装、管理、隔离、扩展并启动正式版或预览版环境，同时让内容与工具链更容易整理。

本仓库是 LeviLauncher 的 **Windows 版**。**Android 版**在 [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid) 开发，两者共用同一套文档 https://docs.levimc.org/LeviLauncher/zh-CN/ （按 Windows / Android 标签页区分）。Android 原生模组的开发文档在 [LeviLamina 文档](https://docs.levimc.org/LeviLamina/zh/developer_guides/android/)。我们没有开发 iOS 版的计划。

## 安装

### 使用要求

- Windows 10/11
- 拥有 Minecraft 基岩版的正版授权
- Microsoft Edge WebView2 Runtime
- GDK：系统中可用的 Microsoft Gaming Services 与 Microsoft GameInput
- UWP：Windows 开发者模式，以及游戏包清单要求的 UWP 框架依赖
- 原生模组加载器（`preload-native` DLL 模组）：桌面版 Visual C++ 运行库（x64），启动器会检查并提示安装

UWP x64 实例支持在「实例设置 → 启动选项」中启用控制台，并通过「管理模组」以 `.mcmod` 模组包导入与游戏版本兼容的 `preload-native` 模组（选择「所有文件」仍可导入 `.zip` 包或单独的 `.dll` 文件）。UWP x64 的 1.19.70.2 及以上版本支持实例隔离；未启用隔离的 UWP 实例继续共享同一应用渠道原有的 `LocalState` 内容。安装时部署加载器，注册后启动时准备原生加载功能，游戏继续使用 Windows 应用激活。x86/ARM64 包仍可普通启动，但当前原生加载器仅支持 x64。LeviLamina 自动安装仍以受支持的 GDK 版本为准。

### 下载地址

- **GitHub Releases：** https://github.com/LiteLDev/LeviLauncher/releases
- **蓝奏云：** <https://levimc.lanzoue.com/b016ke39hc>（密码：`levi`）

## 使用

请参阅用户文档：

- **用户文档：** https://docs.levimc.org/LeviLauncher/
- **中文文档：** https://docs.levimc.org/LeviLauncher/zh-CN/

### 命令行

不带参数运行 `LeviLauncher.exe` 打开启动器窗口；带子命令时不显示窗口，执行完即退出：

```text
LeviLauncher.exe list                   列出已安装的实例
LeviLauncher.exe launch <实例>          启动实例
LeviLauncher.exe call                   列出 GUI 使用的全部后端方法
LeviLauncher.exe call <方法> [参数...]  调用其中一个
LeviLauncher.exe help                   显示用法
```

GUI 的所有操作都通过 `call` 列出的方法完成，因此命令行可以完成 GUI 能做的一切（下载、安装、导入内容、管理模组、设置等）。字符串参数按原样传入，其他类型按 JSON 解析（`true`、`42`、`["a"]`）。结果输出到 stdout（纯字符串直接输出，其余为 JSON），进度事件以 JSON 行输出到 stderr，下载会等待完成。退出码：成功为 0，调用失败（Go 错误或 `ERR_...` 错误码）为 1，用法错误为 2。

`LeviLauncher.exe` 是 GUI 程序，终端默认不会等待它结束：需要在提示符返回前拿到输出或退出码时，cmd 中请用 `start /wait LeviLauncher.exe ...`，PowerShell 中可将输出接入管道（`LeviLauncher.exe list | Out-String`）或使用 `Start-Process -Wait -NoNewWindow`。

## 参与贡献

### 问题反馈

- **Issues：** https://github.com/LiteLDev/LeviLauncher/issues
- 提交时建议附带 Windows 版本、LeviLauncher 版本、复现步骤，以及相关日志或截图。

### 社区

- **Discord：** https://discord.gg/v5R5P4vRZk
- **QQ 群：** https://qm.qq.com/q/1z791rJgJG

### 文档本地开发

用户文档发布在 https://docs.levimc.org/LeviLauncher/zh-CN/ 。若你要在本地预览文档站：

```bash
cd docs
npm ci
npm run dev
```

应用本体开发仍沿用仓库现有的 `frontend/`、`build/` 与 Wails 任务流。

## 许可证

本项目使用 GPL-3.0-only 许可。完整许可文本见 `COPYING`，随附的第三方组件见 `THIRD_PARTY_NOTICES`。
