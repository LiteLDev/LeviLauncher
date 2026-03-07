import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"下载与镜像","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/downloads-mirrors.md","filePath":"zh-CN/guide/downloads-mirrors.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/downloads-mirrors.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="下载与镜像" tabindex="-1">下载与镜像 <a class="header-anchor" href="#下载与镜像" aria-label="Permalink to &quot;下载与镜像&quot;">​</a></h1><p>LeviLauncher 提供镜像选择、任务进度与本地包导入等能力，帮助你更稳定地完成安装。</p><h2 id="这个区域适合做什么" tabindex="-1">这个区域适合做什么 <a class="header-anchor" href="#这个区域适合做什么" aria-label="Permalink to &quot;这个区域适合做什么&quot;">​</a></h2><ul><li>浏览可安装版本</li><li>测速或切换镜像源</li><li>查看下载与安装进度</li><li>更容易重试失败任务</li><li>在合适场景下导入本地安装包</li></ul><h2 id="推荐操作流程" tabindex="-1">推荐操作流程 <a class="header-anchor" href="#推荐操作流程" aria-label="Permalink to &quot;推荐操作流程&quot;">​</a></h2><ol><li>打开 <strong>Download</strong>。</li><li>选择 Release 或 Preview。</li><li>让 LeviLauncher 自动测速，或手动选择合适镜像。</li><li>启动安装任务。</li><li>如需更详细进度，进入任务或下载管理视图查看。</li></ol><h2 id="什么时候应该切换镜像" tabindex="-1">什么时候应该切换镜像 <a class="header-anchor" href="#什么时候应该切换镜像" aria-label="Permalink to &quot;什么时候应该切换镜像&quot;">​</a></h2><p>以下情况建议换源：</p><ul><li>下载速度明显异常</li><li>某个镜像连续连接失败</li><li>任务在一个源上长时间卡住，而其他源更稳定</li></ul><h2 id="本地包导入" tabindex="-1">本地包导入 <a class="header-anchor" href="#本地包导入" aria-label="Permalink to &quot;本地包导入&quot;">​</a></h2><p>如果你已经拥有兼容的本地安装包或来源，LeviLauncher 在支持的流程下可以使用本地导入。</p><p>这在以下场景会很有帮助：</p><ul><li>网络不稳定</li><li>你想复用之前下载好的资源</li><li>你需要更可控、更固定的安装来源</li></ul><h2 id="如果任务失败" tabindex="-1">如果任务失败 <a class="header-anchor" href="#如果任务失败" aria-label="Permalink to &quot;如果任务失败&quot;">​</a></h2><p>重试前先确认：</p><ul><li>磁盘空间足够</li><li>目标路径可写</li><li>所需 Windows 组件已安装</li><li>当前选择的镜像仍然可用</li></ul><p>如果仍然失败，请继续查看<a href="./update-troubleshooting">更新与故障排查</a>。</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/downloads-mirrors.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const downloadsMirrors = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  downloadsMirrors as default
};
