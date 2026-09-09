import { expect, test, type Page } from "@playwright/test";
import { ROUTES } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

async function expectContentFits(page: Page) {
  await expect.poll(() => page.locator("main").evaluate((main) => {
    const grid = main.querySelector("table")!;
    const area = grid.closest('[data-slot="table"]')!;
    const rows = Array.from(grid.querySelectorAll("tbody tr"));
    const bounds = area.getBoundingClientRect();
    return {
      pageFits: main.getBoundingClientRect().bottom <= innerHeight + 1,
      rowsFit: rows.length > 0 && rows.every((row) => row.getBoundingClientRect().bottom <= bounds.bottom + 1),
      widthFits: grid.getBoundingClientRect().right <= innerWidth + 1,
      noScroll: [document.documentElement, document.body, document.getElementById("root")!, main, main.firstElementChild!, area].every((el) => el.scrollHeight <= el.clientHeight + 1),
    };
  })).toEqual({ pageFits: true, rowsFit: true, widthFits: true, noScroll: true });
  await expect(page.locator("main nav")).toBeInViewport({ ratio: 1 });
}

for (const layout of ["sidebar", "navbar"]) {
  test(`download pagination fits the viewport in ${layout} layout`, async ({ page }) => {
    await mockWailsRuntime(page, {
      "minecraft.FetchHistoricalVersions": {
        previewVersions: [],
        releaseVersions: Array.from({ length: 75 }, (_, i) => ({
          version: `Release 1.26.${75 - i}.01`,
          urls: ["https://example.invalid/game.msixvc"],
        })),
      },
    });
    await seedCompletedSetup(page);
    await page.addInitScript((mode) => {
      localStorage.setItem("app.layoutMode", mode);
      localStorage.setItem("i18nextLng", "zh_CN");
    }, layout);
    await page.setViewportSize({ width: 960, height: 600 });
    await page.goto("/#" + ROUTES.download);
    await expect(page.getByRole("grid").getByText("1.26.75.01", { exact: true })).toBeVisible();
    await expectContentFits(page);
    const smallRows = await page.locator("tbody tr").count();
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect.poll(() => page.locator("tbody tr").count()).toBeGreaterThan(smallRows);
    await expectContentFits(page);
    await expect(page.locator("tbody tr").last().locator("td > div").last()).toHaveCSS("opacity", "1");
    await page.screenshot({ path: `.artifacts/download-layout-${layout}.png` });
    await page.locator("main nav button").last().click();
    await expect(page.getByRole("grid").getByText("1.26.75.01", { exact: true })).toHaveCount(0);
    await expectContentFits(page);
    await page.setViewportSize({ width: 800, height: 600 });
    await expectContentFits(page);
    await expect(page.locator("tbody tr").last().locator("td > div").last()).toHaveCSS("opacity", "1");
    await page.screenshot({ path: `.artifacts/download-layout-${layout}-compact.png` });
    await page.getByRole("textbox").fill("1.26.75.01");
    await expect(page.locator("tbody tr")).toHaveCount(1);
    await expectContentFits(page);
  });
}
