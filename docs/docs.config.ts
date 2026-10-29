// LeviLauncher user documentation — levimc-kit site config (replaces .vitepress/config.mts).
// One site for both versions of LeviLauncher: the Windows guide under guide/, the Android version (formerly
// docs.levimc.org/LeviLaunchroid/, its own repository LiteLDev/LeviLaunchroid) under android/.
import { defineDocsConfig, type NavEntry } from './kit/docs/node/config.ts';

const repo = 'https://github.com/LiteLDev/LeviLauncher';

// --- Sidebars -------------------------------------------------------------------------------------

interface SidebarLabels {
  quickStart: string;
  getStarted: string;
  requirements: string;
  firstLaunch: string;
  coreWorkflows: string;
  versions: string;
  downloads: string;
  content: string;
  advanced: string;
  mods: string;
  worldTools: string;
  settings: string;
  help: string;
  troubleshooting: string;
  faq: string;
}

function buildSidebar(prefix: string, t: SidebarLabels): NavEntry[] {
  return [
    {
      text: t.quickStart,
      items: [
        { text: t.getStarted, link: `${prefix}quick-start` },
        { text: t.requirements, link: `${prefix}requirements-installation` },
        { text: t.firstLaunch, link: `${prefix}first-launch` },
      ],
    },
    {
      text: t.coreWorkflows,
      items: [
        { text: t.versions, link: `${prefix}version-management` },
        { text: t.downloads, link: `${prefix}downloads-mirrors` },
        { text: t.content, link: `${prefix}content-management` },
      ],
    },
    {
      text: t.advanced,
      items: [
        { text: t.mods, link: `${prefix}mods-integrations` },
        { text: t.worldTools, link: `${prefix}world-tools` },
        { text: t.settings, link: `${prefix}settings-personalization` },
      ],
    },
    {
      text: t.help,
      items: [
        { text: t.troubleshooting, link: `${prefix}update-troubleshooting` },
        { text: t.faq, link: `${prefix}faq-community-feedback` },
      ],
    },
  ];
}

const SIDEBAR: Record<string, SidebarLabels> = {
  'en-US': {
    quickStart: 'Quick Start',
    getStarted: 'Get Started in 5 Minutes',
    requirements: 'Requirements & Installation',
    firstLaunch: 'First Launch',
    coreWorkflows: 'Core Workflows',
    versions: 'Version Management',
    downloads: 'Downloads & Mirrors',
    content: 'Content Management',
    advanced: 'Advanced Features',
    mods: 'Mods & Integrations',
    worldTools: 'World Tools',
    settings: 'Settings & Personalization',
    help: 'Help',
    troubleshooting: 'Update & Troubleshooting',
    faq: 'FAQ, Community & Feedback',
  },
  'es-ES': {
    quickStart: 'Inicio rápido',
    getStarted: 'Comienza en 5 minutos',
    requirements: 'Requisitos e instalación',
    firstLaunch: 'Primer inicio',
    coreWorkflows: 'Flujos principales',
    versions: 'Gestión de versiones',
    downloads: 'Descargas y espejos',
    content: 'Gestión de contenido',
    advanced: 'Funciones avanzadas',
    mods: 'Mods e integraciones',
    worldTools: 'Herramientas para mundos',
    settings: 'Ajustes y personalización',
    help: 'Ayuda',
    troubleshooting: 'Actualizaciones y solución de problemas',
    faq: 'Preguntas frecuentes, comunidad y comentarios',
  },
  'zh-CN': {
    quickStart: '快速开始',
    getStarted: '5 分钟上手',
    requirements: '系统要求与安装',
    firstLaunch: '首次启动',
    coreWorkflows: '核心功能',
    versions: '版本管理',
    downloads: '下载与镜像',
    content: '内容管理',
    advanced: '高级功能',
    mods: '模组与集成',
    worldTools: '世界工具',
    settings: '设置与个性化',
    help: '帮助',
    troubleshooting: '更新与故障排查',
    faq: '常见问题、社区与反馈',
  },
  'zh-HK': {
    quickStart: '快速入門',
    getStarted: '5 分鐘上手',
    requirements: '系統要求與安裝',
    firstLaunch: '首次啟動',
    coreWorkflows: '核心功能',
    versions: '版本管理',
    downloads: '下載與鏡像',
    content: '內容管理',
    advanced: '進階功能',
    mods: '模組與整合',
    worldTools: '世界工具',
    settings: '設定與個人化',
    help: '協助',
    troubleshooting: '更新與故障排查',
    faq: '常見問題、社群與回饋',
  },
  'ru-RU': {
    quickStart: 'Быстрый старт',
    getStarted: 'Начало работы за 5 минут',
    requirements: 'Требования и установка',
    firstLaunch: 'Первый запуск',
    coreWorkflows: 'Основные рабочие процессы',
    versions: 'Управление версиями',
    downloads: 'Загрузки и зеркала',
    content: 'Управление контентом',
    advanced: 'Расширенные функции',
    mods: 'Моды и интеграции',
    worldTools: 'Инструменты для миров',
    settings: 'Настройки и персонализация',
    help: 'Помощь',
    troubleshooting: 'Обновление и устранение неполадок',
    faq: 'ЧаВО, сообщество и обратная связь',
  },
  'de-DE': {
    quickStart: 'Schnellstart',
    getStarted: 'In 5 Minuten loslegen',
    requirements: 'Anforderungen & Installation',
    firstLaunch: 'Erster Start',
    coreWorkflows: 'Kernabläufe',
    versions: 'Versionsverwaltung',
    downloads: 'Downloads & Spiegel',
    content: 'Inhaltsverwaltung',
    advanced: 'Erweiterte Funktionen',
    mods: 'Mods & Integrationen',
    worldTools: 'Welt-Tools',
    settings: 'Einstellungen & Personalisierung',
    help: 'Hilfe',
    troubleshooting: 'Update & Fehlerbehebung',
    faq: 'FAQ, Community & Feedback',
  },
  'ja-JP': {
    quickStart: 'クイックスタート',
    getStarted: '5 分で始める',
    requirements: 'システム要件とインストール',
    firstLaunch: '初回起動',
    coreWorkflows: 'コア機能',
    versions: 'バージョン管理',
    downloads: 'ダウンロードとミラー',
    content: 'コンテンツ管理',
    advanced: '高度な機能',
    mods: 'Mod と統合',
    worldTools: 'ワールドツール',
    settings: '設定とカスタマイズ',
    help: 'ヘルプ',
    troubleshooting: '更新とトラブルシューティング',
    faq: 'FAQ、コミュニティ、フィードバック',
  },
  'ko-KR': {
    quickStart: '빠른 시작',
    getStarted: '5분 만에 시작하기',
    requirements: '시스템 요구사항 및 설치',
    firstLaunch: '처음 실행하기',
    coreWorkflows: '핵심 워크플로우',
    versions: '버전 관리',
    downloads: '다운로드 및 미러',
    content: '콘텐츠 관리',
    advanced: '고급 기능',
    mods: '모드 및 통합',
    worldTools: '월드 도구',
    settings: '설정 및 개인화',
    help: '도움말',
    troubleshooting: '업데이트 및 문제 해결',
    faq: '자주 묻는 질문, 커뮤니티 및 피드백',
  },
  'fr-FR': {
    quickStart: 'Démarrage rapide',
    getStarted: 'Commencer en 5 minutes',
    requirements: 'Configuration requise & Installation',
    firstLaunch: 'Premier lancement',
    coreWorkflows: 'Fonctionnalités principales',
    versions: 'Gestion des versions',
    downloads: 'Téléchargements & Miroirs',
    content: 'Gestion du contenu',
    advanced: 'Fonctionnalités avancées',
    mods: 'Mods & Intégrations',
    worldTools: 'Outils pour les mondes',
    settings: 'Paramètres & Personnalisation',
    help: 'Aide',
    troubleshooting: 'Mises à jour & Dépannage',
    faq: 'FAQ, communauté & Retour',
  },
};

// --- Android sidebar ------------------------------------------------------------------------------

// The Android pages exist in English and Simplified Chinese (zh-HK converts them, the other locales show English).
// API names stay English in every locale, as in the pages.
interface AndroidLabels {
  guide: string;
  overview: string;
  gettingStarted: string;
  features: string;
  compatibility: string;
  privacy: string;
  developer: string;
  nativeMod: string;
  build: string;
  memoryHook: string;
  sdkHeaders: string;
}

function buildAndroidSidebar(prefix: string, t: AndroidLabels): NavEntry[] {
  return [
    {
      text: t.guide,
      items: [
        { text: t.overview, link: prefix },
        { text: t.gettingStarted, link: `${prefix}guide/getting-started` },
        { text: t.features, link: `${prefix}guide/features` },
        { text: t.compatibility, link: `${prefix}guide/compatibility` },
        { text: t.privacy, link: `${prefix}privacy-policy` },
      ],
    },
    {
      text: t.developer,
      items: [
        { text: t.nativeMod, link: `${prefix}guide/developer` },
        { text: t.build, link: `${prefix}guide/build` },
        { text: 'Mod API', link: `${prefix}api/mod` },
        { text: 'Mod Menu API', link: `${prefix}api/mod-menu` },
        { text: 'Config API', link: `${prefix}api/config` },
        { text: 'Input API', link: `${prefix}api/input` },
        { text: 'Hook API', link: `${prefix}api/hook` },
        { text: 'Signature API', link: `${prefix}api/signature` },
        { text: 'Patch API', link: `${prefix}api/patch` },
        { text: t.memoryHook, link: `${prefix}api/memory-hook-macros` },
        { text: t.sdkHeaders, link: `${prefix}api/types-and-macros` },
      ],
    },
  ];
}

const ANDROID_EN: AndroidLabels = {
  guide: 'Android Version',
  overview: 'Overview',
  gettingStarted: 'Getting Started',
  features: 'Features',
  compatibility: 'Compatibility and FAQ',
  privacy: 'Privacy Policy',
  developer: 'Developer',
  nativeMod: 'Native Mod Quick Start',
  build: 'Build from Source',
  memoryHook: 'Memory Hook Example',
  sdkHeaders: 'SDK Headers and Macros',
};

const ANDROID: Record<string, AndroidLabels> = {
  'en-US': ANDROID_EN,
  'es-ES': { ...ANDROID_EN, guide: 'Versión para Android', overview: 'Introducción', gettingStarted: 'Primeros pasos', features: 'Funciones', compatibility: 'Compatibilidad y preguntas frecuentes', privacy: 'Política de privacidad', developer: 'Desarrolladores', nativeMod: 'Inicio rápido de Mods nativos', build: 'Compilar desde el código fuente' },
  'zh-CN': { guide: 'Android 版', overview: '概览', gettingStarted: '快速开始', features: '功能概览', compatibility: '兼容性与常见问题', privacy: '隐私政策', developer: '开发者', nativeMod: '原生模组快速开始', build: '从源码构建', memoryHook: 'Memory Hook 示例', sdkHeaders: 'SDK 头文件与宏' },
  'zh-HK': { guide: 'Android 版', overview: '概覽', gettingStarted: '快速開始', features: '功能概覽', compatibility: '兼容性與常見問題', privacy: '私隱政策', developer: '開發者', nativeMod: '原生模組快速開始', build: '從原始碼建置', memoryHook: 'Memory Hook 示例', sdkHeaders: 'SDK 標頭檔與巨集' },
  'ru-RU': { ...ANDROID_EN, guide: 'Версия для Android', overview: 'Обзор', gettingStarted: 'Начало работы', features: 'Возможности', compatibility: 'Совместимость и ЧаВО', privacy: 'Политика конфиденциальности', developer: 'Разработчикам', nativeMod: 'Быстрый старт: нативные моды', build: 'Сборка из исходного кода' },
  'de-DE': { ...ANDROID_EN, guide: 'Android-Version', overview: 'Überblick', gettingStarted: 'Erste Schritte', features: 'Funktionen', compatibility: 'Kompatibilität & FAQ', privacy: 'Datenschutzerklärung', developer: 'Entwickler', nativeMod: 'Schnellstart: native Mods', build: 'Aus dem Quellcode bauen' },
  'ja-JP': { ...ANDROID_EN, guide: 'Android 版', overview: '概要', gettingStarted: 'はじめに', features: '機能', compatibility: '互換性と FAQ', privacy: 'プライバシーポリシー', developer: '開発者向け', nativeMod: 'ネイティブ Mod クイックスタート', build: 'ソースからビルド' },
  'ko-KR': { ...ANDROID_EN, guide: 'Android 버전', overview: '개요', gettingStarted: '시작하기', features: '기능', compatibility: '호환성 및 FAQ', privacy: '개인정보 처리방침', developer: '개발자', nativeMod: '네이티브 모드 빠른 시작', build: '소스에서 빌드' },
  'fr-FR': { ...ANDROID_EN, guide: 'Version Android', overview: 'Présentation', gettingStarted: 'Premiers pas', features: 'Fonctionnalités', compatibility: 'Compatibilité & FAQ', privacy: 'Politique de confidentialité', developer: 'Développeurs', nativeMod: 'Démarrage rapide : Mods natifs', build: 'Compiler depuis les sources' },
};

// --- Navbar ---------------------------------------------------------------------------------------

interface NavLabels {
  windows: string;
  android: string;
  download: string;
}

function buildNav(prefix: string, t: NavLabels): NavEntry[] {
  return [
    { text: t.windows, link: `${prefix}guide/quick-start` },
    { text: t.android, link: `${prefix}android/` },
    // the Download page offers both versions and selects the visitor's (tabs named after the platforms)
    { text: t.download, link: `${prefix}download` },
    // GitHub and Community (Discord) are not docs tabs: the LeviMC bar already has a GitHub button (`repo`) and a
    // Community tab, and the footer links the same Discord invite, so the docs row carries no duplicates.
  ];
}

const NAV: Record<string, NavLabels> = {
  'en-US': { windows: 'Windows', android: 'Android', download: 'Download' },
  'es-ES': { windows: 'Windows', android: 'Android', download: 'Descargas' },
  'zh-CN': { windows: 'Windows 版', android: 'Android 版', download: '下载' },
  'zh-HK': { windows: 'Windows 版', android: 'Android 版', download: '下載' },
  'ru-RU': { windows: 'Windows', android: 'Android', download: 'Загрузки' },
  'de-DE': { windows: 'Windows', android: 'Android', download: 'Downloads' },
  'ja-JP': { windows: 'Windows 版', android: 'Android 版', download: 'ダウンロード' },
  'ko-KR': { windows: 'Windows', android: 'Android', download: '다운로드' },
  'fr-FR': { windows: 'Windows', android: 'Android', download: 'Téléchargements' },
};

// --- Locales & UI labels --------------------------------------------------------------------------

const LOCALES = [
  { code: 'en-US', label: 'English', lang: 'en-US', prefix: '' },
  { code: 'es-ES', label: 'Español', lang: 'es-ES', prefix: 'es-ES/' },
  { code: 'zh-CN', label: '简体中文', lang: 'zh-CN', prefix: 'zh-CN/' },
  // the Android pages it has no translation of are converted from zh-CN (levimc-kit convertFrom)
  { code: 'zh-HK', label: '繁體中文', lang: 'zh-HK', prefix: 'zh-HK/', convertFrom: 'zh-CN' },
  { code: 'ru-RU', label: 'Русский', lang: 'ru-RU', prefix: 'ru-RU/' },
  { code: 'de-DE', label: 'Deutsch', lang: 'de-DE', prefix: 'de-DE/' },
  { code: 'ja-JP', label: '日本語', lang: 'ja-JP', prefix: 'ja-JP/' },
  { code: 'ko-KR', label: '한국어', lang: 'ko-KR', prefix: 'ko-KR/' },
  { code: 'fr-FR', label: 'Français', lang: 'fr-FR', prefix: 'fr-FR/' },
];

// VitePress docFooter / editLink.text / returnToTopLabel / sidebarMenuLabel, as kit UI strings. The LeviMC bar's own
// labels (its theme menu: VitePress's darkModeSwitchLabel) stay the kit's, identical on every LeviMC site.
// `backToTop` (returnToTopLabel) is not rendered: the kit has no back-to-top control; the key is kept so the
// translations apply once it adds one.
interface UiLabels {
  previous: string;
  next: string;
  editPage: string;
  backToTop: string;
  menu: string;
}

const UI: Record<string, UiLabels> = {
  'en-US': { previous: 'Previous page', next: 'Next page', editPage: 'Edit this page on GitHub', backToTop: 'Back to top', menu: 'Menu' },
  'es-ES': { previous: 'Página anterior', next: 'Siguiente página', editPage: 'Editar esta página en GitHub', backToTop: 'Volver arriba', menu: 'Menú' },
  'zh-CN': { previous: '上一页', next: '下一页', editPage: '在 GitHub 上编辑此页', backToTop: '返回顶部', menu: '菜单' },
  'zh-HK': { previous: '上一頁', next: '下一頁', editPage: '在 GitHub 上編輯此頁', backToTop: '返回頂部', menu: '選單' },
  'ru-RU': { previous: 'Предыдущая страница', next: 'Следующая страница', editPage: 'Редактировать эту страницу на GitHub', backToTop: 'Вернуться наверх', menu: 'Меню' },
  'de-DE': { previous: 'Vorherige Seite', next: 'Nächste Seite', editPage: 'Diese Seite auf GitHub bearbeiten', backToTop: 'Zurück nach oben', menu: 'Menü' },
  'ja-JP': { previous: '前のページ', next: '次のページ', editPage: 'GitHub でこのページを編集', backToTop: 'トップに戻る', menu: 'メニュー' },
  'ko-KR': { previous: '이전 페이지', next: '다음 페이지', editPage: 'GitHub에서 이 페이지 편집', backToTop: '맨 위로 돌아가기', menu: '메뉴' },
  'fr-FR': { previous: 'Page précédente', next: 'Page suivante', editPage: 'Modifier cette page sur GitHub', backToTop: 'Retour en haut', menu: 'Menu' },
};

const perLocale = <T>(fn: (code: string, prefix: string) => T): Record<string, T> =>
  Object.fromEntries(LOCALES.map((l) => [l.code, fn(l.code, l.prefix ? `/${l.prefix}` : '/')]));

export default defineDocsConfig({
  base: '/LeviLauncher/',
  title: 'LeviLauncher',
  description: 'User documentation for LeviLauncher, the Minecraft Bedrock Edition launcher for Windows (GDK / UWP) and Android.',
  project: 'LeviLauncher',
  logo: { src: '/appicon.png', alt: 'LeviLauncher' },
  favicon: '/appicon.png',
  contentDir: '.',
  repo,
  editLink: `${repo}/edit/main/docs/:path`,
  // fallback: a locale without its own Android pages shows the English ones at its own URLs
  i18n: { mode: 'folder', defaultLocale: 'en-US', locales: LOCALES, fallback: true },
  urlStyle: 'html',
  cleanUrls: true,
  slug: 'vitepress',
  // MkDocs containers too, for the `=== "Windows"` / `=== "Android"` tabs of the Download page
  markdown: { compat: 'commonmark', lineNumbers: true, containers: { vitepress: true, mkdocs: true } },
  outline: [2, 3],
  lastUpdated: true,
  hostname: 'https://docs.levimc.org',
  sitemap: { hostname: 'https://docs.levimc.org' },
  nav: perLocale((code, prefix) => buildNav(prefix, NAV[code]!)),
  sidebar: perLocale((code, prefix) => ({
    [`${prefix}guide/`]: buildSidebar(`${prefix}guide/`, SIDEBAR[code]!),
    [`${prefix}android/`]: buildAndroidSidebar(`${prefix}android/`, ANDROID[code]!),
  })),
  social: [{ icon: 'github', link: repo, label: 'GitHub' }],
  ui: UI,
});
