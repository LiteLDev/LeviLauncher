# 系统要求与安装

这一页说明 LeviLauncher 正常安装与管理 Minecraft Bedrock (GDK) 版本前，需要满足哪些条件。

## 系统要求

| 项目 | 要求 |
| --- | --- |
| 操作系统 | Windows 10 或 Windows 11 |
| 游戏版本 | Minecraft Bedrock Edition (GDK) |
| 授权 | 绑定在 Microsoft 账号下的正版授权 |
| 网络 | 用于下载版本、获取元数据、测速镜像与检查更新 |

## 必要的 Windows 组件

首次启动或安装前，LeviLauncher 可能会引导你安装缺失组件。

- **Microsoft Gaming Services**
- **Microsoft GameInput**
- **WebView2 Runtime**

具体是否缺失，取决于你的 Windows 环境状态。

## 在安装版本之前

请先完成这份检查清单：

1. 至少从 Microsoft Store 安装过一次 Minecraft Bedrock。
2. 如果商店状态异常，先启动一次游戏确认安装完整。
3. 使用 LeviLauncher 安装或管理版本前，先关闭游戏。

## 安装 LeviLauncher 本体

### 方案 A：GitHub Releases

适合希望直接从官方发布页下载并查看更新记录的用户。

1. 打开 [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases)。
2. 下载安装程序。
3. 运行并完成安装向导。

### 方案 B：蓝奏云镜像

如果你所在地区访问 GitHub 速度较慢，这个入口通常更方便。

1. 打开 [蓝奏云](https://levimc.lanzoue.com/b016ke39hc)。
2. 输入密码 `levi`。
3. 下载后在本地运行安装程序。

## 安装第一个托管版本

1. 在 LeviLauncher 内打开 **Download**。
2. 选择 **Release** 或 **Preview**。
3. 选择目标版本条目。
4. 决定是否启用隔离。
5. 开始安装并等待完成。

## 推荐安装策略

### 什么时候选 Release

- 你想要更稳定的日常游玩环境
- 你在经营长期世界
- 你希望 Mods 与资源包变化更少

### 什么时候选 Preview

- 你想提前体验未来功能
- 你能接受不稳定或兼容性变化
- 你愿意把 Preview 环境与正式游玩环境分开

::: tip 多数玩家的推荐做法
先建立一个**隔离的 Release 版本**。只有在明确需要时，再额外添加 Preview。
:::

## 如果安装无法继续

以下问题都可以继续参考[更新与故障排查](./update-troubleshooting)：

- 缺少 Gaming Services
- 缺少 GameInput
- 商店授权或安装状态不完整
- 目标路径不可写
- 下载或镜像失败

