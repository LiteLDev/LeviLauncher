import { expect, test, type Page } from "@playwright/test";
import { ROUTES, routeTo } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

const packageRoute = "/#" + routeTo.lipPackage("example/test-mod");
const readme = `<div align="center"><img src="./logo.png" alt="Mod logo" width="96" /><h1>Test Mod</h1></div>

[Features](#features)

## Features

**Formatted introduction** with a [relative link](./docs/guide.md).

<details><summary>More information</summary>HTML content works.</details>

<script>window.__readmeScriptRan = true</script>
<img src="./logo.png" alt="Unsafe image" onerror="window.__readmeScriptRan = true" />
<a href="javascript:alert(1)">Unsafe link</a>

## Features

Second section.
`;

async function setupPackage(
  page: Page,
  options: {
    readme?: string;
    mapping?: boolean;
    chinese?: boolean;
    instance?: boolean;
  } = {},
) {
  await mockWailsRuntime(page, {
    "minecraft.GetLipStatus": { installed: true, upToDate: true },
    "minecraft.GetLIPPackageReadme": options.readme ?? readme,
    "versionservice.ListVersionMetas": options.instance
      ? [{ name: "Test instance", gameVersion: "1.21.100" }]
      : [],
    "minecraft.FetchLeviLaminaVersionDB":
      options.mapping === false
        ? {}
        : {
            "1.21.100": ["1.2.0"],
            "1.21.90": ["1.1.0"],
          },
  });
  await seedCompletedSetup(page);
  if (options.chinese) {
    await page.addInitScript(() => {
      localStorage.setItem("i18nextLng", "zh_CN");
      localStorage.setItem("theme", "dark");
      localStorage.setItem("app.themeMode", "dark");
    });
  }
  await page.route("**/levilauncher.json", (route) =>
    route.fulfill({
      json: {
        packages: {
          "example/test-mod": {
            info: {
              name: "Test Mod",
              description: "Fallback introduction",
              tags: ["utility"],
            },
            variants: {
              client: {
                label: "client",
                versions: {
                  "3.0.0": {
                    dependencies: {
                      "liteldev/levilamina#client": ">=1.2.0",
                      "example/library": "^1.0.0",
                    },
                  },
                  "2.0.0": {
                    dependencies: {
                      "liteldev/levilamina#client": ">=1.1.0 <1.3.0",
                    },
                  },
                  "1.0.0": {
                    dependencies: { "liteldev/levilamina#client": "1.1.0" },
                  },
                  "0.9.0": {
                    dependencies: { "liteldev/levilamina#client": "9.0.0" },
                  },
                  "0.5.0": { dependencies: {} },
                },
              },
            },
          },
        },
      },
    }),
  );
  await page.route("https://raw.githubusercontent.com/**", (route) =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96"><rect width="96" height="96" fill="#37a866"/></svg>',
    }),
  );
  await page.goto(packageRoute);
  await expect(
    page.getByRole("heading", { name: "Test Mod", exact: true }).first(),
  ).toBeVisible();
}

test("Bedrinth renders sanitized README HTML and anchors preserve routing", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await setupPackage(page);
  const description = page.getByRole("tabpanel");
  await expect(
    description.getByRole("img", { name: "Mod logo", exact: true }),
  ).toHaveAttribute(
    "src",
    "https://raw.githubusercontent.com/example/test-mod/HEAD/logo.png",
  );
  await expect(description.locator("strong")).toHaveText(
    "Formatted introduction",
  );
  await expect(
    description.getByRole("link", { name: "relative link" }),
  ).toHaveAttribute(
    "href",
    "https://github.com/example/test-mod/blob/HEAD/docs/guide.md",
  );
  await expect(
    description.locator("script, [onerror], [href^='javascript:']"),
  ).toHaveCount(0);
  await description.getByText("More information", { exact: true }).click();
  await expect(
    description.getByText("HTML content works.", { exact: false }),
  ).toBeVisible();
  await description
    .getByRole("link", { name: "Features", exact: true })
    .click();
  await expect(page).toHaveURL(
    new RegExp(packageRoute.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "$"),
  );
  await expect(description.locator("#user-content-features")).toBeVisible();
  await expect(description.locator("#user-content-features-1")).toBeAttached();
  await page.getByRole("tab", { name: "Versions", exact: true }).click();
  await page.getByRole("tab", { name: "Details", exact: true }).click();
  await expect(description.locator("#user-content-features")).toBeAttached();
  expect(errors).toEqual([]);
  await page.screenshot({ path: ".artifacts/bedrinth-readme.png" });
});

test("Bedrinth groups versions by MC and LL and filters multi-version releases", async ({
  page,
}) => {
  await setupPackage(page);
  await page.getByRole("tab", { name: "Versions", exact: true }).click();
  const panel = page.getByRole("tabpanel");
  const groups = panel.locator("div.space-y-3 > details");
  await expect(groups).toHaveCount(6);
  await expect(groups.first().locator("summary").first()).toContainText(
    "Minecraft 1.21.100",
  );
  await expect(
    groups.first().getByText("Test Mod 3.0.0", { exact: true }),
  ).toBeVisible();
  await expect(panel.getByText("Test Mod 2.0.0", { exact: true })).toHaveCount(
    2,
  );
  await panel.getByRole("button", { name: "1.21.90", exact: true }).click();
  await expect(groups).toHaveCount(3);
  await expect(panel.getByText("Test Mod 3.0.0", { exact: true })).toHaveCount(
    0,
  );
  await expect(panel.getByText("Test Mod 2.0.0", { exact: true })).toHaveCount(
    1,
  );
  await expect(panel.getByText("Test Mod 0.5.0", { exact: true })).toHaveCount(
    1,
  );
  const summary = groups.first().locator("summary").first();
  await summary.focus();
  await page.keyboard.press("Enter");
  await expect(groups.first()).not.toHaveAttribute("open");
  await page.keyboard.press("Enter");
  await expect(groups.first()).toHaveAttribute("open", "");
  await page.setViewportSize({ width: 800, height: 600 });
  await expect
    .poll(() => panel.evaluate((el) => el.scrollWidth <= el.clientWidth + 1))
    .toBe(true);
  await page.screenshot({ path: ".artifacts/bedrinth-versions-compact.png" });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.screenshot({ path: ".artifacts/bedrinth-versions.png" });
});

test("Bedrinth keeps unmapped versions visible and falls back to the summary", async ({
  page,
}) => {
  await setupPackage(page, { readme: "", mapping: false });
  await expect(page.getByRole("tabpanel")).toContainText(
    "Fallback introduction",
  );
  await page.getByRole("tab", { name: "Versions", exact: true }).click();
  const panel = page.getByRole("tabpanel");
  await expect(
    panel
      .getByText("Game version mapping unavailable", { exact: true })
      .first(),
  ).toBeVisible();
  await expect(
    panel.getByRole("button", { name: "Install Test Mod 3.0.0", exact: true }),
  ).toBeDisabled();
  await expect(
    panel.getByText("Test Mod 0.9.0", { exact: true }),
  ).toBeAttached();
});

test("download filters default to release and remember explicit selections", async ({
  page,
}) => {
  await mockWailsRuntime(page, {
    "minecraft.FetchHistoricalVersions": {
      releaseVersions: [
        {
          version: "Release 1.21.100.01",
          urls: ["https://example.invalid/game.msixvc"],
        },
      ],
      previewVersions: [
        {
          version: "Preview 1.21.110.01",
          urls: ["https://example.invalid/preview.msixvc"],
        },
      ],
    },
    "minecraft.FetchLeviLaminaVersionDB": { "1.21.100.01": ["1.2.0"] },
  });
  await seedCompletedSetup(page);
  await page.goto("/#" + ROUTES.download);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await expect(page.locator("tbody")).toContainText("1.21.100.01");
  await page.getByRole("button", { name: "Types", exact: true }).click();
  await page.getByRole("menuitemradio", { name: "All", exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.reload();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.getByRole("button", { name: "Loader", exact: true }).click();
  await page
    .getByRole("menuitemradio", { name: "LeviLamina", exact: true })
    .click();
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "Status", exact: true }).click();
  await page
    .getByRole("menuitemradio", { name: "Not downloaded", exact: true })
    .click();
  await page.goto("/#" + ROUTES.lip);
  await page.goto("/#" + ROUTES.download);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.reload();
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "Loader", exact: true }).click();
  await expect(
    page.getByRole("menuitemradio", { name: "LeviLamina", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Status", exact: true }).click();
  await expect(
    page.getByRole("menuitemradio", { name: "Not downloaded", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
});

test("grouped install opens the selected release and Chinese dark layout fits", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await setupPackage(page, { chinese: true, instance: true });
  await page.getByRole("tab", { name: "版本", exact: true }).click();
  const panel = page.getByRole("tabpanel");
  await expect(
    panel.getByRole("button", { name: "安装 Test Mod 3.0.0", exact: true }),
  ).toBeEnabled();
  await page.screenshot({ path: ".artifacts/bedrinth-versions-zh-dark.png" });
  await panel
    .getByRole("button", { name: "安装 Test Mod 3.0.0", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toContainText("3.0.0");
  await expect(page.getByRole("dialog")).toContainText("Test instance");
});

for (const stored of [
  "broken JSON",
  '{"type":"removed","loader":"unknown","status":42}',
]) {
  test(`invalid saved download filters recover: ${stored}`, async ({
    page,
  }) => {
    await mockWailsRuntime(page, {
      "minecraft.FetchHistoricalVersions": {
        releaseVersions: [{ version: "Release 1.21.100.01", urls: [] }],
        previewVersions: [{ version: "Preview 1.21.110.01", urls: [] }],
      },
    });
    await seedCompletedSetup(page);
    await page.addInitScript(
      (value) => localStorage.setItem("download.filters", value),
      stored,
    );
    await page.goto("/#" + ROUTES.download);
    await expect(page.locator("tbody tr")).toHaveCount(1);
    await expect(page.locator("tbody")).toContainText("1.21.100.01");
  });
}
