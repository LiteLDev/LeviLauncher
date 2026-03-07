import { defineConfig, type DefaultTheme } from "vitepress";

const repo = "https://github.com/LiteLDev/LeviLauncher";
const releases = `${repo}/releases`;
const discord = "https://discord.gg/v5R5P4vRZk";
const lanzou = "https://levimc.lanzoue.com/b016ke39hc";

const base = "/";

function buildGuideSidebar(
  prefix: string,
  zh = false,
): DefaultTheme.SidebarItem[] {
  return [
    {
      text: zh ? "快速开始" : "Quick Start",
      items: [
        {
          text: zh ? "5 分钟上手" : "Get Started in 5 Minutes",
          link: `${prefix}quick-start`,
        },
        {
          text: zh ? "系统要求与安装" : "Requirements & Installation",
          link: `${prefix}requirements-installation`,
        },
        {
          text: zh ? "首次启动" : "First Launch",
          link: `${prefix}first-launch`,
        },
      ],
    },
    {
      text: zh ? "核心功能" : "Core Workflows",
      items: [
        {
          text: zh ? "版本管理" : "Version Management",
          link: `${prefix}version-management`,
        },
        {
          text: zh ? "下载与镜像" : "Downloads & Mirrors",
          link: `${prefix}downloads-mirrors`,
        },
        {
          text: zh ? "内容管理" : "Content Management",
          link: `${prefix}content-management`,
        },
      ],
    },
    {
      text: zh ? "高级能力" : "Advanced Features",
      items: [
        {
          text: zh ? "Mods 与集成" : "Mods & Integrations",
          link: `${prefix}mods-integrations`,
        },
        {
          text: zh ? "世界工具" : "World Tools",
          link: `${prefix}world-tools`,
        },
        {
          text: zh ? "设置与个性化" : "Settings & Personalization",
          link: `${prefix}settings-personalization`,
        },
      ],
    },
    {
      text: zh ? "帮助" : "Help",
      items: [
        {
          text: zh ? "更新与故障排查" : "Update & Troubleshooting",
          link: `${prefix}update-troubleshooting`,
        },
        {
          text: zh ? "常见问题、社区与反馈" : "FAQ, Community & Feedback",
          link: `${prefix}faq-community-feedback`,
        },
      ],
    },
  ];
}

function buildEnglishNav(): DefaultTheme.NavItem[] {
  return [
    { text: "Guide", link: "/guide/quick-start" },
    { text: "Releases", link: releases },
    { text: "Mirror", link: lanzou },
    { text: "Community", link: discord },
    { text: "GitHub", link: repo },
  ];
}

function buildChineseNav(): DefaultTheme.NavItem[] {
  return [
    { text: "文档", link: "/zh-CN/guide/quick-start" },
    { text: "下载", link: releases },
    { text: "蓝奏云", link: lanzou },
    { text: "社区", link: discord },
    { text: "GitHub", link: repo },
  ];
}

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
          "/guide/": buildGuideSidebar("/guide/"),
        },
        outline: {
          level: [2, 3],
        },
        docFooter: {
          prev: "Previous page",
          next: "Next page",
        },
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
    "zh-CN": {
      label: "简体中文",
      lang: "zh-CN",
      link: "/zh-CN/",
      themeConfig: {
        nav: buildChineseNav(),
        sidebar: {
          "/zh-CN/guide/": buildGuideSidebar("/zh-CN/guide/", true),
        },
        outline: {
          level: [2, 3],
        },
        docFooter: {
          prev: "上一页",
          next: "下一页",
        },
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
  },
  markdown: {
    lineNumbers: true,
  },
  sitemap: {
    hostname: "https://levilauncher.levimc.org/",
  },
});
