import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const modules = {};
function load(path, alias) {
  const { outputText } = ts.transpileModule(
    readFileSync(new URL(path, import.meta.url), "utf8"),
    { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } },
  );
  const exports = {};
  const require = (name) => {
    if (!(name in modules)) throw new Error(`unexpected import ${name}`);
    return modules[name];
  };
  runInNewContext(outputText, { exports, require, Intl, encodeURIComponent });
  if (alias) modules[alias] = exports;
  return exports;
}

load("../src/utils/faqEntries.ts", "@/utils/faqEntries");
const tr = load("../src/utils/troubleshoot.ts", "@/utils/troubleshoot");
const diag = load("../src/utils/diagnostics.ts");
const { FAQ_ENTRIES } = modules["@/utils/faqEntries"];

const locale = (name) =>
  JSON.parse(readFileSync(new URL(`../src/assets/locales/${name}.json`, import.meta.url), "utf8"));
const LOCALES = { en_US: locale("en_US"), zh_CN: locale("zh_CN"), zh_HK: locale("zh_HK") };
const lookup = (json, key) => key.split(".").reduce((o, k) => (o == null ? undefined : o[k]), json);

// A t() over one locale that interpolates {{params}}; "" for missing keys
// when a defaultValue is given, like i18next.
const tFor = (json) => (key, options = {}) => {
  let value = lookup(json, key);
  if (value === undefined && options.count !== undefined) value = lookup(json, `${key}_other`);
  if (typeof value !== "string") return options.defaultValue ?? key;
  return value.replace(/\{\{(\w+)\}\}/g, (_, name) => String(options[name] ?? `{{${name}}}`));
};

test("every guide's texts exist in English, Simplified and Traditional Chinese", () => {
  for (const [name, json] of Object.entries(LOCALES)) {
    for (const guide of Object.values(tr.GUIDES)) {
      for (const cause of guide.causes) {
        assert.equal(typeof lookup(json, `troubleshoot.guide.${guide.id}.cause.${cause}`), "string", `${name}: ${guide.id}.cause.${cause}`);
      }
      for (const step of guide.steps) {
        assert.equal(typeof lookup(json, `troubleshoot.guide.${guide.id}.step.${step.key}`), "string", `${name}: ${guide.id}.step.${step.key}`);
        if (step.action) {
          assert.equal(typeof lookup(json, `troubleshoot.action.${step.action}`), "string", `${name}: action ${step.action}`);
        }
      }
      if (guide.faq) assert.ok(FAQ_ENTRIES[guide.faq], `${guide.id}: unknown FAQ ${guide.faq}`);
    }
  }
});

test("codes map to their guides, other failures by their detail", () => {
  assert.equal(tr.guideFor("ERR_LAUNCH_GAME").id, "launch_game");
  assert.equal(tr.guideFor("err_license_not_entitled").id, "license_not_entitled");
  assert.equal(tr.guideFor("ERR_DEV_MODE_REQUIRED").id, "developer_mode");
  assert.equal(tr.guideFor("", ["access_denied"]).id, "write_access");
  assert.equal(tr.guideFor("", ["timeout"], "lip").id, "lip_install");
  assert.equal(tr.guideFor("", ["dns"]).id, "download_network");
  assert.equal(tr.guideFor("", ["tls"], "license").id, "license_network");
  assert.equal(tr.guideFor("ERR_SOMETHING_NEW", []), undefined);
});

test("license combinations are explained", () => {
  assert.equal(tr.licenseInsight("not_entitled", "authorized"), "preview_only");
  assert.equal(tr.licenseInsight("authorized", "not_entitled"), "release_only");
  assert.equal(tr.licenseInsight("authorized", "trial"), "release_only");
  assert.equal(tr.licenseInsight("trial", "not_entitled"), "trial");
  assert.equal(tr.licenseInsight("not_entitled", "not_entitled"), "none");
  assert.equal(tr.licenseInsight("error", "error"), "error");
  assert.equal(tr.licenseInsight("authorized", "error"), "partial_error");
  assert.equal(tr.licenseInsight("authorized", "authorized"), "");
  assert.equal(tr.licenseInsight("checking", "not_entitled"), "");
  for (const [name, json] of Object.entries(LOCALES)) {
    for (const key of ["preview_only", "release_only", "trial", "none", "error", "partial_error"]) {
      assert.equal(typeof lookup(json, `troubleshoot.license.${key}.body`), "string", `${name}: ${key}`);
    }
  }
});

test("FAQ links follow the reader's language", () => {
  assert.match(tr.faqUrl("launch_game", "en_US"), /^https:\/\/levimc\.org\/faq\/play\/windows\/errors\/#after-installing/);
  assert.match(tr.faqUrl("launch_game", "zh_CN"), /^https:\/\/levimc\.org\/zh-Hans\/faq\/play\/windows\/errors\/#/);
  assert.match(tr.faqUrl("launch_game", "zh_HK"), /^https:\/\/levimc\.org\/zh-Hant\/faq\//);
  assert.equal(tr.faqIndexUrl("de_DE"), "https://levimc.org/faq/");
});

// Results the backend produces (internal/diagnose); every one must read as
// a sentence in each maintained language, never as a key.
const SAMPLES = [
  { id: "vcruntime", status: "ok" },
  { id: "vcruntime", status: "fail", params: { reason: "missing" } },
  { id: "gameinput", status: "skip", params: { reason: "uwp" } },
  { id: "gaming_services", status: "fail", params: { reason: "disabled" } },
  { id: "xbox_services", status: "warn", params: { reason: "disabled", services: "XblAuthManager" } },
  { id: "developer_mode", status: "fail", params: { reason: "off" } },
  { id: "developer_mode", status: "info", params: { reason: "off" } },
  { id: "developer_mode", status: "skip", params: { reason: "gdk" } },
  { id: "webview2", status: "warn", params: { reason: "missing" } },
  { id: "data_root", status: "warn", params: { reason: "non_ascii", path: "D:\\游戏" } },
  { id: "disk_space", status: "fail", params: { reason: "low", free: String(2 * 1024 ** 3), path: "C:\\x" } },
  { id: "appdata_link", status: "warn", params: { reason: "link", path: "C:\\x" } },
  { id: "elevation", status: "warn", params: { reason: "elevated" } },
  { id: "elevation", status: "info", params: { reason: "elevated" } },
  { id: "account", status: "fail", params: { reason: "signed_out" } },
  { id: "license", status: "warn", params: { release: "not_entitled", preview: "authorized", channel: "" } },
  { id: "license", status: "fail", params: { release: "not_entitled", preview: "authorized", channel: "release" } },
  { id: "license", status: "skip", params: { reason: "signed_out" } },
  { id: "proxy", status: "ok", params: { mode: "none" } },
  { id: "proxy", status: "info", params: { mode: "manual", address: "127.0.0.1:7890" } },
  { id: "proxy", status: "fail", params: { mode: "manual", reason: "unreachable", address: "127.0.0.1:7890", local: "true" } },
  { id: "mirror", status: "info", params: { enabled: "true" } },
  { id: "mirror", status: "warn", params: { enabled: "false", reason: "github_unreachable" } },
  { id: "clock", status: "fail", params: { reason: "skew", skew: "-600" } },
  { id: "clock", status: "skip", params: { reason: "no_server" } },
  { id: "ep_xbox_store", status: "ok", params: { latency: "320", http: "200" } },
  { id: "ep_version_db", status: "fail", params: { reason: "dns", latency: "12" } },
  { id: "ep_github", status: "warn", params: { reason: "slow", latency: "5200", http: "200" } },
  { id: "ep_github_proxy", status: "ok", params: { proxy: "https://gh-proxy.org/https://github.com", active: "false" } },
  { id: "ep_github_proxy", status: "fail", params: { proxy: "https://x", active: "true", reason: "timeout" } },
  { id: "ep_ll_db", status: "ok", params: { count: "12" } },
  { id: "ll_support", status: "warn", params: { reason: "unsupported", version: "1.26.99" } },
  { id: "instance_files", status: "fail", params: { reason: "exe_missing", path: "C:\\x\\Minecraft.Windows.exe" } },
  { id: "instance_path", status: "warn", params: { reason: "non_ascii", path: "C:\\实例" } },
  { id: "instance_running", status: "warn", params: { reason: "running" } },
  { id: "registration", status: "ok", params: { state: "this", channel: "release" } },
  { id: "registration", status: "warn", params: { state: "other", channel: "preview", path: "C:\\Program Files\\WindowsApps\\x" } },
  { id: "instance_files", status: "fail", params: { reason: "timeout" } },
  { id: "proxy", status: "fail", params: { mode: "env", reason: "unreachable", address: "127.0.0.1:7890", local: "true" } },
  { id: "account", status: "fail", params: { reason: "unreachable" } },
  { id: "license", status: "skip", params: { reason: "unreachable" } },
];

test("every diagnostics result has a title and a message", () => {
  for (const [name, json] of Object.entries(LOCALES)) {
    const t = tFor(json);
    for (const check of SAMPLES) {
      const params = diag.checkParams(check, t, name);
      const title = t(`diagnostics.check.${check.id}.title`, params);
      assert.ok(!title.startsWith("diagnostics."), `${name}: no title for ${check.id}`);
      const message = diag.checkMessage(check, t, params);
      assert.ok(message && !message.includes("diagnostics.") && !message.includes("{{"), `${name}: ${check.id}/${check.status} → ${message}`);
    }
  }
});

test("the release-unavailable, preview-available case is explained", () => {
  const t = tFor(LOCALES.en_US);
  const check = { id: "license", status: "warn", params: { release: "not_entitled", preview: "authorized", channel: "" } };
  const message = diag.checkMessage(check, t, diag.checkParams(check, t));
  assert.match(message, /Release: Not authorized; Preview: Authorized/);
  assert.match(message, /another Microsoft account/);
  assert.deepEqual([...diag.checkActions(check)], ["open_account", "open_buy", "open_devices"]);
  const previewMissing = { id: "license", status: "warn", params: { release: "authorized", preview: "not_entitled", channel: "" } };
  assert.ok(diag.checkActions(previewMissing).includes("open_preview_store"));
});

test("problems sort first and the report hides paths", () => {
  const sorted = diag.sortChecks([
    { id: "a", status: "ok" },
    { id: "b", status: "warn" },
    { id: "c", status: "fail" },
    { id: "d", status: "info" },
  ]);
  assert.deepEqual(sorted.map((c) => c.id), ["c", "b", "d", "a"]);
  const text = diag.reportText(
    { topic: "launch", instance: "A", durationMs: 10, checks: [{ id: "data_root", status: "warn", params: { reason: "non_ascii", path: "C:\\Users\\name" } }] },
    tFor(LOCALES.en_US),
  );
  assert.ok(!text.includes("C:\\Users\\name"));
  assert.match(text, /\[warn\] data_root reason=non_ascii/);
});

test("an environment proxy is not sent to Windows' proxy page", () => {
  const t = tFor(LOCALES.en_US);
  const check = { id: "proxy", status: "fail", params: { mode: "env", reason: "unreachable", address: "127.0.0.1:7890" } };
  assert.match(diag.checkMessage(check, t, diag.checkParams(check, t)), /environment variables/);
  assert.deepEqual([...diag.checkActions(check)], []);
  const manual = { ...check, params: { ...check.params, mode: "manual" } };
  assert.match(diag.checkMessage(manual, t, diag.checkParams(manual, t)), /set in Windows/);
  assert.deepEqual([...diag.checkActions(manual)], ["open_proxy_settings"]);
});

test("failures a dead proxy causes are folded under it", () => {
  const checks = [
    { id: "proxy", status: "fail", params: { mode: "env", reason: "unreachable" } },
    { id: "ep_github", status: "fail", params: { reason: "proxy" } },
    { id: "account", status: "fail", params: { reason: "unreachable" } },
    { id: "mirror", status: "warn", params: { enabled: "false", reason: "github_unreachable" } },
    { id: "disk_space", status: "warn", params: { reason: "low" } },
    { id: "vcruntime", status: "ok" },
  ];
  const folded = diag.foldConsequences(checks);
  assert.deepEqual(folded.checks.map((c) => c.id), ["proxy", "disk_space", "vcruntime"]);
  assert.deepEqual(folded.causedBy.proxy.map((c) => c.id), ["ep_github", "account", "mirror"]);
  // Without a dead proxy nothing is folded.
  const alone = diag.foldConsequences(checks.slice(1));
  assert.equal(alone.checks.length, 5);
  assert.deepEqual({ ...alone.causedBy }, {});
  for (const [name, json] of Object.entries(LOCALES)) {
    assert.equal(typeof lookup(json, "diagnostics.caused_by_other"), "string", name);
    for (const key of ["title", "body", "banner", "other_account"]) {
      assert.equal(typeof lookup(json, `troubleshoot.license.unreachable.${key}`), "string", `${name}: ${key}`);
    }
  }
});
