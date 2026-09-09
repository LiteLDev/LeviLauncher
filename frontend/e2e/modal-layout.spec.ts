import { expect, test, type Locator, type Page } from "@playwright/test";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";
import zh from "../src/assets/locales/zh_CN.json" with { type: "json" };
import { readFile } from "node:fs/promises";

const instance = { name: "1.26.40.05", gameVersion: "1.26.40.05", enableIsolation: true };

async function setup(page: Page, theme = "light") {
  await mockWailsRuntime(page, {
    "versionservice.ListVersionMetas": [instance],
    "versionservice.ListVersionMetasWithRegistered": [instance],
    "versionservice.GetVersionMeta": instance,
    "versionservice.CreateDesktopShortcut": "",
    "minecraft.GetLipStatus": { installed: true, upToDate: true },
  });
  await seedCompletedSetup(page);
  await page.addInitScript(({ theme, name }) => {
    localStorage.setItem("theme", theme);
    localStorage.setItem("app.themeMode", theme);
    localStorage.setItem("i18nextLng", "zh_CN");
    localStorage.setItem("ll.currentVersionName", name);
  }, { theme, name: instance.name });
}

async function expectFits(dialog: Locator, width?: number) {
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('[data-slot="modal-close-trigger"], .modal__close-trigger')).toHaveCount(0);
  if (width) await expect(dialog).toHaveCSS("width", `${width}px`);
  await expect.poll(() => dialog.evaluate((el) => {
    const bounds = el.getBoundingClientRect();
    return bounds.left >= 0 && bounds.right <= innerWidth + 1 &&
      bounds.top >= 0 && bounds.bottom <= innerHeight + 1 &&
      el.scrollWidth <= el.clientWidth + 1;
  })).toBe(true);
  await expect(dialog.locator(".modal__footer")).toBeInViewport({ ratio: 1 });
  for (const button of await dialog.locator(".modal__footer button").all()) {
    await expect(button).toHaveCSS("border-radius", "12px");
    await expect(button).toHaveCSS("font-size", "14px");
  }
  const animatedBody = dialog.locator(".modal__body > div");
  if (await animatedBody.count()) await expect(animatedBody.first()).toHaveCSS("opacity", "1");
}

for (const theme of ["light", "dark"]) {
  test(`success and delete confirmations use wide desktop proportions in ${theme}`, async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await setup(page, theme);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/#" + ROUTES.home);
    await page.getByRole("button", { name: zh.launcherpage.tip.quick_actions_menu, exact: true }).click();
    await page.getByRole("menuitem", { name: zh.launcherpage.shortcut.create_button, exact: true }).click();
    const success = page.getByRole("dialog", { name: zh.launcherpage.shortcut.success.title });
    await expectFits(success, 680);
    expect((await success.boundingBox())!.height).toBeLessThan(320);
    await page.screenshot({ path: `.artifacts/modals/success-${theme}.png` });
    await success.getByRole("button", { name: zh.common.close, exact: true }).click();
    await expect(success).toHaveCount(0);

    await page.goto("/#" + ROUTES.instances);
    await page.getByRole("button", { name: zh.audit.primary.instances.settings.replace("{{name}}", instance.name), exact: true }).click();
    await page.getByRole("tab", { name: zh.versions.edit.tabs.manage, exact: true }).click();
    await page.getByRole("button", { name: zh.common.delete, exact: true }).click();
    const deletion = page.getByRole("dialog", { name: zh.launcherpage.delete.confirm.title });
    await expectFits(deletion, 680);
    expect((await deletion.boundingBox())!.height).toBeLessThan(360);
    await expect(deletion).toContainText(instance.name);
    await page.screenshot({ path: `.artifacts/modals/delete-${theme}.png` });
    for (const viewport of [{ width: 800, height: 600 }, { width: 320, height: 568 }]) {
      await page.setViewportSize(viewport);
      await expectFits(deletion);
    }
    await page.keyboard.press("Escape");
    await expect(deletion).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test("long terms keep their heading and actions inside a short window", async ({ page }) => {
  await setup(page);
  await page.addInitScript(() => localStorage.removeItem("ll.termsAccepted"));
  await page.setViewportSize({ width: 1440, height: 600 });
  await page.goto("/#" + ROUTES.home);
  const dialog = page.getByRole("dialog", { name: zh.terms.title });
  await expectFits(dialog, 880);
  await expect(dialog.getByRole("heading")).toBeInViewport({ ratio: 1 });
  await page.keyboard.press("Escape");
  await expect(dialog).toBeVisible();
  await page.mouse.click(5, 300);
  await expect(dialog).toBeVisible();
  await page.screenshot({ path: ".artifacts/modals/terms-short-window.png" });
});

test("pending confirmation blocks dismissal and recovers after an error", async ({ page }) => {
  await setup(page);
  const source = await readFile("bindings/github.com/liteldev/LeviLauncher/minecraft.js", "utf8");
  const declaration = source.slice(source.indexOf("export function ResetBaseRoot("));
  const methodID = Number(declaration.match(/\$Call\.ByID\((\d+)/)?.[1]);
  expect(methodID).toBeGreaterThan(0);
  let release: () => void = () => {};
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route("**/wails/runtime", async route => {
    if (Number(route.request().postDataJSON()?.args?.methodID) !== methodID) return route.fallback();
    await gate;
    await route.fulfill({ json: "ERR_CREATE_TARGET_DIR" });
  });
  await page.goto("/#" + ROUTES.settings);
  await page.getByRole("button", { name: zh.settings.body.paths.reset, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: zh.settings.reset.confirm.title });
  await expectFits(dialog, 680);
  const confirm = dialog.getByRole("button", { name: new RegExp(zh.common.confirm + "$") });
  await confirm.click();
  try {
    await expect(confirm).toBeDisabled();
    await expect(dialog.getByRole("button", { name: zh.common.cancel, exact: true })).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeVisible();
    await page.mouse.click(5, 300);
    await expect(dialog).toBeVisible();
  } finally {
    release();
  }
  await expect(dialog.getByRole("alert")).toBeVisible();
  await expect(confirm).toBeEnabled();
  await dialog.getByRole("button", { name: zh.common.cancel, exact: true }).click();
  await expect(dialog).toHaveCount(0);
});

test("instance selection uses the wide preset and keeps its form responsive", async ({ page }) => {
  await setup(page);
  await page.route("**/levilauncher.json", route => route.fulfill({ json: {
    packages: {
      "example/modal-test": {
        info: { name: "Modal Test", description: "Dialog layout test", tags: [] },
        variants: { client: { label: "client", versions: { "3.9.3": { dependencies: {} } } } },
      },
    },
  } }));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/#" + routeTo.lipPackage("example/modal-test"));
  await page.getByRole("tab", { name: "版本", exact: true }).click();
  await page.getByRole("button", { name: /Modal Test 3\.9\.3/ }).click();
  const dialog = page.getByRole("dialog", { name: zh.lip.files.select_instance_title });
  await expectFits(dialog, 880);
  await expect(dialog).toContainText(instance.name);
  await expect(dialog).toContainText("3.9.3");
  expect((await dialog.boundingBox())!.height).toBeLessThan(500);
  await page.screenshot({ path: ".artifacts/modals/instance-selection.png" });
  await page.setViewportSize({ width: 320, height: 568 });
  await expectFits(dialog);
  await dialog.getByRole("button", { name: zh.common.cancel, exact: true }).click();
  await expect(dialog).toHaveCount(0);
});

test("DLL import uses aligned fields and preserves its validation and cancel action", async ({ page }) => {
  await setup(page);
  await page.route("**/levilauncher.json", route => route.fulfill({ json: { packages: {} } }));
  await page.route("**/wails/runtime", async route => {
    if (route.request().postDataJSON().object === 5) {
      return route.fulfill({ json: ["D:\\layout-test.dll"] });
    }
    return route.fallback();
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/#" + ROUTES.mods);
  await page.getByRole("button", { name: zh.mods.import_button, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: zh.mods.dll_modal_title });
  await expectFits(dialog, 880);
  const fields = dialog.getByRole("textbox");
  await expect(fields).toHaveCount(3);
  const [name, type, version] = await Promise.all((await fields.all()).map(field => field.boundingBox()));
  expect(name!.width).toBeGreaterThan(type!.width * 1.9);
  expect(Math.abs(type!.y - version!.y)).toBeLessThan(1);
  expect(version!.x).toBeGreaterThan(type!.x);
  await fields.first().fill("");
  await expect(dialog.getByRole("button", { name: zh.common.confirm, exact: true })).toBeDisabled();
  await fields.first().fill("Layout test");
  await expect(dialog.getByRole("button", { name: zh.common.confirm, exact: true })).toBeEnabled();
  await page.screenshot({ path: ".artifacts/modals/dll-form.png" });
  await page.setViewportSize({ width: 320, height: 568 });
  await expectFits(dialog);
  const narrowType = await fields.nth(1).boundingBox();
  const narrowVersion = await fields.nth(2).boundingBox();
  expect(narrowVersion!.y).toBeGreaterThan(narrowType!.y);
  await dialog.getByRole("button", { name: zh.common.cancel, exact: true }).click();
  await expect(dialog).toHaveCount(0);
});
