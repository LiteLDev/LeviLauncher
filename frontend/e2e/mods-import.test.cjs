// Run against npm run dev:ui-review. The mod file picker and imports use fixture APIs.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const translations = require('../src/assets/locales/zh_CN.json');
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
const viewport = { width: 1024, height: 640 };
let browser;

const importButton = page => page.getByRole('button', { name: translations.mods.import_button, exact: true });
const progressDialog = page => page.getByRole('dialog', { name: translations.mods.importing_title, exact: true });
const importCalls = page => page.evaluate(() => window.__audit.calls.filter(call => call.name.startsWith('Import')).map(call => call.name));
const progressText = (done, total) => translations.mods.importing_count.replace('{{done}}', done).replace('{{total}}', total);
const release = async (page, finish = false) => {
  await page.waitForFunction(() => typeof window.__audit.releaseImport === 'function');
  await page.evaluate(finish => {
    const resolve = window.__audit.releaseImport;
    window.__audit.releaseImport = null;
    if (finish) window.__audit.holdImport = false;
    resolve();
  }, finish);
};

async function closeResult(page, title) {
  const dialog = page.getByRole('dialog', { name: title, exact: true });
  await dialog.waitFor();
  await dialog.getByRole('button', { name: translations.common.confirm, exact: true }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);
  return dialog;
}

async function run(label, check) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto(`${baseURL}?scenario=mods-import#/mods`);
    await importButton(page).waitFor();
    await check(page);
    assert.deepEqual(errors, []);
    console.log(`PASS ${label}`);
  } catch (error) {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `mods-import-failed-${label}.png`) });
    throw error;
  } finally {
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || undefined, headless: true });

  await run('picker-filters', async page => {
    await page.evaluate(() => { window.__audit.modFiles = []; });
    await importButton(page).click();
    await page.waitForFunction(() => window.__audit.calls.some(call => call.name === 'OpenModFiles'));
    const [options] = await page.evaluate(() => window.__audit.calls.find(call => call.name === 'OpenModFiles').args);
    assert.deepEqual(options.Filters, [
      { DisplayName: translations.mods.file_filter_mcmod, Pattern: '*.mcmod' },
      { DisplayName: translations.mods.file_filter_all, Pattern: '*.*' },
    ]);
  });

  await run('routing-and-progress', async page => {
    await page.evaluate(() => Object.assign(window.__audit, { holdImport: true, modFiles: ['C:\\Fixture\\first.mcmod', 'C:\\Fixture\\legacy.dll', 'C:\\Fixture\\notes.txt', 'C:\\Fixture\\second.zip'] }));
    await importButton(page).click();
    await progressDialog(page).getByText(progressText(0, 4)).waitFor();
    await progressDialog(page).getByText('first.mcmod').waitFor();
    await release(page);
    const dll = page.getByRole('dialog', { name: translations.mods.dll_modal_title, exact: true });
    await dll.getByRole('button', { name: translations.common.confirm, exact: true }).click();
    await progressDialog(page).getByText(progressText(1, 4)).waitFor();
    await release(page);
    // notes.txt is reported without an import call; second.zip is file 4.
    await progressDialog(page).getByText(progressText(3, 4)).waitFor();
    await release(page, true);
    const dialog = page.getByRole('dialog', { name: translations.mods.summary_title_partial, exact: true });
    await dialog.getByText(translations.mods.err_unsupported_file).waitFor();
    await closeResult(page, translations.mods.summary_title_partial);
    assert.deepEqual(await importCalls(page), ['ImportModZipPath', 'ImportModDllPath', 'ImportModZipPath']);
  });

  await run('only-unsupported', async page => {
    await page.evaluate(() => { window.__audit.modFiles = ['C:\\Fixture\\readme.txt']; });
    await importButton(page).click();
    await closeResult(page, translations.mods.summary_failed);
    assert.deepEqual(await importCalls(page), []);
  });

  await run('overwrite-keeps-count', async page => {
    await page.evaluate(() => Object.assign(window.__audit, { importDuplicate: true, modFiles: ['C:\\Fixture\\a.mcmod', 'C:\\Fixture\\b.mcmod'] }));
    await importButton(page).click();
    const dup = page.getByRole('dialog', { name: translations.mods.overwrite_modal_title, exact: true });
    await dup.waitFor();
    await page.evaluate(() => { window.__audit.holdImport = true; });
    await dup.getByRole('button', { name: translations.common.confirm, exact: true }).click();
    await progressDialog(page).getByText(progressText(0, 2)).waitFor();
    await page.evaluate(() => { window.__audit.importDuplicate = false; });
    await release(page, true);
    await closeResult(page, translations.mods.summary_title_done);
  });

  await run('drop-while-busy-is-ignored', async page => {
    await page.evaluate(() => { window.__audit.holdImport = true; });
    await page.evaluate(() => window._wails.dispatchWailsEvent({ name: 'files-dropped', data: { files: ['C:\\Fixture\\one.mcmod'] } }));
    await progressDialog(page).getByText(progressText(0, 1)).waitFor();
    await page.evaluate(() => window._wails.dispatchWailsEvent({ name: 'files-dropped', data: { files: ['C:\\Fixture\\two.mcmod'] } }));
    await release(page, true);
    await closeResult(page, translations.mods.summary_title_done);
    assert.deepEqual(await importCalls(page), ['ImportModZipPath']);
  });

  await run('escape-on-prompts-releases-import', async page => {
    // Escape closes the prompt through onOpenChange only; the import must
    // treat it as a cancel and finish, or later imports stay blocked.
    await page.evaluate(() => { window.__audit.modFiles = ['C:\Fixture\\legacy.dll']; });
    await importButton(page).click();
    const dll = page.getByRole('dialog', { name: translations.mods.dll_modal_title, exact: true });
    await dll.waitFor();
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);

    await page.evaluate(() => Object.assign(window.__audit, { importDuplicate: true, modFiles: ['C:\Fixture\\a.mcmod'] }));
    await importButton(page).click();
    const dup = page.getByRole('dialog', { name: translations.mods.overwrite_modal_title, exact: true });
    await dup.waitFor();
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => document.querySelectorAll('[role="dialog"]').length === 0);

    await page.evaluate(() => Object.assign(window.__audit, { importDuplicate: false, modFiles: ['C:\Fixture\\b.mcmod'] }));
    await importButton(page).click();
    await closeResult(page, translations.mods.summary_title_done);
    assert.deepEqual(await importCalls(page), ['ImportModZipPath', 'ImportModZipPath']);
  });
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
