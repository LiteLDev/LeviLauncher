import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { ROUTES } from "../src/constants/routes";
import en from "../src/assets/locales/en_US.json" with { type: "json" };
import { APPEARANCE_KEY } from "../src/utils/backgroundAppearance";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const copy = en.settings.appearance.material;
const fixture = (fill: string) =>
  "data:image/svg+xml;base64," +
  Buffer.from(
    `<svg xmlns="http://www.w3.org/2000/svg" width="1440" height="900"><defs><pattern id="detail" width="32" height="32" patternUnits="userSpaceOnUse"><rect width="32" height="32" fill="white"/><rect width="16" height="16" fill="black"/><rect x="16" y="16" width="16" height="16" fill="black"/></pattern></defs><rect width="1440" height="900" fill="${fill}"/></svg>`,
  ).toString("base64");

const setup = async (
  page: Page,
  image: string,
  settings: Record<string, string> = {},
) => {
  await mockWailsRuntime(page, {
    "minecraft.ListDir": [
      {
        name: "wallpaper.png",
        path: "D:/wallpapers/wallpaper.png",
        isDir: false,
      },
    ],
    "minecraft.GetImageURL": image,
    "minecraft.GetImageBase64": image,
  });
  await seedCompletedSetup(page);
  await page.addInitScript(
    (values) => {
      // Seed once so reload assertions exercise the real persistence path.
      if (sessionStorage.getItem("appearance-test-seeded")) return;
      sessionStorage.setItem("appearance-test-seeded", "true");
      Object.entries(values).forEach(([key, value]) =>
        localStorage.setItem(key, value),
      );
    },
    {
      "app.backgroundImage": "D:/wallpapers",
      "app.themeMode": "light",
      "app.disableAnimations": "true",
      ...settings,
    },
  );
};

const openSettings = async (page: Page) => {
  await page.goto("/#" + ROUTES.settings);
  await page
    .getByRole("tab", { name: en.settings.tabs.personalization, exact: true })
    .click();
  const material = page.getByRole("region", { name: copy.title, exact: true });
  await material.scrollIntoViewIfNeeded();
  return material;
};

for (const theme of ["light", "dark"] as const) {
  for (const [name, image, brightness, blur] of [
    ["white", fixture("white"), "200", "0"],
    ["black", fixture("black"), "0", "0"],
    ["detail", fixture("url(#detail)"), "100", "0"],
    ["blurred", fixture("url(#detail)"), "100", "50"],
  ]) {
    test(`${theme} ${name} wallpaper keeps cards translucent and content usable`, async ({
      page,
    }) => {
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await setup(page, image, {
        "app.themeMode": theme,
        "app.backgroundBrightness": brightness,
        "app.backgroundBlur": blur,
      });
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto("/#" + ROUTES.home);
      await expect(page.locator(".app-backdrop")).toHaveAttribute(
        "data-wallpaper-active",
        "true",
      );
      const card = page.locator("#main-content .launcher-glass").first();
      await expect(card).toBeVisible();
      const appearance = await card.evaluate((element) => {
        const style = getComputedStyle(element);
        return {
          background: style.backgroundColor,
          blur: style.backdropFilter,
          opacity: style.opacity,
        };
      });
      expect(appearance.background).toMatch(/^rgba\(/);
      expect(appearance.blur).toBe("blur(12px)");
      expect(appearance.opacity).toBe("1");
      await expect(page.getByTestId("primary-launch-button")).toBeEnabled();
      const a11y = await new AxeBuilder({ page })
        .include("#main-content")
        .withTags(["wcag2a", "wcag2aa"])
        .analyze();
      expect(a11y.violations).toEqual([]);
      await page.screenshot({
        path: `.artifacts/personalization/${theme}-${name}.png`,
      });
      expect(errors).toEqual([]);
    });
  }
}

test("material presets, keyboard sliders and theme profiles persist independently", async ({
  page,
}) => {
  const image =
    "data:image/jpeg;base64," +
    (await readFile("src/assets/images/world-preview-default.jpg")).toString(
      "base64",
    );
  await setup(page, image);
  await page.setViewportSize({ width: 1280, height: 900 });
  const material = await openSettings(page);
  await expect(page.locator(".app-backdrop")).toHaveAttribute(
    "data-wallpaper-active",
    "true",
  );
  await material.getByRole("button", { name: new RegExp(copy.clear) }).click();
  await expect(
    material.getByRole("switch", { name: copy.readability }),
  ).not.toBeChecked();
  const slider = material.getByRole("slider", { name: copy.opacity });
  await slider.focus();
  await page.keyboard.press("ArrowRight");
  await expect(slider).toHaveValue("36");
  await material
    .getByRole("button", {
      name: en.settings.appearance.theme_dark,
      exact: true,
    })
    .click();
  await expect(slider).toHaveValue("60");
  await material.getByRole("button", { name: new RegExp(copy.solid) }).click();
  await expect(page.locator("html")).toHaveClass(/light/);
  await page.reload();
  await page
    .getByRole("tab", { name: en.settings.tabs.personalization, exact: true })
    .click();
  await material
    .getByRole("button", {
      name: en.settings.appearance.theme_light,
      exact: true,
    })
    .click();
  await expect(slider).toHaveValue("36");
  await material
    .getByRole("button", {
      name: en.settings.appearance.theme_dark,
      exact: true,
    })
    .click();
  await expect(slider).toHaveValue("100");
  await material.getByRole("button", { name: copy.reset }).click();
  await expect(slider).toHaveValue("60");
  await material
    .getByRole("button", {
      name: en.settings.appearance.theme_light,
      exact: true,
    })
    .click();
  await expect(slider).toHaveValue("36");
  await material.getByRole("button", { name: copy.reset }).click();
  const a11y = await new AxeBuilder({ page })
    .include(`section[aria-label="${copy.title}"]`)
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(a11y.violations).toEqual([]);
  for (const width of [960, 1440]) {
    await page.setViewportSize({ width, height: width === 960 ? 600 : 900 });
    await material.scrollIntoViewIfNeeded();
    expect(
      await material.evaluate(
        (element) => element.scrollWidth <= element.clientWidth,
      ),
    ).toBe(true);
    await material.screenshot({
      path: `.artifacts/personalization/settings-${width}.png`,
    });
  }
  await page.goto("/#" + ROUTES.home);
  await page.screenshot({ path: ".artifacts/personalization/home-light.png" });
  await page.evaluate(() => {
    localStorage.setItem("app.themeMode", "dark");
    window.dispatchEvent(new Event("app-theme-mode-changed"));
  });
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.screenshot({ path: ".artifacts/personalization/home-dark.png" });
});

for (const state of ["none", "broken", "invisible"] as const) {
  test(`${state} wallpaper falls back to ordinary surfaces`, async ({
    page,
  }) => {
    await setup(
      page,
      state === "broken" ? "data:image/png;base64,invalid" : fixture("white"),
      state === "none"
        ? { "app.backgroundImage": "" }
        : state === "invisible"
          ? { "app.backgroundOpacity": "0" }
          : {},
    );
    const material = await openSettings(page);
    await expect(
      material.getByText(copy.no_image, { exact: true }),
    ).toBeVisible();
    await expect(page.locator(".app-backdrop")).not.toHaveAttribute(
      "data-wallpaper-active",
    );
    await expect(
      page.locator("#main-content .launcher-glass").first(),
    ).toHaveCSS("backdrop-filter", "none");
  });
}

test("invalid settings and accessibility preferences use safe values", async ({
  page,
}) => {
  await setup(page, fixture("white"), {
    [APPEARANCE_KEY]: JSON.stringify({
      light: {
        surfaceOpacity: "invalid",
        surfaceBlur: -500,
        overlayOpacity: 900,
      },
    }),
    "app.backgroundBlur": "NaN",
    "app.backgroundBrightness": "Infinity",
  });
  await page.goto("/#" + ROUTES.home);
  await expect(page.locator(".app-backdrop")).toHaveAttribute(
    "data-wallpaper-active",
    "true",
  );
  const card = page.locator("#main-content .launcher-glass").first();
  await expect(card).toHaveCSS("backdrop-filter", "blur(0px)");
  await expect(page.locator(".wallpaper-image")).toHaveCSS(
    "filter",
    "blur(0px) brightness(1)",
  );
  await page.emulateMedia({ contrast: "more" });
  await expect(card).toHaveCSS("backdrop-filter", "none");
  await openSettings(page);
  await expect(
    page.getByTestId("appearance-preview").locator(".launcher-glass").first(),
  ).toHaveCSS("backdrop-filter", "none");
  await expect(
    page.getByTestId("appearance-preview").locator(".launcher-glass").first(),
  ).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await page.emulateMedia({ forcedColors: "active" });
  await expect(page.locator(".app-backdrop > .wallpaper-layers")).toBeHidden();
  await expect(card).toHaveCSS("backdrop-filter", "none");
});

test("image fitting and navbar layout share material with portal menus", async ({
  page,
}) => {
  await setup(page, fixture("url(#detail)"), { "app.layoutMode": "navbar" });
  await page.setViewportSize({ width: 960, height: 600 });
  const material = await openSettings(page);
  const previewImage = page
    .getByTestId("appearance-preview")
    .locator(".wallpaper-image");
  for (const [mode, size, repeat] of [
    ["fit", "contain", "no-repeat"],
    ["stretch", "100% 100%", "no-repeat"],
    ["tile", "auto", "repeat"],
  ]) {
    await page.evaluate((value) => {
      localStorage.setItem("app.backgroundFitMode", value);
      window.dispatchEvent(new Event("app-background-settings-changed"));
    }, mode);
    await expect(previewImage).toHaveCSS("background-size", size);
    await expect(previewImage).toHaveCSS("background-repeat", repeat);
    await expect(
      page.locator(".app-backdrop > .wallpaper-layers .wallpaper-image"),
    ).toHaveCSS("background-size", size);
  }
  await material.getByRole("button", { name: new RegExp(copy.clear) }).click();
  await page.goto("/#" + ROUTES.home);
  await expect(page.getByTestId("primary-launch-button")).toBeVisible();
  await page
    .getByRole("button", {
      name: en.launcherpage.tip.quick_actions_menu,
      exact: true,
    })
    .click();
  await expect(page.getByRole("menu")).toBeVisible();
  const translucent = await page.getByRole("menu").evaluate((element) => {
    const surface =
      element.closest('[data-slot="dropdown-content"]') || element;
    return getComputedStyle(surface).getPropertyValue(
      "--wallpaper-surface-opacity",
    );
  });
  expect(Number(translucent)).toBeCloseTo(0.35, 2);
  await page.keyboard.press("Escape");
});

test("next image updates the thumbnail and preview without reloading the page", async ({
  page,
}) => {
  await setup(page, fixture("white"), {
    "app.backgroundPlayOrder": "sequential",
  });
  const binding = await readFile(
    "bindings/github.com/liteldev/LeviLauncher/minecraft.js",
    "utf8",
  );
  const methodID = Number(
    binding
      .slice(binding.indexOf("export function ListDir("))
      .match(/\$Call\.ByID\((\d+)/)?.[1],
  );
  const imageID = Number(
    binding
      .slice(binding.indexOf("export function GetImageURL("))
      .match(/\$Call\.ByID\((\d+)/)?.[1],
  );
  await page.route("**/wails/runtime", async (route) => {
    const request = route.request().postDataJSON();
    if (Number(request?.args?.methodID) === imageID) {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          fixture(
            String(request?.args?.args?.[0]).includes("wall10")
              ? "black"
              : "white",
          ),
        ),
      });
    }
    if (Number(request?.args?.methodID) !== methodID) return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        { name: "wall10.png", path: "D:/wallpapers/wall10.png", isDir: false },
        { name: "wall2.png", path: "D:/wallpapers/wall2.png", isDir: false },
      ]),
    });
  });
  await openSettings(page);
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.currentBackgroundImage")),
    )
    .toBe("D:/wallpapers/wall2.png");
  await page
    .getByRole("button", { name: copy.next_image, exact: true })
    .click();
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.currentBackgroundImage")),
    )
    .toBe("D:/wallpapers/wall10.png");
  await expect(
    page.getByTestId("appearance-preview").locator(".wallpaper-image"),
  ).toHaveCSS("background-image", `url("${fixture("black")}")`);
  await expect(page.locator(`img[src="${fixture("black")}"]`)).toBeVisible();
  await expect(page.locator(".app-backdrop")).toHaveAttribute(
    "data-wallpaper-active",
    "true",
  );
  await page
    .getByRole("button", { name: copy.next_image, exact: true })
    .click();
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.currentBackgroundImage")),
    )
    .toBe("D:/wallpapers/wall2.png");
  await page.evaluate(() =>
    localStorage.setItem("app.backgroundPlayOrder", "random"),
  );
  await page
    .getByRole("button", { name: copy.next_image, exact: true })
    .click();
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.currentBackgroundImage")),
    )
    .toBe("D:/wallpapers/wall10.png");
  // Clearing while settings are open updates both the app and the preview.
  await page
    .getByRole("button", {
      name: en.settings.appearance.clear_image,
      exact: true,
    })
    .click();
  await expect(page.getByText(copy.no_image, { exact: true })).toBeVisible();
  await expect(page.locator(".app-backdrop .wallpaper-image")).toHaveCount(0);
});

test("a slow folder selection cannot restore an older wallpaper", async ({
  page,
}) => {
  await setup(page, fixture("white"));
  await page.goto("/#" + ROUTES.home);
  await expect(page.locator(".app-backdrop")).toHaveAttribute(
    "data-wallpaper-active",
    "true",
  );
  const binding = await readFile(
    "bindings/github.com/liteldev/LeviLauncher/minecraft.js",
    "utf8",
  );
  const methodID = Number(
    binding
      .slice(binding.indexOf("export function ListDir("))
      .match(/\$Call\.ByID\((\d+)/)?.[1],
  );
  let releaseOld: () => void = () => {};
  const oldRequest = new Promise<void>((resolve) => {
    releaseOld = resolve;
  });
  let oldPending = false;
  let oldFinished = false;
  await page.route("**/wails/runtime", async (route) => {
    const request = route.request().postDataJSON();
    if (Number(request?.args?.methodID) !== methodID) return route.fallback();
    const folder = String(request?.args?.args?.[0]);
    if (folder === "D:/old") {
      oldPending = true;
      await oldRequest;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        { name: "wall.png", path: folder + "/wall.png", isDir: false },
      ]),
    });
    if (folder === "D:/old") oldFinished = true;
  });
  const selectFolder = (folder: string) =>
    page.evaluate((value) => {
      localStorage.setItem("app.backgroundImage", value);
      window.dispatchEvent(new Event("app-background-changed"));
    }, folder);
  await selectFolder("D:/old");
  await expect.poll(() => oldPending).toBe(true);
  await selectFolder("D:/new");
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("app.currentBackgroundImage")),
    )
    .toBe("D:/new/wall.png");
  releaseOld();
  await expect.poll(() => oldFinished).toBe(true);
  // Re-selecting the same image also completes rather than leaving readiness false.
  await selectFolder("D:/new");
  await expect(page.locator(".app-backdrop")).toHaveAttribute(
    "data-wallpaper-active",
    "true",
  );
  expect(
    await page.evaluate(() =>
      localStorage.getItem("app.currentBackgroundImage"),
    ),
  ).toBe("D:/new/wall.png");
});
