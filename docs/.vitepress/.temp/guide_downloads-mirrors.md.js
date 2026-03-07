import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"Downloads & Mirrors","description":"","frontmatter":{},"headers":[],"relativePath":"guide/downloads-mirrors.md","filePath":"guide/downloads-mirrors.md","lastUpdated":null}');
const _sfc_main = { name: "guide/downloads-mirrors.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="downloads-mirrors" tabindex="-1">Downloads &amp; Mirrors <a class="header-anchor" href="#downloads-mirrors" aria-label="Permalink to &quot;Downloads &amp; Mirrors&quot;">​</a></h1><p>LeviLauncher includes tools to help you choose a usable download source, track progress, and use local installation files when needed.</p><h2 id="what-this-area-is-for" tabindex="-1">What this area is for <a class="header-anchor" href="#what-this-area-is-for" aria-label="Permalink to &quot;What this area is for&quot;">​</a></h2><ul><li>browse installable versions</li><li>test or select available mirrors</li><li>track active download and install progress</li><li>retry failed tasks more easily</li><li>import a local installer package when supported by your workflow</li></ul><h2 id="recommended-workflow" tabindex="-1">Recommended workflow <a class="header-anchor" href="#recommended-workflow" aria-label="Permalink to &quot;Recommended workflow&quot;">​</a></h2><ol><li>Open <strong>Download</strong>.</li><li>Choose Release or Preview.</li><li>Let LeviLauncher test or use your preferred mirror.</li><li>Start the task.</li><li>Open the task or download manager view if you want more detailed progress.</li></ol><h2 id="when-to-change-mirrors" tabindex="-1">When to change mirrors <a class="header-anchor" href="#when-to-change-mirrors" aria-label="Permalink to &quot;When to change mirrors&quot;">​</a></h2><p>Consider switching mirrors when:</p><ul><li>downloads are unusually slow</li><li>a mirror repeatedly fails to connect</li><li>a task stalls for too long compared with another source</li></ul><h2 id="local-package-import" tabindex="-1">Local package import <a class="header-anchor" href="#local-package-import" aria-label="Permalink to &quot;Local package import&quot;">​</a></h2><p>If you already have a compatible local package or installer source, LeviLauncher can use local import flows where supported.</p><p>This is useful when:</p><ul><li>your network is unstable</li><li>you want to reuse a previously downloaded package</li><li>you need a more predictable installation source</li></ul><h2 id="if-a-task-fails" tabindex="-1">If a task fails <a class="header-anchor" href="#if-a-task-fails" aria-label="Permalink to &quot;If a task fails&quot;">​</a></h2><p>Check the following before retrying:</p><ul><li>enough disk space is available</li><li>the target path is writable</li><li>required Windows components are installed</li><li>your selected mirror is still responsive</li></ul><p>Then continue with <a href="./update-troubleshooting">Update &amp; Troubleshooting</a> if the failure persists.</p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("guide/downloads-mirrors.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const downloadsMirrors = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  downloadsMirrors as default
};
