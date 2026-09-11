import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";
import ts from "typescript";

// Execute the real utility with native bindings and toast delivery replaced.
// This tests error handling without opening folders or mounting the application.
const source = readFileSync(new URL("../src/utils/explorer.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});

function setup(invoke) {
  const calls = [];
  const notifications = [];
  const bind = (method) => (value) => {
    calls.push({ method, value });
    return invoke(value);
  };
  const dependencies = {
    "@heroui/react": { toast: { danger: (...args) => notifications.push(args) } },
    "@/i18n": { __esModule: true, default: { t: (key) => key } },
    "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft": { OpenPathDir: bind("directory") },
    "bindings/github.com/liteldev/LeviLauncher/internal/app/modsservice": { OpenModsExplorer: bind("mods") },
  };
  const exports = {};
  runInNewContext(outputText, {
    exports,
    Error,
    require(name) {
      assert.ok(name in dependencies, `unexpected dependency: ${name}`);
      return dependencies[name];
    },
  });
  return { ...exports, calls, notifications };
}

for (const [method, operation, value] of [
  ["directory", "openDirectory", "C:\\中文 space & $literal\\mods"],
  ["mods", "openModsDirectory", "my-instance"],
]) {
  test(`${operation} waits for the native result and forwards its argument`, async () => {
    let resolve;
    const nativeResult = new Promise((done) => { resolve = done; });
    const api = setup(() => nativeResult);
    let settled = false;
    const result = api[operation](value).then((success) => {
      settled = true;
      return success;
    });
    await Promise.resolve();
    assert.equal(settled, false);
    assert.deepEqual(api.calls, [{ method, value }]);
    resolve();
    assert.equal(await result, true);
    assert.equal(api.notifications.length, 0);
  });

  test(`${operation} reports the native failure and resolves to false`, async () => {
    const api = setup(() => Promise.reject(new Error("Access is denied")));
    assert.equal(await api[operation](value), false);
    assert.equal(api.notifications.length, 1);
    assert.equal(api.notifications[0][0], "common.open_folder_failed");
    assert.equal(api.notifications[0][1].description, "Access is denied");
  });
}

test("a synchronous binding failure is handled and a later retry can succeed", async () => {
  let attempts = 0;
  const api = setup(() => {
    if (++attempts === 1) throw new Error("Runtime unavailable");
    return Promise.resolve();
  });
  assert.equal(await api.openDirectory("C:\\mods"), false);
  assert.equal(await api.openDirectory("C:\\mods"), true);
  assert.equal(api.notifications.length, 1);
});

test("path resolution errors retain their text in the shared notification", () => {
  const api = setup(() => Promise.resolve());
  api.showDirectoryOpenError("Versions directory is unavailable");
  assert.equal(api.notifications[0][1].description, "Versions directory is unavailable");
  assert.equal(api.calls.length, 0);
});
