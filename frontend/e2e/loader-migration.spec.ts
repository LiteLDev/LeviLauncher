import { expect, test } from "@playwright/test";
import en from "../src/assets/locales/en_US.json" with { type: "json" };
import { ROUTES } from "../src/constants/routes";
import { mockWailsRuntime, seedCompletedSetup } from "./support/mockWails";

test("failed loader migration remains visible until acknowledged", async ({ page }) => {
  const error = "migrate example: Access is denied.";
  await mockWailsRuntime(page, {
    "minecraft.NeedsLoaderMigration": true,
    "minecraft.RunLoaderMigration": error,
  });
  await seedCompletedSetup(page);
  await page.goto("/#" + ROUTES.home);

  const dialog = page.getByRole("dialog", { name: en.common.error, exact: true });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(error, { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: en.common.close, exact: true }).click();
  await expect(dialog).not.toBeVisible();
});

test("successful loader migration closes its progress dialog", async ({ page }) => {
  await mockWailsRuntime(page, {
    "minecraft.NeedsLoaderMigration": true,
    "minecraft.RunLoaderMigration": "",
  });
  await seedCompletedSetup(page);
  await page.goto("/#" + ROUTES.home);

  await expect(page.getByTestId("primary-launch-button")).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
