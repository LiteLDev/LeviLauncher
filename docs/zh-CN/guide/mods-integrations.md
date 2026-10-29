# 模组与集成

LeviLauncher 支持超出“基础启动器”范围的模组相关工作流。

## 当前可用能力

=== "Windows"

    - 导入 `.zip` 模组
    - 导入 `.dll` 模组
    - 启用或禁用已安装模组
    - 删除不再需要的模组
    - 使用 CurseForge 相关浏览与获取流程
    - 使用包安装器 [lip](https://github.com/LiteLDev/lip) 安装软件包
    - 配合 LeviLamina 等加载器工作流

    ::: warning 这是高级区域
    模组、加载器与包安装集成的兼容性变化往往更快，尤其是在 Minecraft 预览版（Preview）上。请把这里当作高级能力，逐步测试。
    :::

=== "Android"

    - 向游戏加载原生模组（`.so` 库）
    - 在隔离版本中维护带模组的环境

    想开发原生模组？请参阅 LeviLamina 文档中的 [Android 原生模组](https://docs.levimc.org/LeviLamina/zh/developer_guides/android/)。

## 推荐的模组工作流

1. 从一个干净的隔离版本开始。
2. 先确认该版本在无模组情况下可以正常启动。
3. 一次只增加一个模组或一小组改动。
4. 每次改动后都重新测试启动。
5. 重要世界始终保留备份路径。

## CurseForge <Badge platform="windows" />

启动器提供 CurseForge 相关浏览与包流程，帮助你在应用内更方便地寻找合适资源。

适合以下需求：

- 需要更引导式的查找体验
- 想更快看到某个项目的兼容文件
- 不想所有搜索都在启动器外完成

## lip 与 LeviLamina <Badge platform="windows" />

这类集成更适合希望使用包驱动或加载器驱动工作流的高级用户。

最佳实践：

- 先从稳定的正式版（Release）版本开始
- 不要一次变更多个高风险因素
- 记录下“最后一次成功工作”的版本组合

## 原生模组 <Badge platform="android" />

原生模组会在游戏进程中运行原生代码。只安装来自可信来源的模组，并先在隔离版本中测试。

如果要自己开发，请参阅 LeviLamina 文档中的 [Android 原生模组](https://docs.levimc.org/LeviLamina/zh/developer_guides/android/)。

## 如果加了模组后出错

- 先禁用最新加入的模组
- 与上一个可正常工作的状态进行对比
- 进一步测试前先转移或备份重要世界
- 然后继续查看[更新与故障排查](./update-troubleshooting)
