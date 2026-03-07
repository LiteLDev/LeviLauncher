import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"设置与个性化","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/settings-personalization.md","filePath":"zh-CN/guide/settings-personalization.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/settings-personalization.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="设置与个性化" tabindex="-1">设置与个性化 <a class="header-anchor" href="#设置与个性化" aria-label="Permalink to &quot;设置与个性化&quot;">​</a></h1><p>LeviLauncher 提供了较完整的设置页，可用于调整路径、外观、组件管理、隐私偏好与更新策略。</p><h2 id="主要设置分区" tabindex="-1">主要设置分区 <a class="header-anchor" href="#主要设置分区" aria-label="Permalink to &quot;主要设置分区&quot;">​</a></h2><p>应用当前会把设置分为多个区域，例如：</p><ul><li>General</li><li>Personalization</li><li>Components</li><li>Others</li><li>Privacy</li><li>Updates</li><li>About</li></ul><h2 id="你可以调整什么" tabindex="-1">你可以调整什么 <a class="header-anchor" href="#你可以调整什么" aria-label="Permalink to &quot;你可以调整什么&quot;">​</a></h2><h3 id="路径与存储" tabindex="-1">路径与存储 <a class="header-anchor" href="#路径与存储" aria-label="Permalink to &quot;路径与存储&quot;">​</a></h3><ul><li>基础内容路径</li><li>托管文件所在的可写位置</li><li>更适合你磁盘布局的环境放置方式</li></ul><h3 id="外观" tabindex="-1">外观 <a class="header-anchor" href="#外观" aria-label="Permalink to &quot;外观&quot;">​</a></h3><ul><li>语言</li><li>明暗主题偏好</li><li>主题色行为</li><li>背景与视觉个性化项</li></ul><h3 id="组件" tabindex="-1">组件 <a class="header-anchor" href="#组件" aria-label="Permalink to &quot;组件&quot;">​</a></h3><ul><li>GDK 相关组件状态</li><li>LIP 状态与更新</li><li>其他由启动器维护的辅助资源</li></ul><h3 id="隐私" tabindex="-1">隐私 <a class="header-anchor" href="#隐私" aria-label="Permalink to &quot;隐私&quot;">​</a></h3><ul><li>应用提供的 analytics 相关选择</li><li>外部隐私政策与条款链接</li></ul><h3 id="更新" tabindex="-1">更新 <a class="header-anchor" href="#更新" aria-label="Permalink to &quot;更新&quot;">​</a></h3><ul><li>更新检查</li><li>Beta 更新通道偏好</li></ul><h2 id="推荐默认值" tabindex="-1">推荐默认值 <a class="header-anchor" href="#推荐默认值" aria-label="Permalink to &quot;推荐默认值&quot;">​</a></h2><ul><li>把托管文件放在一个可写且空间充足的磁盘位置</li><li>如果你不主动追 Beta，优先保持稳定更新通道</li><li>先让基础环境稳定，再去细调外观</li></ul><h2 id="什么时候值得重新检查设置" tabindex="-1">什么时候值得重新检查设置 <a class="header-anchor" href="#什么时候值得重新检查设置" aria-label="Permalink to &quot;什么时候值得重新检查设置&quot;">​</a></h2><p>当你遇到这些情况时，建议回设置页看一遍：</p><ul><li>想把安装迁移到另一块磁盘</li><li>遇到写权限错误</li><li>想在不同机器上使用不同外观或路径策略</li><li>需要排查下载或组件识别问题</li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/settings-personalization.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const settingsPersonalization = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  settingsPersonalization as default
};
