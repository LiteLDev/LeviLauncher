import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"世界工具","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/world-tools.md","filePath":"zh-CN/guide/world-tools.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/world-tools.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="世界工具" tabindex="-1">世界工具 <a class="header-anchor" href="#世界工具" aria-label="Permalink to &quot;世界工具&quot;">​</a></h1><p>LeviLauncher 提供了一些帮助你保护和检查世界数据的工具。</p><h2 id="主要能力" tabindex="-1">主要能力 <a class="header-anchor" href="#主要能力" aria-label="Permalink to &quot;主要能力&quot;">​</a></h2><ul><li>把世界导出为 <code>.mcworld</code> 等备份包</li><li>查看世界相关信息</li><li>编辑部分 <code>level.dat</code> 字段</li><li>通过启动器流程更安全地修改世界名称</li></ul><h2 id="什么时候应该备份世界" tabindex="-1">什么时候应该备份世界 <a class="header-anchor" href="#什么时候应该备份世界" aria-label="Permalink to &quot;什么时候应该备份世界&quot;">​</a></h2><p>以下场景都建议先备份：</p><ul><li>测试 Preview 版本前</li><li>安装或更新多个 Mods 前</li><li>修改重要世界元数据前</li><li>在差异较大的环境之间迁移世界前</li></ul><h2 id="关于-level-dat-编辑" tabindex="-1">关于 <code>level.dat</code> 编辑 <a class="header-anchor" href="#关于-level-dat-编辑" aria-label="Permalink to &quot;关于 \`level.dat\` 编辑&quot;">​</a></h2><p>LeviLauncher 可以把 <code>level.dat</code> 中的部分字段以更易处理的方式暴露出来。</p><div class="warning custom-block"><p class="custom-block-title">请谨慎编辑</p><p><code>level.dat</code> 属于敏感的世界元数据。请先备份，并且只修改自己理解的字段。</p></div><h2 id="推荐备份习惯" tabindex="-1">推荐备份习惯 <a class="header-anchor" href="#推荐备份习惯" aria-label="Permalink to &quot;推荐备份习惯&quot;">​</a></h2><ol><li>大版本更新前备份。</li><li>引入新 Mods 或资源包前备份。</li><li>至少保留一份不放在当前版本工作区内的备份。</li></ol><h2 id="相关阅读" tabindex="-1">相关阅读 <a class="header-anchor" href="#相关阅读" aria-label="Permalink to &quot;相关阅读&quot;">​</a></h2><ul><li><a href="./content-management">内容管理</a></li><li><a href="./version-management">版本管理</a></li><li><a href="./update-troubleshooting">更新与故障排查</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/world-tools.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const worldTools = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  worldTools as default
};
