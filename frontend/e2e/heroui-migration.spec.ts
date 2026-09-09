import { expect, test, type Page } from "@playwright/test";
import { ROUTES, routeTo } from "../src/constants/routes";
import en from "../src/assets/locales/en_US.json" with { type: "json" };
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const setup = async (page: Page, responses: Record<string, unknown> = {}) => {
  await mockWailsRuntime(page, {
    "minecraft.FetchHistoricalVersions": {
      previewVersions: [],
      releaseVersions: [],
    },
    "minecraft.GetCurseForgeGameVersions": [
      { name: "1.21.0" },
      { name: "1.20.0" },
    ],
    "minecraft.GetCurseForgeCategories": [
      { id: 100, name: "Addons", isClass: true, displayIndex: 1 },
      {
        id: 101,
        name: "Technology",
        isClass: false,
        classId: 100,
        displayIndex: 1,
      },
      {
        id: 102,
        name: "Adventure",
        isClass: false,
        classId: 100,
        displayIndex: 2,
      },
    ],
    "minecraft.SearchCurseForgeMods": {
      data: [],
      pagination: { totalCount: 200 },
    },
    ...responses,
  });
  await seedCompletedSetup(page);
};

test("single/multiple selects update filters and pagination handles boundaries", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await setup(page);
  await page.goto(`/#${ROUTES.curseForge}`);
  const version = page.getByRole("button", {
    name: new RegExp(en.curseforge.minecraft_version),
  });
  await version.click();
  await page.getByRole("option", { name: "1.21.0", exact: true }).click();
  await expect(version).toContainText("1.21.0");
  await expect(page.getByRole("listbox")).toHaveCount(0);

  const classSelect = page.getByRole("button", {
    name: new RegExp(en.curseforge.class),
  });
  await classSelect.click();
  await page.getByRole("option", { name: "Addons", exact: true }).click();
  const category = page.getByRole("button", {
    name: new RegExp(en.curseforge.category),
  });
  await category.click();
  await page.getByRole("option", { name: "Technology", exact: true }).click();
  await page.getByRole("option", { name: "Adventure", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await category.click();
  await expect(
    page.getByRole("option", { name: "Technology", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(
    page.getByRole("option", { name: "Adventure", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(page.locator('[role="option"]:focus')).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`${ROUTES.curseForge}$`));

  const pagination = page.getByRole("navigation", { name: /pagination/i });
  await expect(pagination).toBeVisible();
  await pagination
    .getByRole("button", { name: "Page 10", exact: true })
    .click();
  await expect(pagination.locator('[aria-current="page"]')).toHaveText("10");
  await expect(
    pagination.getByRole("button", { name: /next/i }),
  ).toBeDisabled();
  await pagination.getByRole("button", { name: "Page 1", exact: true }).click();
  await expect(
    pagination.getByRole("button", { name: /previous/i }),
  ).toBeDisabled();
  await expect(
    pagination.getByRole("button", { name: "Page 1", exact: true }),
  ).toHaveCount(1);
  expect(errors).toEqual([]);
});

test("settings tabs, switch and theme palette retain controlled state", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await setup(page);
  await page.goto(`/#${ROUTES.settings}`);
  await page
    .getByRole("tab", { name: en.settings.tabs.personalization, exact: true })
    .click();
  await expect(
    page.getByRole("tab", {
      name: en.settings.tabs.personalization,
      exact: true,
    }),
  ).toHaveAttribute("aria-selected", "true");
  const switches = page.getByRole("switch");
  await expect(switches).toHaveCount(2);
  await page.locator('[data-slot="switch-content"]').last().click();
  await expect(switches.last()).toBeChecked();
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.disableAnimations")),
    )
    .toBe("true");
  await switches.last().press("Space");
  await expect(switches.last()).not.toBeChecked();
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.disableAnimations")),
    )
    .toBe("false");
  await page.screenshot({
    path: ".artifacts/heroui-settings-light.png",
    fullPage: true,
  });
  expect(errors).toEqual([]);
});

test("input clearing and dropdown keyboard selection remain functional", async ({
  page,
}) => {
  await setup(page);
  await page.goto(`/#${ROUTES.instances}`);
  const search = page.getByRole("textbox");
  await search.fill("migration-test");
  await expect(search).toHaveValue("migration-test");
  await page.getByRole("button", { name: "Clear", exact: true }).click();
  await expect(search).toHaveValue("");

  await page.goto(`/#${ROUTES.onboarding}`);
  // Completed setup redirects onboarding, so exercise the settings language menu.
  await page.goto(`/#${ROUTES.settings}`);
  const languageButton = page.getByRole("button", {
    name: en.settings.body.language.button,
    exact: true,
  });
  await languageButton.click();
  await expect(page.getByRole("menu")).toBeVisible();
  await page
    .getByRole("menuitemradio", { name: "简体中文", exact: true })
    .click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("menu")).toHaveCount(0);
});

test("controlled dependency dialog closes on Escape without route navigation", async ({
  page,
}) => {
  await setup(page, { "minecraft.IsGameInputInstalled": false });
  await page.goto(`/#${ROUTES.home}`);
  const dialog = page.getByRole("dialog", {
    name: "GameInput Component Missing",
  });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(page).toHaveURL(/\/#\/$/);
});

test("detail tabs retain description and populated file table", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  await setup(page, {
    "minecraft.GetCurseForgeModsByIDs": {
      data: [
        {
          id: 42,
          name: "Migration Addon",
          summary: "Test addon",
          authors: [],
          categories: [],
          downloadCount: 25,
          dateCreated: "2026-01-01T00:00:00Z",
          dateModified: "2026-01-02T00:00:00Z",
          links: {},
          latestFiles: [],
        },
      ],
    },
    "minecraft.GetCurseForgeModDescription": {
      data: "<p>Migration description content</p>",
    },
    "minecraft.GetCurseForgeModFiles": {
      data: [
        {
          id: 420,
          displayName: "Migration file",
          fileName: "migration.mcaddon",
          fileDate: "2026-01-01T00:00:00Z",
          fileLength: 1024,
          downloadCount: 25,
          releaseType: 1,
          gameVersions: ["1.21.0"],
          downloadUrl: "https://example.invalid/migration.mcaddon",
        },
      ],
    },
  });
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await expect(
    page.getByText("Migration description content", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("tab", { name: en.curseforge.mod_tabs.files, exact: true })
    .click();
  await expect(page.getByRole("grid")).toBeVisible();
  await expect(
    page.getByRole("grid").getByText("Migration file", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("tab", { name: en.curseforge.mod_tabs.description, exact: true })
    .click();
  await expect(
    page.getByText("Migration description content", { exact: true }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test("instance card selection emits a toast and keeps its settings action separate", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  const instance = {
    name: "Migration Instance",
    gameVersion: "1.21.0",
    isPreview: false,
  };
  await setup(page, {
    "versionservice.ListVersionMetas": [instance],
    "versionservice.ListVersionMetasWithRegistered": [instance],
    "versionservice.GetVersionMeta": instance,
  });
  await page.goto(`/#${ROUTES.instances}`);
  const select = page.getByRole("button", { name: instance.name, exact: true });
  await select.click();
  await expect(select).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByText(`${en.launcherpage.currentVersion}: ${instance.name}`, {
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.locator("button button")).toHaveCount(0);
  await page.getByRole("button", { name: "settings", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`${ROUTES.instanceSettings}$`));
  expect(errors).toEqual([]);
});

for (const [name, route] of Object.entries({
  home: ROUTES.home,
  download: ROUTES.download,
  instances: ROUTES.instances,
  settings: ROUTES.settings,
  about: ROUTES.about,
  curseforge: ROUTES.curseForge,
  tasks: ROUTES.downloadTasks,
})) {
  for (const theme of ["light", "dark"] as const) {
    test(`${name} renders in ${theme} without runtime errors`, async ({
      page,
    }) => {
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.emulateMedia({ reducedMotion: "reduce" });
      await setup(page);
      await page.addInitScript((theme) => {
        localStorage.setItem("theme", theme);
        localStorage.setItem("app.themeMode", theme);
      }, theme);
      await page.goto(`/#${route}`);
      await expect(page.locator("main")).toBeVisible();
      await expect(page.locator("[data-startup-content][inert]")).toHaveCount(
        0,
      );
      await expect(page.locator("html")).toHaveClass(new RegExp(theme));
      if (name === "home" || name === "settings" || name === "download")
        await page.screenshot({
          path: `.artifacts/heroui-${name}-${theme}.png`,
          fullPage: true,
        });
      expect(errors).toEqual([]);
    });
  }
}
