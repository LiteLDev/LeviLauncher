import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"内容管理","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/content-management.md","filePath":"zh-CN/guide/content-management.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/content-management.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="内容管理" tabindex="-1">内容管理 <a class="header-anchor" href="#内容管理" aria-label="Permalink to &quot;内容管理&quot;">​</a></h1><p>LeviLauncher 能管理与版本相关的内容，尤其适合配合隔离环境一起使用。</p><h2 id="启动器中可管理的内容类型" tabindex="-1">启动器中可管理的内容类型 <a class="header-anchor" href="#启动器中可管理的内容类型" aria-label="Permalink to &quot;启动器中可管理的内容类型&quot;">​</a></h2><ul><li>世界</li><li>资源包</li><li>行为包</li><li>皮肤包</li><li>截图</li><li>服务器列表</li></ul><h2 id="为什么这部分很重要" tabindex="-1">为什么这部分很重要 <a class="header-anchor" href="#为什么这部分很重要" aria-label="Permalink to &quot;为什么这部分很重要&quot;">​</a></h2><p>对许多玩家来说，真正开始长期使用启动器的原因，不是“装上版本”，而是“把不同环境整理清楚”。</p><p>例如：</p><ul><li>一个版本专门放原版生存世界</li><li>一个版本专门测试资源包</li><li>一个 Preview 版本单独保存截图和服务器列表</li></ul><h2 id="常见操作" tabindex="-1">常见操作 <a class="header-anchor" href="#常见操作" aria-label="Permalink to &quot;常见操作&quot;">​</a></h2><h3 id="打开内容总览" tabindex="-1">打开内容总览 <a class="header-anchor" href="#打开内容总览" aria-label="Permalink to &quot;打开内容总览&quot;">​</a></h3><p>在应用中进入 <strong>Content</strong>，可以看到各类内容的数量与快捷入口。</p><h3 id="导入内容" tabindex="-1">导入内容 <a class="header-anchor" href="#导入内容" aria-label="Permalink to &quot;导入内容&quot;">​</a></h3><p>LeviLauncher 对部分内容类型支持导入流程，例如拖拽导入。</p><h3 id="打开真实文件夹" tabindex="-1">打开真实文件夹 <a class="header-anchor" href="#打开真实文件夹" aria-label="Permalink to &quot;打开真实文件夹&quot;">​</a></h3><p>如果你需要手动检查或清理文件，可以使用启动器提供的打开目录动作。</p><h3 id="用隔离保持内容独立" tabindex="-1">用隔离保持内容独立 <a class="header-anchor" href="#用隔离保持内容独立" aria-label="Permalink to &quot;用隔离保持内容独立&quot;">​</a></h3><p>当版本开启隔离后，该版本内容会保存在独立工作区，非常适合测试与归档整理。</p><h2 id="推荐整理习惯" tabindex="-1">推荐整理习惯 <a class="header-anchor" href="#推荐整理习惯" aria-label="Permalink to &quot;推荐整理习惯&quot;">​</a></h2><ul><li>测试用资源包不要和主世界混用</li><li>重大改动前先备份重要世界</li><li>为版本取清晰名字，避免搞混内容归属</li></ul><h2 id="关于截图与服务器" tabindex="-1">关于截图与服务器 <a class="header-anchor" href="#关于截图与服务器" aria-label="Permalink to &quot;关于截图与服务器&quot;">​</a></h2><p>如果不同版本承担不同用途，那么把截图和服务器列表也按版本分开，通常会更清晰。</p><h2 id="相关阅读" tabindex="-1">相关阅读 <a class="header-anchor" href="#相关阅读" aria-label="Permalink to &quot;相关阅读&quot;">​</a></h2><ul><li><a href="./version-management">版本管理</a></li><li><a href="./world-tools">世界工具</a></li><li><a href="./settings-personalization">设置与个性化</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/content-management.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const contentManagement = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  contentManagement as default
};
