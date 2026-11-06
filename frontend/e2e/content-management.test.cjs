// Run against npm run dev:ui-review. Native reads and writes use the preview fixture.
const assert = require("node:assert/strict");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const translations = require("../src/assets/locales/zh_CN.json");
const baseURL = process.env.UI_REVIEW_URL || "http://127.0.0.1:5174/";

async function run() {
  const browser = await chromium.launch({ channel: process.env.UI_REVIEW_BROWSER || "msedge" });
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(`${baseURL}?scenario=resource-packs#/about`);
    await page.waitForFunction(() => window.__audit?.calls?.length > 0);
    await page.evaluate(() => {
      const fixture = window.__audit;
      const original = fixture.call;
      fixture.writes = [];
      fixture.packReads = [];
      fixture.call = async (name, ...args) => {
        if (["ListPackDetailsForVersion", "ListPacksForVersion", "GetPackInfo", "GetPathModTime"].includes(name)) {
          fixture.packReads.push({ name, args });
        }
        if (name === "ListPackDetailsForVersion") {
          return [1, 2].map(index => ({
            path: `C:/Fixture/pack-${args[2]}-${index}`,
            name: `Pack ${args[2]}-${index}${fixture.packSuffix}`,
            modTime: 1700000000 + index - 1,
          }));
        }
        if (name === "ReadWorldLevelDatFields") {
          return { version: 9, fields: [
            { name: "Greeting", tag: "string", valueString: "old" },
            { name: "Payload", tag: "list", valueJSON: '"AQI="' },
          ] };
        }
        if (["SetWorldLevelName", "WriteWorldLevelDatFields", "WriteWorldLevelDatFieldsAt"].includes(name)) {
          fixture.writes.push({ name, args });
          return "";
        }
        return original(name, ...args);
      };
    });
    const navigate = route => page.evaluate(hash => { location.hash = hash; }, route);
    const date = await page.evaluate(() => new Date(1700000000 * 1000).toLocaleString());
    for (const [route, kind, player] of [
      ["/content/resourcePacks", 6, ""],
      ["/content/behaviorPacks", 4, ""],
      ["/content/skinPacks", 7, "player-one"],
    ]) {
      await page.evaluate(() => { window.__audit.packSuffix = ""; });
      await navigate(route);
      const heading = suffix => page.getByRole("heading", { name: `Pack ${kind}-${suffix}`, exact: true });
      await heading("1").waitFor();
      await heading("2").waitFor();
      await page.getByText(date, { exact: true }).waitFor();
      await page.getByText("1.00 MB", { exact: true }).first().waitFor();
      await page.evaluate(() => {
        window.__audit.packReads = [];
        window.__audit.packSuffix = " refreshed";
      });
      await page.getByRole("button", { name: translations.common.refresh, exact: true }).click();
      await heading("2 refreshed").waitFor();
      assert.deepEqual(await page.evaluate(() => window.__audit.packReads), [{
        name: "ListPackDetailsForVersion", args: ["UI test instance", player, kind],
      }]);
    }

    await navigate("/content/worlds/worldEdit?path=C%3A%2FFixture%2Fhealthy");
    const save = page.getByRole("button", { name: translations.common.save, exact: true });
    const greeting = page.getByRole("textbox", { name: "Greeting", exact: true });
    await greeting.waitFor();
    await page.getByPlaceholder("My World").fill("Renamed");
    await save.click();
    await page.waitForFunction(() => window.__audit.writes.length === 1);
    await greeting.fill("new");
    await save.click();
    await page.waitForFunction(() => window.__audit.writes.length === 2);
    const writes = await page.evaluate(() => window.__audit.writes);
    assert.deepEqual(writes.map(write => write.name), ["SetWorldLevelName", "WriteWorldLevelDatFields"]);
    assert.deepEqual(writes[1].args[1], {
      version: 9, fields: [{ name: "Greeting", tag: "string", valueString: "new" }],
    });
    assert.deepEqual(errors, []);
    console.log("One metadata call per refresh for resources, behaviors and skins; dates, sizes and changed-field saves passed.");
  } finally {
    await browser.close();
  }
}

run().catch(error => { console.error(error); process.exitCode = 1; });
