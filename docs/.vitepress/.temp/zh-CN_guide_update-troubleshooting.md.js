import { ssrRenderAttrs } from "vue/server-renderer";
import { useSSRContext } from "vue";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const __pageData = JSON.parse('{"title":"更新与故障排查","description":"","frontmatter":{},"headers":[],"relativePath":"zh-CN/guide/update-troubleshooting.md","filePath":"zh-CN/guide/update-troubleshooting.md","lastUpdated":null}');
const _sfc_main = { name: "zh-CN/guide/update-troubleshooting.md" };
function _sfc_ssrRender(_ctx, _push, _parent, _attrs, $props, $setup, $data, $options) {
  _push(`<div${ssrRenderAttrs(_attrs)}><h1 id="更新与故障排查" tabindex="-1">更新与故障排查 <a class="header-anchor" href="#更新与故障排查" aria-label="Permalink to &quot;更新与故障排查&quot;">​</a></h1><p>当 LeviLauncher 的安装、更新或启动行为不符合预期时，请优先参考这一页。</p><h2 id="常见问题" tabindex="-1">常见问题 <a class="header-anchor" href="#常见问题" aria-label="Permalink to &quot;常见问题&quot;">​</a></h2><h3 id="gaming-services-缺失或损坏" tabindex="-1">Gaming Services 缺失或损坏 <a class="header-anchor" href="#gaming-services-缺失或损坏" aria-label="Permalink to &quot;Gaming Services 缺失或损坏&quot;">​</a></h3><ul><li>使用启动器提供的引导进行安装或修复。</li><li>必要时回到 Microsoft Store 检查 Minecraft Bedrock 的安装状态。</li></ul><h3 id="gameinput-缺失" tabindex="-1">GameInput 缺失 <a class="header-anchor" href="#gameinput-缺失" aria-label="Permalink to &quot;GameInput 缺失&quot;">​</a></h3><ul><li>按 LeviLauncher 提示安装所需 redistributable。</li><li>安装完成后，如有需要请重启启动器。</li></ul><h3 id="安装路径不可写" tabindex="-1">安装路径不可写 <a class="header-anchor" href="#安装路径不可写" aria-label="Permalink to &quot;安装路径不可写&quot;">​</a></h3><ul><li>在 <strong>Settings</strong> 中把托管内容路径改到可写位置。</li><li>如果启动器在安装或自更新时要求提升权限，只在你确认可信时允许执行。</li></ul><h3 id="某个版本无法启动" tabindex="-1">某个版本无法启动 <a class="header-anchor" href="#某个版本无法启动" aria-label="Permalink to &quot;某个版本无法启动&quot;">​</a></h3><ul><li>先测试该版本在无 Mods 情况下能否启动</li><li>检查所需 Windows 组件是否仍然存在</li><li>确认该版本是否已经完整安装</li><li>试着建立一个干净的隔离测试环境复现问题</li></ul><h3 id="下载很慢或反复失败" tabindex="-1">下载很慢或反复失败 <a class="header-anchor" href="#下载很慢或反复失败" aria-label="Permalink to &quot;下载很慢或反复失败&quot;">​</a></h3><ul><li>切换镜像</li><li>稍后重试</li><li>如条件允许，改用本地包来源</li></ul><h2 id="更安全的恢复步骤" tabindex="-1">更安全的恢复步骤 <a class="header-anchor" href="#更安全的恢复步骤" aria-label="Permalink to &quot;更安全的恢复步骤&quot;">​</a></h2><ol><li>先备份重要世界。</li><li>移除最近新增的 Mods 或资源包。</li><li>用一个干净的隔离版本做对照测试。</li><li>必要时重新安装受影响版本。</li></ol><h2 id="关于自更新" tabindex="-1">关于自更新 <a class="header-anchor" href="#关于自更新" aria-label="Permalink to &quot;关于自更新&quot;">​</a></h2><p>LeviLauncher 可以检查、下载并安装应用更新。若安装目录不可写，某些环境下可能需要管理员权限。</p><h2 id="什么时候应该提-issue" tabindex="-1">什么时候应该提 Issue <a class="header-anchor" href="#什么时候应该提-issue" aria-label="Permalink to &quot;什么时候应该提 Issue&quot;">​</a></h2><p>当满足以下条件时，建议到 GitHub 提交问题：</p><ul><li>你能稳定复现问题</li><li>做过基础恢复步骤后仍然存在</li><li>问题看起来更像 LeviLauncher 自身，而不是一般 Windows 环境问题</li></ul><p>提交时请尽量附带：</p><ul><li>Windows 版本</li><li>LeviLauncher 版本</li><li>你的具体操作步骤</li><li>截图或日志（如果有）</li></ul><p>问题反馈入口： <a href="https://github.com/LiteLDev/LeviLauncher/issues" target="_blank" rel="noreferrer">GitHub Issues</a></p></div>`);
}
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("zh-CN/guide/update-troubleshooting.md");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const updateTroubleshooting = /* @__PURE__ */ _export_sfc(_sfc_main, [["ssrRender", _sfc_ssrRender]]);
export {
  __pageData,
  updateTroubleshooting as default
};
