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
  storage-migration.json              从旧布局迁移的记录；没有旧数据的机器上第一次启动也会写一份（不含搬迁项）
  storage-migration.journal           迁移进行中的日志，完成后删除
  storage-migration.lock              决定布局时独占锁定，运行期间共享锁定
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
- 桌面快捷方式只写当前用户的桌面（`FOLDERID_Desktop`），不写公共桌面。迁移后修复快捷方式时，旧版可能写过的桌面（`%USERPROFILE%\Desktop`、OneDrive 桌面、公共桌面）也一并检查。

## 从旧布局迁移

旧布局是 `%APPDATA%\<exe 名>\`，里面放 `config.json`、`logs\`、`microsoft-account\`、`EBWebView\`、`user_gamertag_map.json`、`gdk_pending_versions.json`，数据根下是 `versions\`、`backups\`、`installers\`、`updates\`。实例里则是 `version.json`、`LargeLogo.ico` 和 `.levilauncher-xbox-auth-key-v1`。

`internal/storage` 在启动时迁移旧布局：拿到单实例锁之后、读 `config.json` 之前执行，VC++ 和 WebView2 运行库的检查也排在它之后，下载的安装程序因此落在数据根的 `Cache\Installers`。

### 去哪里找旧数据

按顺序检查下面这些目录，每个有旧数据的目录都在同一次启动里迁移：

1. `%APPDATA%\<当前 exe 名>`、`%APPDATA%\levilauncher.exe`；
2. `%LOCALAPPDATA%\<当前 exe 名>`、`%LOCALAPPDATA%\levilauncher.exe`：没有 `APPDATA` 环境变量时，旧版把数据放在这里；
3. 两个文件夹里其他所有 `<名称>.exe` 目录：用别的 exe 名运行过旧版。这类目录要像 LeviLauncher 的才算：`config.json` 里有 `base_root`，或者 `versions\` 下有实例；
4. `%LOCALAPPDATA%\LeviLauncher`：0.0.9 及更早的版本把 `config.json`、`versions\`、`installers\` 放在这里，0.0.8 和 0.0.9 的世界备份在 `backup\worlds\`。这里只看 `config.json` 和小写的旧数据目录。

一个目录有下面任何一项，就算有待迁移的旧数据，不管新的 `config.json` 是否已经存在：

- `config.json`（旧版第一次启动就会写；迁移后改名为 `config.migrated.json`）；
- `versions\`、`backups\`、`backup\`、`installers\`、`updates\` 目录。在新布局正在使用的数据根里只认原样的小写名，因为原地迁移会把 `backups` 改成 `Backups`，Windows 两个名字都能找到；其他地方大小写不论。同名的文件不算；
- 这台机器还没有迁移记录时，`microsoft-account\`、`EBWebView\`、`user_gamertag_map.json`、`gdk_pending_versions.json` 单独存在也算。有了迁移记录就不算，因为跨卷迁移会复制它们并保留原件。

### 数据去哪里

- **新布局没有用户数据时**以旧 `config.json` 为准。没有用户数据是指：没有 `storage-migration.json`，新 `config.json` 没有 `base_root`，`%LOCALAPPDATA%\LeviLauncher\Instances` 里也没有实例。这样的新布局只可能是某次跳过了迁移的启动或者某个工具建出来的。
  - 设置过 `base_root` 的，数据留在原处，只改目录名，`base_root` 不变；
  - 默认数据根与 `%LOCALAPPDATA%` 在同一个卷上，数据搬到 `%LOCALAPPDATA%\LeviLauncher`；
  - 不在同一个卷上（比如漫游配置文件被重定向），数据留在原处，`base_root` 写成旧目录。
- **新布局有用户数据时**以新的 `config.json` 为准：`base_root` 原样保留，旧数据并入它指向的数据根。这包括已有迁移记录的情况，旧目录没有 `config.json` 而新的已经存在的情况，以及用户在新版本里设置过数据根或建过实例、之后又运行了一次旧版本的情况。旧 `config.json` 指向的数据根暂时不在（比如移动硬盘没插）时，什么都不动，本次以新布局运行（`Deferred`），下次再试：旧 `config.json` 是唯一指向它的记录。
- 旧 `base_root` 是相对路径时，按启动器所在目录解析，旧版启动时会切换到那里。`base_root` 键名大小写不论，与 `config.Load` 一致。指向旧目录的写法（短文件名、联接）也认作默认数据根。
- 0.0.8 和 0.0.9 的 `backup\worlds\` 并入 `Backups\Worlds\`。
- 旧 `base_root` 设在别处时，旧目录自己的 `versions\`、`installers\` 等（旧版在读 `config.json` 之前就会建）也一并迁入。旧版以旧布局运行时（见下）写下的 `<数据根>\Cache\` 并入新的 `Cache\`。

### 怎么搬

- **无损：** 同一个卷上 rename；跨卷时（包括经过指向别的盘的联接）复制，保留原件。原件是 `versions\` 这类数据目录时，原地改名为 `<名称>.migrated`，不再算待迁移。旧布局里的文件一个都不删。旧的 `config.json` 原地改名为 `config.migrated.json`；已经有了就改名为 `config.migrated-2.json`，依此类推。目录的子项全部 rename 走之后，空的旧目录会删除（同样记进日志，撤销时重建）。
- **合并进已有的新布局：** 目标已存在时合并目录。`Backups\` 和 `Cache\` 下逐层合并，比如 `backups\worlds\` 并入已有的 `Backups\Worlds\`。其他目录只合并一层：同名的一项（比如同名实例）改名为 `<名称>-legacy` 后保留。实例的名称以文件夹名为准，所以 `A-legacy` 会以这个名字列出。
- 目标位置被一个同名文件占着时（比如一个叫 `Instances` 的文件），这个文件改名为 `<名称>-before-migration` 后保留。旧数据目录是联接或符号链接、目标是空目录时，链接本身替换空目录。
- **冲突时以谁为准：** `config.json` 按键合并，只在一边有的键都保留。
  - 新布局没有用户数据时，以旧数据为准：`config.json` 中旧值优先；`Accounts\`、`WebView2\EBWebView\`、`gamertags.json`、`gdk-pending-versions.json` 整体换成旧的，已有的一项改名为 `<名称>-before-migration` 后保留。
  - 新布局有用户数据时，旧数据只可能来自之后又运行了旧版本，所以以新数据为准：`config.json` 中新值优先；上面这几项已存在时保留新的，旧的留在原处不动。新的迁移记录追加到原有记录之后。
  - 被替换的 `config.json` 和 `storage-migration.json` 先改名为 `<名称>-before-migration.json` 记进日志，提交后删除，撤销时还原；提交后、删除前退出的话，下次启动删除。无法解析的新 `config.json` 改名为 `config-before-migration.json` 保留。
- **旧 `config.json` 损坏时**（比如旧版写到一半崩溃留下的空文件或截断文件），按空配置迁移，能从文本里找到 `base_root` 就用它，原文件同样改名为 `config.migrated.json` 保留。
- **账号记录跨卷复制后**，重新设置只允许当前用户和 SYSTEM 访问的权限，与账号代码设置的一致。

### 出错时

- **全有或全无：** 每一步先写进 `storage-migration.journal` 并刷盘，然后才执行，做法与 lip 的 `.lip/journal` 相同。失败时按日志倒序撤销，每撤销一步就把日志截短到还有效的记录，撤销中途出错或退出时从停下的地方接着撤销，不会把一步撤销两次。进程中途退出的话，下次启动先撤销再重试。日志里已经有 `Commit` 的迁移算完成，即使日志文件暂时删不掉。
- **撤销时原位置又出现了东西**（中途退出后运行过旧版，它会重建 `versions\`、`config.json`、WebView2 配置、世界备份），不覆盖，以搬回去的原件为准：目录合并回去（`Backups`、`Cache` 逐层合并），名字被占用时新出现的那一项改名为 `-legacy`；文件也是新出现的改名为 `-legacy`，再把原件放回。原件是联接、新出现的是普通目录时无法合并，新出现的目录改名为 `<名称>-legacy` 后保留；旧目录里的 `versions-legacy`、`backups-legacy` 这类目录下次迁移时同样并入对应的位置。
- **暂时无法迁移时：** 例如游戏还在从某个实例运行，目录无法 rename，就撤销全部改动，下次启动再试。用户数据在旧布局里时（第一次迁移），本次以旧布局运行（`apppath.SetLegacyMode`）；用户数据已经在新布局里时，本次照常以新布局运行（`storage.Deferred`）。
- **以旧布局运行时**，实例元数据仍写 `version.json`，不搬进 `.levilauncher`，这样旧版还能读到。
- **没有旧数据时**，第一次启动写一份不含搬迁项的 `storage-migration.json`，表示新布局从此存放用户数据。
- **多个进程：** 决定布局的进程独占锁定 `storage-migration.lock`，之后共享锁定直到退出。拿不到独占锁说明另一个启动器进程正在迁移或正在运行（比如窗口开着时又在命令行运行），这时不动文件：等它迁移完，再按上面的规则以旧布局或新布局运行。

### 迁移之后

- **系统注册：** Windows 里登记的已注册实例仍指向旧路径。`storage-migration.json` 记录了每一项从哪里搬到哪里，包括改名为 `-legacy` 的子项，`storage.TranslateLegacyPath` 据此把旧路径映射到新位置，取最具体的一条，同样具体时取最新的一条。合并进已有目录的每个子项都单独记一条。仍然存在的旧路径不映射：迁移会 rename 走或把复制过的原件改名，所以还在原处的就是还没迁移的数据。下次启动该实例时会在新位置重新注册：UWP 在 `uwp.LaunchWithPreparation` 中处理，GDK 在 `versionlaunch` 中处理。移除旧注册时始终带 `-PreserveApplicationData`。
- **快捷方式：** 迁移后第一次启动时，后台把桌面上 `Minecraft - *.lnk` 的图标指向新位置。实例被改名为 `-legacy` 时，`--launch=` 参数也改成新名字。
- **实例名以文件夹为准：** 列出实例时，元数据里的名字与文件夹名不同（改名为 `-legacy` 的实例，或者在启动器外改过名的文件夹），就把名字改成文件夹名并写回。
- **实例内文件按需迁移：** `version.json` 在第一次读取时原样搬到 `.levilauncher\instance.json`；两个都在时以较新的为准。LeviLauncher.dll 读取的也是两者中较新的一个。Xbox 验证标记在第一次使用时搬过去。图标在下次生成时写到新位置，旧的 `LargeLogo.ico` 保留，因为未修复的快捷方式可能还指向它。
