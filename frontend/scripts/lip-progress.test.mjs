import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const source = readFileSync(new URL("../src/utils/lipTaskProgress.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const lip = {};
runInNewContext(outputText, { exports: lip });

const apply = (state, ...events) => events.reduce((acc, event) => lip.applyLipProgressEvent(acc, event), state);

test("bars are keyed by progressId and replace each other", () => {
  const state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "dl-1", stage: "download", current: 10, total: 100, unit: "bytes" },
    { taskId: "t1", progressId: "dl-2", stage: "download", current: 0, total: 0, unit: "bytes", indeterminate: true },
    { taskId: "t1", progressId: "dl-1", stage: "download", current: 50, total: 100, unit: "bytes" });
  const secondary = lip.selectLipSecondaryBars(state);
  assert.equal(secondary.length, 2);
  assert.equal(secondary[0].progressId, "dl-1");
  assert.equal(secondary[0].percentage, 50);
  assert.equal(secondary[1].percentage, undefined, "an unknown total is indeterminate");
});

test("done bars leave the secondary list", () => {
  const state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "x", stage: "extract", current: 5, total: 10, unit: "bytes" },
    { taskId: "t1", progressId: "x", stage: "extract", current: 10, total: 10, unit: "bytes", done: true });
  assert.equal(lip.selectLipSecondaryBars(state).length, 0);
});

test("primary is the task bar, the resolve bar before it", () => {
  let state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "r1", stage: "resolve", current: 1, total: 2, unit: "packages" });
  assert.equal(lip.selectLipPrimaryBar(state).progressId, "r1");
  state = apply(state,
    { taskId: "t1", progressId: "r2", stage: "resolve", current: 4, total: 0, unit: "packages", indeterminate: true });
  assert.equal(lip.selectLipPrimaryBar(state).progressId, "r2");
  assert.equal(lip.selectLipPrimaryBar(state).percentage, undefined);
  state = apply(state, { taskId: "t1", progressId: "task", stage: "task", current: 1, total: 4, unit: "packages", item: "a@1" });
  const primary = lip.selectLipPrimaryBar(state);
  assert.equal(primary.stage, "task");
  assert.equal(primary.percentage, 25);
  assert.equal(primary.item, "a@1");
});

test("a new task replaces the bars of the previous one", () => {
  let state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "task", stage: "task", current: 1, total: 1, unit: "packages", done: true });
  state = lip.beginLipTaskBars(state, "t2");
  assert.equal(lip.selectLipPrimaryBar(state), null);
  state = apply(state,
    { taskId: "t1", progressId: "late", stage: "download", current: 1, total: 2, unit: "bytes" },
    { taskId: "t2", progressId: "task", stage: "task", current: 0, total: 2, unit: "packages" });
  assert.equal(lip.selectLipSecondaryBars(state).length, 0, "updates of an earlier task are ignored");
  assert.equal(lip.selectLipPrimaryBar(state).taskId, "t2");
});

test("legacy updates have an empty stage and values are clamped", () => {
  assert.equal(lip.isLegacyLipProgress({ progressId: "a", percentage: 30 }), true);
  assert.equal(lip.isLegacyLipProgress({ progressId: "a", stage: "download" }), false);
  const state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "p", stage: "place", current: 15, total: 10, unit: "files" });
  assert.equal(lip.selectLipSecondaryBars(state)[0].percentage, 100);
  assert.equal(lip.clampPercentage(140), 100);
  assert.equal(lip.clampPercentage(Number.NaN), 0);
});

test("each package applied is its own step; open bars are the active steps", () => {
  let state = apply(lip.EMPTY_LIP_PROGRESS_BARS,
    { taskId: "t1", progressId: "task", stage: "task", current: 0, total: 2, unit: "packages", item: "a@1" },
    { taskId: "t1", progressId: "dl", stage: "download", current: 1, total: 2, unit: "bytes" });
  const first = lip.lipStepKey(lip.selectLipPrimaryBar(state));
  assert.deepEqual([...lip.activeLipStepKeys(state, null, null).sort()], [first, "bar:t1::dl"].sort());
  state = apply(state,
    { taskId: "t1", progressId: "task", stage: "task", current: 1, total: 2, unit: "packages", item: "b@2" },
    { taskId: "t1", progressId: "dl", stage: "download", current: 2, total: 2, unit: "bytes", done: true });
  const second = lip.lipStepKey(lip.selectLipPrimaryBar(state));
  assert.notEqual(first, second);
  assert.deepEqual([...lip.activeLipStepKeys(state, null, null)], [second]);
  assert.deepEqual(Array.from(lip.activeLipStepKeys(lip.EMPTY_LIP_PROGRESS_BARS, { percentage: 5, message: "x" },
    { completed: 1, current: 2, total: 3, label: "m" })), ["legacy:x", "outer:2:m"]);
});

test("overall percentage counts finished batch items plus the running one", () => {
  const primary = { percentage: 50 };
  assert.equal(lip.overallLipPercentage(primary, null, null), 50);
  assert.equal(lip.overallLipPercentage(null, null, null), undefined);
  assert.equal(lip.overallLipPercentage({ percentage: undefined }, null, null), undefined);
  assert.equal(lip.overallLipPercentage(null, { percentage: 30, message: "" }, null), 30);
  assert.equal(lip.overallLipPercentage(primary, null, { completed: 1, current: 2, total: 4, label: "" }), 37.5);
  assert.equal(lip.overallLipPercentage(primary, null, { completed: 4, current: 4, total: 4, label: "" }), 100);
});
