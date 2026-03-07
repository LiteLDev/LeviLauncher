import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"版本管理","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/version-management.md","filePath":"zh-CN/guide/version-management.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/version-management.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="版本管理" tabindex="-1">版本管理 <a class="header-anchor" href="#版本管理" aria-label="Permalink to &quot;版本管理&quot;">​</a></h1><p>LeviLauncher 会把每个已安装环境视为一个可独立管理的版本。</p><h2 id="你可以做什么" tabindex="-1">你可以做什么 <a class="header-anchor" href="#你可以做什么" aria-label="Permalink to &quot;你可以做什么&quot;">​</a></h2><ul><li>安装正式版或预览版</li><li>为版本设置便于识别的名称</li><li>快速启动指定版本</li><li>删除不再需要的版本</li><li>打开版本专属设置</li><li>为版本创建桌面快捷方式</li></ul><h2 id="release-与-preview-的区别" tabindex="-1">Release 与 Preview 的区别 <a class="header-anchor" href="#release-与-preview-的区别" aria-label="Permalink to &quot;Release 与 Preview 的区别&quot;">​</a></h2><table tabindex="0"><thead><tr><th>类型</th><th>适合谁</th><th>说明</th></tr></thead><tbody><tr><td>Release</td><td>稳定日常游玩</td><td>适合大多数玩家</td></tr><tr><td>Preview</td><td>提前测试新内容</td><td>更容易出现世界、包或 Mod 兼容性变化</td></tr></tbody></table><h2 id="什么是隔离" tabindex="-1">什么是隔离 <a class="header-anchor" href="#什么是隔离" aria-label="Permalink to &quot;什么是隔离&quot;">​</a></h2><p>启用隔离后，LeviLauncher 会把该版本的游戏数据保存在自己的工作区中，而不是与默认数据位置共享。</p><p>隔离适合这些需求：</p><ul><li>不同版本使用不同内容组合</li><li>更安全地测试 Mods 或 Preview</li><li>降低一个环境影响另一个环境的风险</li></ul><h2 id="常见版本操作" tabindex="-1">常见版本操作 <a class="header-anchor" href="#常见版本操作" aria-label="Permalink to &quot;常见版本操作&quot;">​</a></h2><h3 id="安装新版本" tabindex="-1">安装新版本 <a class="header-anchor" href="#安装新版本" aria-label="Permalink to &quot;安装新版本&quot;">​</a></h3><p>在 <strong>Download</strong> 页面完成安装，结束后回到 <strong>Home</strong> 即可启动。</p><h3 id="重命名版本" tabindex="-1">重命名版本 <a class="header-anchor" href="#重命名版本" aria-label="Permalink to &quot;重命名版本&quot;">​</a></h3><p>在 <strong>Versions</strong> 或版本设置页，为该环境设置更容易识别的用途名，例如 <code>Release Survival</code> 或 <code>Preview Test</code>。</p><h3 id="删除版本" tabindex="-1">删除版本 <a class="header-anchor" href="#删除版本" aria-label="Permalink to &quot;删除版本&quot;">​</a></h3><p>移除不再需要的版本，尤其是已经完成测试的 Preview 环境。</p><div class="warning custom-block"><p class="custom-block-title">删除前先确认</p><p>删除版本可能会移除该托管环境中的相关内容。重要世界请先备份。</p></div><h3 id="创建桌面快捷方式" tabindex="-1">创建桌面快捷方式 <a class="header-anchor" href="#创建桌面快捷方式" aria-label="Permalink to &quot;创建桌面快捷方式&quot;">​</a></h3><p>LeviLauncher 可以为指定版本创建桌面快捷方式，方便你直接启动。</p><h2 id="推荐的版本策略" tabindex="-1">推荐的版本策略 <a class="header-anchor" href="#推荐的版本策略" aria-label="Permalink to &quot;推荐的版本策略&quot;">​</a></h2><ul><li>一个隔离的 Release 版本用于日常游玩</li><li>一个隔离的 Preview 版本只在需要时使用</li><li>一个临时测试版本用于 Mods 或排错</li></ul><h2 id="相关阅读" tabindex="-1">相关阅读 <a class="header-anchor" href="#相关阅读" aria-label="Permalink to &quot;相关阅读&quot;">​</a></h2><ul><li><a href="./downloads-mirrors">下载与镜像</a></li><li><a href="./world-tools">世界工具</a></li><li><a href="./update-troubleshooting">更新与故障排查</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/version-management.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const versionManagement = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  versionManagement as default
};
