import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"World Tools","description":"","frontmatter":{},"headers":[],"relativePath":"guide/world-tools.md","filePath":"guide/world-tools.md","lastUpdated":null}');
const _sfc_main = { name: "guide/world-tools.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="world-tools" tabindex="-1">World Tools <a class="header-anchor" href="#world-tools" aria-label="Permalink to &quot;World Tools&quot;">​</a></h1><p>LeviLauncher includes utilities that help you protect and inspect your worlds.</p><h2 id="main-world-tools" tabindex="-1">Main world tools <a class="header-anchor" href="#main-world-tools" aria-label="Permalink to &quot;Main world tools&quot;">​</a></h2><ul><li>export a world as a backup package such as <code>.mcworld</code></li><li>inspect world-related information</li><li>edit selected <code>level.dat</code> fields</li><li>rename a world more safely through the launcher workflow</li></ul><h2 id="when-to-back-up-a-world" tabindex="-1">When to back up a world <a class="header-anchor" href="#when-to-back-up-a-world" aria-label="Permalink to &quot;When to back up a world&quot;">​</a></h2><p>Always create a backup before you:</p><ul><li>test Preview versions</li><li>install or update multiple mods</li><li>change important world metadata</li><li>move a world between very different setups</li></ul><h2 id="editing-level-dat" tabindex="-1">Editing <code>level.dat</code> <a class="header-anchor" href="#editing-level-dat" aria-label="Permalink to &quot;Editing \`level.dat\`&quot;">​</a></h2><p>LeviLauncher can expose selected fields from <code>level.dat</code> for easier editing.</p><div class="warning custom-block"><p class="custom-block-title">Edit carefully</p><p><code>level.dat</code> is sensitive world metadata. Make a backup first and change only what you understand.</p></div><h2 id="recommended-backup-routine" tabindex="-1">Recommended backup routine <a class="header-anchor" href="#recommended-backup-routine" aria-label="Permalink to &quot;Recommended backup routine&quot;">​</a></h2><ol><li>Back up before major updates.</li><li>Back up before introducing new mods or packs.</li><li>Keep at least one backup outside the version workspace.</li></ol><h2 id="related-guides" tabindex="-1">Related guides <a class="header-anchor" href="#related-guides" aria-label="Permalink to &quot;Related guides&quot;">​</a></h2><ul><li><a href="./content-management">Content Management</a></li><li><a href="./version-management">Version Management</a></li><li><a href="./update-troubleshooting">Update &amp; Troubleshooting</a></li></ul></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("guide/world-tools.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const worldTools = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  worldTools as default
};
