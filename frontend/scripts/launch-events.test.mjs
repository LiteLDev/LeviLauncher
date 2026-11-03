import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const source = readFileSync(new URL("../src/utils/launchEvents.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const exports = {};
runInNewContext(outputText, { exports });
const { subscribeToLaunchEvents } = exports;

function setup({ active = true, failingUnsubscribe } = {}) {
  const listeners = new Map();
  const removed = [];
  const state = { loading: false, failed: false, errorCode: "" };
  const launchRequestActiveRef = { current: active };
  const unsubscribe = subscribeToLaunchEvents({
    On(name, handler) {
      assert.equal(listeners.has(name), false, `duplicate listener: ${name}`);
      listeners.set(name, handler);
      return () => {
        removed.push(name);
        listeners.delete(name);
        if (name === failingUnsubscribe) throw new Error("Listener removal failed");
      };
    },
  }, {
    launchRequestActiveRef,
    loadingDisclosure: {
      open: () => { state.loading = true; },
      close: () => { state.loading = false; },
    },
    failedDisclosure: { open: () => { state.failed = true; } },
    setLaunchErrorCode: (code) => { state.errorCode = code; },
  });
  return {
    state, launchRequestActiveRef, listeners, removed, unsubscribe,
    emit: (name, event = {}) => listeners.get(name)?.(event),
  };
}

test("a game window closes the wait dialog before launch completion", () => {
  const api = setup();
  api.emit("mc.launch.start");
  assert.equal(api.state.loading, true);

  api.emit("mc.launch.window_detected");
  assert.equal(api.state.loading, false);
  assert.equal(api.launchRequestActiveRef.current, true);
  assert.equal(api.state.failed, false);

  api.emit("mc.launch.done");
  assert.equal(api.state.loading, false);
  assert.equal(api.launchRequestActiveRef.current, false);
  assert.equal(api.state.failed, false);
});

test("activation failure still reports its error after the window appears", () => {
  const api = setup();
  api.emit("mc.launch.start");
  api.emit("mc.launch.window_detected");
  api.emit("mc.launch.failed", { data: ["ERR_UWP_ACTIVATION"] });
  assert.equal(api.state.loading, false);
  assert.equal(api.launchRequestActiveRef.current, false);
  assert.equal(api.state.failed, true);
  assert.equal(api.state.errorCode, "ERR_UWP_ACTIVATION");
});

test("failure without a game window closes the wait dialog and keeps the fallback error", () => {
  const api = setup();
  api.emit("mc.launch.start");
  api.emit("mc.launch.failed", { data: [] });
  assert.equal(api.state.loading, false);
  assert.equal(api.launchRequestActiveRef.current, false);
  assert.equal(api.state.failed, true);
  assert.equal(api.state.errorCode, "ERR_LAUNCH_GAME");
});

test("external launch events do not open a wait dialog without an active request", () => {
  const api = setup({ active: false });
  api.emit("mc.launch.start");
  api.emit("mc.launch.window_detected");
  assert.equal(api.state.loading, false);
  assert.equal(api.launchRequestActiveRef.current, false);
});

test("unmount removes every launch listener, including the early window event", () => {
  const api = setup({ failingUnsubscribe: "mc.launch.start" });
  const subscribedEvents = [...api.listeners.keys()];
  assert.equal(subscribedEvents.length, 4);
  assert.ok(subscribedEvents.includes("mc.launch.window_detected"));
  api.unsubscribe();
  assert.deepEqual(api.removed, subscribedEvents);
  assert.equal(api.listeners.size, 0);

  api.emit("mc.launch.start");
  api.emit("mc.launch.failed", { data: ["ERR_LAUNCH_GAME"] });
  assert.equal(api.state.loading, false);
  assert.equal(api.state.failed, false);
  assert.equal(api.launchRequestActiveRef.current, true);
});
