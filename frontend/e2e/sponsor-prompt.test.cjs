// Run against npm run dev:ui-review. External links use fixture APIs.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');

function readConstants(name) {
  const exports = {};
  runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, `../src/constants/${name}.ts`), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText, { exports });
  return exports;
}
const { ROUTES } = readConstants('routes');
const { SPONSORSHIP_URLS } = readConstants('sponsorship');
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

async function run(viewport, { scenario = 'sponsor', count = 100, locale = 'zh_CN', theme = 'light', action = 'dismiss' } = {}) {
  const t = require(`../src/assets/locales/${locale}.json`);
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    const params = new URLSearchParams({ scenario, count, locale, theme });
    await page.goto(`${baseURL}?${params}#${ROUTES.about}`);
    const dialog = page.getByRole('dialog', { name: t.sponsor_prompt.title, exact: true });
    const beforeSponsor = {
      'sponsor-update': [t.settings.body.version.hasnew + '99.0.0', t.common.cancel],
      'sponsor-lip': [t.settings.lip.startup_prompt.title, t.settings.lip.startup_prompt.later_button],
      'sponsor-terms': [t.terms.title, t.terms.agree],
      'sponsor-clarity': [t.clarity.prompt.title, t.clarity.prompt.disable],
    }[scenario];
    if (beforeSponsor) {
      const prior = page.getByRole('dialog', { name: beforeSponsor[0], exact: true });
      await prior.waitFor();
      assert.equal(await dialog.count(), 0, 'sponsor prompt must wait for the current startup modal');
      assert.equal(await page.evaluate(() => window.__audit.calls.some(c => c.name === 'TakeSponsorPrompt')), false);
      await prior.getByRole('button', { name: beforeSponsor[1], exact: true }).click();
    }
    if (count % 100 !== 0 || count === 0) {
      await page.waitForFunction(() => window.__audit.sponsorTaken);
      assert.equal(await dialog.count(), 0, 'ordinary launches must not show a prompt');
    } else {
      await dialog.waitFor();
      await dialog.getByText(t.sponsor_prompt.body.replace('{{count}}', String(count)), { exact: true }).waitFor();
      assert.equal(await dialog.getByRole('button').count(), 3, 'only dismiss and two sponsorship buttons are offered');
      await page.waitForFunction(() => document.documentElement.classList.contains('dark') === (new URLSearchParams(location.search).get('theme') === 'dark'));
      // Inspect the dialog and all actions after the entry animation settles.
      await page.waitForTimeout(400);
      for (const target of [dialog, ...await dialog.getByRole('button').all()]) {
        const box = await target.boundingBox();
        assert.ok(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width + 1 && box.y + box.height <= viewport.height + 1,
          'sponsor dialog and buttons must fit the launcher window');
      }
      const dismiss = dialog.getByRole('button', { name: t.sponsor_prompt.dismiss, exact: true });
      assert.equal(await dismiss.evaluate(el => el === document.activeElement), true, 'the optional dismiss action receives initial focus');
      await page.keyboard.press('Tab');
      assert.equal(await dialog.getByRole('button', { name: t.about.patreon, exact: true }).evaluate(el => el === document.activeElement), true);
      await page.keyboard.press('Tab');
      assert.equal(await dialog.getByRole('button', { name: t.about.afdian, exact: true }).evaluate(el => el === document.activeElement), true);
      await page.keyboard.press('Tab');
      assert.equal(await dismiss.evaluate(el => el === document.activeElement), true, 'Tab stays inside the dialog');
      if (outputDir) await page.screenshot({ path: path.join(outputDir, `sponsor-${locale}-${theme}-${count}-${viewport.width}.png`) });

      if (scenario === 'sponsor-link-error') {
        await dialog.getByRole('button', { name: t.about.afdian, exact: true }).click();
        await page.getByText(t.sponsor_prompt.open_error, { exact: true }).waitFor();
        assert.equal(await dialog.isVisible(), true, 'a failed URL open keeps the prompt available');
        await page.evaluate(() => { window.__audit.recovered = true; });
      }
      if (action === 'escape') await page.keyboard.press('Escape');
      else if (action === 'backdrop') await page.mouse.click(10, viewport.height - 10);
      else if (action === 'dismiss') await dismiss.press('Enter');
      else await dialog.getByRole('button', { name: t.about[action], exact: true }).click();
      await dialog.waitFor({ state: 'hidden' });
      const urls = await page.evaluate(() => window.__audit.openedSponsorURLs);
      assert.deepEqual(urls, SPONSORSHIP_URLS[action] ? [SPONSORSHIP_URLS[action]] : []);
      // Route changes must not consume or display the same milestone again.
      await page.evaluate(route => { location.hash = `#${route}`; }, ROUTES.home);
      await page.waitForURL(url => url.hash === `#${ROUTES.home}`);
      assert.equal(await dialog.count(), 0);
      assert.equal(await page.evaluate(() => window.__audit.calls.filter(c => c.name === 'TakeSponsorPrompt').length), 1);
    }
    assert.deepEqual(errors, []);
    console.log(`PASS ${scenario} count=${count} ${locale} ${theme} ${action} at ${viewport.width}x${viewport.height}`);
  } catch (error) {
    console.error(await page.locator('body').innerText());
    throw error;
  } finally {
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) {
    await run(viewport);
    await run(viewport, { count: 200, theme: 'dark', action: 'afdian' });
    await run(viewport, { locale: 'en_US', action: 'patreon' });
    await run(viewport, { locale: 'de_DE', theme: 'dark', action: 'escape' });
    await run(viewport, { action: 'backdrop' });
    for (const count of [0, 99, 101, 199, 201]) await run(viewport, { count });
  }
  for (const scenario of ['sponsor-update', 'sponsor-lip', 'sponsor-terms', 'sponsor-clarity', 'sponsor-check-error', 'sponsor-link-error']) {
    await run({ width: 960, height: 600 }, { scenario, action: scenario === 'sponsor-link-error' ? 'afdian' : 'dismiss' });
  }
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
