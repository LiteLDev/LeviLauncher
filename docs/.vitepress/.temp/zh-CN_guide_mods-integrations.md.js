import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"Mods 与集成","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/mods-integrations.md","filePath":"zh-CN/guide/mods-integrations.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/mods-integrations.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="mods-与集成" tabindex="-1">Mods 与集成 <a class="header-anchor" href="#mods-与集成" aria-label="Permalink to &quot;Mods 与集成&quot;">​</a></h1><p>LeviLauncher 支持超出“基础启动器”范围的 Mod 相关工作流。</p><h2 id="当前可用能力" tabindex="-1">当前可用能力 <a class="header-anchor" href="#当前可用能力" aria-label="Permalink to &quot;当前可用能力&quot;">​</a></h2><ul><li>导入 <code>.zip</code> Mods</li><li>导入 <code>.dll</code> Mods</li><li>启用或禁用已安装 Mods</li><li>删除不再需要的 Mods</li><li>使用 CurseForge 相关浏览与获取流程</li><li>使用 LIP 包管理流程</li><li>配合 LeviLamina 等加载器工作流</li></ul><div class="warning custom-block"><p class="custom-block-title">这是高级区域</p><p>Mods、加载器与包管理集成的兼容性变化往往更快，尤其是在 Preview 上。请把这里当作高级能力，逐步测试。</p></div><h2 id="推荐的-mod-工作流" tabindex="-1">推荐的 Mod 工作流 <a class="header-anchor" href="#推荐的-mod-工作流" aria-label="Permalink to &quot;推荐的 Mod 工作流&quot;">​</a></h2><ol><li>从一个干净的隔离版本开始。</li><li>先确认该版本在无 Mods 情况下可以正常启动。</li><li>一次只增加一个 Mod 或一小组改动。</li><li>每次改动后都重新测试启动。</li><li>重要世界始终保留备份路径。</li></ol><h2 id="curseforge" tabindex="-1">CurseForge <a class="header-anchor" href="#curseforge" aria-label="Permalink to &quot;CurseForge&quot;">​</a></h2><p>启动器提供 CurseForge 相关浏览与包流程，帮助你在应用内更方便地寻找合适资源。</p><p>适合以下需求：</p><ul><li>需要更引导式的查找体验</li><li>想更快看到某个项目的兼容文件</li><li>不想所有搜索都在启动器外完成</li></ul><h2 id="lip-与-levilamina" tabindex="-1">LIP 与 LeviLamina <a class="header-anchor" href="#lip-与-levilamina" aria-label="Permalink to &quot;LIP 与 LeviLamina&quot;">​</a></h2><p>这类集成更适合希望使用包驱动或加载器驱动工作流的高级用户。</p><p>最佳实践：</p><ul><li>先从稳定的 Release 版本开始</li><li>不要一次变更多个高风险因素</li><li>记录下“最后一次成功工作”的版本组合</li></ul><h2 id="如果加了-mods-后出错" tabindex="-1">如果加了 Mods 后出错 <a class="header-anchor" href="#如果加了-mods-后出错" aria-label="Permalink to &quot;如果加了 Mods 后出错&quot;">​</a></h2><ul><li>先禁用最新加入的 Mod</li><li>与上一个可正常工作的状态进行对比</li><li>进一步测试前先转移或备份重要世界</li><li>然后继续查看<a href="./update-troubleshooting">更新与故障排查</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/mods-integrations.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const modsIntegrations = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  modsIntegrations as default
};
