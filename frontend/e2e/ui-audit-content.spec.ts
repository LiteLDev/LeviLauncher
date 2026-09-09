import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const roots = { base: "C:\\Game", usersRoot: "C:\\Users", resourcePacks: "C:\\Packs", behaviorPacks: "C:\\Behavior", isIsolation: true, isPreview: false };
const pixel = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a/dkAAAAASUVORK5CYII=";

async function setup(page: Page, responses: Record<string, unknown> = {}) {
  await mockWailsRuntime(page, {
    "contentservice.GetContentRoots": roots,
    "userservice.GetUserGamertagMap": { A: "Alice", B: "Bob" },
    "minecraft.GetWorldIconDataUrl": pixel,
    "minecraft.GetPathSize": 100,
    "minecraft.GetPathModTime": 1,
    "minecraft.ReadWorldLevelDatFields": { version: 10, fields: [{ name: "Example", tag: "string", valueString: "original" }], order: ["Example"] },
    "minecraft.GetWorldLevelName": "Saved World",
    ...responses,
  });
  await seedCompletedSetup(page);
  await page.addInitScript(() => localStorage.setItem("ll.currentVersionName", "Audit"));
}

async function override(page: Page, service: string, method: string, handler: (args: unknown[]) => unknown | Promise<unknown>) {
  const source = await readFile(`bindings/github.com/liteldev/LeviLauncher/${service}.js`, "utf8");
  const match = source.match(new RegExp(`export function ${method}\\([^]*?\\$Call\\.ByID\\((\\d+)`));
  if (!match) throw new Error(`Missing binding ${service}.${method}`);
  await page.route("**/wails/runtime", async (route) => {
    const data = route.request().postDataJSON();
    if (Number(data?.args?.methodID) !== Number(match[1])) return route.fallback();
    const result = await handler(data.args.args || []);
    if (result instanceof Error) {
      await route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ message: result.message }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(result) });
  });
}

async function navigateWithState(page: Page, path: string, state: Record<string, unknown>) {
  await page.goto(`/#${ROUTES.home}`);
  await expect(page.getByRole("heading", { name: "Minecraft", exact: true })).toBeVisible();
  await page.evaluate(({ path, state }) => {
    localStorage.setItem("ll.currentVersionName", "Audit");
    history.pushState({ ...history.state, usr: state, key: "content-test", idx: (history.state?.idx || 0) + 1 }, "", `/#${path}`);
    window.dispatchEvent(new PopStateEvent("popstate", { state: history.state }));
  }, { path, state });
}

test("world editor guards refresh and native history, and saves focused drafts", async ({ page }) => {
  await setup(page);
  const writes: unknown[][] = [];
  await override(page, "minecraft", "WriteWorldLevelDatFields", (args) => { writes.push(args); return ""; });
  await navigateWithState(page, routeTo.contentWorldEditor("C:\\World"), {});
  await page.getByRole("textbox", { name: "Level Name", exact: true }).fill("Changed World");
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Unsaved changes" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Continue editing" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("textbox", { name: "Level Name", exact: true })).toHaveValue("Changed World");
  await expect(page.getByText("Unsaved changes", { exact: true })).toBeVisible();
  await page.evaluate(() => history.back());
  await expect(dialog).toBeVisible();
  await expect(page).toHaveURL(/worldEdit/);
  await dialog.getByRole("button", { name: "Continue editing" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("textbox", { name: "Level Name", exact: true })).toHaveValue("Changed World");
  await page.getByRole("textbox", { name: "Example", exact: true }).fill("typed draft");
  await expect(page.getByRole("textbox", { name: "Example", exact: true })).toHaveValue("typed draft");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByText("Unsaved changes", { exact: true })).toHaveCount(0);
  expect(writes.at(-1)?.[1]).toMatchObject({ levelName: "Changed World", fields: [{ name: "Example", valueString: "typed draft" }] });
  await page.getByRole("textbox", { name: "Level Name", exact: true }).fill("Discard me");
  await page.evaluate(() => history.back());
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Discard changes" }).click();
  await expect(page).not.toHaveURL(/worldEdit/);
});

test("world player switches discard old selection and partial string errors retain failed items", async ({ page }) => {
  await setup(page);
  let releaseOld: (() => void) | undefined;
  let delayAlice = false;
  let failedOnce = true;
  let releaseGamertag: () => void = () => {};
  const gamertagGate = new Promise<void>((resolve) => { releaseGamertag = resolve; });
  await override(page, "userservice", "GetLocalUserGamertag", async () => { await gamertagGate; return "Alice"; });
  const deleted: string[] = [];
  const removed = new Set<string>();
  await override(page, "minecraft", "ListDir", async ([path]) => {
    const value = String(path);
    if (value === roots.usersRoot) return ["A", "B"].map((name) => ({ name, path: `C:\\Users\\${name}`, isDir: true }));
    if (value.includes("\\A\\") && delayAlice) await new Promise<void>((resolve) => { releaseOld = resolve; });
    const names = value.includes("\\B\\") ? ["B-one", "B-two"] : ["A-one"];
    return names.filter((name) => !removed.has(name)).map((name) => ({ name, path: `${value}\\${name}`, isDir: true }));
  });
  await override(page, "minecraft", "GetWorldLevelName", ([path]) => String(path).split("\\").at(-1));
  await override(page, "contentservice", "DeleteWorld", ([, path]) => {
    const name = String(path).split("\\").at(-1)!;
    deleted.push(name);
    if (name === "B-two" && failedOnce) { failedOnce = false; return "ERR_PERMISSION_DENIED"; }
    removed.add(name);
    return "";
  });
  await navigateWithState(page, ROUTES.contentWorlds, { versionName: "Audit" });
  await expect(page.getByRole("heading", { name: "A-one" })).toBeVisible();
  await page.getByRole("button", { name: "Select mode", exact: true }).click();
  await page.getByRole("checkbox", { name: "Select A-one", exact: true }).press("Space");
  delayAlice = true;
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect.poll(() => Boolean(releaseOld)).toBe(true);
  await page.getByRole("button", { name: "Alice", exact: true }).click();
  await page.getByRole("menuitemradio", { name: "Bob", exact: true }).click();
  await expect(page.getByRole("heading", { name: "B-one" })).toBeVisible();
  releaseOld!();
  releaseGamertag();
  await expect(page.getByRole("heading", { name: "A-one" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Bob", exact: true })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Select all" })).toHaveCount(0);
  await page.getByRole("button", { name: "Select mode", exact: true }).click();
  await page.getByRole("checkbox", { name: "Select all", exact: true }).press("Space");
  await page.getByRole("button", { name: "Delete", exact: true }).first().click();
  const dialog = page.getByRole("dialog", { name: "Confirm Delete" });
  await expect(dialog).toContainText("B-one");
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("Deleted 1; failed 1");
  await expect(dialog).toContainText("ERR_PERMISSION_DENIED");
  expect(deleted).toEqual(["B-one", "B-two"]);
  await expect(page.getByRole("checkbox", { name: "Select B-two", exact: true })).toBeChecked();
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(deleted).toEqual(["B-one", "B-two", "B-two"]);
});

test("screenshots show metadata before image completion and recover a failed image", async ({ page }) => {
  await setup(page, { "contentservice.ListScreenshots": [{ name: "one.png", path: "C:\\one.png", captureTime: 1 }, { name: "two.png", path: "C:\\two.png", captureTime: 2 }] });
  let releaseImages: (() => void) | undefined;
  const gate = new Promise<void>((resolve) => { releaseImages = resolve; });
  let failed = false;
  await override(page, "minecraft", "GetImageURL", async ([path]) => {
    await gate;
    if (String(path).endsWith("two.png") && !failed) { failed = true; return ""; }
    return pixel;
  });
  await navigateWithState(page, ROUTES.contentScreenshots, { player: "A" });
  await expect(page.getByRole("status", { name: "Loading", exact: true })).toHaveCount(2);
  releaseImages!();
  await expect(page.getByRole("img", { name: "one.png", exact: true })).toBeVisible();
  await expect(page.getByText("Unable to read two.png", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("img", { name: "two.png", exact: true })).toBeVisible();
  await page.screenshot({ path: ".artifacts/content-screenshots-recovered.png", fullPage: true });
});

test("screenshot deep link explains missing player", async ({ page }) => {
  await setup(page);
  await page.goto(`/#${ROUTES.contentScreenshots}`);
  await expect(page.getByText("Choose a player in Content before viewing screenshots.")).toBeVisible();
  await expect(page.getByText("No screenshots", { exact: true })).toHaveCount(0);
});

test("deleting the final item on the last page returns to a populated page", async ({ page }) => {
  await setup(page);
  let removedLast = false;
  await override(page, "minecraft", "ListDir", ([path]) => {
    if (path === roots.usersRoot) return [{ name: "A", path: "C:\\Users\\A", isDir: true }];
    return Array.from({ length: removedLast ? 20 : 21 }, (_, index) => {
      const name = `World-${String(index + 1).padStart(2, "0")}`;
      return { name, path: `${path}\\${name}`, isDir: true };
    });
  });
  await override(page, "minecraft", "GetWorldLevelName", ([path]) => String(path).split("\\").at(-1));
  await override(page, "contentservice", "DeleteWorld", () => { removedLast = true; return ""; });
  await navigateWithState(page, ROUTES.contentWorlds, { player: "A", versionName: "Audit" });
  await page.getByRole("button", { name: "Page 2", exact: true }).click();
  await expect(page.getByRole("heading", { name: "World-21", exact: true })).toBeVisible();
  const removeButton = page.getByRole("button", { name: "Delete", exact: true });
  await removeButton.focus();
  await removeButton.press("Enter");
  await page.getByRole("dialog", { name: "Confirm Delete" }).getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.getByRole("heading", { name: "World-01", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "World-21", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Page 2", exact: true })).toHaveCount(0);
});

test("screenshot list errors offer retry instead of an empty-library message", async ({ page }) => {
  await setup(page);
  let fail = true;
  await override(page, "contentservice", "ListScreenshots", () => fail ? new Error("mock-list-failed") : []);
  await navigateWithState(page, ROUTES.contentScreenshots, { player: "A" });
  await expect(page.getByRole("alert")).toContainText("Unable to load screenshots");
  await expect(page.getByText("No screenshots", { exact: true })).toHaveCount(0);
  fail = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByText("No screenshots", { exact: true })).toBeVisible();
});

test("an open screenshot preview updates when the pending image finishes loading", async ({ page }) => {
  await setup(page, { "contentservice.ListScreenshots": Array.from({ length: 5 }, (_, index) => ({
    name: `shot-${index + 1}.png`, path: `C:\\shot-${index + 1}.png`, captureTime: index + 1,
  })) });
  let releaseLast: () => void = () => {};
  const lastImageGate = new Promise<void>((resolve) => { releaseLast = resolve; });
  await override(page, "minecraft", "GetImageURL", async ([path]) => {
    if (String(path).endsWith("shot-1.png")) await lastImageGate;
    return pixel;
  });
  await navigateWithState(page, ROUTES.contentScreenshots, { player: "A" });
  await expect(page.getByRole("img", { name: "shot-2.png", exact: true })).toBeVisible();
  const open = page.getByRole("button", { name: "View larger", exact: true }).nth(3);
  await open.focus();
  await open.press("Enter");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Next screenshot", exact: true }).click();
  await expect(dialog.getByRole("heading", { name: "shot-1.png", exact: true })).toBeVisible();
  await expect(dialog.getByText("Loading", { exact: true })).toBeVisible();
  releaseLast();
  await expect(dialog.getByRole("img", { name: "shot-1.png", exact: true })).toBeVisible();
  await expect(dialog.getByRole("status")).toHaveCount(0);
});
