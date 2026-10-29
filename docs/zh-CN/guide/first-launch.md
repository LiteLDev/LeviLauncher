# 首次启动

第一次打开 LeviLauncher 时，应用会先确认你的设备与 Minecraft 已经准备好进行托管版本管理。

## 你可能会看到什么

=== "Windows"

    首次启动时，LeviLauncher 可能会：

    - 检查 Gaming Services 是否可用（GDK）
    - 检查 GameInput 是否可用（GDK）
    - 检查原生模组加载器所需的 Visual C++ 运行库是否已安装
    - 验证 Minecraft 基岩版（GDK / UWP）环境是否已准备就绪
    - 在缺失关键组件时显示引导或提示

=== "Android"

    首次启动时，LeviLauncher 可能会：

    - 请你接受启动器协议
    - 请求所需权限
    - 检查是否安装了 Google Play 官方 Minecraft（1.21.80 或更高版本）
    - 迁移旧版启动器留下的数据目录

## 常见首次启动流程

=== "Windows"

    1. 打开 LeviLauncher。
    2. 阅读启动器给出的引导信息。
    3. 安装或修复缺失的前置组件。
    4. 返回启动器重新检查状态。
    5. 满足条件后进入 **下载** 页面。

=== "Android"

    1. 打开 LeviLauncher。
    2. 接受协议并授予请求的权限。
    3. 如果启动器正在迁移旧数据，请等待完成。在确认世界、资源包和设置都可用之前，不要删除旧目录。
    4. 进入版本页面。

## 如果提示缺少组件 <Badge platform="windows" />

### Gaming Services

如果 Gaming Services 缺失或损坏，LeviLauncher 可能会引导你去 Microsoft Store 安装或修复。

### GameInput

如果缺少 GameInput，请按提示安装所需 redistributable。

### 未检测到 Minecraft 基岩版

请确认：

- 当前 Microsoft 账号拥有游戏授权
- 当前电脑上已经从 Microsoft Store 安装过游戏
- 你不是试图绕过官方授权流程使用启动器

## 给新用户的好默认值

- 先使用一个隔离的正式版（Release）版本
- 把下载与内容路径放在可写的磁盘位置
- 第一次成功纯净启动之前，不要急着导入大量模组

## 首次启动之后建议继续阅读

- [版本管理](./version-management)
- [内容管理](./content-management)
- [设置与个性化](./settings-personalization)
