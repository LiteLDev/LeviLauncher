// Run against npm run dev:ui-review. lip tasks replay fixture events that
// stop at each pause until window.__audit.lipContinue() is called.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');
const translations = require('../src/assets/locales/zh_CN.json');
const instanceStrings = require('../src/assets/locales/fragments/instances/zh_CN.json').instances;
const llInstallTitle = instanceStrings.ll.dialog_title.replace('{{action}}', translations.lip.files.install);
const routes = {};
runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, '../src/constants/routes.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText, { exports: routes });
const { ROUTES } = routes;
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
const viewport = { width: 1024, height: 720 };
const progress = translations.lip.task_progress;
let browser;

async function shot(page, name) {
  if (!outputDir) return;
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(outputDir, `${name}.png`) });
}

async function paused(page, count) {
  await page.waitForFunction(n => window.__audit.lipPaused >= n, count);
}

async function open(page, scenario, route, usr) {
  if (usr) await page.context().addInitScript(state => history.replaceState({ usr: state, key: 'lip-progress', idx: 0 }, ''), usr);
  await page.goto(`${baseURL}?scenario=${scenario}#${route}`);
}

async function run(label, check) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
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

(async () => {
  browser = await chromium.launch();
  try {
    await run('levilamina-install-inline', async page => {
      await open(page, 'lip-progress', ROUTES.instanceSettings, { name: 'UI test instance', tab: 'loader', returnTo: ROUTES.instances });
      await page.getByRole('button', { name: translations.lip.files.install, exact: true }).click();
      // Picking the version and confirming happen in one dialog.
      const confirm = page.getByRole('dialog', { name: llInstallTitle, exact: true });
      await confirm.getByRole('button', { name: translations.lip.files.install, exact: true }).click();

      // Solving: indeterminate primary bar counting manifests, a download of
      // unknown length.
      await paused(page, 1);
      await confirm.getByText(progress.stage_solve, { exact: true }).waitFor();
      await confirm.getByText(progress.amount_unknown.replace('{{unit}}', progress.unit_manifests).replace('{{current}}', '7')).waitFor();
      assert.equal(await confirm.locator('[role="progressbar"]:not([aria-valuenow])').count() >= 2, true, 'unknown totals must be indeterminate');
      await shot(page, 'll-install-inline-indeterminate');

      // Applying: packages 1 of 3, an object count and a byte count.
      await page.evaluate(() => window.__audit.lipContinue());
      await paused(page, 2);
      await confirm.getByText(progress.stage_task, { exact: true }).waitFor();
      await confirm.getByText('3.40 MB / 9.10 MB').waitFor();
      await confirm.getByText(progress.amount.replace('{{unit}}', progress.unit_objects).replace('{{current}}', '212').replace('{{total}}', '480')).waitFor();
      assert.equal(await page.locator('[role="dialog"]').count(), 1, 'inline progress must not open another dialog');
      await shot(page, 'll-install-inline-determinate');

      await page.evaluate(() => window.__audit.lipContinue());
      await confirm.waitFor({ state: 'detached' });
    });

    await run('levilamina-remove-progress-and-error', async page => {
      await open(page, 'lip-progress', ROUTES.instanceSettings, { name: 'UI test instance', tab: 'loader', returnTo: ROUTES.instances });
      // Install first (no pauses checked) so that the remove button exists.
      await page.getByRole('button', { name: translations.lip.files.install, exact: true }).click();
      await page.getByRole('dialog', { name: llInstallTitle, exact: true })
        .getByRole('button', { name: translations.lip.files.install, exact: true }).click();
      await paused(page, 1);
      await page.evaluate(() => window.__audit.lipContinue());
      await paused(page, 2);
      await page.evaluate(() => window.__audit.lipContinue());

      await page.getByRole('button', { name: translations.common.remove, exact: true }).click();
      const removeDialog = page.getByRole('dialog', { name: translations.versions.edit.loader.ll_remove_confirm_title, exact: true });
      await page.evaluate(() => { window.__audit.lipFail = true; });
      await removeDialog.getByRole('button', { name: translations.common.remove, exact: true }).click();

      // The progress dialog opens over the pending confirmation.
      await paused(page, 3);
      const title = `${translations.lip.task_console.title_uninstall} - UI test instance`;
      const progressDialog = page.getByRole('dialog', { name: title, exact: true });
      await progressDialog.getByText(progress.stage_remove, { exact: true }).waitFor();
      await progressDialog.getByText(progress.amount_unknown.replace('{{unit}}', progress.unit_files).replace('{{current}}', '152')).waitFor();
      assert.equal(await progressDialog.evaluate(element => !!element.closest('[inert]')), false, 'the progress dialog must be the active one');
      await shot(page, 'll-remove-progress-modal');
      await progressDialog.getByRole('button', { name: progress.show_log, exact: true }).click();
      await progressDialog.getByText('Fixture task Uninstall', { exact: false }).waitFor();
      await shot(page, 'll-remove-progress-modal-log');

      await page.evaluate(() => window.__audit.lipContinue());
      await progressDialog.getByText(translations.lip.task_console.status_failed, { exact: true }).waitFor();
      await progressDialog.getByRole('button', { name: translations.lip.task_console.copy_logs, exact: true }).waitFor();
      await shot(page, 'll-remove-error-console');
    });

    await run('install-page-levilamina-step', async page => {
      await open(page, 'lip-progress', ROUTES.install, {
        mirrorVersion: '1.21.0', mirrorType: 'Release', packageType: 'gdk', isLeviLaminaSupported: true,
        installerPath: 'C:\\Fixture\\installers\\Release 1.21.0.msixvc', returnTo: ROUTES.download,
      });
      const llSwitch = page.getByRole('switch', { name: translations.downloadpage.install.levilamina_label });
      await llSwitch.waitFor();
      await llSwitch.press('Space');
      await page.getByRole('button', { name: translations.downloadpage.customappx.modal['1'].footer.install_button, exact: true }).click();
      await paused(page, 1);
      await page.getByText(progress.stage_solve, { exact: true }).waitFor();
      await page.evaluate(() => window.__audit.lipContinue());
      await paused(page, 2);
      await page.getByText(progress.stage_task, { exact: true }).waitFor();
      await page.getByText('3.40 MB / 9.10 MB').waitFor();
      assert.equal(await page.locator('[role="dialog"]').count(), 0);
      await shot(page, 'install-page-levilamina-step');
      await page.evaluate(() => window.__audit.lipContinue());
      await page.getByText(translations.downloadpage.install.success, { exact: true }).waitFor();
    });

    await run('mods-batch-uninstall-outer', async page => {
      await open(page, 'lip-progress', ROUTES.mods);
      await page.getByText('Fixture mod A', { exact: true }).first().waitFor();
      await page.getByRole('checkbox', { name: translations.common.select_all }).first().click({ force: true });
      await page.getByRole('button', { name: translations.mods.action_uninstall, exact: true }).click();
      const confirm = page.getByRole('dialog', { name: translations.mods.batch_uninstall_title, exact: true });
      await confirm.getByRole('button', { name: translations.common.confirm, exact: true }).click();

      const title = `${translations.lip.task_console.title_uninstall} - UI test instance`;
      const progressDialog = page.getByRole('dialog', { name: title, exact: true });
      for (const [index, mod] of ['Fixture mod A', 'Fixture mod B', 'Fixture mod C'].entries()) {
        await paused(page, index + 1);
        // Outer count: item i of N, its bar at the items finished.
        await progressDialog.getByText(progress.outer.replace('{{current}}', String(index + 1)).replace('{{total}}', '3'), { exact: true }).waitFor();
        await progressDialog.getByText(mod, { exact: true }).first().waitFor();
        await progressDialog.getByText(progress.amount.replace('{{unit}}', progress.unit_files).replace('{{current}}', '30').replace('{{total}}', '120')).waitFor();
        if (index === 1) await shot(page, 'mods-batch-uninstall-outer');
        await page.evaluate(() => window.__audit.lipContinue());
      }
      await progressDialog.waitFor({ state: 'detached' });
      assert.equal(await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'DeleteMod').length), 3);
    });

    await run('mods-batch-uninstall-plain-delete', async page => {
      // Only Fixture mod A runs a lip task; B and C are deleted directly.
      await open(page, 'lip-progress-delete', ROUTES.mods);
      await page.getByText('Fixture mod A', { exact: true }).first().waitFor();
      await page.getByRole('checkbox', { name: translations.common.select_all }).first().click({ force: true });
      await page.getByRole('button', { name: translations.mods.action_uninstall, exact: true }).click();
      const confirm = page.getByRole('dialog', { name: translations.mods.batch_uninstall_title, exact: true });
      await confirm.getByRole('button', { name: translations.common.confirm, exact: true }).click();

      const title = `${translations.lip.task_console.title_uninstall} - UI test instance`;
      const progressDialog = page.getByRole('dialog', { name: title, exact: true });
      await paused(page, 1);
      await progressDialog.getByText(progress.stage_task, { exact: true }).waitFor();
      await page.evaluate(() => window.__audit.lipContinue());
      await paused(page, 2);
      await progressDialog.getByText(progress.outer.replace('{{current}}', '2').replace('{{total}}', '3'), { exact: true }).waitFor();
      // The late final update of mod A's task (sent 30 ms after its call
      // returned) must not show up under mod B.
      await page.waitForTimeout(300);
      assert.equal(await progressDialog.getByText(progress.stage_task, { exact: true }).count(), 0);
      // No lip task runs for mod B: no "waiting for the package manager".
      assert.equal(await progressDialog.getByText(translations.lip.task_console.waiting_start, { exact: true }).count(), 0);
      await shot(page, 'mods-batch-uninstall-plain-delete');
      await page.evaluate(() => window.__audit.lipContinue());
      await paused(page, 3);
      await page.evaluate(() => window.__audit.lipContinue());
      await progressDialog.waitFor({ state: 'detached' });
    });

    await run('mods-single-uninstall-plain-delete', async page => {
      await open(page, 'lip-progress-delete', ROUTES.mods);
      await page.getByRole('button', { name: translations.audit.mods.more_actions.replace('{{name}}', 'Fixture mod B'), exact: true }).click();
      await page.getByRole('menuitem', { name: translations.mods.action_uninstall }).click();
      const confirm = page.getByRole('dialog', { name: translations.mods.action_uninstall, exact: true });
      await confirm.getByRole('button', { name: translations.common.delete, exact: true }).click();
      await paused(page, 1);
      // A plain folder delete has nothing to measure: no progress dialog.
      await page.waitForTimeout(300);
      const title = `${translations.lip.task_console.title_uninstall} - UI test instance`;
      assert.equal(await page.getByRole('dialog', { name: title, exact: true }).count(), 0);
      await page.evaluate(() => window.__audit.lipContinue());
      await confirm.waitFor({ state: 'detached' });
      assert.equal(await page.getByText(translations.lip.task_console.waiting_start, { exact: true }).count(), 0);
    });

    await run('cache-clean-then-mods-uninstall', async page => {
      // The cache clean does not lock navigation; a lip session started
      // meanwhile must survive the cache clean finishing first.
      await open(page, 'lip-progress', ROUTES.settings, { tab: 'components' });
      await page.getByText(translations.settings.lip.title, { exact: true }).first().waitFor().catch(async () => {
        await page.getByRole('tab', { name: translations.settings.tabs.components }).click();
      });
      await page.getByRole('button', { name: translations.settings.lip.cache_clean_button, exact: true }).click();
      await paused(page, 1);
      await page.evaluate(route => { location.hash = '#' + route; }, ROUTES.mods);
      await page.getByText('Fixture mod A', { exact: true }).first().waitFor();
      await page.getByRole('checkbox', { name: translations.common.select_all }).first().click({ force: true });
      await page.getByRole('button', { name: translations.mods.action_uninstall, exact: true }).click();
      const confirm = page.getByRole('dialog', { name: translations.mods.batch_uninstall_title, exact: true });
      await confirm.getByRole('button', { name: translations.common.confirm, exact: true }).click();
      const title = `${translations.lip.task_console.title_uninstall} - UI test instance`;
      const progressDialog = page.getByRole('dialog', { name: title, exact: true });
      await paused(page, 2);
      const files = progress.amount.replace('{{unit}}', progress.unit_files).replace('{{current}}', '30').replace('{{total}}', '120');
      await progressDialog.getByText(files).waitFor();
      // Resumes the cache clean, which then finishes.
      await page.evaluate(() => window.__audit.lipContinue());
      await page.waitForTimeout(300);
      await progressDialog.getByText(files).waitFor({ timeout: 1000 });
      for (let index = 0; index < 3; index++) {
        await page.evaluate(() => window.__audit.lipContinue());
        if (index < 2) await paused(page, 3 + index);
      }
      await progressDialog.waitFor({ state: 'detached' });
      assert.equal(await page.evaluate(() => window.__audit.calls.filter(call => call.name === 'DeleteMod').length), 3);
    });

    await run('cache-clean-inline', async page => {
      await open(page, 'lip-progress', ROUTES.settings, { tab: 'components' });
      await page.getByText(translations.settings.lip.title, { exact: true }).first().waitFor().catch(async () => {
        await page.getByRole('tab', { name: translations.settings.tabs.components }).click();
      });
      await page.getByRole('button', { name: translations.settings.lip.cache_clean_button, exact: true }).click();
      await paused(page, 1);
      await page.getByText(progress.stage_cache_clean, { exact: true }).waitFor();
      await page.getByText(progress.amount.replace('{{unit}}', progress.unit_files).replace('{{current}}', '412').replace('{{total}}', '1380')).waitFor();
      assert.equal(await page.locator('[role="dialog"]').count(), 0);
      await shot(page, 'cache-clean-inline');
      await page.evaluate(() => window.__audit.lipContinue());
      await page.getByText(translations.settings.lip.cache_clean_success, { exact: true }).waitFor();
      assert.equal(await page.getByText(progress.stage_cache_clean, { exact: true }).count(), 0);
      const call = await page.evaluate(() => window.__audit.calls.filter(item => item.name === 'CacheClean').length);
      assert.equal(call, 1);
    });
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
