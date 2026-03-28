import { defineConfig, type DefaultTheme } from "vitepress";

const repo = "https://github.com/LiteLDev/LeviLauncher";
const releases = `${repo}/releases`;
const discord = "https://discord.gg/v5R5P4vRZk";
const lanzou = "https://levimc.lanzoue.com/b016ke39hc";

const base = "/";

// --- Sidebar Builders ---

function buildEnglishSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Quick Start",
      items: [
        { text: "Get Started in 5 Minutes", link: `${prefix}quick-start` },
        {
          text: "Requirements & Installation",
          link: `${prefix}requirements-installation`,
        },
        { text: "First Launch", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "Core Workflows",
      items: [
        { text: "Version Management", link: `${prefix}version-management` },
        { text: "Downloads & Mirrors", link: `${prefix}downloads-mirrors` },
        { text: "Content Management", link: `${prefix}content-management` },
      ],
    },
    {
      text: "Advanced Features",
      items: [
        { text: "Mods & Integrations", link: `${prefix}mods-integrations` },
        { text: "World Tools", link: `${prefix}world-tools` },
        {
          text: "Settings & Personalization",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "Help",
      items: [
        {
          text: "Update & Troubleshooting",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "FAQ, Community & Feedback",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildSpanishSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Inicio rápido",
      items: [
        { text: "Comienza en 5 minutos", link: `${prefix}quick-start` },
        {
          text: "Requisitos e instalación",
          link: `${prefix}requirements-installation`,
        },
        { text: "Primer inicio", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "Flujos principales",
      items: [
        { text: "Gestión de versiones", link: `${prefix}version-management` },
        { text: "Descargas y espejos", link: `${prefix}downloads-mirrors` },
        { text: "Gestión de contenido", link: `${prefix}content-management` },
      ],
    },
    {
      text: "Funciones avanzadas",
      items: [
        { text: "Mods e integraciones", link: `${prefix}mods-integrations` },
        { text: "Herramientas para mundos", link: `${prefix}world-tools` },
        {
          text: "Ajustes y personalización",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "Ayuda",
      items: [
        {
          text: "Actualizaciones y solución de problemas",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "Preguntas frecuentes, comunidad y comentarios",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildChineseCNSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "快速开始",
      items: [
        { text: "5 分钟上手", link: `${prefix}quick-start` },
        { text: "系统要求与安装", link: `${prefix}requirements-installation` },
        { text: "首次启动", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "核心功能",
      items: [
        { text: "版本管理", link: `${prefix}version-management` },
        { text: "下载与镜像", link: `${prefix}downloads-mirrors` },
        { text: "内容管理", link: `${prefix}content-management` },
      ],
    },
    {
      text: "高级功能",
      items: [
        { text: "Mods 与集成", link: `${prefix}mods-integrations` },
        { text: "世界工具", link: `${prefix}world-tools` },
        { text: "设置与个性化", link: `${prefix}settings-personalization` },
      ],
    },
    {
      text: "帮助",
      items: [
        { text: "更新与故障排查", link: `${prefix}update-troubleshooting` },
        {
          text: "常见问题、社区与反馈",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildChineseHKSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "快速入門",
      items: [
        { text: "5 分鐘上手", link: `${prefix}quick-start` },
        { text: "系統要求與安裝", link: `${prefix}requirements-installation` },
        { text: "首次啟動", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "核心功能",
      items: [
        { text: "版本管理", link: `${prefix}version-management` },
        { text: "下載與鏡像", link: `${prefix}downloads-mirrors` },
        { text: "內容管理", link: `${prefix}content-management` },
      ],
    },
    {
      text: "進階功能",
      items: [
        { text: "Mods 與整合", link: `${prefix}mods-integrations` },
        { text: "世界工具", link: `${prefix}world-tools` },
        { text: "設定與個人化", link: `${prefix}settings-personalization` },
      ],
    },
    {
      text: "協助",
      items: [
        { text: "更新與故障排查", link: `${prefix}update-troubleshooting` },
        {
          text: "常見問題、社群與回饋",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildRussianSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Быстрый старт",
      items: [
        { text: "Начало работы за 5 минут", link: `${prefix}quick-start` },
        {
          text: "Требования и установка",
          link: `${prefix}requirements-installation`,
        },
        { text: "Первый запуск", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "Основные рабочие процессы",
      items: [
        { text: "Управление версиями", link: `${prefix}version-management` },
        { text: "Загрузки и зеркала", link: `${prefix}downloads-mirrors` },
        { text: "Управление контентом", link: `${prefix}content-management` },
      ],
    },
    {
      text: "Расширенные функции",
      items: [
        { text: "Моды и интеграции", link: `${prefix}mods-integrations` },
        { text: "Инструменты для миров", link: `${prefix}world-tools` },
        {
          text: "Настройки и персонализация",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "Помощь",
      items: [
        {
          text: "Обновление и устранение неполадок",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "ЧаВО, сообщество и обратная связь",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildGermanSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Schnellstart",
      items: [
        { text: "In 5 Minuten loslegen", link: `${prefix}quick-start` },
        {
          text: "Anforderungen & Installation",
          link: `${prefix}requirements-installation`,
        },
        { text: "Erster Start", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "Kernabläufe",
      items: [
        { text: "Versionsverwaltung", link: `${prefix}version-management` },
        { text: "Downloads & Spiegel", link: `${prefix}downloads-mirrors` },
        { text: "Inhaltsverwaltung", link: `${prefix}content-management` },
      ],
    },
    {
      text: "Erweiterte Funktionen",
      items: [
        { text: "Mods & Integrationen", link: `${prefix}mods-integrations` },
        { text: "Welt-Tools", link: `${prefix}world-tools` },
        {
          text: "Einstellungen & Personalisierung",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "Hilfe",
      items: [
        {
          text: "Update & Fehlerbehebung",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "FAQ, Community & Feedback",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildJapaneseSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "クイックスタート",
      items: [
        { text: "5 分で始める", link: `${prefix}quick-start` },
        {
          text: "システム要件とインストール",
          link: `${prefix}requirements-installation`,
        },
        { text: "初回起動", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "コア機能",
      items: [
        { text: "バージョン管理", link: `${prefix}version-management` },
        { text: "ダウンロードとミラー", link: `${prefix}downloads-mirrors` },
        { text: "コンテンツ管理", link: `${prefix}content-management` },
      ],
    },
    {
      text: "高度な機能",
      items: [
        { text: "Mod と統合", link: `${prefix}mods-integrations` },
        { text: "ワールドツール", link: `${prefix}world-tools` },
        {
          text: "設定とカスタマイズ",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "ヘルプ",
      items: [
        {
          text: "更新とトラブルシューティング",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "FAQ、コミュニティ、フィードバック",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildKoreanSidebar(prefix: string): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "빠른 시작",
      items: [
        { text: "5분 만에 시작하기", link: `${prefix}quick-start` },
        {
          text: "시스템 요구사항 및 설치",
          link: `${prefix}requirements-installation`,
        },
        { text: "처음 실행하기", link: `${prefix}first-launch` },
      ],
    },
    {
      text: "핵심 워크플로우",
      items: [
        { text: "버전 관리", link: `${prefix}version-management` },
        { text: "다운로드 및 미러", link: `${prefix}downloads-mirrors` },
        { text: "콘텐츠 관리", link: `${prefix}content-management` },
      ],
    },
    {
      text: "고급 기능",
      items: [
        { text: "모드 및 통합", link: `${prefix}mods-integrations` },
        { text: "월드 도구", link: `${prefix}world-tools` },
        {
          text: "설정 및 개인화",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: "도움말",
      items: [
        {
          text: "업데이트 및 문제 해결",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: "자주 묻는 질문, 커뮤니티 및 피드백",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

// --- Navbar Builders ---

function buildEnglishNav(): DefaultTheme.NavItem[] {
  return [
    { text: "Guide", link: "/guide/quick-start" },
    { text: "Downloads", link: releases },
    { text: "Mirror", link: lanzou },
    { text: "Community", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildSpanishNav(): DefaultTheme.NavItem[] {
  return [
    { text: "Guía", link: "/es-ES/guide/quick-start" },
    { text: "Descargas", link: releases },
    { text: "Espejo", link: lanzou },
    { text: "Comunidad", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildChineseCNNav(): DefaultTheme.NavItem[] {
  return [
    { text: "文档", link: "/zh-CN/guide/quick-start" },
    { text: "下载", link: releases },
    { text: "蓝奏云", link: lanzou },
    { text: "社区", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildChineseHKNav(): DefaultTheme.NavItem[] {
  return [
    { text: "文檔", link: "/zh-HK/guide/quick-start" },
    { text: "下載", link: releases },
    { text: "藍奏雲", link: lanzou },
    { text: "社群", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildRussianNav(): DefaultTheme.NavItem[] {
  return [
    { text: "Документация", link: "/ru-RU/guide/quick-start" },
    { text: "Загрузки", link: releases },
    { text: "Зеркало", link: lanzou },
    { text: "Сообщество", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildGermanNav(): DefaultTheme.NavItem[] {
  return [
    { text: "Dokumentation", link: "/de-DE/guide/quick-start" },
    { text: "Downloads", link: releases },
    { text: "Spiegel", link: lanzou },
    { text: "Community", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildJapaneseNav(): DefaultTheme.NavItem[] {
  return [
    { text: "ドキュメント", link: "/ja-JP/guide/quick-start" },
    { text: "ダウンロード", link: releases },
    { text: "ミラー", link: lanzou },
    { text: "コミュニティ", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildKoreanNav(): DefaultTheme.NavItem[] {
  return [
    { text: "가이드", link: "/ko-KR/guide/quick-start" },
    { text: "다운로드", link: releases },
    { text: "미러", link: lanzou },
    { text: "커뮤니티", link: discord },
    { text: "GitHub", link: repo },
  ];
}

// --- Export Config ---

export default defineConfig({
  title: "LeviLauncher",
  description:
    "User documentation for LeviLauncher, a desktop launcher for Minecraft Bedrock Edition (GDK) on Windows.",
  lang: "en-US",
  base,
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ["link", { rel: "icon", href: "/appicon.png" }],
    ["meta", { name: "theme-color", content: "#16a34a" }],
  ],
  themeConfig: {
    logo: "/appicon.png",
    search: {
      provider: "local",
    },
    socialLinks: [{ icon: "github", link: repo }],
    footer: {
      copyright: "Copyright © 2024-2026 LeviMC",
    },
  },
  locales: {
    root: {
      label: "English",
      lang: "en-US",
      link: "/",
      themeConfig: {
        nav: buildEnglishNav(),
        sidebar: {
          "/guide/": buildEnglishSidebar("/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "Previous page", next: "Next page" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "Edit this page on GitHub",
        },
        returnToTopLabel: "Back to top",
        sidebarMenuLabel: "Menu",
        darkModeSwitchLabel: "Appearance",
        lightModeSwitchTitle: "Switch to light theme",
        darkModeSwitchTitle: "Switch to dark theme",
      },
    },
    "es-ES": {
      label: "Español",
      lang: "es-ES",
      link: "/es-ES/",
      themeConfig: {
        nav: buildSpanishNav(),
        sidebar: {
          "/es-ES/guide/": buildSpanishSidebar("/es-ES/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "Página anterior", next: "Siguiente página" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "Editar esta página en GitHub",
        },
        returnToTopLabel: "Volver arriba",
        sidebarMenuLabel: "Menú",
        darkModeSwitchLabel: "Apariencia",
        lightModeSwitchTitle: "Cambiar al tema claro",
        darkModeSwitchTitle: "Cambiar al tema oscuro",
      },
    },
    "zh-CN": {
      label: "简体中文",
      lang: "zh-CN",
      link: "/zh-CN/",
      themeConfig: {
        nav: buildChineseCNNav(),
        sidebar: {
          "/zh-CN/guide/": buildChineseCNSidebar("/zh-CN/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "上一页", next: "下一页" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "在 GitHub 上编辑此页",
        },
        returnToTopLabel: "返回顶部",
        sidebarMenuLabel: "菜单",
        darkModeSwitchLabel: "外观",
        lightModeSwitchTitle: "切换到浅色主题",
        darkModeSwitchTitle: "切换到深色主题",
      },
    },
    "zh-HK": {
      label: "繁體中文",
      lang: "zh-HK",
      link: "/zh-HK/",
      themeConfig: {
        nav: buildChineseHKNav(),
        sidebar: {
          "/zh-HK/guide/": buildChineseHKSidebar("/zh-HK/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "上一頁", next: "下一頁" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "在 GitHub 上編輯此頁",
        },
        returnToTopLabel: "返回頂部",
        sidebarMenuLabel: "選單",
        darkModeSwitchLabel: "外觀",
        lightModeSwitchTitle: "切換到淺色主題",
        darkModeSwitchTitle: "切換到深色主題",
      },
    },
    "ru-RU": {
      label: "Русский",
      lang: "ru-RU",
      link: "/ru-RU/",
      themeConfig: {
        nav: buildRussianNav(),
        sidebar: {
          "/ru-RU/guide/": buildRussianSidebar("/ru-RU/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "Предыдущая страница", next: "Следующая страница" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "Редактировать эту страницу на GitHub",
        },
        returnToTopLabel: "Вернуться наверх",
        sidebarMenuLabel: "Меню",
        darkModeSwitchLabel: "Внешний вид",
        lightModeSwitchTitle: "Переключить на светлую тему",
        darkModeSwitchTitle: "Переключить на темную тему",
      },
    },
    "de-DE": {
      label: "Deutsch",
      lang: "de-DE",
      link: "/de-DE/",
      themeConfig: {
        nav: buildGermanNav(),
        sidebar: {
          "/de-DE/guide/": buildGermanSidebar("/de-DE/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "Vorherige Seite", next: "Nächste Seite" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "Diese Seite auf GitHub bearbeiten",
        },
        returnToTopLabel: "Zurück nach oben",
        sidebarMenuLabel: "Menü",
        darkModeSwitchLabel: "Erscheinungsbild",
        lightModeSwitchTitle: "Zum hellen Thema wechseln",
        darkModeSwitchTitle: "Zum dunklen Thema wechseln",
      },
    },
    "ja-JP": {
      label: "日本語",
      lang: "ja-JP",
      link: "/ja-JP/",
      themeConfig: {
        nav: buildJapaneseNav(),
        sidebar: {
          "/ja-JP/guide/": buildJapaneseSidebar("/ja-JP/guide/"),
        },
        outline: {
          level: [2, 3],
        },
        docFooter: {
          prev: "前のページ",
          next: "次のページ",
        },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "GitHub でこのページを編集",
        },
        returnToTopLabel: "トップに戻る",
        sidebarMenuLabel: "メニュー",
        darkModeSwitchLabel: "外観",
        lightModeSwitchTitle: "ライトモードに切り替え",
        darkModeSwitchTitle: "ダークモードに切り替え",
      },
    },
    "ko-KR": {
      label: "한국어",
      lang: "ko-KR",
      link: "/ko-KR/",
      themeConfig: {
        nav: buildKoreanNav(),
        sidebar: {
          "/ko-KR/gnowledge/": buildKoreanSidebar("/ko-KR/guide/"),
        },
        outline: { level: [2, 3] },
        docFooter: { prev: "이전 페이지", next: "다음 페이지" },
        editLink: {
          pattern: `${repo}/edit/main/docs/:path`,
          text: "GitHub에서 이 페이지 편집",
        },
        returnToTopLabel: "맨 위로 돌아가기",
        sidebarMenuLabel: "메뉴",
        darkModeSwitchLabel: "외관",
        lightModeSwitchTitle: "라이트 테마로 전환",
        darkModeSwitchTitle: "다크 테마로 전환",
      },
    },
  },
  markdown: {
    lineNumbers: true,
  },
  sitemap: {
    hostname: "https://levilauncher.levimc.org/",
  },
});