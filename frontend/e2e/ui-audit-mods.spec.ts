import { expect, test, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const project = {
  id: 42, name: "Audit Project", authors: [], categories: [], latestFiles: [],
  links: { websiteUrl: "https://www.curseforge.com/minecraft-bedrock/addons/audit-project" },
};

const projectResponses = {
  "minecraft.GetCurseForgeModsByIDs": { data: [project] },
  "minecraft.GetCurseForgeModDescription": { data: "" },
  "minecraft.GetCurseForgeModFiles": { data: [] },
};

const methodID = async (method: string) => {
  const source = await readFile(resolve("bindings/github.com/liteldev/LeviLauncher/minecraft.js"), "utf8");
  const match = source.match(new RegExp(`export function ${method}\\([^)]*\\) \\{[\\s\\S]*?\\$Call\\.ByID\\((\\d+)`));
  if (!match) throw new Error(`Missing binding: ${method}`);
  return Number(match[1]);
};

const failMethodUntil = async (page: Page, method: string, shouldFail: () => boolean) => {
  const id = await methodID(method);
  await page.route("**/wails/runtime", async (route) => {
    if (Number(route.request().postDataJSON()?.args?.methodID) === id && shouldFail()) {
      await route.fulfill({ status: 500, contentType: "text/plain", body: "Network unavailable" });
    } else {
      await route.fallback();
    }
  });
};

test.beforeEach(async ({ page }) => {
  await seedCompletedSetup(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
});

test("CurseForge metadata failure has a working retry and empty description completes", async ({ page }) => {
  await mockWailsRuntime(page, projectResponses);
  let failing = true;
  await failMethodUntil(page, "GetCurseForgeModsByIDs", () => failing);
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await expect(page.getByRole("alert")).toContainText("Could not load this project");
  await expect(page.getByText("Mod not found", { exact: true })).toHaveCount(0);
  failing = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("heading", { name: project.name, exact: true })).toBeVisible();
  await expect(page.getByText("No description available.", { exact: true })).toBeVisible();
  await expect(page.getByText("Loading description...")).toHaveCount(0);
});

test("CurseForge description and files fail independently and recover in place", async ({ page }) => {
  await mockWailsRuntime(page, projectResponses);
  let failing = true;
  await failMethodUntil(page, "GetCurseForgeModDescription", () => failing);
  await failMethodUntil(page, "GetCurseForgeModFiles", () => failing);
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await expect(page.getByRole("heading", { name: project.name, exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Could not load the description");
  failing = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByText("No description available.", { exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Files", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Could not load the files");
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("project sharing copies its public URL and reports clipboard failure", async ({ page }) => {
  await mockWailsRuntime(page, projectResponses);
  const copied: string[] = [];
  let failCopy = false;
  await page.route("**/wails/runtime", async (route) => {
    const request = route.request().postDataJSON();
    if (request.object === 1 && request.method === 0) {
      copied.push(request.args.text);
      await route.fulfill({ status: failCopy ? 500 : 200, contentType: "text/plain", body: failCopy ? "Clipboard unavailable" : "" });
    } else await route.fallback();
  });
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await page.getByRole("button", { name: "Copy project link", exact: true }).click();
  await expect(page.getByText("Project link copied", { exact: true })).toBeVisible();
  expect(copied).toEqual([project.links.websiteUrl]);
  failCopy = true;
  await page.getByRole("button", { name: "Copy project link", exact: true }).click();
  await expect(page.getByText("Could not copy the link. Please try again.", { exact: true })).toBeVisible();
});

test("file install icons include their version and missing project URLs hide sharing", async ({ page }) => {
  await mockWailsRuntime(page, {
    ...projectResponses,
    "minecraft.GetCurseForgeModsByIDs": { data: [{ ...project, links: {} }] },
    "minecraft.GetCurseForgeModFiles": { data: [{
      id: 420, displayName: "Audit 1.2.3", fileName: "audit-1.2.3.mcpack",
      gameVersions: ["1.21.0"], releaseType: 1, fileLength: 1024, downloadCount: 5,
    }] },
  });
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await expect(page.getByRole("heading", { name: project.name, exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Copy project link" })).toHaveCount(0);
  await page.getByRole("tab", { name: "Files", exact: true }).click();
  await expect(page.getByRole("button", { name: "Install Audit 1.2.3", exact: true })).toBeVisible();
});

test("CurseForge result cards open details with the keyboard", async ({ page }) => {
  await mockWailsRuntime(page, {
    ...projectResponses,
    "minecraft.SearchCurseForgeMods": { data: [project], pagination: { totalCount: 1 } },
    "minecraft.GetCurseForgeCategories": { data: [] },
    "minecraft.GetCurseForgeGameVersions": { data: [] },
  });
  await page.goto(`/#${ROUTES.curseForge}`);
  const card = page.getByRole("link", { name: "View details for Audit Project" });
  await expect(card).toBeVisible();
  await card.focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(new RegExp(`#${routeTo.curseForgeMod(42)}$`));
});

test("LIP catalog failure is distinct from no results and its retry restores keyboard links", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.GetLipStatus": { installed: true, upToDate: true, currentVersion: "1.0.0" },
  });
  let failing = true;
  await page.route("https://lipr.levimc.org/levilauncher.json", (route) => route.fulfill({
    status: failing ? 500 : 200,
    contentType: "application/json",
    body: JSON.stringify({ packages: { "github.com/example/audit": { info: { name: "Audit Package" }, variants: { client: { versions: { "1.0.0": {} } } } } } }),
  }));
  await page.goto(`/#${ROUTES.lip}`);
  await expect(page.getByText("The package catalog could not be loaded. Please retry the connection.")).toBeVisible();
  await expect(page.getByText("No results", { exact: true })).toHaveCount(0);
  failing = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  const card = page.getByRole("link", { name: "View details for Audit Package" });
  await expect(card).toBeVisible();
  await card.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { name: "Audit Package", exact: true })).toBeVisible();
  const copied = page.waitForRequest((request) => {
    if (!request.url().endsWith("/wails/runtime")) return false;
    const body = request.postDataJSON();
    return body.object === 1 && body.method === 0;
  });
  await page.getByRole("button", { name: "Copy project link", exact: true }).click();
  expect((await copied).postDataJSON().args.text).toBe("https://github.com/example/audit");
});

test("path failures remain visible and a successful reset closes the confirmation", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.GetBaseRoot": "D:\\Games",
    "minecraft.CanWriteToDir": true,
    "minecraft.SetBaseRoot": "ERR_WRITE_FILE",
    "minecraft.ResetBaseRoot": "ERR_CREATE_TARGET_DIR",
  });
  await page.goto(`/#${ROUTES.settings}`);
  await page.getByRole("textbox", { name: "Base Root", exact: true }).fill("D:\\OtherGames");
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.getByRole("button", { name: "Reset", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Restore default path?" });
  await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("alert")).toBeVisible();
  const resetID = await methodID("ResetBaseRoot");
  await page.route("**/wails/runtime", async (route) => {
    if (Number(route.request().postDataJSON()?.args?.methodID) === resetID) {
      await route.fulfill({ status: 200, contentType: "application/json", body: '""' });
    } else await route.fallback();
  });
  await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText("Default storage path restored", { exact: true })).toBeVisible();
});

test("theme swatches expose a name, pressed state and keyboard activation", async ({ page }) => {
  await mockWailsRuntime(page);
  await page.goto(`/#${ROUTES.settings}`);
  await page.getByRole("tab", { name: "Personalization", exact: true }).click();
  const blue = page.getByRole("button", { name: "Blue (#3b82f6)", exact: true });
  await expect(blue).toBeVisible();
  await blue.focus();
  await page.keyboard.press("Space");
  await expect(blue).toHaveAttribute("aria-pressed", "true");
  const custom = page.getByRole("button", { name: "Custom theme color", exact: true });
  await custom.focus();
  await page.keyboard.press("Enter");
  await expect(custom).toHaveAttribute("aria-pressed", "true");
});

test("custom colors use solid previews and validate HEX and RGB fields", async ({ page }) => {
  await mockWailsRuntime(page);
  await page.goto(`/#${ROUTES.settings}`);
  await page.getByRole("tab", { name: "Personalization", exact: true }).click();
  await page.getByRole("button", { name: "Custom theme color", exact: true }).click();
  const hex = page.getByRole("textbox", { name: "HEX", exact: true }).first();
  await hex.fill("#12AB34");
  const preview = page.getByRole("img", { name: "Color preview: #12AB34", exact: true });
  await expect(preview).toBeVisible();
  await expect(preview).toHaveCSS("background-image", "none");
  await hex.fill("#BAD");
  await expect(page.getByText("Enter six hexadecimal digits, for example #10B981.")).toBeVisible();
  await expect(preview).toBeVisible();
  await hex.fill("#12AB34");
  const red = page.getByRole("textbox", { name: "Red (R)", exact: true }).first();
  await red.fill("256");
  await expect(page.getByText("Enter a whole number from 0 to 255.")).toBeVisible();
  await expect(preview).toBeVisible();
  await red.fill("255");
  await expect(hex).toHaveValue("#FFAB34");
  await expect(page.getByRole("img", { name: "Color preview: #FFAB34", exact: true })).toHaveCSS("background-color", "rgb(255, 171, 52)");
  const gradients = await page.locator("main *").evaluateAll((elements) =>
    elements.filter((element) => [null, "::before", "::after"].some((pseudo) =>
      getComputedStyle(element, pseudo).backgroundImage.includes("gradient"),
    )).map((element) => element.tagName),
  );
  expect(gradients).toEqual([]);
});

test("a different path can be saved after an unwritable directory is rejected", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.GetBaseRoot": "D:\\Games",
    "minecraft.CanWriteToDir": false,
    "minecraft.SetBaseRoot": "",
  });
  await page.goto(`/#${ROUTES.settings}`);
  const path = page.getByRole("textbox", { name: "Base Root", exact: true });
  const apply = page.getByRole("button", { name: "Apply", exact: true });
  await path.fill("D:\\ReadOnly");
  await apply.click();
  await expect(page.getByText("Directory not writable", { exact: true })).toBeVisible();
  await expect(apply).toBeDisabled();
  const writableID = await methodID("CanWriteToDir");
  await page.route("**/wails/runtime", async (route) => {
    if (Number(route.request().postDataJSON()?.args?.methodID) === writableID) {
      await route.fulfill({ status: 200, contentType: "application/json", body: "true" });
    } else await route.fallback();
  });
  await path.fill("D:\\Writable");
  await expect(apply).toBeEnabled();
  await apply.click();
  await expect(page.getByText("Storage path saved", { exact: true })).toBeVisible();
});

test("local mod filters recover and the DLL import explains its required name", async ({ page }) => {
  const instance = { name: "Audit Instance", version: "1.21.0", type: "release", installed: true };
  await mockWailsRuntime(page, {
    "versionservice.ListVersionMetas": [instance],
    "versionservice.ListVersionMetasWithRegistered": [instance],
    "versionservice.GetVersionMeta": instance,
    "modsservice.GetMods": [{ name: "Local Audit Mod", folder: "audit", version: "1.0.0", type: "preload-native" }],
    "modsservice.IsModEnabled": true,
  });
  await page.route("https://lipr.levimc.org/levilauncher.json", (route) => route.fulfill({ contentType: "application/json", body: '{"packages":{}}' }));
  await page.route("**/wails/runtime", async (route) => {
    if (route.request().postDataJSON().object === 5) {
      await route.fulfill({ contentType: "application/json", body: JSON.stringify(["D:\\audit.dll"]) });
    } else await route.fallback();
  });
  await page.addInitScript(() => localStorage.setItem("ll.currentVersionName", "Audit Instance"));
  await page.goto(`/#${ROUTES.mods}`);
  await expect(page.getByText("Local Audit Mod", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "More actions for Local Audit Mod" }).click();
  await expect(page.getByRole("menu")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByPlaceholder("Search...").fill("unmatched-audit-query");
  await expect(page.getByText("No installed mods match the current search or filters.")).toBeVisible();
  await page.getByRole("button", { name: "Clear filters", exact: true }).click();
  await expect(page.getByText("Local Audit Mod", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Import .zip/.dll", exact: true }).click();
  const name = page.getByRole("textbox", { name: /^Plugin Name/ });
  await name.fill("");
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Enter a mod name before importing.")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  await name.fill("Audit DLL");
  await expect(dialog.getByRole("button", { name: "Confirm", exact: true })).toBeEnabled();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
});

test("failed updates state the window-closing action and recovery steps", async ({ page }) => {
  await mockWailsRuntime(page, { "minecraft.Update": false });
  await page.goto(`/#${ROUTES.updating}`);
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Close update window", exact: true })).toBeVisible();
  await expect(dialog).toContainText("reopen LeviLauncher and check for updates again");
  await expect(dialog.getByRole("button", { name: "Confirm", exact: true })).toHaveCount(0);
});

test("settings save-and-leave preserves the forward history step", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.GetBaseRoot": "D:\\Games",
    "minecraft.CanWriteToDir": true,
    "minecraft.SetBaseRoot": "",
  });
  await page.goto(`/#${ROUTES.settings}`);
  await page.getByRole("button", { name: "About", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#${ROUTES.about}$`));
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#${ROUTES.settings}$`));
  await page.getByRole("textbox", { name: "Base Root", exact: true }).fill("D:\\NewPath");
  await page.getByRole("button", { name: "Forward", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Unsaved Changes" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Save and Leave", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#${ROUTES.about}$`));
  await expect(page.getByRole("button", { name: "Forward", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#${ROUTES.settings}$`));
});
