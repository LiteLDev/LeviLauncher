# HeroUI v3 全量迁移

## 基线与实施计划

- 基线分支：v1；迁移分支：codex/heroui-v3-migration。已有 build/ios/project.pbxproj 修改保持独立。
- React 19、Tailwind CSS 4 已就绪。HeroUI 从 2.8.10 升级到 3.2.4，样式包使用相同版本。
- 50 个直接导入 HeroUI 的源码文件；下表记录迁移前的全部组件使用量与文件，包含 shim 中未使用的导出另行删除。
- 阶段 1：Button、Card、Chip、Avatar、Image、Spinner、Separator，保留按钮加载反馈、图片回退和键盘行为。
- 阶段 2：TextField/InputGroup、TextArea、Checkbox、Switch、Slider，迁移标签、验证、清除和受控事件。
- 阶段 3：Select/ListBox、Dropdown、Tabs、Popover、Tooltip，明确集合 id/textValue、单选与多选及弹出层位置。
- 阶段 4：BaseModal/UnifiedModal、Toast、Table、Pagination、useOverlayState；验证关闭状态、焦点恢复、进度和表格空态。
- 阶段 5：统一切换依赖，删除 v2 shim、Provider、hero.ts 和配置别名；迁移主题色、语义 token、状态选择器。Framer Motion 仍由页面转场、拖拽覆盖层和动画偏好功能直接使用，保留该应用依赖；HeroUI v3 组件自身使用 CSS 动画。
- 阶段 6：类型检查、生产构建与现有包体积门禁、Playwright 端到端/可访问性及关键页面浅色深色视觉验证。Wails 使用测试模拟；原生安装/启动行为不由浏览器模拟代替验证。

## 组件盘点（迁移前）

| 组件 | JSX 使用数 | 文件 |
| --- | ---: | --- |
| Button | 231 | `src/components/ButtonWithBorderGradient.tsx`、`src/components/ClarityConsentModal.tsx`、`src/components/GlobalNavbar.tsx`、`src/components/LipUpdateModal.tsx`、`src/components/ModdedCard.tsx`、`src/components/SelectionBar.tsx`、`src/components/Sidebar.tsx`、`src/components/TermsModal.tsx`、`src/components/ThemeSwitcher.tsx`、`src/components/TopBar.tsx`、`src/components/UnifiedModal.tsx`、`src/components/UpdateModal.tsx`、`src/components/UserAvatar.tsx`、`src/components/WindowControls.tsx`、`src/pages/AboutPage.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx`、`src/utils/LipTaskConsoleContext.tsx` |
| Chip | 73 | `src/components/LauncherChip.tsx`、`src/components/ModdedCard.tsx`、`src/components/UserAvatar.tsx`、`src/pages/AboutPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/utils/LipTaskConsoleContext.tsx` |
| Card | 59 | `src/components/ContentDownloadCard.tsx`、`src/components/ModdedCard.tsx`、`src/components/SelectionBar.tsx`、`src/pages/AboutPage.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/UpdatingPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| DropdownItem | 59 | `src/components/GlobalNavbar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| CardBody | 58 | `src/components/ContentDownloadCard.tsx`、`src/components/ModdedCard.tsx`、`src/components/SelectionBar.tsx`、`src/pages/AboutPage.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/UpdatingPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Skeleton | 40 | `src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx` |
| SelectItem | 39 | `src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Input | 37 | `src/components/CustomColorPicker.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Tooltip | 37 | `src/components/GlobalNavbar.tsx`、`src/components/Sidebar.tsx`、`src/components/TopBar.tsx`、`src/components/UserAvatar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Tab | 32 | `src/pages/CurseForgeModPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/SettingsPage.tsx` |
| Spinner | 29 | `src/App.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Select | 28 | `src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Progress | 23 | `src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadManagerPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/UpdatingPage.tsx`、`src/pages/WorldsListPage.tsx`、`src/utils/LipTaskConsoleContext.tsx` |
| Dropdown | 22 | `src/components/GlobalNavbar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| DropdownTrigger | 22 | `src/components/GlobalNavbar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| DropdownMenu | 22 | `src/components/GlobalNavbar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/ContentPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/LauncherPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ServersPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Switch | 18 | `src/pages/InstallPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/SettingsPage.tsx`、`src/pages/WorldLevelDatEditorPage.tsx` |
| TableColumn | 17 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| TableCell | 17 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| Checkbox | 14 | `src/components/SelectionBar.tsx`、`src/pages/BehaviorPacksPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Divider | 11 | `src/pages/InstanceSettingsPage.tsx`、`src/pages/OnboardingPage.tsx`、`src/pages/SettingsPage.tsx` |
| Tabs | 9 | `src/pages/CurseForgeModPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/InstanceSettingsPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/ModsPage.tsx`、`src/pages/SettingsPage.tsx` |
| Image | 8 | `src/pages/BehaviorPacksPage.tsx`、`src/pages/CurseForgeModPage.tsx`、`src/pages/LIPPackagePage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/ScreenshotsPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| Pagination | 7 | `src/pages/BehaviorPacksPage.tsx`、`src/pages/CurseForgePage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPage.tsx`、`src/pages/ResourcePacksPage.tsx`、`src/pages/SkinPacksPage.tsx`、`src/pages/WorldsListPage.tsx` |
| CardHeader | 6 | `src/components/ContentDownloadCard.tsx`、`src/components/ModdedCard.tsx`、`src/pages/InstallPage.tsx`、`src/pages/InstanceSelectPage.tsx`、`src/pages/LauncherPage.tsx` |
| Slider | 4 | `src/pages/SettingsPage.tsx` |
| ModalContent | 3 | `src/components/UnifiedModal.tsx`、`src/components/UpdateModal.tsx`、`src/pages/ScreenshotsPage.tsx` |
| Table | 3 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| TableHeader | 3 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| TableBody | 3 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| TableRow | 3 | `src/pages/CurseForgeModPage.tsx`、`src/pages/DownloadPage.tsx`、`src/pages/LIPPackagePage.tsx` |
| Avatar | 2 | `src/components/UserAvatar.tsx` |
| ToastProvider | 1 | `src/App.tsx` |
| Modal | 1 | `src/components/BaseModal.tsx` |
| ModalHeader | 1 | `src/components/BaseModal.tsx` |
| ModalBody | 1 | `src/components/BaseModal.tsx` |
| ModalFooter | 1 | `src/components/BaseModal.tsx` |
| ScrollShadow | 1 | `src/components/ModdedCard.tsx` |
| Popover | 1 | `src/components/UserAvatar.tsx` |
| PopoverTrigger | 1 | `src/components/UserAvatar.tsx` |
| PopoverContent | 1 | `src/components/UserAvatar.tsx` |
| User | 1 | `src/components/UserAvatar.tsx` |
| Link | 1 | `src/pages/CurseForgeModPage.tsx` |
| ButtonGroup | 1 | `src/pages/DownloadPage.tsx` |
| Textarea | 1 | `src/pages/InstanceSettingsPage.tsx` |

## 全部导入点（迁移前）

- `src/App.tsx`：ToastProvider、Spinner
- `src/hero.ts`：heroui
- `src/components/BaseModal.tsx`：Modal、ModalProps、ModalHeader、ModalBody、ModalFooter、ModalHeaderProps、ModalBodyProps、ModalFooterProps
- `src/components/ButtonWithBorderGradient.tsx`：ButtonProps、LinkProps、Button
- `src/components/ClarityConsentModal.tsx`：Button
- `src/components/ContentDownloadCard.tsx`：Card、CardBody、CardHeader
- `src/components/CustomColorPicker.tsx`：Input
- `src/components/GlobalNavbar.tsx`：Button、Tooltip、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem
- `src/components/LauncherChip.tsx`：Chip
- `src/components/LipUpdateModal.tsx`：Button
- `src/components/ModdedCard.tsx`：Card、CardBody、Button、ScrollShadow、Chip、CardHeader
- `src/components/SelectionBar.tsx`：Button、Checkbox、Card、CardBody
- `src/components/Sidebar.tsx`：Button、Tooltip
- `src/components/TermsModal.tsx`：Button
- `src/components/ThemeSwitcher.tsx`：Button
- `src/components/TopBar.tsx`：Button、Tooltip
- `src/components/UnifiedModal.tsx`：ModalContent、Button、ButtonProps
- `src/components/UpdateModal.tsx`：Button、ModalContent
- `src/components/UserAvatar.tsx`：Avatar、Popover、PopoverTrigger、PopoverContent、User、Button、Chip、Tooltip
- `src/components/WindowControls.tsx`：Button
- `src/hooks/useContentPage.ts`：useDisclosure
- `src/hooks/useInstanceSettings.ts`：useDisclosure、addToast
- `src/hooks/useLauncher.ts`：useDisclosure
- `src/hooks/useModsPage.ts`：addToast、useDisclosure
- `src/hooks/useSettings.ts`：useDisclosure
- `src/pages/AboutPage.tsx`：Button、Card、CardBody、Chip
- `src/pages/BehaviorPacksPage.tsx`：Button、Chip、Image、Spinner、Tooltip、useDisclosure、Input、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Checkbox、Pagination、Card、CardBody、addToast、Select、SelectItem、Progress
- `src/pages/ContentPage.tsx`：Button、Card、CardBody、Select、SelectItem、Dropdown、DropdownItem、DropdownMenu、DropdownTrigger、Spinner、Tooltip、Progress
- `src/pages/CurseForgeModPage.tsx`：Button、Spinner、Chip、Image、Link、Card、CardBody、Tabs、Tab、Table、TableHeader、TableColumn、TableBody、TableRow、TableCell、Tooltip、Select、SelectItem、Progress、Skeleton
- `src/pages/CurseForgePage.tsx`：Button、Chip、Input、Select、SelectItem、Pagination、Skeleton、Card、CardBody
- `src/pages/DownloadManagerPage.tsx`：Card、CardBody、Progress、Button、Chip、Tooltip
- `src/pages/DownloadPage.tsx`：Button、Chip、Dropdown、DropdownItem、DropdownMenu、DropdownTrigger、Input、Pagination、Table、TableHeader、TableColumn、TableBody、TableRow、TableCell、Progress、Spinner、useDisclosure、Card、CardBody、ButtonGroup、Tooltip、addToast
- `src/pages/InstallPage.tsx`：Button、Input、Switch、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Card、CardBody、CardHeader、Chip、Spinner、Select、SelectItem、useDisclosure、addToast
- `src/pages/InstanceSelectPage.tsx`：Button、Card、CardBody、CardHeader、Tabs、Tab、Input、Chip、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、addToast
- `src/pages/InstanceSettingsPage.tsx`：Button、Card、CardBody、Checkbox、Input、Select、SelectItem、Switch、Chip、Progress、Textarea、Tabs、Tab、Divider
- `src/pages/LauncherPage.tsx`：Button、Card、CardHeader、CardBody、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Input、Chip、Progress、Spinner
- `src/pages/LIPPackagePage.tsx`：addToast、Button、Card、CardBody、Chip、Image、Select、SelectItem、Spinner、Skeleton、Tab、Table、TableBody、TableCell、TableColumn、TableHeader、TableRow、Tabs、Tooltip
- `src/pages/LIPPage.tsx`：Button、Input、Pagination、Skeleton、Card、CardBody、Chip、Select、SelectItem
- `src/pages/ModsPage.tsx`：Button、Card、CardBody、Chip、Input、Progress、Spinner、Switch、Checkbox、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Tabs、Tab
- `src/pages/OnboardingPage.tsx`：Card、CardBody、Button、Input、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Divider、useDisclosure
- `src/pages/ResourcePacksPage.tsx`：Button、Chip、Image、Spinner、Tooltip、useDisclosure、Input、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Checkbox、Pagination、Card、CardBody、addToast、Select、SelectItem、Progress、Modal、ModalContent、ModalHeader、ModalBody、ModalFooter
- `src/pages/ScreenshotsPage.tsx`：Button、Spinner、Tooltip、useDisclosure、Card、CardBody、addToast、Checkbox、Image、ModalContent
- `src/pages/ServersPage.tsx`：Button、Card、CardBody、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Input、Tooltip、Spinner、Chip、addToast
- `src/pages/SettingsPage.tsx`：Card、CardBody、CardHeader、Button、Chip、Input、Divider、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Spinner、Progress、Switch、Tabs、Tab、Slider、Select、SelectItem、addToast
- `src/pages/SkinPacksPage.tsx`：Button、Chip、Image、Spinner、Tooltip、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Checkbox、Pagination、Card、CardBody、Input、useDisclosure、addToast、Select、SelectItem、Progress
- `src/pages/UpdatingPage.tsx`：Card、CardBody、Progress
- `src/pages/WorldLevelDatEditorPage.tsx`：Button、Card、CardBody、Input、Spinner、Switch、Select、SelectItem、Chip、Tooltip
- `src/pages/WorldsListPage.tsx`：Button、Input、Card、CardBody、Dropdown、DropdownTrigger、DropdownMenu、DropdownItem、Checkbox、Image、Spinner、Tooltip、useDisclosure、Pagination、addToast、Select、SelectItem、Progress
- `src/utils/DownloadsContext.tsx`：addToast
- `src/utils/LipTaskConsoleContext.tsx`：Button、Chip、Progress、addToast

## Hooks、样式与依赖专项

- 68 处 useDisclosure 分布于页面和业务 Hooks；全部替换 useOverlayState，区分 open/close/setOpen/toggle。
- 样式来源：src/style.css、src/hero.ts、src/constants/componentStyles.ts、src/hooks/useThemeColors.ts、ButtonWithBorderGradient 和组件内 classNames。
- 根配置：vite.config.js、tsconfig.json、src/shims/heroui.ts、src/providers/HeroUIProvider.ts、App.tsx。
- 清理 package.json 和 package-lock.json 中的 v2 包；核验 npm 依赖树无 v2 HeroUI。

## 验证结果

全部阶段完成，验证日期：2026-09-09。

| 验证 | 实际结果 |
| --- | --- |
| npm ci | 干净安装通过；npm 报告 0 vulnerabilities |
| npm run type-check | 通过；生产构建也再次执行 tsc --noEmit |
| npm run build | 通过，包含原有 check-bundle 门禁 |
| npm run test:e2e -- --workers=2 | 28 passed (28.0s) |
| git diff --check | 通过 |
| HeroUI 依赖树 | 仅 @heroui/react@3.2.4 与 @heroui/styles@3.2.4；无 v2 子包 |
| 残留扫描 | 无 useDisclosure、HeroUIProvider、v2 shim、旧 classNames/onValueChange API、--heroui 变量 |

生产包预算保持原值：总计 8,261,267 / 9,500,000 字节；JavaScript 2,761,822 / 3,800,000 字节；CSS 332,282 / 370,000 字节；最大 JavaScript 块 380,448 / 850,000 字节。

### 回归覆盖与修复

- 原有 8 项测试：首次设置键盘路径、启动 inert 隔离、未知路由恢复、首页深浅色 WCAG、依赖提示框 WCAG、缩放、减少动画偏好。深色测试现同时设置 app.themeMode 并断言实际根元素主题。
- 新增 20 项：单选/多选与集合身份、菜单选择、清除输入、设置页标签/开关、Escape 弹层关闭与路由隔离、分页首尾边界、详情标签中的描述/非空文件表格、实例选择与 Toast/独立设置按钮、7 个页面的深浅色渲染。
- 逐项修正 v3 Slot 的样式归属，保留表格吸顶与空态、品牌色阶、弹窗拖动区和层级；修复实例卡片嵌套按钮。
- Escape 在弹层尚未完成焦点切换时不再触发全局后退；测试等待弹层退出和键盘焦点稳定。
- 对照原始源码扫描界面翻译调用，原有文案调用未丢失；详情内容使用 Tabs.Panel。
- 默认全量 CSS 超预算后，采用 src/heroui.css 按使用的组件导入样式；未放宽原有预算。直接导入的 tw-animate-css 也显式声明为依赖。
- 浏览器截图已检查，保存在 frontend/.artifacts/heroui-*.png（不纳入生产包）。

### 验证边界

Playwright 在真实 Chromium 中运行生产构建，通过现有 Wails 模拟层验证前端。未执行真实游戏安装、启动、系统依赖安装或原生桌面打包；这些原生行为不属于浏览器模拟验证结论。构建保留 Vite 对既有 __dirname 配置的未来兼容提示，测试运行器也报告环境中 NO_COLOR/FORCE_COLOR 并存提示，均不影响本次检查通过。

### 文档依据

通过 heroui-migration MCP 获取 Full Migration、组件、Hooks 与 Styling 指南；Table、Pagination、Progress 补充读取官方 MDX 文档，并以安装的 3.2.4 类型和实际浏览器行为核对。
