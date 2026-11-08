import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

function load(path) {
  const { outputText } = ts.transpileModule(
    readFileSync(new URL(path, import.meta.url), "utf8"),
    { compilerOptions: { module: ts.ModuleKind.CommonJS } },
  );
  const exports = {};
  runInNewContext(outputText, { exports });
  return exports;
}

const loaded = load("../src/utils/errorDetail.ts");
const { splitErrorResult } = loaded;
const explainErrorDetail = (s) => [...loaded.explainErrorDetail(s)];

test("a bare code has no detail", () => {
  assert.deepEqual({ ...splitErrorResult("ERR_LAUNCH_GAME") }, { code: "ERR_LAUNCH_GAME", detail: "" });
});

test("detail after the code keeps its line breaks", () => {
  const raw = "ERR_LIP_PACKAGE_INSTALL_FAILED: No package found for 'x'\nhint: try another version";
  const got = splitErrorResult(raw);
  assert.equal(got.code, "ERR_LIP_PACKAGE_INSTALL_FAILED");
  assert.equal(got.detail, "No package found for 'x'\nhint: try another version");
});

test("an Error and an error: prefix are unwrapped", () => {
  assert.equal(splitErrorResult(new Error("error: ERR_X: boom")).detail, "boom");
});

test("text without a code is all detail", () => {
  assert.deepEqual({ ...splitErrorResult("something broke") }, { code: "", detail: "something broke" });
  assert.deepEqual({ ...splitErrorResult("") }, { code: "", detail: "" });
});

test("known failures are explained, one cause per group", () => {
  assert.deepEqual(
    explainErrorDetail("could not start Minecraft.Windows.exe: fork/exec: Access is denied."),
    ["access_denied"],
  );
  assert.deepEqual(explainErrorDetail("the game process exited right after starting"), ["game_exited_early"]);
  assert.deepEqual(
    explainErrorDetail("Get \"https://x\": dial tcp: lookup x: no such host"),
    ["dns"],
  );
  assert.deepEqual(
    explainErrorDetail("The process cannot access the file because it is being used by another process"),
    ["file_in_use"],
  );
  assert.deepEqual(explainErrorDetail("unrelated text"), []);
  assert.deepEqual(explainErrorDetail(""), []);
});
