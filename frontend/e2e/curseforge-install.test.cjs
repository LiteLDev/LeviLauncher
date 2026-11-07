// Run against npm run dev:ui-review. CurseForge downloads and imports use fixture APIs.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const translations = require('../src/assets/locales/zh_CN.json');
const install = require('../src/assets/locales/fragments/curseforge/zh_CN.json').curseforge.install;
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

const steps = page => page.getByTestId('curseforge-install-steps');
const dialogButton = (page, name) => page.locator('[role="dialog"]').getByRole('button', { name, exact: true });

async function run(label, viewport, route, check) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=catalog#${route}`);
    await page.locator('main').waitFor({ timeout: 20000 });
    await check(page);
    assert.deepEqual(errors, []);
    console.log(`PASS ${label}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `curseforge-failed-${label}.png`) });
    throw error;
  } finally {
    await context.close();
  }
}

const assertNoHorizontalScroll = async page => {
  const { scrollWidth, innerWidth } = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth }));
  assert.ok(scrollWidth <= innerWidth, `page scrolls horizontally: ${scrollWidth} > ${innerWidth}`);
};

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || undefined, headless: true });

  for (const viewport of [{ width: 640, height: 480 }, { width: 3840, height: 2160 }]) {
    await run(`fits-${viewport.width}`, viewport, '/curseforge', async page => {
      await page.getByTestId('catalog-quick-install').first().waitFor();
      await assertNoHorizontalScroll(page);
      await page.goto(`${baseURL}?scenario=catalog#/curseforge/mod/3`);
      await page.getByTestId('curseforge-mod-install').waitFor();
      await assertNoHorizontalScroll(page);
      await page.getByRole('tab', { name: translations.curseforge.mod_tabs.files }).click();
      await assertNoHorizontalScroll(page);
    });
  }

  await run('quick-install-keeps-steps', { width: 640, height: 480 }, '/curseforge', async page => {
    await page.evaluate(() => Object.assign(window.__audit, { cfHoldDownload: true, cfDuplicate: true }));
    await page.getByTestId('catalog-quick-install').first().click();
    // The newest release for the instance is a world, so the player is asked up front.
    await page.getByTestId('curseforge-install-dialog').getByText(translations.curseforge.install.player_label, { exact: true }).waitFor();
    await page.getByTestId('curseforge-install-confirm').click();
    await page.waitForFunction(() => typeof window.__audit.cfFinishDownload === 'function');
    assert.equal(await steps(page).locator('li[data-status="active"]').count(), 1);
    await page.evaluate(() => window.__audit.cfFinishDownload());
    // The duplicate question appears inline, below the finished steps.
    await steps(page).locator('li[data-status="waiting"]').waitFor();
    assert.equal(await steps(page).locator('li[data-status="done"]').count(), 2);
    await dialogButton(page, install.overwrite).click();
    await page.getByText(install.done_success.replace('{{name}}', 'Fixture World 1.2.0').replace('{{instance}}', 'UI test instance')).waitFor();
    assert.equal(await steps(page).locator('li[data-status="done"]').count(), 3);
    const imports = await page.evaluate(() => window.__audit.cfImports);
    assert.deepEqual(imports.map(call => call.args.at(-1)), [false, true]);
    assert.deepEqual(imports[1].args.slice(0, 2), ['UI test instance', 'player-one']);
    await page.waitForTimeout(3500);
    assert.equal(await steps(page).locator('li').count(), 3);
    await dialogButton(page, translations.curseforge.install.close).click();
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
  });

  await run('keep-existing', { width: 1024, height: 640 }, '/curseforge/mod/3', async page => {
    await page.evaluate(() => { window.__audit.cfDuplicate = true; });
    await page.getByTestId('curseforge-mod-install').click();
    await page.getByTestId('curseforge-install-confirm').click();
    await dialogButton(page, install.keep_existing).click();
    await page.getByText(install.done_kept).waitFor();
    assert.equal(await steps(page).locator('li[data-status="skipped"]').count(), 1);
    assert.equal((await page.evaluate(() => window.__audit.cfImports)).length, 1);
  });

  await run('import-error-in-place', { width: 1024, height: 640 }, '/curseforge/mod/3', async page => {
    await page.evaluate(() => { window.__audit.cfImportError = 'ERR_FIXTURE_IMPORT'; });
    await page.getByTestId('curseforge-mod-install').click();
    await page.getByTestId('curseforge-install-confirm').click();
    await steps(page).locator('li[data-status="error"]').waitFor();
    await page.waitForTimeout(3500);
    assert.equal(await steps(page).locator('li[data-status="done"]').count(), 2);
    assert.ok(await page.getByRole('alert').filter({ hasText: 'ERR_FIXTURE_IMPORT' }).isVisible());
  });

  await run('cancel-download', { width: 1024, height: 640 }, '/curseforge/mod/3', async page => {
    await page.evaluate(() => { window.__audit.cfHoldDownload = true; });
    await page.getByTestId('curseforge-mod-install').click();
    await page.getByTestId('curseforge-install-confirm').click();
    await page.waitForFunction(() => typeof window.__audit.cfFinishDownload === 'function');
    await dialogButton(page, translations.common.cancel).click();
    await page.waitForFunction(() => window.__audit.cfCancelled === 1);
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
  });

  await run('back-restores-catalog', { width: 1024, height: 640 }, '/curseforge', async page => {
    await page.getByLabel(translations.curseforge.search_placeholder).first().fill('package 1');
    await page.getByRole('button', { name: translations.curseforge.search, exact: true }).click();
    await page.getByText('Preview package 19').waitFor();
    // The list resets its scroll right after a search lands.
    await page.waitForTimeout(600);
    await page.getByTestId('catalog-results').evaluate(el => { el.scrollTop = 200; });
    const link = page.locator('[data-testid="catalog-result-list"] a').nth(4);
    await link.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    const before = await page.getByTestId('catalog-results').evaluate(el => el.scrollTop);
    assert.ok(before > 0);
    await link.click();
    await page.getByTestId('curseforge-mod-install').waitFor();
    await page.goBack();
    await page.getByTestId('catalog-quick-install').first().waitFor();
    await page.waitForTimeout(400);
    assert.equal(await page.getByLabel(translations.curseforge.search_placeholder).first().inputValue(), 'package 1');
    const after = await page.getByTestId('catalog-results').evaluate(el => el.scrollTop);
    assert.ok(Math.abs(after - before) < 5, `scroll ${before} -> ${after}`);
  });

  await browser.close();
})().catch(async error => {
  console.error(error);
  if (browser) await browser.close();
  process.exit(1);
});
