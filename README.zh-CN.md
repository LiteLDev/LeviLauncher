<p align="center">
  <img src="build/appicon.png" alt="LeviLauncher" width="96">
</p>
<h1 align="center">LeviLauncher</h1>
<p align="center">Minecraft Bedrock Edition GDK 启动器（Windows）</p>

<p align="center">
  <a href="https://github.com/LiteLDev/LeviLauncher/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/LiteLDev/LeviLauncher?style=flat-square&color=blue"></a>
  <a href="https://github.com/LiteLDev/LeviLauncher"><img alt="License" src="https://img.shields.io/github/license/LiteLDev/LeviLauncher"></a>
  <a href="https://github.com/LiteLDev/LeviLauncher/issues"><img alt="Issues" src="https://img.shields.io/github/issues/LiteLDev/LeviLauncher?style=flat-square&color=red"></a>
  <a href="https://github.com/LiteLDev/LeviLauncher"><img alt="Stars" src="https://img.shields.io/github/stars/LiteLDev/LeviLauncher?style=flat-square&color=yellow"></a>
  <a href="https://www.microsoft.com/windows"><img alt="Windows 10/11" src="https://img.shields.io/badge/Windows-10%2F11-green?style=flat-square&logo=windows"></a>
</p>

<p align="center">
  <a href="https://discord.gg/v5R5P4vRZk"><img alt="Discord" src="https://img.shields.io/discord/849252980430864384?style=for-the-badge&logo=discord"></a>
  <a href="https://qm.qq.com/q/1z791rJgJG"><img alt="QQ 群 458083875" src="https://img.shields.io/badge/458083875-red?style=for-the-badge&logo=qq"></a>
</p>

<p align="center">
  <sup>🌐 语言: <a href="./README.md">English</a> • <b>中文</b></sup>
</p>

一个专为 Minecraft Bedrock Edition GDK 版本打造的桌面启动器。

支持在 Windows 上安装、管理和启动正式版与预览版，提供版本隔离、内容管理（世界/资源包/行为包/皮肤包）、模组管理与世界备份等功能。前端基于 React + Vite + Tailwind，后端使用 Go 与 Wails v3。

## 项目状态
- 🚧 仍在积极开发中，功能持续完善，可能存在不稳定情况。

## 适用范围
- 面向 Minecraft GDK（Windows）。需要合法的正版游戏副本。

## 问题反馈
- 使用 GitHub Issues 提交问题或建议：https://github.com/LiteLDev/LeviLauncher/issues
- 提交时请附带操作系统版本、启动器版本、复现步骤与日志/截图。

## 下载地址
- GitHub Releases：https://github.com/LiteLDev/LeviLauncher/releases
- 蓝奏云：https://levimc.lanzoue.com/b016ke39hc（密码：`levi`）

## 主要特性
- 版本管理：安装、删除、重命名与快速启动，支持正式版与预览版。
- 版本隔离：将游戏数据重定向到 `versions/<名称>/Minecraft Bedrock(Preview)`，避免与系统 AppData 混淆。
- 内容管理：统计并管理世界、资源包、行为包、皮肤包，支持一键打开对应目录与拖拽导入。
- 模组管理：导入 `.zip`/`.dll`，启用/禁用、删除模组，自动准备依赖与预加载器。
- 世界工具：备份世界为 `.mcworld`，编辑 `level.dat` 字段与世界名称。
- 下载与镜像：内置镜像测速与选择，支持本地安装包导入与安装进度展示。
- 快捷方式：为指定版本创建桌面快捷方式，支持自定义图标。
- 更新机制：检测新版、下载并安装更新，必要时请求管理员权限。
- 多语言：内置英文与简体中文。

## 系统要求
- 操作系统：Windows 10/11。
- 必备组件：Microsoft Gaming Services、Microsoft GameInput（首次运行可引导安装）。
- WebView2 Runtime：随安装包引导安装或由系统提供。

## 快速开始（开发者）
- 安装依赖：
  - Go `1.24`（或更高，见 `go.mod`）。
  - Node.js `18+`（用于前端构建）。
  - 安装 Wails v3 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- 开发模式：
  - 在项目根目录执行：`wails3 dev -config ./build/config.yml -port 1145`
  - 或分步启动前端：`cd frontend && npm install && npm run dev`
- 构建：
  - `wails3 task build`（按当前平台构建）
  - Windows 专用：`wails3 task windows:build`
- 运行：
  - `wails3 task run`

## 目录结构
- `frontend/`：React + Vite 前端（`package.json`、`src/`、`assets/locales/`）。
- `internal/`：后端逻辑（版本管理、内容处理、更新等）。
- `build/`：跨平台打包配置与 Taskfile 集合。
- `main.go`：应用入口，嵌入前端静态资源并创建窗体。

## 命令行参数
- `--launch=<版本名称>`：启动指定版本并退出启动器。
- `--self-update=<当前版本>`：以管理员权限启动更新流程（用于无写权限目录）。

## 社区
- Discord：`https://discord.gg/v5R5P4vRZk`
- QQ 群：`458083875`（`https://qm.qq.com/q/1z791rJgJG`）

## 常见问题
- 启动提示缺少 GameInput：按照引导下载并安装 `GameInputRedist.msi` 后重试。
- 提示缺少 Gaming Services：打开 Microsoft Store 安装后重试。
- 版本隔离与继承：可在下载/安装时启用隔离，并从同类型隔离版本或 GDK 目录复制数据。
- 目录不可写：在设置中变更内容根目录，或以管理员方式安装/更新。
- 语言切换：设置页支持英文/简体中文切换。

## 参与贡献
- 欢迎通过 Issue 与 Pull Request 参与贡献。
- 在提交前请尽量复现问题并附带必要信息。

## 许可证
Copyright © 2025 LeviMC, All rights reserved.

本项目的非闭源部分以 LGPL-3.0 许可发布；详情请参见仓库中的 [COPYING](COPYING) 和 [COPYING.LESSER](COPYING.LESSER) 文件。
