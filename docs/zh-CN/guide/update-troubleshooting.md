# 更新与故障排查

当 LeviLauncher 的安装、更新或启动行为不符合预期时，请优先参考这一页。

## 常见问题

### Gaming Services 缺失或损坏

- 使用启动器提供的引导进行安装或修复。
- 必要时回到 Microsoft Store 检查 Minecraft Bedrock 的安装状态。

### GameInput 缺失

- 按 LeviLauncher 提示安装所需 redistributable。
- 安装完成后，如有需要请重启启动器。

### 安装路径不可写

- 在 **Settings** 中把托管内容路径改到可写位置。
- 如果启动器在安装或自更新时要求提升权限，只在你确认可信时允许执行。

### 某个版本无法启动

- 先测试该版本在无 Mods 情况下能否启动
- 检查所需 Windows 组件是否仍然存在
- 确认该版本是否已经完整安装
- 试着建立一个干净的隔离测试环境复现问题

### 下载很慢或反复失败

- 切换镜像
- 稍后重试
- 如条件允许，改用本地包来源

## 更安全的恢复步骤

1. 先备份重要世界。
2. 移除最近新增的 Mods 或资源包。
3. 用一个干净的隔离版本做对照测试。
4. 必要时重新安装受影响版本。

## 关于自更新

LeviLauncher 可以检查、下载并安装应用更新。若安装目录不可写，某些环境下可能需要管理员权限。

## 什么时候应该提 Issue

当满足以下条件时，建议到 GitHub 提交问题：

- 你能稳定复现问题
- 做过基础恢复步骤后仍然存在
- 问题看起来更像 LeviLauncher 自身，而不是一般 Windows 环境问题

提交时请尽量附带：

- Windows 版本
- LeviLauncher 版本
- 你的具体操作步骤
- 截图或日志（如果有）

问题反馈入口： [GitHub Issues](https://github.com/LiteLDev/LeviLauncher/issues)

