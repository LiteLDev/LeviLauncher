// Run against `npm run dev:ui-review`; PLAYWRIGHT_MODULE may point to a bundled runtime.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const outputDir = process.env.UI_REVIEW_OUTPUT;
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const report = [];
let browser;

async function runScenario(viewport, scenario) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  const pageErrors = [];
  page.on('pageerror', error => pageErrors.push(error.message));
  const takeScreenshot = async (step) => {
    if (outputDir) await page.screenshot({ path: path.join(outputDir, `uwp-registration-${scenario}-${step}-${viewport.width}.png`) });
  };
  try {
    await page.goto(`${baseURL}?scenario=${scenario}#/`);
    const primary = page.getByTestId('primary-launch-button');
    await primary.waitFor();
    await page.waitForTimeout(2200);
    const initiallyRegistered = ['uwp', 'uwp-registration-lost', 'gdk-launch'].includes(scenario);
    assert.equal((await primary.innerText()).trim(), initiallyRegistered ? '启动' : '注册');
    await takeScreenshot('initial');
    await primary.click();

    if (initiallyRegistered) {
      assert.equal((await primary.innerText()).trim(), '启动中…');
      assert.equal(await primary.isDisabled(), true);
      await takeScreenshot('launch-pending');
      await page.waitForTimeout(1100);
      const calls = await page.evaluate(() => window.__audit.calls);
      assert.equal(calls.filter(call => call.name === 'LaunchVersionByName').length, 1);
      assert.equal(calls.filter(call => call.name === 'RegisterVersionWithWdapp').length, 0);
      if (scenario === 'uwp-registration-lost') {
        assert.equal((await primary.innerText()).trim(), '注册');
        assert.ok((await page.getByRole('dialog').innerText()).includes('尚未注册到系统'));
        await takeScreenshot('lost-registration');
      }
      report.push({ viewport, scenario, status: 'passed', actions: calls.filter(call => /^(LaunchVersion|RegisterVersion|UnregisterVersion)/.test(call.name)), pageErrors });
      return;
    }

    assert.equal((await primary.innerText()).trim(), '注册中…');
    assert.equal(await primary.isDisabled(), true);
    await page.getByRole('dialog').filter({ hasText: '正在注册到系统' }).waitFor();
    await takeScreenshot('register-pending');
    if (scenario !== 'uwp-register-fail') {
      // Native registration has returned, but the real-state confirmation remains pending.
      await page.waitForTimeout(1100);
      assert.equal((await primary.innerText()).trim(), '注册中…');
      const calls = await page.evaluate(() => window.__audit.calls);
      assert.ok(calls.some(call => call.name === 'ListVersionMetasWithRegistered'));
      assert.equal(calls.filter(call => call.name === 'LaunchVersionByName').length, 0);
    }

    const succeeds = scenario === 'uwp-unregistered' || scenario === 'uwp-stale-register';
    const dialog = page.getByRole('dialog').filter({ hasText: succeeds ? '注册完成' : '注册失败' });
    await dialog.waitFor();
    assert.equal((await primary.innerText()).trim(), succeeds ? '启动' : '注册');
    await takeScreenshot(succeeds ? 'registered' : 'register-failed');
    await dialog.getByRole('button', { name: '关闭', exact: true }).click();
    await page.waitForTimeout(scenario === 'uwp-stale-register' ? 2300 : 300);
    assert.equal((await primary.innerText()).trim(), succeeds ? '启动' : '注册');
    const beforeLaunch = await page.evaluate(() => window.__audit.calls);
    assert.equal(beforeLaunch.filter(call => call.name === 'RegisterVersionWithWdapp').length, 1);
    assert.equal(beforeLaunch.filter(call => call.name === 'UnregisterVersionByName').length, 0);
    assert.equal(beforeLaunch.filter(call => call.name === 'LaunchVersionByName').length, 0);
    await takeScreenshot('ready');

    if (succeeds) {
      await primary.click();
      assert.equal((await primary.innerText()).trim(), '启动中…');
      assert.equal(await primary.isDisabled(), true);
      await takeScreenshot('launch-pending');
      await page.waitForTimeout(1100);
      const calls = await page.evaluate(() => window.__audit.calls);
      assert.equal(calls.filter(call => call.name === 'LaunchVersionByName').length, 1);
      assert.equal(calls.filter(call => call.name === 'RegisterVersionWithWdapp').length, 1);
    }

    assert.deepEqual(pageErrors, []);
    const finalCalls = await page.evaluate(() => window.__audit.calls);
    report.push({ viewport, scenario, status: 'passed', actions: finalCalls.filter(call => /^(LaunchVersion|RegisterVersion|UnregisterVersion)/.test(call.name)), pageErrors });
  } catch (error) {
    await takeScreenshot('failed-assertion');
    report.push({ viewport, scenario, status: 'failed', error: String(error), text: await page.locator('body').innerText(), calls: await page.evaluate(() => window.__audit.calls), pageErrors });
    throw error;
  } finally {
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) {
    for (const scenario of process.env.UI_REVIEW_SCENARIOS?.split(',') || ['uwp-unregistered', 'uwp-register-fail', 'uwp-register-unconfirmed', 'uwp-stale-register', 'uwp', 'uwp-registration-lost', 'gdk-launch']) {
      await runScenario(viewport, scenario);
      console.log(`PASS ${viewport.width}x${viewport.height} ${scenario}`);
    }
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
}).finally(async () => {
  if (outputDir) fs.writeFileSync(path.join(outputDir, 'uwp-registration-report.json'), JSON.stringify(report, null, 2));
  if (browser) await browser.close();
});
