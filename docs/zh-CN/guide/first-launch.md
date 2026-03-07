# 首次启动

第一次打开 LeviLauncher 时，应用会优先确认 Windows 与 Minecraft 环境是否满足托管安装要求。

## 你可能会看到什么

首次启动时，LeviLauncher 可能会：

- 检查 Gaming Services 是否可用
- 检查 GameInput 是否可用
- 验证 Minecraft Bedrock (GDK) 环境是否已准备就绪
- 在缺失关键组件时显示引导或提示

## 常见首次启动流程

1. 打开 LeviLauncher。
2. 阅读启动器给出的引导信息。
3. 安装或修复缺失的前置组件。
4. 返回启动器重新检查状态。
5. 满足条件后进入 **Download** 页面。

## 如果提示缺少组件

### Gaming Services

如果 Gaming Services 缺失或损坏，LeviLauncher 可能会引导你去 Microsoft Store 安装或修复。

### GameInput

如果缺少 GameInput，请按提示安装所需 redistributable。

### 未检测到 Minecraft Bedrock

请确认：

- 当前 Microsoft 账号拥有游戏授权
- 当前电脑上已经从 Microsoft Store 安装过游戏
- 你不是试图绕过官方授权流程使用启动器

## 给新用户的好默认值

- 先使用一个隔离的 Release 版本
- 把下载与内容路径放在可写的磁盘位置
- 第一次成功纯净启动之前，不要急着导入大量 Mods

## 首次启动之后建议继续阅读

- [版本管理](./version-management)
- [内容管理](./content-management)
- [设置与个性化](./settings-personalization)

