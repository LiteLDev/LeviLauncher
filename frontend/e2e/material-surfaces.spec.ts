import { readFile } from "node:fs/promises";
import { expect, test, type Locator, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {
  APPEARANCE_EVENT,
  APPEARANCE_KEY,
  getMaterialPreset,
} from "../src/utils/backgroundAppearance";
import { ROUTES } from "../src/constants/routes";
import en from "../src/assets/locales/en_US.json" with { type: "json" };
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const material = en.settings.appearance.material;
const instance = {
  name: "Material test",
  gameVersion: "1.26.0",
  enableIsolation: true,
};
const setup = async (
  page: Page,
  theme: "light" | "dark",
  layout = "sidebar",
) => {
  const wallpaper =
    "data:image/jpeg;base64," +
    (await readFile("src/assets/images/world-preview-default.jpg")).toString(
      "base64",
    );
  await mockWailsRuntime(page, {
    "minecraft.ListDir": [
      { name: "wall.png", path: "D:/wall/wall.png", isDir: false },
    ],
    "minecraft.GetImageURL": wallpaper,
    "versionservice.ListVersionMetas": [instance],
    "versionservice.ListVersionMetasWithRegistered": [instance],
    "versionservice.GetVersionMeta": instance,
    "versionservice.CreateDesktopShortcut": "",
    "minecraft.FetchHistoricalVersions": {
      releaseVersions: [],
      previewVersions: [],
    },
  });
  await seedCompletedSetup(page);
  await page.addInitScript(
    ({ theme, layout, key, profiles, name }) => {
      for (const [key, value] of Object.entries({
        "app.backgroundImage": "D:/wall",
        "app.themeMode": theme,
        theme: theme,
        "app.layoutMode": layout,
        "app.disableAnimations": "true",
        "ll.currentVersionName": name,
      }))
        localStorage.setItem(key, value);
      localStorage.setItem(key, JSON.stringify(profiles));
    },
    {
      theme,
      layout,
      key: APPEARANCE_KEY,
      profiles: {
        light: getMaterialPreset("light", "clear"),
        dark: getMaterialPreset("dark", "clear"),
      },
      name: instance.name,
    },
  );
};

const backgroundColor = (locator: Locator) =>
  locator.evaluate((element) => getComputedStyle(element).backgroundColor);
const changeOpacity = (page: Page, opacity: number) =>
  page.evaluate(
    ({ key, event, opacity }) => {
      const profiles = JSON.parse(localStorage.getItem(key)!);
      const mode = localStorage.getItem("app.themeMode") || "light";
      profiles[mode].surfaceOpacity = opacity;
      localStorage.setItem(key, JSON.stringify(profiles));
      window.dispatchEvent(new Event(event));
    },
    { key: APPEARANCE_KEY, event: APPEARANCE_EVENT, opacity },
  );

for (const theme of ["light", "dark"] as const) {
  for (const layout of ["sidebar", "navbar"]) {
    test(`${theme} ${layout}: chrome, tab track, segment and controls follow the opacity slider`, async ({
      page,
    }) => {
      await setup(page, theme, layout);
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto("/#" + ROUTES.settings);
      await page
        .getByRole("tab", {
          name: en.settings.tabs.personalization,
          exact: true,
        })
        .click();
      const section = page.getByRole("region", {
        name: material.title,
        exact: true,
      });
      await section
        .getByRole("button", {
          name: en.settings.appearance[`theme_${theme}`],
          exact: true,
        })
        .click();
      const slider = section.getByRole("slider", {
        name: material.opacity,
        exact: true,
      });
      const surfaces = [
        page.locator(".launcher-chrome").first(),
        page.locator("#main-content .launcher-glass").first(),
        page.locator(".tabs__list-container").first(),
        page.locator(".tabs__indicator").first(),
        section.getByRole("button", { name: new RegExp(material.solid) }),
      ];
      await expect
        .poll(() => backgroundColor(surfaces[0]))
        .toBe(
          theme === "dark"
            ? "rgba(24, 24, 27, 0.35)"
            : "rgba(255, 255, 255, 0.35)",
        );
      const before = await Promise.all(surfaces.map(backgroundColor));
      expect(before.every((color) => color.startsWith("rgba("))).toBe(true);
      await expect(page.locator(".tabs__list").first()).toHaveCSS(
        "background-color",
        "rgba(0, 0, 0, 0)",
      );
      await slider.focus();
      await page.keyboard.press("End");
      await expect(slider).toHaveValue("100");
      for (let i = 0; i < surfaces.length; i++)
        await expect
          .poll(() => backgroundColor(surfaces[i]))
          .not.toBe(before[i]);
      await expect(surfaces[0]).toHaveCSS(
        "background-color",
        theme === "dark" ? "rgb(24, 24, 27)" : "rgb(255, 255, 255)",
      );
      await page.keyboard.press("Home");
      await expect(slider).toHaveValue("0");
      await expect(surfaces[0]).toHaveCSS(
        "background-color",
        theme === "dark" ? "rgba(24, 24, 27, 0)" : "rgba(255, 255, 255, 0)",
      );
      await section
        .getByRole("button", { name: new RegExp(material.balanced) })
        .click();
      await expect
        .poll(() => backgroundColor(surfaces[0]))
        .toBe(await backgroundColor(surfaces[1]));
      await page
        .locator(".tabs__list-container")
        .first()
        .scrollIntoViewIfNeeded();
      await page.screenshot({
        path: `.artifacts/materials/settings-${theme}-${layout}.png`,
      });
      const a11y = await new AxeBuilder({ page })
        .include(`section[aria-label="${material.title}"]`)
        .withTags(["wcag2a", "wcag2aa"])
        .analyze();
      expect(a11y.violations).toEqual([]);
    });
  }

  test(`${theme}: already open menus and dialogs receive material updates`, async ({
    page,
  }) => {
    await setup(page, theme);
    await page.goto("/#" + ROUTES.home);
    await page
      .getByRole("button", {
        name: en.launcherpage.tip.quick_actions_menu,
        exact: true,
      })
      .click();
    const menu = page.locator(".dropdown__popover");
    await expect(menu).toBeVisible();
    expect(await menu.evaluate((el) => !!el.closest(".app-backdrop"))).toBe(
      false,
    );
    const initialMenu = await backgroundColor(menu);
    await changeOpacity(page, 80);
    await expect.poll(() => backgroundColor(menu)).not.toBe(initialMenu);
    await expect(menu).toHaveCSS("backdrop-filter", "blur(4px)");
    await page
      .getByRole("menuitem", {
        name: en.launcherpage.shortcut.create_button,
        exact: true,
      })
      .click();
    const dialog = page.getByRole("dialog", {
      name: en.launcherpage.shortcut.success.title,
      exact: true,
    });
    await expect(dialog).toBeVisible();
    const initialDialog = await backgroundColor(dialog);
    await changeOpacity(page, 35);
    await expect.poll(() => backgroundColor(dialog)).not.toBe(initialDialog);
    await expect(dialog).toHaveCSS("backdrop-filter", "blur(4px)");
    await page.emulateMedia({ contrast: "more" });
    await expect(dialog).toHaveCSS("backdrop-filter", "none");
    await expect(dialog).toHaveCSS(
      "background-color",
      theme === "dark" ? "rgb(24, 24, 27)" : "rgb(255, 255, 255)",
    );
    await page.emulateMedia({ contrast: "no-preference" });
    await page.evaluate(() => {
      localStorage.setItem("app.backgroundImage", "");
      window.dispatchEvent(new Event("app-background-changed"));
    });
    await expect(page.locator("html")).toHaveAttribute(
      "data-wallpaper-active",
      "false",
    );
    await expect(dialog).toHaveCSS("backdrop-filter", "none");
  });

  test(`${theme}: page navigation keeps the global material contract`, async ({
    page,
  }) => {
    await setup(page, theme);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    for (const route of [
      ROUTES.home,
      ROUTES.download,
      ROUTES.downloadTasks,
      ROUTES.instances,
      ROUTES.mods,
      ROUTES.content,
      ROUTES.contentWorlds,
      ROUTES.contentResourcePacks,
      ROUTES.contentBehaviorPacks,
      ROUTES.contentSkinPacks,
      ROUTES.contentScreenshots,
      ROUTES.contentServers,
      ROUTES.settings,
      ROUTES.about,
    ]) {
      await page.goto("/#" + route);
      await expect(page.locator("html")).toHaveAttribute(
        "data-wallpaper-active",
        "true",
      );
      await expect(page.locator("#main-content .card").first()).toBeVisible();
      await changeOpacity(page, 35);
      await expect(page.locator(".launcher-chrome").first()).toHaveCSS(
        "background-color",
        theme === "dark"
          ? "rgba(24, 24, 27, 0.35)"
          : "rgba(255, 255, 255, 0.35)",
      );
      const firstCard = page.locator("#main-content .card").first();
      const initial = await backgroundColor(firstCard);
      await changeOpacity(page, 85);
      await expect
        .poll(() => backgroundColor(firstCard), route)
        .not.toBe(initial);
    }
    expect(errors).toEqual([]);
  });
}
