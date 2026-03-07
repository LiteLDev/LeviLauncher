var __defProp = Object.defineProperty;
var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
var __publicField = (obj, key, value) => __defNormalProp(obj, typeof key !== "symbol" ? key + "" : key, value);
import { defineComponent, shallowRef, markRaw, computed, ref, watchEffect, watch, createApp, nextTick, onMounted, onBeforeUnmount, unref, useSSRContext } from "vue";
import { ssrRenderTeleport, ssrRenderAttr, ssrRenderClass, ssrIncludeBooleanAttr, ssrRenderList, ssrInterpolate } from "vue/server-renderer";
import { computedAsync, useSessionStorage, useLocalStorage, debouncedWatch, onKeyStroke, useEventListener, useScrollLock } from "@vueuse/core";
import { useFocusTrap } from "@vueuse/integrations/useFocusTrap";
import Mark from "mark.js/src/vanilla.js";
import MiniSearch from "minisearch";
import { u as useData, d as dataSymbol, p as pathToFile, a as useRouter, c as createSearchTranslate, i as inBrowser, e as escapeRegExp } from "./app.js";
import { _ as _export_sfc } from "./plugin-vue_export-helper.1tPrXgE0.js";
const localSearchIndex = { "root": () => import("./@localSearchIndexroot.BqiO_0c2.js"), "zh-CN": () => import("./@localSearchIndexzh-CN.hPIDoCKq.js") };
class LRUCache {
  constructor(max = 10) {
    __publicField(this, "max");
    __publicField(this, "cache");
    this.max = max;
    this.cache = /* @__PURE__ */ new Map();
  }
  get(key) {
    let item = this.cache.get(key);
    if (item !== void 0) {
      this.cache.delete(key);
      this.cache.set(key, item);
    }
    return item;
  }
  set(key, val) {
    if (this.cache.has(key))
      this.cache.delete(key);
    else if (this.cache.size === this.max)
      this.cache.delete(this.first());
    this.cache.set(key, val);
  }
  first() {
    return this.cache.keys().next().value;
  }
  clear() {
    this.cache.clear();
  }
}
const _sfc_main = /* @__PURE__ */ defineComponent({
  __name: "VPLocalSearchBox",
  __ssrInlineRender: true,
  emits: ["close"],
  setup(__props, { emit: __emit }) {
    var _a, _b;
    const emit = __emit;
    const el = shallowRef();
    const resultsEl = shallowRef();
    const searchIndexData = shallowRef(localSearchIndex);
    const vitePressData = useData();
    const { activate } = useFocusTrap(el, {
      immediate: true,
      allowOutsideClick: true,
      clickOutsideDeactivates: true,
      escapeDeactivates: true
    });
    const { localeIndex, theme } = vitePressData;
    const searchIndex = computedAsync(
      async () => {
        var _a2, _b2, _c, _d, _e, _f, _g, _h, _i;
        return markRaw(
          MiniSearch.loadJSON(
            (_c = await ((_b2 = (_a2 = searchIndexData.value)[localeIndex.value]) == null ? void 0 : _b2.call(_a2))) == null ? void 0 : _c.default,
            {
              fields: ["title", "titles", "text"],
              storeFields: ["title", "titles"],
              searchOptions: {
                fuzzy: 0.2,
                prefix: true,
                boost: { title: 4, text: 2, titles: 1 },
                ...((_d = theme.value.search) == null ? void 0 : _d.provider) === "local" && ((_f = (_e = theme.value.search.options) == null ? void 0 : _e.miniSearch) == null ? void 0 : _f.searchOptions)
              },
              ...((_g = theme.value.search) == null ? void 0 : _g.provider) === "local" && ((_i = (_h = theme.value.search.options) == null ? void 0 : _h.miniSearch) == null ? void 0 : _i.options)
            }
          )
        );
      }
    );
    const disableQueryPersistence = computed(() => {
      var _a2, _b2;
      return ((_a2 = theme.value.search) == null ? void 0 : _a2.provider) === "local" && ((_b2 = theme.value.search.options) == null ? void 0 : _b2.disableQueryPersistence) === true;
    });
    const filterText = disableQueryPersistence.value ? ref("") : useSessionStorage("vitepress:local-search-filter", "");
    const showDetailedList = useLocalStorage(
      "vitepress:local-search-detailed-list",
      ((_a = theme.value.search) == null ? void 0 : _a.provider) === "local" && ((_b = theme.value.search.options) == null ? void 0 : _b.detailedView) === true
    );
    const disableDetailedView = computed(() => {
      var _a2, _b2, _c;
      return ((_a2 = theme.value.search) == null ? void 0 : _a2.provider) === "local" && (((_b2 = theme.value.search.options) == null ? void 0 : _b2.disableDetailedView) === true || ((_c = theme.value.search.options) == null ? void 0 : _c.detailedView) === false);
    });
    const buttonText = computed(() => {
      var _a2, _b2, _c, _d, _e, _f, _g;
      const options = ((_a2 = theme.value.search) == null ? void 0 : _a2.options) ?? theme.value.algolia;
      return ((_e = (_d = (_c = (_b2 = options == null ? void 0 : options.locales) == null ? void 0 : _b2[localeIndex.value]) == null ? void 0 : _c.translations) == null ? void 0 : _d.button) == null ? void 0 : _e.buttonText) || ((_g = (_f = options == null ? void 0 : options.translations) == null ? void 0 : _f.button) == null ? void 0 : _g.buttonText) || "Search";
    });
    watchEffect(() => {
      if (disableDetailedView.value) {
        showDetailedList.value = false;
      }
    });
    const results = shallowRef([]);
    const enableNoResults = ref(false);
    watch(filterText, () => {
      enableNoResults.value = false;
    });
    const mark = computedAsync(async () => {
      if (!resultsEl.value) return;
      return markRaw(new Mark(resultsEl.value));
    }, null);
    const cache = new LRUCache(16);
    debouncedWatch(
      () => [searchIndex.value, filterText.value, showDetailedList.value],
      async ([index, filterTextValue, showDetailedListValue], old, onCleanup) => {
        var _a2, _b2, _c, _d;
        if ((old == null ? void 0 : old[0]) !== index) {
          cache.clear();
        }
        let canceled = false;
        onCleanup(() => {
          canceled = true;
        });
        if (!index) return;
        results.value = index.search(filterTextValue).slice(0, 16);
        enableNoResults.value = true;
        const mods = showDetailedListValue ? await Promise.all(results.value.map((r) => fetchExcerpt(r.id))) : [];
        if (canceled) return;
        for (const { id, mod } of mods) {
          const mapId = id.slice(0, id.indexOf("#"));
          let map = cache.get(mapId);
          if (map) continue;
          map = /* @__PURE__ */ new Map();
          cache.set(mapId, map);
          const comp = mod.default ?? mod;
          if ((comp == null ? void 0 : comp.render) || (comp == null ? void 0 : comp.setup)) {
            const app = createApp(comp);
            app.config.warnHandler = () => {
            };
            app.provide(dataSymbol, vitePressData);
            Object.defineProperties(app.config.globalProperties, {
              $frontmatter: {
                get() {
                  return vitePressData.frontmatter.value;
                }
              },
              $params: {
                get() {
                  return vitePressData.page.value.params;
                }
              }
            });
            const div = document.createElement("div");
            app.mount(div);
            const headings = div.querySelectorAll("h1, h2, h3, h4, h5, h6");
            headings.forEach((el2) => {
              var _a3;
              const href = (_a3 = el2.querySelector("a")) == null ? void 0 : _a3.getAttribute("href");
              const anchor = (href == null ? void 0 : href.startsWith("#")) && href.slice(1);
              if (!anchor) return;
              let html = "";
              while ((el2 = el2.nextElementSibling) && !/^h[1-6]$/i.test(el2.tagName))
                html += el2.outerHTML;
              map.set(anchor, html);
            });
            app.unmount();
          }
          if (canceled) return;
        }
        const terms = /* @__PURE__ */ new Set();
        results.value = results.value.map((r) => {
          const [id, anchor] = r.id.split("#");
          const map = cache.get(id);
          const text = (map == null ? void 0 : map.get(anchor)) ?? "";
          for (const term in r.match) {
            terms.add(term);
          }
          return { ...r, text };
        });
        await nextTick();
        if (canceled) return;
        await new Promise((r) => {
          var _a3;
          (_a3 = mark.value) == null ? void 0 : _a3.unmark({
            done: () => {
              var _a4;
              (_a4 = mark.value) == null ? void 0 : _a4.markRegExp(formMarkRegex(terms), { done: r });
            }
          });
        });
        const excerpts = ((_a2 = el.value) == null ? void 0 : _a2.querySelectorAll(".result .excerpt")) ?? [];
        for (const excerpt of excerpts) {
          (_b2 = excerpt.querySelector('mark[data-markjs="true"]')) == null ? void 0 : _b2.scrollIntoView({ block: "center" });
        }
        (_d = (_c = resultsEl.value) == null ? void 0 : _c.firstElementChild) == null ? void 0 : _d.scrollIntoView({ block: "start" });
      },
      { debounce: 200, immediate: true }
    );
    async function fetchExcerpt(id) {
      const file = pathToFile(id.slice(0, id.indexOf("#")));
      try {
        if (!file) throw new Error(`Cannot find file for id: ${id}`);
        return { id, mod: await import(
          /*@vite-ignore*/
          file
        ) };
      } catch (e) {
        console.error(e);
        return { id, mod: {} };
      }
    }
    const searchInput = ref();
    const disableReset = computed(() => {
      var _a2;
      return ((_a2 = filterText.value) == null ? void 0 : _a2.length) <= 0;
    });
    function focusSearchInput(select = true) {
      var _a2, _b2;
      (_a2 = searchInput.value) == null ? void 0 : _a2.focus();
      select && ((_b2 = searchInput.value) == null ? void 0 : _b2.select());
    }
    onMounted(() => {
      focusSearchInput();
    });
    const selectedIndex = ref(-1);
    const disableMouseOver = ref(true);
    watch(results, (r) => {
      selectedIndex.value = r.length ? 0 : -1;
      scrollToSelectedResult();
    });
    function scrollToSelectedResult() {
      nextTick(() => {
        const selectedEl = document.querySelector(".result.selected");
        selectedEl == null ? void 0 : selectedEl.scrollIntoView({ block: "nearest" });
      });
    }
    onKeyStroke("ArrowUp", (event) => {
      event.preventDefault();
      selectedIndex.value--;
      if (selectedIndex.value < 0) {
        selectedIndex.value = results.value.length - 1;
      }
      disableMouseOver.value = true;
      scrollToSelectedResult();
    });
    onKeyStroke("ArrowDown", (event) => {
      event.preventDefault();
      selectedIndex.value++;
      if (selectedIndex.value >= results.value.length) {
        selectedIndex.value = 0;
      }
      disableMouseOver.value = true;
      scrollToSelectedResult();
    });
    const router = useRouter();
    onKeyStroke("Enter", (e) => {
      if (e.isComposing) return;
      if (e.target instanceof HTMLButtonElement && e.target.type !== "submit")
        return;
      const selectedPackage = results.value[selectedIndex.value];
      if (e.target instanceof HTMLInputElement && !selectedPackage) {
        e.preventDefault();
        return;
      }
      if (selectedPackage) {
        router.go(selectedPackage.id);
        emit("close");
      }
    });
    onKeyStroke("Escape", () => {
      emit("close");
    });
    const defaultTranslations = {
      modal: {
        displayDetails: "Display detailed list",
        resetButtonTitle: "Reset search",
        backButtonTitle: "Close search",
        noResultsText: "No results for",
        footer: {
          selectText: "to select",
          selectKeyAriaLabel: "enter",
          navigateText: "to navigate",
          navigateUpKeyAriaLabel: "up arrow",
          navigateDownKeyAriaLabel: "down arrow",
          closeText: "to close",
          closeKeyAriaLabel: "escape"
        }
      }
    };
    const translate = createSearchTranslate(defaultTranslations);
    onMounted(() => {
      window.history.pushState(null, "", null);
    });
    useEventListener("popstate", (event) => {
      event.preventDefault();
      emit("close");
    });
    const isLocked = useScrollLock(inBrowser ? document.body : null);
    onMounted(() => {
      nextTick(() => {
        isLocked.value = true;
        nextTick().then(() => activate());
      });
    });
    onBeforeUnmount(() => {
      isLocked.value = false;
    });
    function formMarkRegex(terms) {
      return new RegExp(
        [...terms].sort((a, b) => b.length - a.length).map((term) => `(${escapeRegExp(term)})`).join("|"),
        "gi"
      );
    }
    return (_ctx, _push, _parent, _attrs) => {
      ssrRenderTeleport(_push, (_push2) => {
        var _a2, _b2, _c, _d, _e;
        _push2(`<div role="button"${ssrRenderAttr("aria-owns", ((_a2 = results.value) == null ? void 0 : _a2.length) ? "localsearch-list" : void 0)} aria-expanded="true" aria-haspopup="listbox" aria-labelledby="localsearch-label" class="VPLocalSearchBox" data-v-ce626c7c><div class="backdrop" data-v-ce626c7c></div><div class="shell" data-v-ce626c7c><form class="search-bar" data-v-ce626c7c><label${ssrRenderAttr("title", buttonText.value)} id="localsearch-label" for="localsearch-input" data-v-ce626c7c><span aria-hidden="true" class="vpi-search search-icon local-search-icon" data-v-ce626c7c></span></label><div class="search-actions before" data-v-ce626c7c><button class="back-button"${ssrRenderAttr("title", unref(translate)("modal.backButtonTitle"))} data-v-ce626c7c><span class="vpi-arrow-left local-search-icon" data-v-ce626c7c></span></button></div><input${ssrRenderAttr("value", unref(filterText))}${ssrRenderAttr("aria-activedescendant", selectedIndex.value > -1 ? "localsearch-item-" + selectedIndex.value : void 0)} aria-autocomplete="both"${ssrRenderAttr("aria-controls", ((_b2 = results.value) == null ? void 0 : _b2.length) ? "localsearch-list" : void 0)} aria-labelledby="localsearch-label" autocapitalize="off" autocomplete="off" autocorrect="off" class="search-input" id="localsearch-input" enterkeyhint="go" maxlength="64"${ssrRenderAttr("placeholder", buttonText.value)} spellcheck="false" type="search" data-v-ce626c7c><div class="search-actions" data-v-ce626c7c>`);
        if (!disableDetailedView.value) {
          _push2(`<button type="button" class="${ssrRenderClass([{ "detailed-list": unref(showDetailedList) }, "toggle-layout-button"])}"${ssrRenderAttr("title", unref(translate)("modal.displayDetails"))} data-v-ce626c7c><span class="vpi-layout-list local-search-icon" data-v-ce626c7c></span></button>`);
        } else {
          _push2(`<!---->`);
        }
        _push2(`<button class="clear-button" type="reset"${ssrIncludeBooleanAttr(disableReset.value) ? " disabled" : ""}${ssrRenderAttr("title", unref(translate)("modal.resetButtonTitle"))} data-v-ce626c7c><span class="vpi-delete local-search-icon" data-v-ce626c7c></span></button></div></form><ul${ssrRenderAttr("id", ((_c = results.value) == null ? void 0 : _c.length) ? "localsearch-list" : void 0)}${ssrRenderAttr("role", ((_d = results.value) == null ? void 0 : _d.length) ? "listbox" : void 0)}${ssrRenderAttr("aria-labelledby", ((_e = results.value) == null ? void 0 : _e.length) ? "localsearch-label" : void 0)} class="results" data-v-ce626c7c><!--[-->`);
        ssrRenderList(results.value, (p, index) => {
          _push2(`<li${ssrRenderAttr("id", "localsearch-item-" + index)}${ssrRenderAttr("aria-selected", selectedIndex.value === index ? "true" : "false")} role="option" data-v-ce626c7c><a${ssrRenderAttr("href", p.id)} class="${ssrRenderClass([{
            selected: selectedIndex.value === index
          }, "result"])}"${ssrRenderAttr("aria-label", [...p.titles, p.title].join(" > "))}${ssrRenderAttr("data-index", index)} data-v-ce626c7c><div data-v-ce626c7c><div class="titles" data-v-ce626c7c><span class="title-icon" data-v-ce626c7c>#</span><!--[-->`);
          ssrRenderList(p.titles, (t, index2) => {
            _push2(`<span class="title" data-v-ce626c7c><span class="text" data-v-ce626c7c>${t ?? ""}</span><span class="vpi-chevron-right local-search-icon" data-v-ce626c7c></span></span>`);
          });
          _push2(`<!--]--><span class="title main" data-v-ce626c7c><span class="text" data-v-ce626c7c>${p.title ?? ""}</span></span></div>`);
          if (unref(showDetailedList)) {
            _push2(`<div class="excerpt-wrapper" data-v-ce626c7c>`);
            if (p.text) {
              _push2(`<div class="excerpt" inert data-v-ce626c7c><div class="vp-doc" data-v-ce626c7c>${p.text ?? ""}</div></div>`);
            } else {
              _push2(`<!---->`);
            }
            _push2(`<div class="excerpt-gradient-bottom" data-v-ce626c7c></div><div class="excerpt-gradient-top" data-v-ce626c7c></div></div>`);
          } else {
            _push2(`<!---->`);
          }
          _push2(`</div></a></li>`);
        });
        _push2(`<!--]-->`);
        if (unref(filterText) && !results.value.length && enableNoResults.value) {
          _push2(`<li class="no-results" data-v-ce626c7c>${ssrInterpolate(unref(translate)("modal.noResultsText"))} &quot;<strong data-v-ce626c7c>${ssrInterpolate(unref(filterText))}</strong>&quot; </li>`);
        } else {
          _push2(`<!---->`);
        }
        _push2(`</ul><div class="search-keyboard-shortcuts" data-v-ce626c7c><span data-v-ce626c7c><kbd${ssrRenderAttr("aria-label", unref(translate)("modal.footer.navigateUpKeyAriaLabel"))} data-v-ce626c7c><span class="vpi-arrow-up navigate-icon" data-v-ce626c7c></span></kbd><kbd${ssrRenderAttr("aria-label", unref(translate)("modal.footer.navigateDownKeyAriaLabel"))} data-v-ce626c7c><span class="vpi-arrow-down navigate-icon" data-v-ce626c7c></span></kbd> ${ssrInterpolate(unref(translate)("modal.footer.navigateText"))}</span><span data-v-ce626c7c><kbd${ssrRenderAttr("aria-label", unref(translate)("modal.footer.selectKeyAriaLabel"))} data-v-ce626c7c><span class="vpi-corner-down-left navigate-icon" data-v-ce626c7c></span></kbd> ${ssrInterpolate(unref(translate)("modal.footer.selectText"))}</span><span data-v-ce626c7c><kbd${ssrRenderAttr("aria-label", unref(translate)("modal.footer.closeKeyAriaLabel"))} data-v-ce626c7c>esc</kbd> ${ssrInterpolate(unref(translate)("modal.footer.closeText"))}</span></div></div></div>`);
      }, "body", false, _parent);
    };
  }
});
const _sfc_setup = _sfc_main.setup;
_sfc_main.setup = (props, ctx) => {
  const ssrContext = useSSRContext();
  (ssrContext.modules || (ssrContext.modules = /* @__PURE__ */ new Set())).add("../node_modules/vitepress/dist/client/theme-default/components/VPLocalSearchBox.vue");
  return _sfc_setup ? _sfc_setup(props, ctx) : void 0;
};
const VPLocalSearchBox = /* @__PURE__ */ _export_sfc(_sfc_main, [["__scopeId", "data-v-ce626c7c"]]);
export {
  VPLocalSearchBox as default
};
