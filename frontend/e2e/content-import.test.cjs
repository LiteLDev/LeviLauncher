// Run against npm run dev:ui-review. File dialogs and imports use fixture APIs.
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

const importButton = page => page.getByRole('button', { name: translations.contentpage.import_button, exact: true });
const playerDialog = page => page.getByRole('dialog', { name: translations.contentpage.select_player_title, exact: true });
const overwriteDialog = page => page.getByRole('dialog', { name: translations.mods.overwrite_modal_title, exact: true });
const importCalls = page => page.evaluate(() => window.__audit.calls.filter(call => call.name.startsWith('Import')));

async function onlyActiveDialog(page, dialog, viewport) {
  await dialog.waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-slot="modal-backdrop"]:not([data-exiting])').length === 1);
  assert.equal(await dialog.evaluate(element => !!element.closest('[inert]')), false, 'the active prompt must accept pointer and keyboard input');
  const box = await dialog.boundingBox();
  assert.ok(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width + 1 && box.y + box.height <= viewport.height + 1,
    'the prompt and its buttons must fit the launcher window');
}

async function drop(page, batches) {
  await page.evaluate(items => {
    for (const files of items) window._wails.dispatchWailsEvent({ name: 'files-dropped', data: { files } });
  }, batches);
}

async function result(page, viewport, failed = false) {
  const dialog = page.getByRole('dialog', { name: failed ? translations.mods.summary_failed : translations.mods.summary_title_done, exact: true });
  await onlyActiveDialog(page, dialog, viewport);
  await dialog.getByRole('button', { name: translations.common.confirm, exact: true }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
  assert.equal(await importButton(page).isEnabled(), true, 'the next import must be available');
}

async function run(viewport, label, check, scenario = 'content-import') {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=${scenario}#${ROUTES.content}`);
    await importButton(page).waitFor();
    if (!scenario.startsWith('uwp')) await page.getByText('Preview player', { exact: true }).waitFor();
    await check(page);
    assert.deepEqual(errors, []);
    console.log(`PASS ${label} at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `import-failed-${label}-${viewport.width}.png`) });
    console.error(await page.locator('[role="dialog"]').allTextContents());
    throw error;
  } finally {
    await context.close();
  }
}

async function suite(viewport) {
  await run(viewport, 'choose-player', async page => {
    await importButton(page).click();
    const dialog = playerDialog(page);
    await onlyActiveDialog(page, dialog, viewport);
    if (outputDir) {
      await page.waitForTimeout(400);
      await page.screenshot({ path: path.join(outputDir, `import-player-${viewport.width}.png`) });
    }
    const callCount = await page.evaluate(() => window.__audit.calls.length);
    await dialog.getByRole('button', { name: 'Second player', exact: true }).click();
    await result(page, viewport);
    assert.deepEqual(await importCalls(page), [{ name: 'ImportMcworldPath', args: ['UI test instance', 'player-two', 'C:\\Fixture\\example.mcworld', false] }]);
    const callsAfterSelection = await page.evaluate(offset => window.__audit.calls.slice(offset), callCount);
    assert.equal(callsAfterSelection[0].name, 'ImportMcworldPath', 'start importing without reloading player counts first');
    await page.getByRole('button', { name: 'Second player', exact: true }).waitFor();
  });

  await run(viewport, 'cancel-and-retry-drops', async page => {
    const first = 'C:\\Fixture\\first.mcworld';
    const ignored = 'C:\\Fixture\\ignored.mcworld';
    // The second event arrives before React can render the busy state.
    await drop(page, [[first], [ignored]]);
    let dialog = playerDialog(page);
    await onlyActiveDialog(page, dialog, viewport);
    await drop(page, [[ignored]]);
    await dialog.getByRole('button', { name: translations.common.cancel, exact: true }).click();
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
    assert.deepEqual(await importCalls(page), []);
    await drop(page, [[first]]);
    dialog = playerDialog(page);
    await onlyActiveDialog(page, dialog, viewport);
    await dialog.press('Escape');
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
    assert.deepEqual(await importCalls(page), []);
    await drop(page, [[first]]);
    dialog = playerDialog(page);
    await onlyActiveDialog(page, dialog, viewport);
    await drop(page, [[ignored]]);
    await dialog.getByRole('button', { name: 'Shared', exact: true }).press('Enter');
    await result(page, viewport);
    assert.deepEqual(await importCalls(page), [{ name: 'ImportMcworldPath', args: ['UI test instance', 'Shared', first, false] }]);
  });

  await run(viewport, 'batch-overwrite', async page => {
    const files = ['example.mcworld', 'resource.mcpack', 'addon.mcaddon'].map(name => `C:\\Fixture\\${name}`);
    await page.evaluate(importFiles => { window.__audit.importFiles = importFiles; window.__audit.importDuplicate = true; }, files);
    await importButton(page).click();
    await onlyActiveDialog(page, playerDialog(page), viewport);
    await playerDialog(page).getByRole('button', { name: 'Preview player', exact: true }).click();
    for (const file of files) {
      const dialog = overwriteDialog(page);
      await dialog.getByText(path.win32.basename(file), { exact: true }).waitFor();
      await onlyActiveDialog(page, dialog, viewport);
      await drop(page, [['C:\\Fixture\\ignored.mcworld']]);
      if (outputDir && file.endsWith('.mcworld')) {
        await page.waitForTimeout(400);
        await page.screenshot({ path: path.join(outputDir, `import-overwrite-${viewport.width}.png`) });
      }
      await dialog.getByRole('button', { name: translations.mods.overwrite_and_import, exact: true }).click();
    }
    await result(page, viewport);
    const names = ['ImportMcworldPath', 'ImportMcpackPathWithPlayer', 'ImportMcaddonPathWithPlayer'];
    assert.deepEqual(await importCalls(page), files.flatMap((file, index) => [false, true].map(overwrite => ({
      name: names[index], args: ['UI test instance', 'player-one', file, overwrite],
    }))));
  });

  await run(viewport, 'skip-overwrite', async page => {
    const files = ['example.mcworld', 'resource.mcpack', 'addon.mcaddon'].map(name => `C:\\Fixture\\${name}`);
    await page.evaluate(importFiles => { window.__audit.importFiles = importFiles; window.__audit.importDuplicate = true; }, files);
    await importButton(page).click();
    await playerDialog(page).getByRole('button', { name: 'Second player', exact: true }).click();
    for (const [index, file] of files.entries()) {
      const dialog = overwriteDialog(page);
      await dialog.getByText(path.win32.basename(file), { exact: true }).waitFor();
      await onlyActiveDialog(page, dialog, viewport);
      if (index === 1) await dialog.press('Escape');
      else await dialog.getByRole('button', { name: translations.common.cancel, exact: true }).click();
    }
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
    assert.equal((await importCalls(page)).length, 3);
    assert.ok((await importCalls(page)).every(call => call.args.at(-1) === false));
    assert.equal(await importButton(page).isEnabled(), true);
    const calls = await page.evaluate(() => window.__audit.calls);
    const lastImportIndex = calls.findLastIndex(call => call.name.startsWith('Import'));
    assert.ok(calls.slice(lastImportIndex + 1).some(call => call.name === 'ListDir' && call.args[0].includes('player-two')),
      'refresh the chosen player counts even when every duplicate was skipped');
  });

  await run(viewport, 'skin-player', async page => {
    await drop(page, [['C:\\Fixture\\skin.mcpack']]);
    await onlyActiveDialog(page, playerDialog(page), viewport);
    await playerDialog(page).getByRole('button', { name: 'Second player', exact: true }).click();
    await result(page, viewport);
    assert.deepEqual(await importCalls(page), [{ name: 'ImportMcpackPathWithPlayer', args: ['UI test instance', 'player-two', 'C:\\Fixture\\skin.mcpack', false] }]);
  });

  await run(viewport, 'failed-import-retry', async page => {
    await page.evaluate(() => { window.__audit.importError = 'ERR_INVALID_ARCHIVE'; });
    await importButton(page).click();
    await playerDialog(page).getByRole('button', { name: 'Preview player', exact: true }).click();
    await result(page, viewport, true);
    await page.evaluate(() => { window.__audit.importError = ''; });
    await importButton(page).click();
    await playerDialog(page).getByRole('button', { name: 'Preview player', exact: true }).click();
    await result(page, viewport);
    assert.equal((await importCalls(page)).length, 2);
  });

  await run(viewport, 'uwp-no-player', async page => {
    await importButton(page).click();
    await result(page, viewport);
    assert.deepEqual(await importCalls(page), [{ name: 'ImportMcworldPath', args: ['UWP release instance', '', 'C:\\Fixture\\example.mcworld', false] }]);
  }, 'uwp-content-import');
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) await suite(viewport);
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
