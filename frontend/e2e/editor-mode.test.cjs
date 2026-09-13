// Run against npm run dev:ui-review. All saves use fixture APIs.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');
const translations = require('../src/assets/locales/zh_CN.json');
const routes = {};
runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, '../src/constants/routes.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText, { exports: routes });
const { ROUTES } = routes;
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

async function inViewport(locator, viewport) {
  const box = await locator.boundingBox();
  assert.ok(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width + 1 && box.y + box.height <= viewport.height + 1,
    'editor controls and minimum-version hint must fit the window');
}

async function run(viewport, tc) {
  const context = await browser.newContext({ viewport });
  const state = { name: 'UWP release instance', tab: 'features', returnTo: ROUTES.instances };
  await context.addInitScript(usr => history.replaceState({ usr, key: 'editor-minimum', idx: 0 }, ''), state);
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    const params = new URLSearchParams({ scenario: 'uwp-editor-minimum', enabled: 'true', version: tc.version, channel: tc.channel, packageType: tc.packageType });
    await page.goto(`${baseURL}?${params}#${ROUTES.instanceSettings}`);
    const toggle = page.getByRole('switch', { name: translations.versions.edit.enable_editor_mode, exact: true });
    await toggle.waitFor();
    await page.getByText(tc.version, { exact: true }).waitFor();
    const minimum = tc.channel === 'preview' ? '1.19.80.20' : '1.21.50';
    const hint = page.getByText(translations.versions.edit.editor_min_version.replace('{{version}}', minimum), { exact: true });
    await hint.waitFor();
    // Supported switches remain disabled until metadata finishes loading.
    if (tc.supported) await page.waitForFunction(() => !document.querySelector('input[role="switch"]').disabled);
    assert.equal(await toggle.isDisabled(), !tc.supported);
    assert.equal(await toggle.isChecked(), tc.supported, 'stale enabled flags must not bypass the version minimum');
    const save = page.getByRole('button', { name: translations.common.ok, exact: true });
    await inViewport(toggle, viewport);
    await inViewport(hint, viewport);
    await inViewport(save, viewport);
    if (tc.supported) {
      await toggle.press('Space');
      assert.equal(await toggle.isChecked(), false);
      await toggle.press('Space');
      assert.equal(await toggle.isChecked(), true);
    } else {
      await page.locator('label').filter({ has: toggle }).click({ force: true });
      assert.equal(await toggle.isChecked(), false);
    }
    if (outputDir) {
      await page.waitForTimeout(350);
      await page.screenshot({ path: path.join(outputDir, `editor-${tc.packageType}-${tc.channel}-${tc.version}-${viewport.width}.png`) });
    }
    await save.click();
    await page.waitForURL(url => url.hash === `#${ROUTES.instances}`);
    const saved = await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'SaveVersionMeta').at(-1));
    assert.equal(saved.args[5], tc.supported);
    await page.evaluate(({ usr, route }) => {
      history.pushState({ usr, key: 'editor-reopen', idx: 1 }, '', `#${route}`);
      dispatchEvent(new PopStateEvent('popstate', { state: history.state }));
    }, { usr: state, route: ROUTES.instanceSettings });
    await toggle.waitFor();
    await page.getByText(tc.version, { exact: true }).waitFor();
    if (tc.supported) {
      await page.waitForFunction(() => !document.querySelector('input[role="switch"]').disabled);
      assert.equal(await toggle.isChecked(), true, 'supported setting must persist after reopening');
      await toggle.press('Space');
      await save.click();
      await page.waitForURL(url => url.hash === `#${ROUTES.instances}`);
      const disabled = await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'SaveVersionMeta').at(-1));
      assert.equal(disabled.args[5], false);
    } else {
      assert.equal(await toggle.isChecked(), false);
      assert.equal(await toggle.isDisabled(), true);
    }
    assert.deepEqual(errors, []);
    console.log(`PASS editor ${tc.packageType} ${tc.channel} ${tc.version} supported=${tc.supported} at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    console.error(await page.locator('body').innerText());
    throw error;
  } finally {
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) {
    for (const [packageType, channel, version, supported] of [
      ['uwp', 'release', '1.21.49.99', false], ['uwp', 'release', '1.21.50.0', true],
      ['uwp', 'preview', '1.19.80.19', false], ['uwp', 'preview', '1.19.80.20', true],
      ['uwp', 'beta', '1.20.0.20', false], ['uwp', 'release', 'unknown', false],
      ['uwp', 'release', '1.21.119.0', true], ['gdk', 'release', '1.21.120.0', true],
      ['uwp', 'preview', '1.21.120.20', true], ['gdk', 'preview', '1.21.120.21', true],
    ]) await run(viewport, { packageType, channel, version, supported });
  }
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
