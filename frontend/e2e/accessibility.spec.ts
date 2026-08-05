import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const WCAG_TAGS = ["wcag2a", "wcag2aa", "wcag22aa"];

const expectNoUnexpectedAccessibilityViolations = async (page: Page) => {
  const nonContrastResults = await new AxeBuilder({ page })
    .withTags(WCAG_TAGS)
    .disableRules(["color-contrast"])
    .analyze();
  expect(nonContrastResults.violations).toEqual([]);

  const contrastResults = await new AxeBuilder({ page })
    .withRules(["color-contrast"])
    .exclude(".brand-primary-foreground")
    .analyze();
  expect(contrastResults.violations).toEqual([]);
};

test("onboarding keeps a complete keyboard navigation path", async ({
  page,
}) => {
  await mockWailsRuntime(page);
  await page.addInitScript(() => {
    localStorage.clear();
    localStorage.setItem("i18nextLng", "en_US");
  });

  await page.goto("/#/onboarding");
  await expect(
    page.getByRole("heading", { name: "First Launch Setup" }),
  ).toBeVisible();

  const visited = new Set<string>();
  for (let index = 0; index < 20; index += 1) {
    await page.keyboard.press("Tab");
    const focusedName = await page.evaluate(() => {
      const element = document.activeElement as HTMLElement | null;
      return (
        element?.getAttribute("aria-label") ||
        element?.textContent?.trim() ||
        element?.getAttribute("placeholder") ||
        ""
      );
    });
    if (focusedName) visited.add(focusedName);
  }

  for (const name of [
    "Skip",
    "Finish",
    "Reset",
    "Base Root",
    "Browse...",
    "Change",
  ]) {
    expect(visited.has(name), `keyboard did not reach ${name}`).toBe(true);
  }

  await page.getByRole("button", { name: "Finish" }).focus();
  await page.keyboard.press("Shift+Tab");
  await expect(page.getByRole("button", { name: "Skip" })).toBeFocused();
});

test("startup shell prevents focus from entering the hidden application", async ({
  page,
}) => {
  await mockWailsRuntime(page);
  await page.addInitScript(() => {
    localStorage.clear();
    localStorage.setItem("i18nextLng", "en_US");
  });
  await page.route("**/assets/OnboardingPage-*.js", async (route) => {
    const response = await route.fetch();
    await new Promise((resolve) => setTimeout(resolve, 800));
    await route.fulfill({ response });
  });

  await page.goto("/#/onboarding");
  await expect(page.getByRole("status")).toBeVisible();
  await expect(page.locator("[data-startup-content][inert]")).toHaveCount(1);

  await page.keyboard.press("Tab");
  const focusEnteredHiddenApplication = await page.evaluate(() =>
    Boolean(document.activeElement?.closest("[data-startup-content][inert]")),
  );
  expect(focusEnteredHiddenApplication).toBe(false);

  await expect(
    page.getByRole("heading", { name: "First Launch Setup" }),
  ).toBeVisible();
  await expect(page.locator("[data-startup-content][inert]")).toHaveCount(0);
});

test("unknown routes recover to home without adding redirect history", async ({
  page,
}) => {
  await mockWailsRuntime(page);
  await seedCompletedSetup(page);

  await page.goto("/#/definitely-unknown");
  await expect(page).toHaveURL(/\/#\/$/);
  await expect(page.locator("main")).toBeVisible();
});

for (const theme of ["light", "dark"] as const) {
  test(`home meets automated WCAG checks in ${theme} theme`, async ({
    page,
  }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await mockWailsRuntime(page);
    await seedCompletedSetup(page);
    await page.addInitScript((selectedTheme) => {
      localStorage.setItem("theme", selectedTheme);
    }, theme);

    await page.goto("/#/");
    await expect(page.locator("main")).toBeVisible();
    await expect(page.locator("[data-startup-content][inert]")).toHaveCount(0);
    const launchButton = page.getByTestId("primary-launch-button");
    await expect(launchButton).toHaveCSS("color", "rgb(255, 255, 255)");

    await expectNoUnexpectedAccessibilityViolations(page);
  });
}

test("dependency warning dialog meets automated WCAG checks", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockWailsRuntime(page, {
    "minecraft.IsGameInputInstalled": false,
  });
  await seedCompletedSetup(page);

  await page.goto("/#/");
  const dialog = page.getByRole("dialog", {
    name: "GameInput Component Missing",
  });
  await expect(dialog).toBeVisible();

  const results = await new AxeBuilder({ page })
    .include('[role="dialog"]')
    .withTags(["wcag2a", "wcag2aa", "wcag22aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});

test("onboarding meets automated WCAG checks and allows zoom", async ({
  page,
}) => {
  await mockWailsRuntime(page);
  await page.addInitScript(() => {
    localStorage.clear();
    localStorage.setItem("i18nextLng", "en_US");
  });

  await page.goto("/#/onboarding");
  await expect(
    page.getByRole("heading", { name: "First Launch Setup" }),
  ).toBeVisible();

  await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
  const viewport = await page
    .locator('meta[name="viewport"]')
    .getAttribute("content");
  expect(viewport).not.toContain("user-scalable=no");
  expect(viewport).not.toContain("maximum-scale=1");

  await expectNoUnexpectedAccessibilityViolations(page);
});

test("system reduced-motion preference disables application animation", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockWailsRuntime(page);
  await page.addInitScript(() => {
    localStorage.clear();
    localStorage.setItem("i18nextLng", "en_US");
  });

  await page.goto("/#/onboarding");
  const finishButton = page.getByRole("button", { name: "Finish" });
  await expect(finishButton).toBeVisible();

  const durations = await finishButton.evaluate((element) => {
    const styles = getComputedStyle(element);
    return {
      animation: styles.animationDuration,
      transition: styles.transitionDuration,
    };
  });
  expect(durations.animation).toBe("0s");
  expect(durations.transition).toBe("0s");
});
