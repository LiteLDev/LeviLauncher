// Run against `npm run dev:ui-review`; no real game or package is modified.
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

async function inViewport(locator, viewport) {
  const box = await locator.boundingBox();
  assert.ok(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width + 1 && box.y + box.height <= viewport.height + 1,
    'control must remain inside the minimum window');
}

async function run(viewport) {
  const context = await browser.newContext({ viewport });
  await context.addInitScript(({ returnTo }) => {
    history.replaceState({ usr: { name: 'UWP release instance', tab: 'launch', returnTo }, key: 'uwp-features', idx: 0 }, '');
  }, { returnTo: ROUTES.instances });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=uwp-features#${ROUTES.instanceSettings}`);
    const toggle = page.getByRole('switch', { name: translations.versions.edit.enable_console });
    await toggle.waitFor();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'GetVersionMeta'));
    const save = page.getByRole('button', { name: translations.common.ok, exact: true });
    await inViewport(toggle, viewport);
    await inViewport(save, viewport);
    const isolation = page.getByRole('switch', { name: translations.versions.edit.enable_isolation });
    await inViewport(isolation, viewport);
    assert.equal(await isolation.isChecked(), false);
    await isolation.press('Space');
    assert.equal(await isolation.isChecked(), true);
    assert.equal(await toggle.isChecked(), false);
    await page.locator('label').filter({ has: toggle }).click();
    assert.equal(await toggle.isChecked(), true);
    await page.waitForTimeout(350); // Let the switch's visual transition finish.
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `uwp-console-${viewport.width}.png`) });
    await page.getByRole('tab', { name: translations.versions.edit.tabs.features, exact: true }).click();
    const editor = page.getByRole('switch', { name: translations.versions.edit.enable_editor_mode });
    await editor.waitFor();
    await inViewport(editor, viewport);
    await inViewport(save, viewport);
    assert.equal(await editor.isChecked(), false);
    await editor.press('Space');
    assert.equal(await editor.isChecked(), true);
    if (outputDir) {
      await page.waitForTimeout(350); // Capture after the tab and switch transitions settle.
      await page.screenshot({ path: path.join(outputDir, `uwp-editor-${viewport.width}.png`) });
    }
    await save.click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'SaveVersionMeta'));
    let saved = await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'SaveVersionMeta').at(-1));
    assert.deepEqual(saved.args.slice(3), [true, true, true, '', '']);
    await page.waitForURL(url => url.hash === `#${ROUTES.instances}`);
    await page.evaluate(content => {
      history.pushState({ usr: null, key: 'uwp-content', idx: 1 }, '', `#${content}`);
      dispatchEvent(new PopStateEvent('popstate', { state: history.state }));
    }, ROUTES.content);
    const openContent = page.getByRole('button', { name: translations.common.open, exact: true });
    await openContent.waitFor();
    await page.getByText(translations.common.yes, { exact: true }).waitFor();
    assert.equal(await page.getByText(translations.uwp.shared_content, { exact: true }).count(), 0);
    await openContent.click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'OpenPathDir'));
    assert.ok((await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'OpenPathDir').at(-1).args[0])).includes('versions\\UWP release instance\\Minecraft Bedrock'));
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `uwp-isolated-content-${viewport.width}.png`) });
    await page.getByRole('button', { name: translations.contentpage.transfer_resources_button, exact: true }).click();
    let transfer = page.getByRole('dialog');
    await transfer.waitFor();
    assert.ok((await transfer.innerText()).includes('UWP beta instance'), 'an isolated Release can transfer to the Beta shared folder');
    await transfer.getByRole('button', { name: translations.common.cancel, exact: true }).click();
    // Re-open without reloading the fixture to verify persisted metadata.
    await page.evaluate(({ settings, instances }) => {
      history.pushState({ usr: { name: 'UWP release instance', tab: 'launch', returnTo: instances }, key: 'uwp-reopen', idx: 1 }, '', `#${settings}`);
      dispatchEvent(new PopStateEvent('popstate', { state: history.state }));
    }, { settings: ROUTES.instanceSettings, instances: ROUTES.instances });
    await toggle.waitFor();
    await page.waitForFunction(() => document.querySelector('input[role="switch"]')?.checked === true);
    assert.equal(await toggle.isChecked(), true);
    assert.equal(await isolation.isChecked(), true);
    await isolation.press('Space');
    await toggle.press('Space');
    await page.getByRole('tab', { name: translations.versions.edit.tabs.features, exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.isChecked(), true, 'editor mode must persist after reopening settings');
    await editor.press('Space');
    assert.equal(await editor.isChecked(), false);
    await page.getByRole('tab', { name: translations.versions.edit.tabs.loader, exact: true }).click();
    const folder = page.getByRole('button', { name: translations.downloadmodal.open_folder, exact: true });
    await folder.waitFor();
    await inViewport(folder, viewport);
    assert.ok((await page.locator('body').innerText()).includes('preload-native'));
    await folder.click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'OpenModsExplorer'));
    assert.deepEqual(await page.evaluate(() => window.__audit.calls.find(call => call.name === 'OpenModsExplorer').args), ['UWP release instance']);
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `uwp-mods-${viewport.width}.png`) });
    await save.click();
    await page.waitForFunction(() => window.__audit.calls.filter(call => call.name === 'SaveVersionMeta').length === 2);
    saved = await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'SaveVersionMeta').at(-1));
    assert.deepEqual(saved.args.slice(3), [false, false, false, '', '']);
    await page.waitForURL(url => url.hash === `#${ROUTES.instances}`);
    await page.evaluate(content => {
      history.pushState({ usr: null, key: 'uwp-shared-content', idx: 2 }, '', `#${content}`);
      dispatchEvent(new PopStateEvent('popstate', { state: history.state }));
    }, ROUTES.content);
    await page.getByText(translations.uwp.shared_content, { exact: true }).waitFor();
    await page.getByRole('button', { name: translations.contentpage.transfer_resources_button, exact: true }).click();
    transfer = page.getByRole('dialog');
    await transfer.waitFor();
    assert.ok((await transfer.innerText()).includes('UWP preview instance'));
    assert.equal((await transfer.innerText()).includes('UWP beta instance'), false, 'shared Release and Beta point at the same folder');
    await transfer.getByRole('button', { name: translations.common.cancel, exact: true }).click();
    assert.deepEqual(errors, []);
    console.log(`PASS UWP editor and console persistence, isolation, content paths and native mod controls at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `uwp-features-failed-${viewport.width}.png`) });
    console.error(await page.locator('body').innerText());
    throw error;
  } finally {
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) await run(viewport);
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
