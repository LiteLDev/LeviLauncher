# 5 分钟上手

如果你只想用最短路径从下载走到开始游戏，请先看这一页。在任意标签页中选择 **Windows** 或 **Android**，整套文档都会跟随你的选择。

## 1. 安装 LeviLauncher

=== "Windows"

    - 从 [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) 页面或[蓝奏云镜像](https://levimc.lanzoue.com/b016ke39hc)下载 LeviLauncher 安装包，蓝奏云密码为 `levi`。
    - 在 Windows 上运行安装程序。
    - 如果安装过程提示需要 WebView2 或其他依赖，请允许其继续安装。

=== "Android"

    - 从 [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) 下载最新 APK。
    - 允许你使用的浏览器或文件管理器安装应用，然后安装 APK。
    - 打开 LeviLauncher，接受启动器协议并授予它请求的权限。

## 2. 准备好 Minecraft

=== "Windows"

    - 登录拥有 **Minecraft 基岩版** 的 Microsoft 账号。
    - 至少在当前电脑上从 Microsoft Store 安装过一次游戏。
    - 确保 **Gaming Services** 可用（GDK 版本需要）。

    ::: warning 必须拥有正版
    LeviLauncher 用于管理 Minecraft 基岩版（GDK / UWP），并不能替代游戏授权。
    :::

=== "Android"

    - 在本设备上从 **Google Play** 安装 **Minecraft 基岩版** 1.21.80 或更高版本。
    - LeviLauncher 把这份官方游戏作为可信的游戏来源，它不能代替游戏授权。

## 3. 选择版本

=== "Windows"

    - 启动 LeviLauncher。
    - 从侧边栏进入 **下载**。
    - 选择 **正式版** 或 **预览版**，以及 **GDK** 或 **UWP** 包。
    - 这里的 **正式版** 和 **预览版** 指的是 Minecraft 的正式版（Release）与预览版（Preview）。
    - 选择版本并开始安装。

=== "Android"

    - 进入版本（实例）页面。
    - 选择要运行的 Minecraft 版本，或导入官方 Minecraft APK，在启动器内运行其他版本。

## 4. 决定是否启用隔离

=== "Windows"

    隔离会把该版本的游戏数据放进独立工作区，而不是与默认 AppData 环境混在一起。GDK 版本以及 1.19.70.2 及以上的 UWP x64 版本支持隔离。

    以下场景建议启用隔离：

    - 测试模组，但不想影响主环境
    - 想把 Minecraft 正式版与预览版分开
    - 想给不同游玩目的保留不同的世界和资源包

=== "Android"

    启动导入版本时，如果它的世界、资源包和设置要与 Google Play 安装的版本分开，请启用版本隔离。导入版本通常需要启用后才能启动。

## 5. 启动已安装版本

=== "Windows"

    - 返回 **启动** 页面。
    - 选中刚安装的版本。
    - 点击 **启动**。

    如果 LeviLauncher 报告缺少 GameInput 或 Gaming Services，请先按提示完成依赖安装。

=== "Android"

    - 回到启动页并点击 **启动**。
    - 如果启动失败，先从失败页面打开启动日志查看原因，再调整设置。

## 6. 继续阅读对应章节

- 需要准备环境：[系统要求与安装](./requirements-installation)
- 需要了解首次进入应用会发生什么：[首次启动](./first-launch)
- 需要整理版本：[版本管理](./version-management)
- 需要管理世界、资源包或截图：[内容管理](./content-management)
- 需要使用模组或集成源：[模组与集成](./mods-integrations)
