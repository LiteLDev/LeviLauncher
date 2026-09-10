import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

function loadUtility(path, dependencies) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  });
  const exports = {};
  runInNewContext(outputText, {
    exports,
    require(name) {
      assert.ok(name in dependencies, `unexpected dependency: ${name}`);
      return dependencies[name];
    },
  });
  return exports;
}

function setup(listDir) {
  const filesystem = loadUtility("../src/utils/fs.ts", {
    "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft": { ListDir: listDir },
  });
  const content = loadUtility("../src/utils/content.ts", {
    "./fs": filesystem,
    "./version": {},
    "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice": {},
  });
  return { ...filesystem, ...content };
}

test("player loading propagates native permission errors to the page", async () => {
  const error = new Error("Access is denied");
  const api = setup(async () => { throw error; });
  await assert.rejects(api.listPlayers("C:\\Users\\players", true), (caught) => caught === error);
  assert.equal((await api.listPlayers("C:\\Users\\players")).length, 0);
});

test("retry can recover, while files and the reserved player directory are excluded", async () => {
  let attempts = 0;
  const api = setup(async () => {
    if (++attempts === 1) throw new Error("Directory unavailable");
    return [
      { isDir: true, name: "player-one", path: "C:\\players\\player-one" },
      { isDir: true, name: "9556213259376595538", path: "C:\\players\\reserved" },
      { isDir: false, name: "settings.json", path: "C:\\players\\settings.json" },
    ];
  });
  await assert.rejects(api.listPlayers("C:\\players", true));
  assert.deepEqual(Array.from(await api.listPlayers("C:\\players", true)), ["player-one"]);
});

test("an absent root is an empty state without a native directory request", async () => {
  const api = setup(() => { throw new Error("Unexpected native call"); });
  assert.equal((await api.listPlayers("", true)).length, 0);
});
