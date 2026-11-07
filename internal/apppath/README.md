# 存储布局与命名规则

LeviLauncher 读写的每一个路径都在这里定义，名称常量在 `apppath.go`。新增路径时先改这份文档，代码评审中遇到文档里没有的路径按问题处理。

## 目录

| 目录 | 位置 | 内容 |
| --- | --- | --- |
| 配置目录 | `%APPDATA%\LeviLauncher\` | 只放 `config.json`：体积小，可以随用户漫游 |
| 本机目录 | `%LOCALAPPDATA%\LeviLauncher\` | `Logs\`、`Accounts\`、`WebView2\`、`storage-migration.json` |
| 数据根 `<BaseRoot>` | 默认就是本机目录；`config.json` 的 `base_root` 可以改到别处 | `Instances\`、`Backups\`、`Cache\` |
| 缓存目录 | `<BaseRoot>\Cache\` | 随时可以删除、不丢任何东西的文件 |

```
%APPDATA%\LeviLauncher\
  config.json
%LOCALAPPDATA%\LeviLauncher\          默认的 <BaseRoot>
  Logs\startup.log                    以及 startup.1.log … startup.4.log
  Accounts\                           DPAPI 加密的微软账号和设备记录，以及 account.lock、device.lock
  WebView2\EBWebView\                 WebView2 用户数据（前端 localStorage）
  storage-migration.json              从旧布局迁移的记录
  storage-migration.journal           迁移进行中的日志，完成后删除
<BaseRoot>\
  Instances\<实例>\                   游戏实例
    .levilauncher\                    启动器在实例内的状态（隐藏）
      instance.json                   实例元数据
      icon.ico                        由 LargeLogo.png 生成的图标
      xbox-auth-key-v1                UWP 实例的 Xbox 验证密钥标记
    .lip\                             lip 的工作区状态（lip 协议规定）
    mods\                             模组（LeviLamina 与 lip 规定）
  Backups\
    Worlds\<版本>\<文件夹>_<世界>\<世界>_<yyyyMMdd-HHmmss>.mcworld
    Instances\<实例>\<实例>_<yyyyMMdd-HHmmss>.zip
  Cache\
    Installers\                       游戏安装包、VC++ 和 WebView2 安装程序
    Updates\                          启动器更新包，安装后删除
    Downloads\                        为导入而下载的文件，超过 1 天后启动时清理
    Imports\                          前端交给后端导入的文件，超过 1 天后启动时清理
    gdk-pending-versions.json
    gamertags.json
```

启动器 exe 旁边只读 `webview2.json`，不写入任何文件。

## 命名规则

| # | 规则 | 例子 |
| --- | --- | --- |
| N1 | 每个已知文件夹下的产品目录一律叫 `LeviLauncher`，不再从 exe 文件名推导 | `%APPDATA%\LeviLauncher` |
| N2 | 启动器自己的目录用 PascalCase 英文名词，存放一类东西的用复数 | `Instances`、`Backups\Worlds`、`Cache\Installers` |
| N3 | 启动器自己的数据文件用小写 kebab-case 加扩展名 | `config.json`、`gdk-pending-versions.json` |
| N4 | 二进制文件用 PascalCase 产品名，这是 Windows 对模块名的惯例 | `LeviLauncher.dll` |
| N5 | 暂存文件和暂存目录叫 `<最终名>.<随机串>.tmp`，建在目标旁边（同一个卷上），写完后原子 rename 到位。不设隐藏属性，因为属性会随 rename 留在最终文件上；也不用前导点，它在 Windows 上没有隐藏的含义。需要续传的部分下载用固定名 `<最终名>.partial.tmp`，同一时间只能有一个写者 | `config.json.3f9a.tmp` |
| N6 | 实例内归启动器管的状态集中放在 `<实例>\.levilauncher\`，和 lip 的 `.lip\` 是同一种结构。只有这种工作区状态目录允许用前导点，同样设隐藏属性 | `.levilauncher\instance.json` |
| N7 | 用户命名的部分（实例名、世界名）按 Windows 规则清洗：去掉 `<>:"/\|?*`，去掉结尾的点和空格，避开 `CON`、`NUL` 等保留名 | |
| N8 | 生成的文件名统一为 `<名称>_<yyyyMMdd-HHmmss>.<扩展名>` | `MyWorld_20261107-142301.mcworld` |
| N9 | 外部格式规定的名称原样保留，不受上面任何一条约束 | 见下文 |

N9 保留的名称：

- 游戏：`Minecraft Bedrock[ Preview]`、`Users`、`games\com.mojang\minecraftWorlds`、`resource_packs`、`level.dat`、`levelname.txt`、`AppxManifest.xml`、`MicrosoftGame.Config`、`Minecraft.Windows.exe`、`LargeLogo.png`、UWP 的 `Packages\<包族名>\LocalState`。
- LeviLamina 与 lip：`mods\`、`manifest.json`、`.lip\`、`tooth_lock.json`、`%APPDATA%\lip`、`%LOCALAPPDATA%\lip\Cache`。
- 用户可见的显示名：桌面快捷方式 `Minecraft - <实例>.lnk`。

## 写入方式

- 一律通过 `internal/fsx` 写入：`WriteFileAtomic`、`CreateTemp`、`MkdirTemp`、`PartialPath`。读者只会看到写入前或写入后的完整文件。
- 不写系统临时目录 `%TEMP%`，这一点与 lip 的 userspace 协议相同。
- `apppath` 的取路径函数只计算路径，不建目录。固定目录由启动时调用一次的 `EnsureDirs` 创建；其他目录由写入方自己创建。
- 桌面快捷方式只写当前用户的桌面（`FOLDERID_Desktop`），不写公共桌面。

## 从旧布局迁移

旧布局是 `%APPDATA%\<exe 名>\`，里面放 `config.json`、`logs\`、`microsoft-account\`、`EBWebView\`、`user_gamertag_map.json`、`gdk_pending_versions.json`，数据根下是 `versions\`、`backups\`、`installers\`、`updates\`。实例里则是 `version.json`、`LargeLogo.ico` 和 `.levilauncher-xbox-auth-key-v1`。

`internal/storage` 在启动时迁移旧布局：拿到单实例锁之后、读 `config.json` 之前执行，只在新的 `config.json` 不存在时运行。

- **无损：** 只在同一个卷上 rename；跨卷时复制，并保留旧文件。旧布局里的文件一个都不删，旧的 `config.json` 也原样保留。目标已存在时合并目录，同名的一项改名为 `<名称>-legacy` 后保留。
- **数据不跨卷搬：** 自己设置过 `base_root` 的用户，数据留在原处，只改目录名。默认数据根与 `%LOCALAPPDATA%` 不在同一个卷时（比如漫游配置文件被重定向），数据也留在原处，`base_root` 写成旧目录。
- **全有或全无：** 每一步先写进 `storage-migration.journal` 并刷盘，然后才执行，做法与 lip 的 `.lip/journal` 相同。失败时按日志倒序撤销。进程中途退出的话，下次启动先撤销再重试。
- **暂时无法迁移时：** 例如游戏还在从某个实例运行，目录无法 rename。这时撤销全部改动，本次以旧布局运行（`apppath.SetLegacyMode`），下次启动再试。
- **系统注册：** Windows 里登记的已注册实例仍指向旧路径。`storage-migration.json` 记录了每一项从哪里搬到哪里，`storage.TranslateLegacyPath` 据此把旧路径映射到新位置。下次启动该实例时会在新位置重新注册：UWP 在 `uwp.LaunchWithPreparation` 中处理，GDK 在 `versionlaunch` 中处理。移除旧注册时始终带 `-PreserveApplicationData`。
- **快捷方式：** 迁移后第一次启动时，后台把桌面上 `Minecraft - *.lnk` 的图标指向新位置。
- **实例内文件按需迁移：** `version.json` 在第一次读取时原样搬到 `.levilauncher\instance.json`，Xbox 验证标记也在第一次使用时搬过去。图标在下次生成时写到新位置，旧的 `LargeLogo.ico` 保留，因为未修复的快捷方式可能还指向它。
