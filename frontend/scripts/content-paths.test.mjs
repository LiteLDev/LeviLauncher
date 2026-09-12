import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";
import ts from "typescript";

const source = readFileSync(new URL("../src/utils/content.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const exports = {};
runInNewContext(outputText, {
  exports,
  require(name) {
    assert.ok(["./fs", "./version", "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice"].includes(name));
    return {};
  },
});

test("UWP content uses direct folders without a player, including stale GDK selections", () => {
  const root = "C:\\Users\\玩家\\AppData\\Local\\Packages\\Microsoft.MinecraftUWP_8wekyb3d8bbwe\\LocalState\\games\\com.mojang";
  const roots = { packageType: "uwp", comMojangRoot: root, worlds: `${root}\\minecraftWorlds`, skinPacks: `${root}\\skin_packs`, screenshots: `${root}\\Screenshots`, usersRoot: "" };
  for (const kind of ["minecraftWorlds", "skin_packs", "Screenshots", "minecraftpe"]) {
    for (const player of ["", "123456789"]) {
      assert.equal(exports.resolveContentPath(roots, kind, player), `${root}\\${kind}`);
    }
  }
});

test("GDK still requires a valid single player folder", () => {
  const roots = { usersRoot: "D:\\Minecraft Bedrock\\Users" };
  assert.equal(exports.resolveContentPath(roots, "minecraftWorlds", "123"), "D:\\Minecraft Bedrock\\Users\\123\\games\\com.mojang\\minecraftWorlds");
  for (const player of ["", ".", "..", "../other", "..\\other", "C:\\other"]) {
    assert.equal(exports.resolveContentPath(roots, "minecraftWorlds", player), "");
  }
});

test("unresolved roots never manufacture relative content paths", () => {
  assert.equal(exports.resolveContentPath(exports.EMPTY_CONTENT_ROOTS, "minecraftWorlds", "123"), "");
  assert.equal(exports.resolveContentPath({ packageType: "uwp" }, "Screenshots"), "");
});
