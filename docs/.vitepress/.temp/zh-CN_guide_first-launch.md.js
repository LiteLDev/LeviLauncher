import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"首次启动","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/first-launch.md","filePath":"zh-CN/guide/first-launch.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/first-launch.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="首次启动" tabindex="-1">首次启动 <a class="header-anchor" href="#首次启动" aria-label="Permalink to &quot;首次启动&quot;">​</a></h1><p>第一次打开 LeviLauncher 时，应用会优先确认 Windows 与 Minecraft 环境是否满足托管安装要求。</p><h2 id="你可能会看到什么" tabindex="-1">你可能会看到什么 <a class="header-anchor" href="#你可能会看到什么" aria-label="Permalink to &quot;你可能会看到什么&quot;">​</a></h2><p>首次启动时，LeviLauncher 可能会：</p><ul><li>检查 Gaming Services 是否可用</li><li>检查 GameInput 是否可用</li><li>验证 Minecraft Bedrock (GDK) 环境是否已准备就绪</li><li>在缺失关键组件时显示引导或提示</li></ul><h2 id="常见首次启动流程" tabindex="-1">常见首次启动流程 <a class="header-anchor" href="#常见首次启动流程" aria-label="Permalink to &quot;常见首次启动流程&quot;">​</a></h2><ol><li>打开 LeviLauncher。</li><li>阅读启动器给出的引导信息。</li><li>安装或修复缺失的前置组件。</li><li>返回启动器重新检查状态。</li><li>满足条件后进入 <strong>Download</strong> 页面。</li></ol><h2 id="如果提示缺少组件" tabindex="-1">如果提示缺少组件 <a class="header-anchor" href="#如果提示缺少组件" aria-label="Permalink to &quot;如果提示缺少组件&quot;">​</a></h2><h3 id="gaming-services" tabindex="-1">Gaming Services <a class="header-anchor" href="#gaming-services" aria-label="Permalink to &quot;Gaming Services&quot;">​</a></h3><p>如果 Gaming Services 缺失或损坏，LeviLauncher 可能会引导你去 Microsoft Store 安装或修复。</p><h3 id="gameinput" tabindex="-1">GameInput <a class="header-anchor" href="#gameinput" aria-label="Permalink to &quot;GameInput&quot;">​</a></h3><p>如果缺少 GameInput，请按提示安装所需 redistributable。</p><h3 id="未检测到-minecraft-bedrock" tabindex="-1">未检测到 Minecraft Bedrock <a class="header-anchor" href="#未检测到-minecraft-bedrock" aria-label="Permalink to &quot;未检测到 Minecraft Bedrock&quot;">​</a></h3><p>请确认：</p><ul><li>当前 Microsoft 账号拥有游戏授权</li><li>当前电脑上已经从 Microsoft Store 安装过游戏</li><li>你不是试图绕过官方授权流程使用启动器</li></ul><h2 id="给新用户的好默认值" tabindex="-1">给新用户的好默认值 <a class="header-anchor" href="#给新用户的好默认值" aria-label="Permalink to &quot;给新用户的好默认值&quot;">​</a></h2><ul><li>先使用一个隔离的 Release 版本</li><li>把下载与内容路径放在可写的磁盘位置</li><li>第一次成功纯净启动之前，不要急着导入大量 Mods</li></ul><h2 id="首次启动之后建议继续阅读" tabindex="-1">首次启动之后建议继续阅读 <a class="header-anchor" href="#首次启动之后建议继续阅读" aria-label="Permalink to &quot;首次启动之后建议继续阅读&quot;">​</a></h2><ul><li><a href="./version-management">版本管理</a></li><li><a href="./content-management">内容管理</a></li><li><a href="./settings-personalization">设置与个性化</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/first-launch.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const firstLaunch = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  firstLaunch as default
};
