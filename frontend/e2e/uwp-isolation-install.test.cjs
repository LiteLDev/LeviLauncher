// Run against `npm run dev:ui-review`; only fixture APIs are called.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');
const translations = require('../src/assets/locales/zh_CN.json');
const routeExports = {};
runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, '../src/constants/routes.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText, { exports: routeExports });
const { ROUTES } = routeExports;
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

async function run(viewport, scenario, enabled) {
  const context = await browser.newContext({ viewport });
  await context.addInitScript(({ returnTo, local }) => {
    localStorage.setItem('ll.install.enableIsolation', 'false');
    history.replaceState({ usr: {
      mirrorVersion: '1.21.100.0', mirrorType: 'Release', packageType: 'uwp', returnTo,
      installerPath: local ? 'C:\\Fixture\\local-preview.appx' : '',
    }, key: 'uwp-install', idx: 0 }, '');
  }, { returnTo: ROUTES.download, local: scenario === 'uwp-isolation-local' });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=${scenario}#${ROUTES.install}`);
    const isolation = page.getByRole('switch', { name: translations.downloadpage.install_folder.enable_isolation });
    await isolation.waitFor();
    assert.equal(await isolation.isChecked(), false);
    await isolation.scrollIntoViewIfNeeded();
    if (enabled) await isolation.press('Space');
    assert.equal(await isolation.isChecked(), enabled);
    await page.locator('label').filter({ has: isolation }).hover();
    await page.mouse.wheel(0, 250);
    await page.waitForTimeout(350); // Finish the switch and page-entry transitions before visual checks.
    const box = await isolation.boundingBox();
    assert.ok(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width && box.y + box.height <= viewport.height);
    assert.equal(await page.getByText(translations.downloadpage.install_folder.inherit_label, { exact: true }).count(), 0);
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `${scenario}-${enabled}-${viewport.width}.png`) });
    await page.getByRole('button', { name: translations.downloadpage.customappx.modal['1'].footer.install_button, exact: true }).click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'SaveVersionMeta'));
    const saved = await page.evaluate(() => window.__audit.calls.find(call => call.name === 'SaveVersionMeta'));
    assert.deepEqual(saved.args, ['1.21.100.0', scenario === 'uwp-isolation-local' ? '1.21.80.3' : '1.21.100.0',
      scenario === 'uwp-isolation-local' ? 'preview' : 'release', enabled, false, false, '', '']);
    if (scenario === 'uwp-isolation-save-error') {
      await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'DeleteVersionFolder'));
      assert.deepEqual(await page.evaluate(() => window.__audit.calls.find(call => call.name === 'DeleteVersionFolder').args), ['1.21.100.0']);
      assert.equal(await page.getByText(translations.downloadpage.install.success, { exact: true }).count(), 0);
    } else {
      await page.getByText(translations.downloadpage.install.success, { exact: true }).waitFor();
      assert.equal(await page.evaluate(() => window.__audit.uwpMetas.find(meta => meta.name === '1.21.100.0').enableIsolation), enabled);
      assert.equal(await page.evaluate(() => window.__audit.calls.some(call => call.name === 'DeleteVersionFolder' || call.name.startsWith('CopyVersionData'))), false);
    }
    assert.deepEqual(errors, []);
    console.log(`PASS ${scenario} isolation=${enabled} at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `failed-${scenario}-${enabled}-${viewport.width}.png`) });
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
    for (const scenario of ['uwp-isolation-download', 'uwp-isolation-local']) {
      for (const enabled of [false, true]) await run(viewport, scenario, enabled);
    }
    await run(viewport, 'uwp-isolation-save-error', true);
  }
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
