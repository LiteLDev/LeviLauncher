import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"快速开始","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/quick-start.md","filePath":"zh-CN/guide/quick-start.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/quick-start.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="快速开始" tabindex="-1">快速开始 <a class="header-anchor" href="#快速开始" aria-label="Permalink to &quot;快速开始&quot;">​</a></h1><p>如果你只想用最短路径从下载走到进游戏，先看这一页。</p><h2 id="_1-安装-levilauncher" tabindex="-1">1. 安装 LeviLauncher <a class="header-anchor" href="#_1-安装-levilauncher" aria-label="Permalink to &quot;1. 安装 LeviLauncher&quot;">​</a></h2><ul><li>从 <a href="https://github.com/LiteLDev/LeviLauncher/releases" target="_blank" rel="noreferrer">GitHub Releases</a> 或<a href="https://levimc.lanzoue.com/b016ke39hc" target="_blank" rel="noreferrer">蓝奏云镜像</a>下载安装包。</li><li>在 Windows 上运行安装程序。</li><li>如果安装过程提示需要 WebView2 或其他依赖，请允许其继续安装。</li></ul><h2 id="_2-确认-microsoft-store-环境已经准备好" tabindex="-1">2. 确认 Microsoft Store 环境已经准备好 <a class="header-anchor" href="#_2-确认-microsoft-store-环境已经准备好" aria-label="Permalink to &quot;2. 确认 Microsoft Store 环境已经准备好&quot;">​</a></h2><ul><li>登录拥有 <strong>Minecraft Bedrock Edition</strong> 的 Microsoft 账号。</li><li>至少在当前电脑上从 Microsoft Store 安装过一次游戏。</li><li>确保 <strong>Gaming Services</strong> 可用。</li></ul><div class="warning custom-block"><p class="custom-block-title">必须拥有正版</p><p>LeviLauncher 用于管理 Minecraft Bedrock (GDK)，并不能替代游戏授权。</p></div><h2 id="_3-打开下载页" tabindex="-1">3. 打开下载页 <a class="header-anchor" href="#_3-打开下载页" aria-label="Permalink to &quot;3. 打开下载页&quot;">​</a></h2><ul><li>启动 LeviLauncher。</li><li>从侧边栏进入 <strong>Download</strong>。</li><li>选择 <strong>Release</strong> 或 <strong>Preview</strong>。</li><li>选择版本并开始安装。</li></ul><h2 id="_4-决定是否启用隔离" tabindex="-1">4. 决定是否启用隔离 <a class="header-anchor" href="#_4-决定是否启用隔离" aria-label="Permalink to &quot;4. 决定是否启用隔离&quot;">​</a></h2><p>隔离会把该版本的游戏数据放进独立工作区，而不是与默认 AppData 环境混在一起。</p><p>以下场景建议启用隔离：</p><ul><li>测试 Mods，但不想影响主环境</li><li>想把正式版与预览版分开</li><li>想给不同游玩目的保留不同的世界和资源包</li></ul><h2 id="_5-启动已安装版本" tabindex="-1">5. 启动已安装版本 <a class="header-anchor" href="#_5-启动已安装版本" aria-label="Permalink to &quot;5. 启动已安装版本&quot;">​</a></h2><ul><li>返回 <strong>Home</strong>。</li><li>选中刚安装的版本。</li><li>点击 <strong>Launch</strong>。</li></ul><p>如果 LeviLauncher 报告缺少 GameInput 或 Gaming Services，请先按提示完成依赖安装。</p><h2 id="_6-继续阅读对应章节" tabindex="-1">6. 继续阅读对应章节 <a class="header-anchor" href="#_6-继续阅读对应章节" aria-label="Permalink to &quot;6. 继续阅读对应章节&quot;">​</a></h2><ul><li>需要准备环境：<a href="./requirements-installation">系统要求与安装</a></li><li>需要了解首次进入应用会发生什么：<a href="./first-launch">首次启动</a></li><li>需要整理版本：<a href="./version-management">版本管理</a></li><li>需要管理世界、资源包或截图：<a href="./content-management">内容管理</a></li><li>需要使用 Mods 或集成源：<a href="./mods-integrations">Mods 与集成</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/quick-start.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const quickStart = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  quickStart as default
};
