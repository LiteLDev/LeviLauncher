import { expect, test } from "@playwright/test";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { THEMES, generateTheme, getAccentForeground, getAccentHover, getReadableAccent } from "../src/constants/themes";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const luminance = (channels: number[]) => {
  const linear = channels.map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
};
const contrast = (background: number[], foreground = [255, 255, 255]) => {
  const values = [luminance(background), luminance(foreground)];
  return (Math.max(...values) + 0.05) / (Math.min(...values) + 0.05);
};

test("custom pastel shades remain ordered and do not become saturated red", () => {
  const channels = (hex: string) => [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16));
  for (const base of ["#ffb6c1", "#fefefe", "#ffffff", "#000000", "#777777"]) {
    const palette = generateTheme(base);
    const shades = Object.values(palette).map(color => luminance(channels(color)));
    expect(palette[500]).toBe(base);
    expect(shades).toEqual([...shades].sort((a, b) => b - a));
  }
  const pink = channels(generateTheme("#ffb6c1")[700]);
  // A darker pastel retains its red/green balance instead of losing green.
  expect(pink[1] / pink[0]).toBeCloseTo(182 / 255, 2);
});

test("application styles contain no gradient backgrounds, text, borders or SVG fills", async () => {
  const violations: string[] = [];
  const inspect = async (directory: string) => {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) await inspect(path);
      else if (/\.(?:css|tsx?|jsx?|svg)$/.test(entry.name)) {
        const source = await readFile(path, "utf8");
        if (/(?:linear|radial|conic)-gradient\(|\bbg-(?:gradient|linear|radial|conic)-|(?:linear|radial)Gradient/i.test(source)) {
          violations.push(path);
        }
      }
    }
  };
  await inspect("src");
  expect(violations).toEqual([]);
});

test("solid accents preserve label contrast for every palette and extreme custom colors", () => {
  const channels = (hex: string) => [1, 3, 5].map((offset) => parseInt(hex.slice(offset, offset + 2), 16));
  for (const base of [...Object.values(THEMES).map((theme) => theme[500]), "#ffffff", "#ffff00", "#000000", "#fefefe", "#ffb6c1", "#777777"]) {
    const foreground = getAccentForeground(base);
    for (const accent of [base, getAccentHover(base)]) {
      expect(contrast(channels(accent), channels(foreground)), `${base} → ${accent} / ${foreground}`).toBeGreaterThanOrEqual(4.5);
    }
    for (const surface of ["#f4f4f5", "#27272a"]) {
      for (const ratio of [3.1, 4.6]) {
        expect(contrast(channels(getReadableAccent(base, surface, ratio)), channels(surface))).toBeGreaterThanOrEqual(ratio);
      }
    }
  }
});

for (const theme of ["light", "dark"] as const) {
  test(`action colors remain neutral across pages and settings tabs in ${theme}`, async ({ page }, testInfo) => {
    test.setTimeout(120_000);
    await mockWailsRuntime(page);
    await seedCompletedSetup(page);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.addInitScript(theme => {
      localStorage.setItem("theme", theme);
      localStorage.setItem("app.themeMode", theme);
    }, theme);
    await page.route("**/levilauncher.json", route => route.fulfill({ json: { packages: {} } }));
    const expected = theme === "dark" ? "rgb(244, 244, 245)" : "rgb(63, 63, 70)";
    const counts: Record<string, number> = {};
    const inspect = async (name: string) => {
      const actions = page.locator('button:visible, [role="button"]:visible, .pagination__link:visible');
      await expect(actions.first()).toBeVisible();
      const colors = await actions.evaluateAll(elements => elements.map(element => ({
        label: element.getAttribute("aria-label") || element.textContent?.trim(),
        color: getComputedStyle(element).color,
        solid: element.matches(".button--primary, .button--danger, .button.bg-brand-500, .pagination__link.bg-accent"),
        danger: element.matches(".button--danger"),
      })));
      counts[name] = colors.length;
      expect(colors.filter(action => action.color !== (action.danger ? "rgb(244, 244, 245)" : action.solid ? "rgb(24, 24, 27)" : expected)), name).toEqual([]);
    };
    for (const route of [ROUTES.home, ROUTES.instances, ROUTES.download, ROUTES.downloadTasks,
      ROUTES.mods, ROUTES.curseForge, ROUTES.lip, ROUTES.content, ROUTES.contentWorlds,
      ROUTES.contentResourcePacks, ROUTES.contentBehaviorPacks, ROUTES.contentSkinPacks,
      ROUTES.contentServers, ROUTES.contentScreenshots, ROUTES.settings, ROUTES.about]) {
      await page.goto(`/#${route}`);
      await expect(page.locator("html")).toHaveClass(new RegExp(theme));
      await expect(page.locator("main")).toBeVisible();
      await expect(page.locator("main .animate-spin, main .skeleton")).toHaveCount(0);
      await inspect(route);
    }
    await page.goto(`/#${ROUTES.settings}`);
    const settingsTabs = page.getByRole("tablist").first().getByRole("tab");
    await expect(settingsTabs.first()).toBeVisible();
    for (const tab of await settingsTabs.all()) {
      await tab.click();
      await inspect(`settings/${await tab.textContent()}`);
    }
    await testInfo.attach("action-color-counts", { body: JSON.stringify(counts, null, 2), contentType: "application/json" });
  });

  for (const color of ["emerald", "amber", "pink", "custom", "custom_black", "custom_pink"]) {
    test(`primary action has readable neutral labels in ${theme}/${color}`, async ({ page }) => {
      await mockWailsRuntime(page);
      await seedCompletedSetup(page);
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addInitScript(({ theme, color }) => {
        localStorage.setItem("theme", theme);
        localStorage.setItem("app.themeMode", theme);
        localStorage.setItem(`app.${theme}ThemeColor`, color.startsWith("custom") ? "custom" : color);
        localStorage.setItem(`app.${theme}CustomThemeColor`, color === "custom_black" ? "#000000" : color === "custom_pink" ? "#ffb6c1" : "#ffffff");
      }, { theme, color });
      await page.goto(`/#${ROUTES.home}`);
      const launch = page.getByTestId("primary-launch-button");
      await expect(launch).toBeVisible();
      const foreground = color === "custom_black" ? [255, 255, 255] : [24, 24, 27];
      await expect(launch).toHaveCSS("color", `rgb(${foreground.join(", ")})`);
      const base = color === "custom_black" ? "#000000" : color === "custom_pink" ? "#ffb6c1" : color === "custom" ? "#ffffff" : THEMES[color][500];
      const baseChannels = [1, 3, 5].map(offset => parseInt(base.slice(offset, offset + 2), 16));
      await expect(launch).toHaveCSS("background-color", `rgb(${baseChannels.join(", ")})`);
      for (const hover of [false, true]) {
        if (hover) await launch.hover();
        const background = await launch.evaluate((element) => {
          // Normalize rgb(), oklch() and color(srgb ...) through the browser.
          const context = document.createElement("canvas").getContext("2d")!;
          context.fillStyle = getComputedStyle(element).backgroundColor;
          context.fillRect(0, 0, 1, 1);
          return Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3);
        });
        expect(contrast(background, foreground)).toBeGreaterThanOrEqual(4.5);
      }
      if (color === "pink" || color === "custom_pink") {
        await page.mouse.move(0, 0);
        await page.screenshot({ path: `.artifacts/personalization/${theme}-${color}.png` });
      }
      if (theme === "dark" && color === "custom_black") {
        const focus = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--focus"));
        const channels = focus.match(/[\d.]+/g)!.slice(0, 3).map(Number);
        expect(contrast(channels, [0, 0, 0])).toBeGreaterThanOrEqual(3);
      }
    });
  }
}

test("Chinese breadcrumbs use route labels and detail names instead of path fragments", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.GetCurseForgeModsByIDs": { data: [{ id: 42, name: "审计测试模组", authors: [], categories: [], links: {}, latestFiles: [] }] },
    "minecraft.GetCurseForgeModDescription": { data: "<p>可以复制的说明内容</p>" },
    "minecraft.GetCurseForgeModFiles": { data: [] },
  });
  await seedCompletedSetup(page);
  await page.addInitScript(() => localStorage.setItem("i18nextLng", "zh_CN"));
  await page.goto(`/#${ROUTES.contentResourcePacks}`);
  const breadcrumbs = page.getByRole("navigation", { name: "面包屑" });
  await expect(breadcrumbs).toContainText("首页");
  await expect(breadcrumbs).toContainText("资源包");
  await expect(breadcrumbs).not.toContainText("ResourcePacks");
  await page.goto(`/#${routeTo.curseForgeMod(42)}`);
  await expect(breadcrumbs.locator('[aria-current="page"]')).toHaveText("审计测试模组");
  await expect(breadcrumbs.locator('[aria-current="page"]')).toHaveAttribute("title", "审计测试模组");
  await expect(page.getByText("可以复制的说明内容", { exact: true })).toHaveCSS("user-select", "text");
});
