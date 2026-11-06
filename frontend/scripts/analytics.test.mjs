import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import { matchPath } from "react-router-dom";
import ts from "typescript";

function loadUtility(path, dependencies = {}, globals = {}) {
  const { outputText } = ts.transpileModule(
    readFileSync(new URL(path, import.meta.url), "utf8"),
    { compilerOptions: { module: ts.ModuleKind.CommonJS } },
  );
  const exports = {};
  runInNewContext(outputText, {
    exports,
    require(name) {
      assert.ok(name in dependencies, `unexpected dependency: ${name}`);
      return dependencies[name];
    },
    ...globals,
  });
  return exports;
}

function setup(saved = {}) {
  const storage = new Map(Object.entries(saved));
  let storageBlocked = false;
  const localStorage = {
    getItem(key) {
      if (storageBlocked) throw new Error("Storage unavailable");
      return storage.get(key) ?? null;
    },
    setItem(key, value) {
      if (storageBlocked) throw new Error("Storage unavailable");
      storage.set(key, value);
    },
  };
  const window = new EventTarget();
  window.location = { origin: "http://wails.localhost" };
  const scripts = [];
  const document = {
    getElementById: (id) => scripts.find((script) => script.id === id),
    createElement: () => ({}),
    head: { appendChild: (script) => scripts.push(script) },
  };
  const globals = { window, document, localStorage, CustomEvent, URL };
  const consent = loadUtility("../src/utils/analyticsConsent.ts", {}, globals);
  const { ROUTES } = loadUtility("../src/constants/routes.ts");
  const analytics = loadUtility("../src/utils/analytics.ts", {
    "react-router-dom": { matchPath },
    "../constants/routes": { ROUTES },
    "./analyticsConsent": consent,
  }, globals);
  let pathname = "/about";
  const trackPageView = analytics.initializeAnalytics(() => pathname);
  return {
    ...consent,
    ...analytics,
    window,
    storage,
    scripts,
    navigate(path) { pathname = path; trackPageView(path); },
    pageViews: () => (window.dataLayer || []).filter((args) => args[0] === "event" && args[1] === "page_view"),
    blockStorage: () => { storageBlocked = true; },
  };
}

test("analytics stays unloaded without a recorded choice", () => {
  for (const saved of [{}, { "ll.clarity.enabled": "true" }]) {
    const app = setup(saved);
    assert.equal(app.readAnalyticsEnabled(), false);
    assert.equal(app.hasAnalyticsChoiceMade(), false);
    app.navigate("/settings");
    assert.equal(app.scripts.length, 0);
    assert.equal(app.window.dataLayer, undefined);
    assert.equal(app.window[`ga-disable-${app.GA_MEASUREMENT_ID}`], true);
  }
});

test("saved opt-ins enable GA4", () => {
  for (const key of ["ll.clarity.choiceMade", "ll.clarity.choiceMade3"]) {
    const app = setup({ "ll.clarity.enabled": "true", [key]: "1" });
    assert.equal(app.hasAnalyticsChoiceMade(), true);
    assert.equal(app.readAnalyticsEnabled(), true);
    assert.equal(app.storage.get(app.ANALYTICS_ENABLED_KEY), "true");
    assert.equal(app.scripts.length, 1);
    assert.equal(app.pageViews().length, 1);
  }
});

test("saved opt-outs stay disabled without another consent prompt", () => {
  const app = setup({ "ll.clarity.enabled": "false", "ll.clarity.choiceMade": "1" });
  assert.equal(app.hasAnalyticsChoiceMade(), true);
  assert.equal(app.readAnalyticsEnabled(), false);
  assert.equal(app.scripts.length, 0);
});

test("updated choices take effect immediately", () => {
  for (const enabled of ["false", "true"]) {
    const app = setup({
      "ll.clarity.choiceMade3": "1",
      "ll.clarity.enabled": enabled,
    });
    assert.equal(app.readAnalyticsEnabled(), enabled === "true");
    assert.equal(app.scripts.length, enabled === "true" ? 1 : 0);
    app.persistAnalyticsChoice(enabled !== "true");
    assert.equal(app.readAnalyticsEnabled(), enabled !== "true");
    app.persistAnalyticsChoice(false);
    app.hasAnalyticsChoiceMade();
    assert.equal(app.readAnalyticsEnabled(), false);
  }
});

test("declining consent is persisted without loading a Google tag", () => {
  const app = setup();
  app.persistAnalyticsChoice(false);
  assert.equal(app.hasAnalyticsChoiceMade(), true);
  assert.equal(app.readAnalyticsEnabled(), false);
  assert.equal(app.scripts.length, 0);
});

test("opting in loads the configured GA4 tag once and disables advertising", () => {
  const app = setup();
  app.persistAnalyticsChoice(true);
  app.persistAnalyticsChoice(true);
  assert.equal(app.readAnalyticsEnabled(), true);
  assert.equal(app.scripts.length, 1);
  assert.equal(app.scripts[0].src, "https://www.googletagmanager.com/gtag/js?id=G-ZHT4LTLTNK");
  assert.equal(app.pageViews().length, 1);
  const defaults = app.window.dataLayer.find((args) => args[0] === "consent" && args[1] === "default")[2];
  for (const key of ["ad_storage", "ad_user_data", "ad_personalization"]) {
    assert.equal(defaults[key], "denied");
  }
  const config = app.window.dataLayer.find((args) => args[0] === "config")[2];
  assert.equal(config.send_page_view, false);
  assert.equal(config.allow_google_signals, false);
  assert.equal(config.allow_ad_personalization_signals, false);
});

test("route visits exclude dynamic identifiers and unknown paths", () => {
  const app = setup({ "ll.clarity.enabled": "true", "ll.clarity.choiceMade": "1" });
  app.navigate("/lip/package/private-package");
  app.navigate("/lip/package/another-private-package");
  app.navigate("/content/worlds/worldEdit");
  app.navigate("/private-folder");
  const views = app.pageViews();
  assert.equal(views.length, 5);
  assert.equal(views[1][2].page_location, "http://wails.localhost/lip/package/:id");
  assert.equal(views[2][2].page_referrer, "http://wails.localhost/lip/package/:id");
  assert.equal(views[3][2].page_location, "http://wails.localhost/content/worlds/worldEdit");
  assert.equal(views[4][2].page_location, "http://wails.localhost/other");
  assert.doesNotMatch(JSON.stringify(app.window.dataLayer), /private-package|private-folder/);
});

test("duplicate router notifications do not inflate page views", () => {
  const app = setup({ "ll.clarity.enabled": "true", "ll.clarity.choiceMade": "1" });
  app.navigate("/about");
  app.navigate("/settings");
  app.navigate("/settings");
  assert.equal(app.pageViews().length, 2);
});

test("withdrawing consent blocks pending and future collection until another opt-in", () => {
  const app = setup();
  app.persistAnalyticsChoice(true);
  app.persistAnalyticsChoice(false);
  // The global opt-out flag also applies if the asynchronous tag is still loading.
  assert.equal(app.window[`ga-disable-${app.GA_MEASUREMENT_ID}`], true);
  app.navigate("/settings");
  assert.equal(app.pageViews().length, 1);
  app.persistAnalyticsChoice(true);
  assert.equal(app.window[`ga-disable-${app.GA_MEASUREMENT_ID}`], false);
  assert.equal(app.scripts.length, 1);
  assert.equal(app.pageViews().length, 2);
  assert.equal(app.pageViews()[1][2].page_referrer, "");
});

test("withdrawal takes effect even when saving the preference fails", () => {
  const app = setup();
  app.persistAnalyticsChoice(true);
  app.blockStorage();
  app.persistAnalyticsChoice(false);
  assert.equal(app.readAnalyticsEnabled(), false);
  assert.equal(app.window[`ga-disable-${app.GA_MEASUREMENT_ID}`], true);
  app.navigate("/settings");
  assert.equal(app.pageViews().length, 1);
});
