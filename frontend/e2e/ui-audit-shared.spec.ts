import { expect, test } from "@playwright/test";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { THEMES, getSolidAccent } from "../src/constants/themes";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const whiteContrast = (channels: number[]) => {
  const linear = channels.map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 1.05 / (0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2] + 0.05);
};

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

test("solid accents preserve white-label contrast for every palette and extreme custom colors", () => {
  for (const base of [...Object.values(THEMES).map((theme) => theme[500]), "#ffffff", "#ffff00", "#000000", "#fefefe"]) {
    const accent = getSolidAccent(base);
    const channels = [1, 3, 5].map((offset) => parseInt(accent.slice(offset, offset + 2), 16));
    expect(whiteContrast(channels), `${base} → ${accent}`).toBeGreaterThanOrEqual(4.5);
  }
});

for (const theme of ["light", "dark"] as const) {
  for (const color of ["emerald", "amber", "custom", "custom_black"]) {
    test(`primary action has readable white labels in ${theme}/${color}`, async ({ page }) => {
      await mockWailsRuntime(page);
      await seedCompletedSetup(page);
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addInitScript(({ theme, color }) => {
        localStorage.setItem("theme", theme);
        localStorage.setItem("app.themeMode", theme);
        localStorage.setItem(`app.${theme}ThemeColor`, color === "custom_black" ? "custom" : color);
        localStorage.setItem(`app.${theme}CustomThemeColor`, color === "custom_black" ? "#000000" : "#ffffff");
      }, { theme, color });
      await page.goto(`/#${ROUTES.home}`);
      const launch = page.getByTestId("primary-launch-button");
      await expect(launch).toBeVisible();
      await expect(launch).toHaveCSS("color", "rgb(255, 255, 255)");
      for (const hover of [false, true]) {
        if (hover) await launch.hover();
        const background = await launch.evaluate((element) => getComputedStyle(element).backgroundColor);
        expect(whiteContrast(background.match(/[\d.]+/g)!.slice(0, 3).map(Number))).toBeGreaterThanOrEqual(4.5);
      }
      if (theme === "dark" && color === "custom_black") {
        const focus = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--focus"));
        const channels = focus.match(/[\d.]+/g)!.slice(0, 3).map(Number);
        expect(21 / whiteContrast(channels)).toBeGreaterThanOrEqual(3);
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
