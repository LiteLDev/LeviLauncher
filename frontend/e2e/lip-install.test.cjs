// Run against npm run dev:ui-review. The catalog scenario's instance has no
// LeviLamina, so installing a package must install it first, in place.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const path = require('node:path');
const translations = require('../src/assets/locales/zh_CN.json');
const { lip_install: install } = require('../src/assets/locales/fragments/lip/zh_CN.json');
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

const fill = (text, values) => text.replace(/\{\{(\w+)\}\}/g, (_, key) => values[key]);

async function run(label, check) {
  const context = await browser.newContext({ viewport: { width: 1024, height: 720 } });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=catalog#/lip`);
    await page.getByText('Preview package 01', { exact: true }).waitFor({ timeout: 20000 });
    await check(page);
    assert.deepEqual(errors, []);
    console.log(`PASS ${label}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `failed-${label}.png`) });
    throw error;
  } finally {
    await context.close();
  }
}

const installCalls = page => page.evaluate(() =>
  window.__audit.calls.filter(call => /^Install/.test(call.name)).map(call => [call.name, ...call.args]));

(async () => {
  browser = await chromium.launch();
  try {
    await run('catalog-install-with-levilamina', async page => {
      // Catalog -> Install -> confirm: no page change, LeviLamina first.
      await page.getByRole('button', { name: fill(install.quick_install_aria, { name: 'Preview package 01' }) }).click();
      const dialog = page.getByRole('dialog', { name: fill(install.title, { name: 'Preview package 01' }) });
      await dialog.getByText(fill(install.ll_will_install, { version: '0.2.0' })).waitFor();
      await dialog.getByRole('button', { name: fill(install.confirm_with_ll, { action: translations.lip.files.install }) }).click();
      await dialog.getByText(fill(install.success_summary, { name: 'Preview package 01', version: '1.0.0', instance: 'UI test instance' })).waitFor();
      const steps = await dialog.locator('ol li').evaluateAll(items => items.map(item => item.dataset.status));
      assert.deepEqual(steps, ['success', 'success']);
      assert.deepEqual(await installCalls(page), [
        ['InstallLeviLamina', '1.21.0', 'UI test instance', '0.2.0'],
        ['InstallLIPPackage', 'UI test instance', 'fixture/package-1', '1.0.0'],
      ]);
      assert.equal(new URL(page.url()).hash, '#/lip');
      // The finished run stays until the user closes it.
      await page.waitForTimeout(1000);
      await dialog.getByRole('button', { name: translations.common.done, exact: true }).click();
      await dialog.waitFor({ state: 'detached' });
    });

    await run('package-version-row-and-back', async page => {
      await page.getByRole('textbox').first().fill('package 2');
      await page.getByRole('button', { name: translations.common.search, exact: true }).click();
      const link = page.getByRole('link', { name: /Preview package 29/ });
      await link.scrollIntoViewIfNeeded();
      const scrollTop = await page.getByTestId('catalog-results').evaluate(element => element.scrollTop);
      await link.click();
      await page.getByRole('heading', { name: 'Preview package 29' }).waitFor();

      // Installing an older release from its row keeps that release.
      await page.getByRole('tab', { name: translations.lip.files.tab_label }).click();
      await page.getByRole('button', { name: `${translations.lip.files.install} Preview package 29 0.9.0` }).click();
      const dialog = page.getByRole('dialog');
      await dialog.getByText(fill(install.ll_will_install, { version: '0.2.0' })).waitFor();
      await page.waitForTimeout(300);
      await dialog.getByRole('button', { name: fill(install.confirm_with_ll, { action: translations.lip.files.install }) }).click();
      await dialog.getByText(fill(install.success_summary, { name: 'Preview package 29', version: '0.9.0', instance: 'UI test instance' })).waitFor();
      assert.deepEqual((await installCalls(page)).at(-1), ['InstallLIPPackage', 'UI test instance', 'fixture/package-29', '0.9.0']);
      await dialog.getByRole('button', { name: translations.common.done, exact: true }).click();
      await dialog.waitFor({ state: 'detached' });

      // Back restores the search and the scroll position.
      await page.goBack();
      await page.getByText('Preview package 29', { exact: true }).waitFor();
      assert.equal(await page.getByRole('textbox').first().inputValue(), 'package 2');
      assert.equal(await page.getByTestId('catalog-results').evaluate(element => element.scrollTop), scrollTop);
    });
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
