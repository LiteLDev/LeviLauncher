// LeviLauncher user documentation — levimc-kit site config (replaces .vitepress/config.mts).
// Nav, sidebars and UI labels are ported 1:1 from the former VitePress config.
import { defineDocsConfig, type NavEntry } from './kit/docs/node/config.ts';

const repo = 'https://github.com/LiteLDev/LeviLauncher';
const releases = `${repo}/releases`;
const lanzou = 'https://levimc.lanzoue.com/b016ke39hc';

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
    mods: 'Mods 与集成',
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
    mods: 'Mods 與整合',
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

// --- Navbar ---------------------------------------------------------------------------------------

interface NavLabels {
  guide: string;
  downloads: string;
  mirror: string;
}

function buildNav(prefix: string, t: NavLabels): NavEntry[] {
  return [
    { text: t.guide, link: `${prefix}guide/quick-start` },
    { text: t.downloads, link: releases },
    { text: t.mirror, link: lanzou },
    // GitHub and Community (Discord) are not docs tabs: the LeviMC bar already has a GitHub button (`repo`) and a
    // Community tab, and the footer links the same Discord invite, so the docs row carries no duplicates.
  ];
}

const NAV: Record<string, NavLabels> = {
  'en-US': { guide: 'Guide', downloads: 'Downloads', mirror: 'Mirror' },
  'es-ES': { guide: 'Guía', downloads: 'Descargas', mirror: 'Espejo' },
  'zh-CN': { guide: '文档', downloads: '下载', mirror: '蓝奏云' },
  'zh-HK': { guide: '文檔', downloads: '下載', mirror: '藍奏雲' },
  'ru-RU': { guide: 'Документация', downloads: 'Загрузки', mirror: 'Зеркало' },
  'de-DE': { guide: 'Dokumentation', downloads: 'Downloads', mirror: 'Spiegel' },
  'ja-JP': { guide: 'ドキュメント', downloads: 'ダウンロード', mirror: 'ミラー' },
  'ko-KR': { guide: '가이드', downloads: '다운로드', mirror: '미러' },
  'fr-FR': { guide: 'Guide', downloads: 'Téléchargements', mirror: 'Miroir' },
};

// --- Locales & UI labels --------------------------------------------------------------------------

const LOCALES = [
  { code: 'en-US', label: 'English', lang: 'en-US', prefix: '' },
  { code: 'es-ES', label: 'Español', lang: 'es-ES', prefix: 'es-ES/' },
  { code: 'zh-CN', label: '简体中文', lang: 'zh-CN', prefix: 'zh-CN/' },
  { code: 'zh-HK', label: '繁體中文', lang: 'zh-HK', prefix: 'zh-HK/' },
  { code: 'ru-RU', label: 'Русский', lang: 'ru-RU', prefix: 'ru-RU/' },
  { code: 'de-DE', label: 'Deutsch', lang: 'de-DE', prefix: 'de-DE/' },
  { code: 'ja-JP', label: '日本語', lang: 'ja-JP', prefix: 'ja-JP/' },
  { code: 'ko-KR', label: '한국어', lang: 'ko-KR', prefix: 'ko-KR/' },
  { code: 'fr-FR', label: 'Français', lang: 'fr-FR', prefix: 'fr-FR/' },
];

// VitePress docFooter / editLink.text / returnToTopLabel / sidebarMenuLabel / darkModeSwitchLabel, as kit UI strings.
// `theme` (darkModeSwitchLabel) names the LeviMC bar's theme menu ("Appearance: Auto"). The kit's Russian
// state names agree with «Тема» (feminine), so ru-RU also gets masculine forms that agree with «Внешний вид»
// (the Auto state, «Авто», is invariable). lightModeSwitchTitle / darkModeSwitchTitle have no kit equivalent.
// `backToTop` (returnToTopLabel) is not rendered: the kit has no back-to-top control; the key is kept so the
// translations apply once it adds one.
interface UiLabels {
  previous: string;
  next: string;
  editPage: string;
  backToTop: string;
  menu: string;
  theme: string;
  themeLight?: string;
  themeDark?: string;
}

const UI: Record<string, UiLabels> = {
  'en-US': { previous: 'Previous page', next: 'Next page', editPage: 'Edit this page on GitHub', backToTop: 'Back to top', menu: 'Menu', theme: 'Appearance' },
  'es-ES': { previous: 'Página anterior', next: 'Siguiente página', editPage: 'Editar esta página en GitHub', backToTop: 'Volver arriba', menu: 'Menú', theme: 'Apariencia' },
  'zh-CN': { previous: '上一页', next: '下一页', editPage: '在 GitHub 上编辑此页', backToTop: '返回顶部', menu: '菜单', theme: '外观' },
  'zh-HK': { previous: '上一頁', next: '下一頁', editPage: '在 GitHub 上編輯此頁', backToTop: '返回頂部', menu: '選單', theme: '外觀' },
  'ru-RU': { previous: 'Предыдущая страница', next: 'Следующая страница', editPage: 'Редактировать эту страницу на GitHub', backToTop: 'Вернуться наверх', menu: 'Меню', theme: 'Внешний вид', themeLight: 'Светлый', themeDark: 'Тёмный' },
  'de-DE': { previous: 'Vorherige Seite', next: 'Nächste Seite', editPage: 'Diese Seite auf GitHub bearbeiten', backToTop: 'Zurück nach oben', menu: 'Menü', theme: 'Erscheinungsbild' },
  'ja-JP': { previous: '前のページ', next: '次のページ', editPage: 'GitHub でこのページを編集', backToTop: 'トップに戻る', menu: 'メニュー', theme: '外観' },
  'ko-KR': { previous: '이전 페이지', next: '다음 페이지', editPage: 'GitHub에서 이 페이지 편집', backToTop: '맨 위로 돌아가기', menu: '메뉴', theme: '외관' },
  'fr-FR': { previous: 'Page précédente', next: 'Page suivante', editPage: 'Modifier cette page sur GitHub', backToTop: 'Retour en haut', menu: 'Menu', theme: 'Apparence' },
};

const perLocale = <T>(fn: (code: string, prefix: string) => T): Record<string, T> =>
  Object.fromEntries(LOCALES.map((l) => [l.code, fn(l.code, l.prefix ? `/${l.prefix}` : '/')]));

export default defineDocsConfig({
  base: '/LeviLauncher/',
  title: 'LeviLauncher',
  description: 'User documentation for LeviLauncher, a desktop launcher for Minecraft Bedrock Edition (GDK) on Windows.',
  project: 'LeviLauncher',
  logo: { src: '/appicon.png', alt: 'LeviLauncher' },
  favicon: '/appicon.png',
  contentDir: '.',
  repo,
  editLink: `${repo}/edit/main/docs/:path`,
  i18n: { mode: 'folder', defaultLocale: 'en-US', locales: LOCALES },
  urlStyle: 'html',
  cleanUrls: true,
  slug: 'vitepress',
  markdown: { compat: 'commonmark', lineNumbers: true },
  outline: [2, 3],
  lastUpdated: true,
  hostname: 'https://docs.levimc.org',
  sitemap: { hostname: 'https://docs.levimc.org' },
  nav: perLocale((code, prefix) => buildNav(prefix, NAV[code]!)),
  sidebar: perLocale((code, prefix) => ({ [`${prefix}guide/`]: buildSidebar(`${prefix}guide/`, SIDEBAR[code]!) })),
  social: [{ icon: 'github', link: repo, label: 'GitHub' }],
  footer: { copyright: 'Copyright © 2024-2026 LeviMC' },
  ui: UI,
});
