// Run against npm run dev:ui-review.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const baseURL = process.env.UI_REVIEW_URL || 'http://127.0.0.1:5174/';
const outputDir = process.env.UI_REVIEW_OUTPUT;
let browser;

async function checkLogo(page, shell, viewport, label) {
  await shell.waitFor({ timeout: 10000 });
  const logo = shell.getByRole('img', { name: 'LeviLauncher logo' });
  await logo.waitFor({ timeout: 5000 });
  await logo.evaluate(image => image.decode());
  assert.ok(await logo.evaluate(image => image.naturalWidth > 0 && image.naturalHeight > 0),
    'the startup logo must load and decode');
  const box = await logo.boundingBox();
  assert.ok(box && box.width === 48 && box.height === 48,
    'the startup logo retains its 48px size');
  assert.ok(box.x >= 0 && box.y >= 0 && box.x + box.width <= viewport.width && box.y + box.height <= viewport.height,
    'the startup logo fits the launcher window');
  if (outputDir) await page.screenshot({ path: path.join(outputDir, `${label}-${viewport.width}.png`) });
}

async function run(viewport, failStartup) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  let releaseApp;
  const appGate = new Promise(resolve => { releaseApp = resolve; });
  // Keep the real bootloader visible before any application module can finish loading.
  await page.route(/\/src\/App\.tsx(?:\?|$)/, async route => {
    await appGate;
    if (failStartup) await route.abort();
    else await route.continue();
  });
  try {
    await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
    const loading = page.getByRole('status').filter({ hasText: 'LeviLauncher' });
    await checkLogo(page, loading, viewport, 'startup-loading');
    releaseApp();
    if (failStartup) {
      const error = page.getByRole('alert');
      await checkLogo(page, error, viewport, 'startup-error');
      assert.equal(await error.getByRole('button').isVisible(), true,
        'startup failures still offer a reload action');
    } else {
      await loading.waitFor({ state: 'hidden', timeout: 30000 });
    }
    console.log(`PASS startup logo ${failStartup ? 'failure' : 'success'} at ${viewport.width}x${viewport.height}`);
  } finally {
    releaseApp();
    await context.close();
  }
}

(async () => {
  if (outputDir) fs.mkdirSync(outputDir, { recursive: true });
  browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || 'msedge', headless: true });
  for (const viewport of [{ width: 960, height: 600 }, { width: 1024, height: 640 }]) {
    await run(viewport, false);
    await run(viewport, true);
  }
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
});
