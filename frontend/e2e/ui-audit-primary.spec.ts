import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { ROUTES } from "../src/constants/routes";
import en from "../src/assets/locales/en_US.json" with { type: "json" };
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const instance = { name: "Audit Instance With A Long Distinctive Name", gameVersion: "1.21.0", type: "release", enableIsolation: true, envVars: "", launchArgs: "" };
const setup = async (page: Page, responses: Record<string, unknown> = {}) => {
  await mockWailsRuntime(page, {
    "minecraft.FetchHistoricalVersions": { previewVersions: [], releaseVersions: [] },
    "versionservice.ListVersionMetas": [instance],
    "versionservice.GetVersionMeta": instance,
    "versionservice.SaveVersionMeta": "",
    "versionservice.ValidateVersionFolderName": "",
    ...responses,
  });
  await seedCompletedSetup(page);
};

const overrideMethod = async (page: Page, service: string, method: string, handler: (args: unknown[]) => unknown) => {
  const source = await readFile(resolve("bindings/github.com/liteldev/LeviLauncher", service + ".js"), "utf8");
  const declaration = source.slice(source.indexOf("export function " + method + "("));
  const methodID = Number(declaration.match(/\$Call\.ByID\((\d+)/)?.[1]);
  expect(methodID).toBeGreaterThan(0);
  await page.route("**/wails/runtime", async (route) => {
    const request = route.request().postDataJSON();
    if (Number(request?.args?.methodID) !== methodID) return route.fallback();
    const response = handler(request?.args?.args || []);
    if (response instanceof Error) return route.fulfill({ status: 500, body: response.message });
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(response) });
  });
};

const openInstanceSettings = async (page: Page) => {
  await page.goto("/#" + ROUTES.instances);
  await page.getByRole("button", { name: "Settings for " + instance.name, exact: true }).click();
  await page.getByRole("tab", { name: en.versions.edit.tabs.launch, exact: true }).click();
};

for (const field of ["env_vars", "launch_args"] as const) {
  test("PRI-01 only editing " + field + " prompts and saves before leaving", async ({ page }) => {
    await setup(page);
    let saved: unknown[] = [];
    await overrideMethod(page, "versionservice", "SaveVersionMeta", (args) => { saved = args; return ""; });
    await openInstanceSettings(page);
    const value = field === "env_vars" ? "AUDIT_VALUE=1" : "-Editor true";
    await page.getByRole("textbox", { name: en.versions.edit[field + "_placeholder" as "env_vars_placeholder" | "launch_args_placeholder"], exact: true }).fill(value);
    await page.evaluate((path) => window.dispatchEvent(new CustomEvent("ll-try-nav", { detail: { path } })), ROUTES.download);
    const dialog = page.getByRole("dialog", { name: en.settings.unsaved.title });
    await expect(dialog).toBeVisible();
    await expect(page).toHaveURL(new RegExp(ROUTES.instanceSettings + "$"));
    await dialog.getByRole("button", { name: en.settings.unsaved.save, exact: true }).click();
    await expect(page).toHaveURL(new RegExp(ROUTES.download + "$"));
    expect(saved).toContain(value);
  });
}

test("PRI-02 history forward preserves its numeric target after save", async ({ page }) => {
  await setup(page);
  await openInstanceSettings(page);
  await page.evaluate((path) => window.dispatchEvent(new CustomEvent("ll-try-nav", { detail: { path } })), ROUTES.download);
  await expect(page).toHaveURL(new RegExp(ROUTES.download + "$"));
  await page.goBack();
  await page.getByRole("tab", { name: en.versions.edit.tabs.launch, exact: true }).click();
  await page.getByRole("textbox", { name: en.versions.edit.env_vars_placeholder, exact: true }).fill("AUDIT_FORWARD=1");
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("ll-try-nav", { detail: { path: 1 } })));
  await page.getByRole("dialog").getByRole("button", { name: en.settings.unsaved.save, exact: true }).click();
  await expect(page).toHaveURL(new RegExp(ROUTES.download + "$"));
});

test("PRI-03 first version request failure stays retryable and recovers", async ({ page }) => {
  await setup(page);
  let fail = true;
  await overrideMethod(page, "minecraft", "FetchHistoricalVersions", () => fail ? new Error("offline") : { previewVersions: [], releaseVersions: [{ version: "Release 1.21.0", urls: ["https://example.invalid/game.msixvc"] }] });
  await page.goto("/#" + ROUTES.download);
  await expect(page.getByText(en.audit.primary.download.failed, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: en.common.refresh, exact: true })).toBeEnabled();
  fail = false;
  await page.getByRole("button", { name: en.download_manager.actions.retry, exact: true }).click();
  await expect(page.getByRole("grid").getByText("1.21.0", { exact: true })).toBeVisible();
  await page.getByRole("textbox").fill("missing-version");
  await expect(page.getByText(en.audit.primary.download.no_matches, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: en.audit.primary.clear_filters, exact: true }).click();
  await expect(page.getByRole("grid").getByText("1.21.0", { exact: true })).toBeVisible();
});

test("PRI-04 instance failure retries into a useful first-instance empty state", async ({ page }) => {
  await setup(page);
  let fail = true;
  await overrideMethod(page, "versionservice", "ListVersionMetas", () => fail ? new Error("unavailable") : []);
  await page.goto("/#" + ROUTES.instances);
  await expect(page.getByText(en.audit.primary.instances.failed, { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole("button", { name: en.download_manager.actions.retry, exact: true }).click();
  await expect(page.getByText(en.audit.primary.instances.empty, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: en.audit.primary.local_install, exact: true })).toBeVisible();
  await page.getByRole("button", { name: en.audit.primary.download_minecraft, exact: true }).click();
  await expect(page).toHaveURL(new RegExp(ROUTES.download + "$"));
});

test("PRI-05 verifying tasks expose no cancellation or removal until complete", async ({ page }) => {
  await setup(page);
  await page.goto("/#" + ROUTES.downloadTasks);
  await expect(page.getByText(en.download_manager.no_downloads, { exact: true })).toBeVisible();
  await page.evaluate(() => (window as any)._wails.dispatchWailsEvent({ name: "msixvc_download_status", data: { Dest: "D:/test/game.msixvc", Status: "verifying" } }));
  await expect(page.getByText(en.audit.primary.download.verifying, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: en.audit.primary.download.cancel, exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: en.audit.primary.download.remove, exact: true })).toHaveCount(0);
  await page.evaluate(() => (window as any)._wails.dispatchWailsEvent({ name: "msixvc_download_done", data: { Dest: "D:/test/game.msixvc" } }));
  await page.getByRole("button", { name: en.audit.primary.download.remove, exact: true }).click();
  await expect(page.getByText(en.download_manager.no_downloads, { exact: true })).toBeVisible();
});

test("PRI-06/12 install failure persists and a retry selects the completed instance", async ({ page }) => {
  await setup(page, { "minecraft.ResolveDownloadedMsixvc": "D:/test/game.msixvc", "versionservice.GetInstallerDir": "D:/test" });
  await page.addInitScript(({ route }) => { if (location.hash === "#" + route) history.replaceState({ usr: { mirrorVersion: "Audit New Instance", mirrorType: "Release" }, idx: 0, key: "audit-install" }, ""); }, { route: ROUTES.install });
  let fail = true;
  await overrideMethod(page, "minecraft", "InstallExtractMsixvc", () => fail ? "ERR_AUDIT_EXTRACTION" : "");
  await page.goto("/#" + ROUTES.install);
  await expect(page.getByRole("switch", { name: en.downloadpage.install_folder.enable_isolation, exact: true })).toBeChecked();
  await page.getByRole("button", { name: en.downloadpage.customappx.modal["1"].footer.install_button, exact: true }).click();
  await expect(page.locator("pre").filter({ hasText: "ERR_AUDIT_EXTRACTION" })).toBeVisible();
  await expect(page.getByRole("button", { name: en.audit.primary.install.copy_details, exact: true })).toBeVisible();
  fail = false;
  await page.getByRole("button", { name: en.download_manager.actions.retry, exact: true }).click();
  await page.getByRole("button", { name: en.audit.primary.install.go_launch, exact: true }).click();
  await expect(page).toHaveURL(new RegExp("/#" + ROUTES.home + "$"));
  expect(await page.evaluate(() => localStorage.getItem("ll.currentVersionName"))).toBe("Audit New Instance");
});

test("PRI-07 unwritable onboarding directory requires explicit use of current directory", async ({ page }) => {
  await setup(page, { "minecraft.GetBaseRoot": "D:/MinecraftData", "minecraft.CanWriteToDir": false });
  await page.addInitScript(() => localStorage.removeItem("ll.onboarded"));
  await page.goto("/#" + ROUTES.onboarding);
  await page.getByRole("textbox", { name: en.settings.body.paths.base_root, exact: true }).fill("Z:/ReadOnly");
  await page.getByRole("button", { name: en.settings.body.paths.apply, exact: true }).click();
  await expect(page.getByText(en.settings.body.paths.not_writable, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: en.onboarding.finish, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: en.onboarding.unsaved.title });
  await expect(dialog).toContainText("D:/MinecraftData");
  expect(await page.evaluate(() => localStorage.getItem("ll.onboarded"))).toBeNull();
  await dialog.getByRole("button", { name: en.audit.primary.onboarding.keep_current, exact: true }).click();
  await expect(page).toHaveURL(new RegExp("/#" + ROUTES.home + "$"));
});


test("PRI-04/11 empty home offers download and instance names remain inspectable", async ({ page }) => {
  await setup(page, { "versionservice.ListVersionMetas": [] });
  await page.goto("/#" + ROUTES.home);
  await expect(page.getByTestId("primary-launch-button")).toHaveText(en.audit.primary.download_minecraft);
  await page.getByRole("button", { name: en.launcherpage.tip.quick_actions_menu, exact: true }).click();
  await expect(page.getByRole("menuitem", { name: en.launcherpage.shortcut.create_button, exact: true })).toBeDisabled();
  await expect(page.getByRole("menuitem", { name: en.launcherpage.open_exe_dir, exact: true })).toBeDisabled();
  await expect(page.getByText(en.audit.primary.select_instance_first, { exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByTestId("primary-launch-button").click();
  await expect(page).toHaveURL(new RegExp(ROUTES.download + "$"));
});

test("PRI-04/11 instance search can clear filters and exposes a full-name tooltip", async ({ page }) => {
  await setup(page);
  await page.setViewportSize({ width: 960, height: 600 });
  await page.goto("/#" + ROUTES.instances);
  await page.getByRole("textbox").fill("not-an-instance");
  await expect(page.getByText(en.audit.primary.instances.no_matches, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: en.audit.primary.clear_filters, exact: true }).click();
  const select = page.getByRole("button", { name: instance.name, exact: true });
  await page.keyboard.press("Tab");
  await select.focus();
  await expect(page.getByRole("tooltip")).toHaveText(instance.name);
  await page.screenshot({ path: ".artifacts/ui-ux-fixes/primary-instance-tooltip.png", fullPage: true });
});
