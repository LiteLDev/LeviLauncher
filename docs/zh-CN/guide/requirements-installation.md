# 系统要求与安装

本页说明 LeviLauncher 在 Windows 与 Android 上安装并启动 Minecraft 基岩版之前需要具备的条件。

UWP 版本需启用 Windows 开发者模式，并安装包清单要求的 UWP 框架依赖（例如 Microsoft.VCLibs）。下文 Gaming Services 与 GameInput 的要求适用于 GDK。GDK 版本以及 1.19.70.2 及以上的 UWP x64 版本支持实例隔离；未启用隔离的 UWP 实例共享该 Windows 应用的 `LocalState` 内容。UWP x64 实例还支持控制台和兼容的 `preload-native` DLL 模组，但 LeviLamina 自动安装仅适用于受支持的 GDK 版本。

## 系统要求

=== "Windows"

    | 项目 | 要求 |
    | --- | --- |
    | 操作系统 | Windows 10 或 Windows 11 |
    | 游戏版本 | Minecraft 基岩版（GDK / UWP） |
    | 授权 | 绑定在 Microsoft 账号下的正版授权 |
    | 网络 | 用于下载版本、获取元数据、测速镜像与检查更新 |

=== "Android"

    | 项目 | 要求 |
    | --- | --- |
    | 操作系统 | Android 9.0（API 28）或更高版本 |
    | 架构 | ARM64（arm64-v8a） |
    | 内存 | 1 GB 可用内存（推荐 3 GB 以上） |
    | 存储 | 2 GB 可用空间用于 Minecraft 与游戏数据（推荐 5 GB 以上） |
    | 游戏 | 从 Google Play 安装的 Minecraft 基岩版 1.21.80 或更高版本 |
    | 授权 | 在 Google Play 购买的正版 Minecraft |

## 必要的 Windows 组件 <Badge platform="windows" />

首次启动或安装前，LeviLauncher 可能会引导你安装缺失组件。

- **Microsoft Gaming Services**（GDK）
- **Microsoft GameInput**（GDK）
- **WebView2 Runtime**
- **Microsoft Visual C++ 运行库（x64）**：原生模组加载器需要，LeviLauncher 会检查并提示安装

具体是否缺失，取决于你的 Windows 环境状态。

### 使用自定义 WebView2 Fixed Version Runtime

如果系统中没有 Evergreen WebView2 Runtime，但你已经自行管理了 WebView2 Fixed Version Runtime，可以使用以下任一种方式启动 LeviLauncher。

通过命令行参数指定：

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

或者在 `LeviLauncher.exe` 同一目录创建 `webview2.json`：

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

相对路径以 `webview2.json` 所在目录为基准。指定目录必须直接包含 `msedgewebview2.exe`。命令行参数的优先级高于配置文件；如果两者都没有配置，LeviLauncher 会继续使用系统安装的 WebView2 Runtime。

## 在安装版本之前

=== "Windows"

    请先完成这份检查清单：

    1. 至少从 Microsoft Store 安装过一次 Minecraft 基岩版。
    2. 如果商店状态异常，先启动一次游戏确认安装完整。
    3. 使用 LeviLauncher 安装或管理版本前，先关闭游戏。

=== "Android"

    1. 在设备上从 Google Play 安装 Minecraft 基岩版。
    2. 更新到 1.21.80 或更高版本：更旧的版本会被启动器拒绝。
    3. 确认有足够的存储空间存放你要保留的版本和世界。

## 安装 LeviLauncher 本体

=== "Windows"

    ### 方案 A：GitHub Releases 页面

    适合希望直接从 LeviLauncher 官方下载页获取安装包，并顺便查看更新记录的用户。

    1. 打开 LeviLauncher 的 [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) 页面。
    2. 下载安装程序。
    3. 运行并完成安装向导。

    ### 方案 B：蓝奏云镜像

    如果你所在地区访问 GitHub 速度较慢，这个入口通常更方便。

    1. 打开 [蓝奏云](https://levimc.lanzoue.com/b016ke39hc)。
    2. 输入密码 `levi`。
    3. 下载后在本地运行安装程序。

=== "Android"

    1. 打开 Android 版的 [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) 页面。
    2. 下载最新 APK。
    3. 允许你用来下载的浏览器或文件管理器安装应用。
    4. 安装并打开 APK（设备上显示为“LeviLauncher”）。
    5. 接受启动器协议，并授予应用请求的权限。

## 安装第一个托管版本

=== "Windows"

    1. 在 LeviLauncher 内打开 **下载**。
    2. 选择你要安装的 Minecraft **正式版** 或 **预览版**，以及 **GDK** 或 **UWP** 包类型。
    3. 选择目标版本条目。
    4. 决定是否启用隔离。
    5. 开始安装并等待完成。

=== "Android"

    1. 打开版本（实例）页面。
    2. 选择已安装的 Google Play 版本，或导入官方 Minecraft APK。
    3. 对需要独立数据的导入版本启用版本隔离。
    4. 回到启动页并点击 **启动**。

## 推荐安装策略 <Badge platform="windows" />

### 什么时候选正式版（Release）

- 你想要更稳定的日常游玩环境
- 你在经营长期世界
- 你希望模组与资源包变化更少

### 什么时候选预览版（Preview）

- 你想提前体验未来功能
- 你能接受不稳定或兼容性变化
- 你愿意把 Minecraft 预览版（Preview）环境与日常游玩环境分开

::: tip 多数玩家的推荐做法
先建立一个**隔离的正式版（Release）版本**。只有在明确需要体验预览内容时，再额外添加**预览版（Preview）**。
:::

## 如果安装无法继续

=== "Windows"

    以下问题都可以继续参考[更新与故障排查](./update-troubleshooting)：

    - 缺少 Gaming Services
    - 缺少 GameInput
    - 商店授权或安装状态不完整
    - 目标路径不可写
    - 下载或镜像失败

=== "Android"

    - 如果启动器提示找不到 Minecraft 或版本不受支持，请先安装或更新 Google Play 版本。
    - 不支持低于 1.21.80 的版本。
    - 启动失败请参阅[更新与故障排查](./update-troubleshooting)。
