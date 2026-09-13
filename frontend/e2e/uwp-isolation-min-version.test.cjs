// Run against `npm run dev:ui-review`; tests only fixture APIs.
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

async function run(viewport, mode, version, supported) {
  const context = await browser.newContext({ viewport });
  await context.addInitScript(({ mode, version, instances, download }) => {
    localStorage.setItem('ll.install.enableIsolation', 'true');
    const usr = mode === 'settings'
      ? { name: 'UWP release instance', tab: 'launch', returnTo: instances }
      : { mirrorVersion: mode === 'local' ? '' : version, mirrorType: 'Release', packageType: 'uwp', returnTo: download,
          installerPath: mode === 'local' ? 'C:\\Fixture\\local.appx' : '' };
    history.replaceState({ usr, key: 'minimum-version', idx: 0 }, '');
  }, { mode, version, instances: ROUTES.instances, download: ROUTES.download });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    const route = mode === 'settings' ? ROUTES.instanceSettings : ROUTES.install;
    await page.goto(`${baseURL}?scenario=uwp-isolation-min-${mode}&version=${version}#${route}`);
    const label = mode === 'settings' ? translations.versions.edit.enable_isolation : translations.downloadpage.install_folder.enable_isolation;
    const toggle = page.getByRole('switch', { name: label, exact: true });
    await toggle.waitFor();
    if (mode === 'settings') await page.getByText(version, { exact: true }).waitFor();
    // Local package versions are determined from their manifest after extraction.
    const selectable = mode === 'local' || supported;
    assert.equal(await toggle.isDisabled(), !selectable);
    assert.equal(await toggle.isChecked(), selectable);
    await page.getByText(translations.uwp.isolation_min_version.replace('{{version}}', '1.19.70.2'), { exact: true }).waitFor();
    await page.locator('label').filter({ has: toggle }).scrollIntoViewIfNeeded();
    const switchBox = await toggle.boundingBox();
    assert.ok(switchBox);
    await page.mouse.move(switchBox.x + switchBox.width / 2, switchBox.y + switchBox.height / 2);
    if (mode !== 'settings') {
      await page.mouse.wheel(0, 250);
      if (mode === 'local') await page.getByRole('textbox').first().fill('local-minimum-test');
    }
    await page.waitForTimeout(350);
    if (!selectable) {
      await page.locator('label').filter({ has: toggle }).click({ force: true });
      assert.equal(await toggle.isChecked(), false);
    }
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `minimum-${mode}-${version}-${viewport.width}.png`) });
    const save = page.getByRole('button', { name: mode === 'settings' ? translations.common.ok : translations.downloadpage.customappx.modal['1'].footer.install_button, exact: true });
    await save.click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'SaveVersionMeta'));
    const call = await page.evaluate(() => window.__audit.calls.find(call => call.name === 'SaveVersionMeta'));
    assert.equal(call.args[1], version);
    assert.equal(call.args[3], supported);
    assert.equal(await page.evaluate(() => localStorage.getItem('ll.install.enableIsolation')), 'true', 'unsupported versions must not erase the install preference');
    assert.deepEqual(errors, []);
    console.log(`PASS minimum ${mode} ${version} supported=${supported} at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `failed-minimum-${mode}-${version}-${viewport.width}.png`) });
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
    for (const mode of ['settings', 'download', 'local']) {
      for (const [version, supported] of [['1.19.70.1', false], ['1.19.70.2', true]]) {
        await run(viewport, mode, version, supported);
      }
    }
  }
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
