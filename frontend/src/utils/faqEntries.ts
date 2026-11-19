// Deep links into the LeviMC FAQ (https://levimc.org/faq/), one entry per
// question the launcher refers to. The site generates anchors from each
// language's question text, so each language has its own anchor; the
// questions are in the same order on every language's page.
//
// Generated from the FAQ pages; refresh when the FAQ changes.

export type FaqLanguage = "en" | "zh-Hans" | "zh-Hant";

type FaqEntry = { section: "errors" | "install" | "versions" | "usage" } & Record<FaqLanguage, string>;

export const FAQ_ENTRIES = {
  brd: {
    section: "errors",
    "en": "after-using-betterrenderdragon-in-levilauncher-pc-the-game-crashes-shows-a-white-screen-has-no-effect-or-looks-wrong-in-a-world-how-do-i-troubleshoot",
    "zh-Hans": "在-levilauncher-pc-中使用-betterrenderdragon-更好的渲染龙-后游戏闪退、白屏、不生效或进世界画面异常-怎么排查",
    "zh-Hant": "在-levilauncher-pc-中使用-betterrenderdragon-更好的渲染龍-後遊戲閃退、白屏、不生效或進世界畫面異常-怎麼排查",
  },
  register: {
    section: "errors",
    "en": "what-is-register-to-system-in-levilauncher-for-and-how-do-i-fix-a-registration-failure-cannot-register-appx",
    "zh-Hans": "levilauncher-的-注册到系统-有什么用-注册失败-无法注册-appx-怎么解决",
    "zh-Hant": "levilauncher-的-註冊到系統-有什麼用-註冊失敗-無法註冊-appx-怎麼解決",
  },
  launch_game: {
    section: "errors",
    "en": "after-installing-the-game-with-the-windows-levilauncher-no-mods-installed-launching-reports-err-launch-game-or-cannot-start-and-other-versions-behave-the-same-what-should-i-do-and-which-dependencies-are-needed",
    "zh-Hans": "windows-版-levilauncher-安装游戏后-没装任何-mod-点启动就报-err-launch-game-或无法启动-换版本也一样-怎么办-需要哪些依赖",
    "zh-Hant": "windows-版-levilauncher-安裝遊戲後-沒裝任何-mod-點啟動就報-err-launch-game-或無法啟動-換版本也一樣-怎麼辦-需要哪些依賴",
  },
  gaming_services: {
    section: "errors",
    "en": "levilauncher-reports-gaming-services-missing-or-detection-failed-when-launching-the-gdk-edition-of-minecraft-and-the-microsoft-store-cannot-install-it-or-the-system-is-ltsc-without-a-store-what-should-i-do",
    "zh-Hans": "levilauncher-启动-gdk-版-minecraft-时提示缺少-gaming-services-游戏服务-或检测失败-微软商店装不上或系统是-ltsc-没有商店-怎么办",
    "zh-Hant": "levilauncher-啟動-gdk-版-minecraft-時提示缺少-gaming-services-遊戲服務-或檢測失敗-微軟商店裝不上或系統是-ltsc-沒有商店-怎麼辦",
  },
  launcher_startup: {
    section: "errors",
    "en": "the-pc-levilauncher-crashes-or-does-nothing-after-a-double-click-it-exits-before-any-window-appears-and-there-is-no-process-in-task-manager-how-do-i-troubleshoot",
    "zh-Hans": "pc-版-levilauncher-双击后闪退或没反应-界面出现前就退出-任务管理器里也没有进程-怎么排查",
    "zh-Hant": "pc-版-levilauncher-雙擊後閃退或沒反應-界面出現前就退出-任務管理器裡也沒有進程-怎麼排查",
  },
  install_crash: {
    section: "errors",
    "en": "after-levilauncher-finishes-downloading-minecraft-clicking-install-crashes-the-launcher-the-version-folder-was-created-but-is-empty-how-do-i-fix-it",
    "zh-Hans": "levilauncher-下载完-minecraft-后点-安装-启动器就闪退-版本文件夹已创建但为空-怎么解决",
    "zh-Hant": "levilauncher-下載完-minecraft-後點-安裝-啟動器就閃退-版本文件夾已創建但為空-怎麼解決",
  },
  long_break_crash: {
    section: "errors",
    "en": "after-not-playing-for-a-long-time-every-mc-version-in-levilauncher-crashes-immediately-on-launch-no-log-the-console-flashes-by-but-launching-the-official-way-works-what-should-i-do",
    "zh-Hans": "很久没玩后-levilauncher-里的-mc-版本全部启动即闪退-没有日志、控制台一闪而过-但官方方式启动正常-怎么办",
    "zh-Hant": "很久沒玩後-levilauncher-裡的-mc-版本全部啟動即閃退-沒有日誌、控制台一閃而過-但官方方式啟動正常-怎麼辦",
  },
  crash_logs: {
    section: "errors",
    "en": "where-do-i-find-the-crash-logs-after-the-game-crashes-in-levilauncher-pc",
    "zh-Hans": "levilauncher-pc-游戏崩溃后-崩溃日志在哪里找",
    "zh-Hant": "levilauncher-pc-遊戲崩潰後-崩潰日誌在哪裡找",
  },
  licensed_copy_crash: {
    section: "errors",
    "en": "the-pc-levilauncher-reports-an-error-when-launching-mc-it-looks-like-it-demands-a-licensed-copy-or-the-game-crashes-right-after-launching-what-should-i-do",
    "zh-Hans": "pc-版-levilauncher-启动-mc-时报错-看起来像要求正版-或启动后立即崩溃-怎么处理",
    "zh-Hant": "pc-版-levilauncher-啟動-mc-時報錯-看起來像要求正版-或啟動後立即崩潰-怎麼處理",
  },
  missing_dll: {
    section: "errors",
    "en": "levilauncher-shows-a-missing-dll-missing-file-error-and-still-cannot-start-after-updating-vcredist-what-should-i-do",
    "zh-Hans": "使用-levilauncher-时弹出缺失-dll-文件缺失的错误-更新-vcredist-后仍然进不去-怎么办",
    "zh-Hant": "使用-levilauncher-時彈出缺失-dll-文件缺失的錯誤-更新-vcredist-後仍然進不去-怎麼辦",
  },
  mod_crash: {
    section: "errors",
    "en": "after-installing-a-mod-in-levilauncher-for-example-a-mod-that-removes-oreui-the-game-crashes-what-is-going-on",
    "zh-Hans": "在-levilauncher-中装了某个-mod-如去除-oreui-的-mod-后游戏闪退-怎么回事",
    "zh-Hant": "在-levilauncher-中裝了某個-mod-如去除-oreui-的-mod-後遊戲閃退-怎麼回事",
  },
  bedrockboot: {
    section: "errors",
    "en": "after-installing-the-game-with-levilauncher-it-reports-insufficient-storage-and-worlds-and-mods-are-lost-version-isolation-is-on-bedrockboot-was-installed-before-what-is-the-cause",
    "zh-Hans": "用-levilauncher-安装游戏后提示存储不足-存档、模组丢失-开启了版本隔离-曾安装过-bedrockboot-是什么原因",
    "zh-Hant": "用-levilauncher-安裝遊戲後提示存儲不足-存檔、模組丟失-開啟了版本隔離-曾安裝過-bedrockboot-是什麼原因",
  },
  gdk_hang: {
    section: "errors",
    "en": "the-bedrock-edition-gdk-installed-through-levilauncher-cannot-be-started-from-the-start-menu-a-shortcut-or-the-launcher-and-the-process-hangs-in-task-manager-what-should-i-do",
    "zh-Hans": "通过-levilauncher-安装的基岩版-gdk-版-从开始菜单、快捷方式或启动器都无法启动-任务管理器里进程卡住-怎么办",
    "zh-Hant": "通過-levilauncher-安裝的基岩版-gdk-版-從開始菜單、快捷方式或啟動器都無法啟動-任務管理器裡進程卡住-怎麼辦",
  },
  ll_version_missing: {
    section: "install",
    "en": "a-new-levilamina-client-version-has-been-released-but-it-still-does-not-appear-in-levilauncher-after-refreshing-what-should-i-do",
    "zh-Hans": "levilamina-已发布新的客户端版本-但-levilauncher-里刷新后迟迟看不到-怎么办",
    "zh-Hant": "levilamina-已發佈新的客戶端版本-但-levilauncher-裡刷新後遲遲看不到-怎麼辦",
  },
  slow_download: {
    section: "install",
    "en": "downloading-a-minecraft-version-in-the-pc-version-of-levilauncher-is-very-slow-do-i-need-a-proxy",
    "zh-Hans": "在-pc-版-levilauncher-里下载-minecraft-版本很慢-必须挂代理吗",
    "zh-Hant": "在-pc-版-levilauncher-裡下載-minecraft-版本很慢-必須掛代理嗎",
  },
  license_not_entitled: {
    section: "install",
    "en": "levilauncher-windows-says-the-current-account-has-no-license-for-this-version-err-license-not-entitled-when-downloading-or-installing-a-game-version-what-should-i-do",
    "zh-Hans": "levilauncher-windows-下载-安装游戏版本时提示-当前账户没有此版本的授权-err-license-not-entitled-怎么办",
    "zh-Hant": "levilauncher-windows-下載-安裝遊戲版本時提示-當前賬戶沒有此版本的授權-err-license-not-entitled-怎麼辦",
  },
  download_failing: {
    section: "install",
    "en": "downloading-game-versions-or-other-content-in-levilauncher-keeps-failing-network-error-and-changing-networks-or-using-a-proxy-does-not-help-how-do-i-troubleshoot-where-is-the-launcher-log",
    "zh-Hans": "levilauncher-下载游戏版本或其他内容一直失败-网络错误-换网络、开代理都不行-怎么排查-启动器日志在哪",
    "zh-Hant": "levilauncher-下載遊戲版本或其他內容一直失敗-網絡錯誤-換網絡、開代理都不行-怎麼排查-啟動器日誌在哪",
  },
  lip_install_failed: {
    section: "install",
    "en": "installing-levilamina-into-an-instance-in-levilauncher-keeps-failing-or-hangs-on-resolving-with-err-lip-install-failed-context-deadline-exceeded-http-404-500-504-connection-to-127-0-0-1-443-failed-etc-how-do-i-fix-it",
    "zh-Hans": "在-levilauncher-中给实例安装-levilamina-一直失败或卡在-resolving-报-err-lip-install-failed-context-deadline-exceeded、http-404-500-504、连接-127-0-0-1-443-失败等-怎么解决",
    "zh-Hant": "在-levilauncher-中給實例安裝-levilamina-一直失敗或卡在-resolving-報-err-lip-install-failed-context-deadline-exceeded、http-404-500-504、連接-127-0-0-1-443-失敗等-怎麼解決",
  },
  missing_components: {
    section: "install",
    "en": "levilauncher-windows-reports-missing-components-or-cannot-start-when-installing-or-launching-a-version-is-it-an-environment-problem",
    "zh-Hans": "levilauncher-windows-安装或启动版本时提示缺少组件或无法启动-是环境问题吗",
    "zh-Hant": "levilauncher-windows-安裝或啟動版本時提示缺少組件或無法啟動-是環境問題嗎",
  },
  non_ascii_path: {
    section: "install",
    "en": "levilauncher-fails-to-install-the-game-mod-management-clicks-do-nothing-or-the-game-will-not-start-after-downloading-and-a-path-or-file-name-contains-chinese-characters-how-do-i-fix-it",
    "zh-Hans": "levilauncher-安装游戏报错、模组管理点击无效或下载完无法启动-路径或文件名中带中文-怎么解决",
    "zh-Hant": "levilauncher-安裝遊戲報錯、模組管理點擊無效或下載完無法啟動-路徑或文件名中帶中文-怎麼解決",
  },
  uwp_support: {
    section: "versions",
    "en": "does-levilauncher-windows-support-the-uwp-edition-and-minecraft-versions-older-than-1-21-120-can-it-import-a-local-appx-installer",
    "zh-Hans": "levilauncher-windows-版-支持-uwp-版-1-21-120-之前的旧版本-minecraft-吗-能导入本地-appx-安装包吗",
    "zh-Hant": "levilauncher-windows-版-支持-uwp-版-1-21-120-之前的舊版本-minecraft-嗎-能導入本地-appx-安裝包嗎",
  },
  injection_clients: {
    section: "versions",
    "en": "can-levilauncher-be-used-together-with-third-party-injection-clients-such-as-onix-flarial-latite-and-oderso-why-do-flarial-and-others-not-detect-a-game-launched-with-levilauncher",
    "zh-Hans": "levilauncher-能否和-onix、flarial、latite、oderso-等第三方注入客户端一起使用-为什么-flarial-等识别不到用-levilauncher-启动的游戏",
    "zh-Hant": "levilauncher-能否和-onix、flarial、latite、oderso-等第三方注入客戶端一起使用-為什麼-flarial-等識別不到用-levilauncher-啟動的遊戲",
  },
  preview_license: {
    section: "versions",
    "en": "i-own-a-licensed-copy-but-levilauncher-says-i-need-to-unlock-the-full-game-turns-into-a-trial-or-the-microsoft-store-only-shows-the-education-edition-when-downloading-or-launching-the-preview-what-should-i-do",
    "zh-Hans": "已购买正版-但用-levilauncher-下载或启动预览版-preview-时提示需要-解锁完整游戏-、变成试玩版-或微软商店只显示教育版-怎么办",
    "zh-Hant": "已購買正版-但用-levilauncher-下載或啟動預覽版-preview-時提示需要-解鎖完整遊戲-、變成試玩版-或微軟商店只顯示教育版-怎麼辦",
  },
  greyed_out: {
    section: "versions",
    "en": "why-is-a-mod-or-the-option-to-install-levilamina-greyed-out-and-unclickable-in-levilauncher",
    "zh-Hans": "levilauncher-中某个模组或安装-levilamina-的选项显示为灰色、不能点-是什么原因",
    "zh-Hant": "levilauncher-中某個模組或安裝-levilamina-的選項顯示為灰色、不能點-是什麼原因",
  },
  windows_update: {
    section: "versions",
    "en": "levilauncher-says-windows-needs-an-update-but-the-system-for-example-a-rented-cloud-pc-does-not-allow-updates-what-should-i-do",
    "zh-Hans": "levilauncher-提示需要更新-windows-但系统-如租用的云电脑-不允许更新-怎么办",
    "zh-Hant": "levilauncher-提示需要更新-windows-但系統-如租用的雲電腦-不允許更新-怎麼辦",
  },
  ll_unsupported: {
    section: "versions",
    "en": "installing-levilamina-for-a-new-game-version-such-as-26-51-3-with-levilauncher-says-unsupported-version-do-i-need-to-install-it-manually-with-lip",
    "zh-Hans": "使用-levilauncher-为新游戏版本-如-26-51-3-安装-levilamina-时提示-不支持的版本-需要手动用-lip-安装吗",
    "zh-Hant": "使用-levilauncher-為新遊戲版本-如-26-51-3-安裝-levilamina-時提示-不支持的版本-需要手動用-lip-安裝嗎",
  },
  old_uwp_auth: {
    section: "versions",
    "en": "why-does-licensed-copy-verification-fail-when-i-run-an-old-uwp-version-of-bedrock-edition-such-as-1-18-12-with-the-launcher-and-how-do-i-fix-it",
    "zh-Hans": "用启动器运行-uwp-旧版本基岩版-如-1-18-12-时正版验证失效-原因是什么-如何修复",
    "zh-Hant": "用啟動器運行-uwp-舊版本基岩版-如-1-18-12-時正版驗證失效-原因是什麼-如何修復",
  },
  switch_account: {
    section: "usage",
    "en": "how-do-i-switch-the-microsoft-xbox-account-in-the-pc-version-of-levilauncher-it-keeps-going-back-to-the-original-account-after-i-click-switch-account",
    "zh-Hans": "pc-版-levilauncher-怎么切换微软-xbox-账号-点-切换账户-后一直回到原来的账号怎么办",
    "zh-Hant": "pc-版-levilauncher-怎麼切換微軟-xbox-賬號-點-切換賬戶-後一直回到原來的賬號怎麼辦",
  },
  download_no_progress: {
    section: "usage",
    "en": "levilauncher-shows-no-progress-when-downloading-a-game-version-and-nothing-happens-for-a-long-time-is-it-stuck",
    "zh-Hans": "levilauncher-下载游戏版本时看不到进度、长时间没反应-是卡死了吗",
    "zh-Hant": "levilauncher-下載遊戲版本時看不到進度、長時間沒反應-是卡死了嗎",
  },
  need_license: {
    section: "usage",
    "en": "do-i-need-a-licensed-minecraft-bedrock-edition-to-use-levilauncher-can-i-use-it-offline-or-without-buying-the-game",
    "zh-Hans": "使用-levilauncher-必须拥有正版-minecraft-基岩版吗-能否离线-不买正版使用",
    "zh-Hant": "使用-levilauncher-必須擁有正版-minecraft-基岩版嗎-能否離線-不買正版使用",
  },
} satisfies Record<string, FaqEntry>;

export type FaqId = keyof typeof FAQ_ENTRIES;
