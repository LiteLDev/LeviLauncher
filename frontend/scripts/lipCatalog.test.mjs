// The launcher's port of the levimc.org/mods catalogue logic
// (src/utils/lipCatalog.ts) against the shared fixture: the same JSON bytes
// the site's own tests, the Go lipindex tests and LeviLauncher-Android read. The
// expected values are the site's (mods/src/lib/{catalog-query,search}.ts) on
// these bytes.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const source = readFileSync(new URL("../src/utils/lipCatalog.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const lip = {};
runInNewContext(outputText, { exports: lip });

const testdata = new URL("../../internal/lipindex/testdata/", import.meta.url);
const readJSON = (path) => JSON.parse(readFileSync(new URL(path, testdata), "utf8"));
const entries = lip.parseLipCatalog(readJSON("index.json"));
const detail = (id) => lip.parseLipModDetail(readJSON(`${id}/index.json`));

// Plain copies: values made in the vm context have other prototypes.
const plain = (value) => JSON.parse(JSON.stringify(value));
const ANDROID = ["github.com/MiracleForest/iAutomaticFishing", "github.com/MiracleForest/iInfiniteNightVision"];
const ids = (list) => plain(list.map((e) => e.id));
const run = (state = {}, q = "", list = entries, extra) =>
  lip.runQuery(list, { ...lip.DEFAULT_QUERY_STATE, ...state, q }, lip.searchCatalog(list, q), extra);

test("the catalogue parses in index order", () => {
  assert.equal(entries.length, 111);
  assert.equal(entries[0].id, "github.com/LiteLDev/Arke");
  assert.equal(entries[0].owner, "LiteLDev");
  assert.deepEqual(plain(entries[0].side), ["client"]);
  assert.deepEqual(plain(entries[0].platform), ["win-x64"]);
  assert.deepEqual(plain(entries[0].deps), { "github.com/LiteLDev/LeviLamina": "26.60.*" });
  assert.equal(entries[0].license, undefined);
  assert.ok(entries.every((e, i) => e.order === i));
  assert.equal(lip.ownerUrl("LiteLDev"), "https://github.com/LiteLDev");
});

test("the old (tooth-era) shape is rejected", () => {
  assert.deepEqual(plain(lip.parseLipCatalog({ mods: [{ tooth: "github.com/a/b", env: ["client"] }] })), []);
  assert.equal(lip.parseLipModDetail({ tooth: "github.com/a/b", versions: [] }), null);
});

test("the launcher lists what has a client entry for its platform", () => {
  const client = lip.launcherEntries(entries, "client", "win-x64");
  assert.equal(client.length, 62);
  assert.ok(client.some((e) => e.id === "github.com/LiteLDev/Arke"));
  assert.ok(!client.some((e) => e.id === "github.com/IceBlcokMC/PLand"), "server only");
  assert.deepEqual(ids(lip.launcherEntries(entries, "client", "android-arm64")), ANDROID);
  // The forced side and platform give the same set as the site's filters.
  assert.equal(run({ side: ["client"], platform: ["win-x64"] }).matched.length, 62);
});

test("the launcher lists only what has an installable variant (as Go and LeviLauncher-Android)", () => {
  assert.equal(lip.hasLauncherVariant(detail("github.com/LiteLDev/Arke")), true);
  assert.equal(lip.hasLauncherVariant(detail("github.com/LiteLDev/LegacyScriptEngine")), true);
  // PLand's latest versions run on the server only, but its versions up to
  // 0.10.0 have client entries: some version is enough (Go variantsOf).
  assert.equal(lip.hasLauncherVariant(detail("github.com/IceBlcokMC/PLand")), true);
  assert.equal(lip.hasLauncherVariant(detail("github.com/IceBlcokMC/PLand"), "client", "linux-x64"), false);
  assert.equal(lip.hasLauncherVariant(detail("github.com/LiteLDev/Arke"), "client", "android-arm64"), false);
  assert.equal(lip.hasLauncherVariant(null), false, "a detail that failed");
  // The index's side and platform are unions: client entries only for
  // Android and server entries only for Windows pass launcherEntries for
  // Windows, yet no variant is installable there.
  const split = lip.parseLipModDetail({
    ...readJSON("github.com/LiteLDev/Arke/index.json"),
    versions: [
      {
        version: "1.0.0",
        variants: {
          "": [{ side: "server", platform: "win-x64", deps: {} }],
          client: [{ side: "client", platform: "android-arm64", deps: {} }],
        },
      },
    ],
  });
  assert.ok(split);
  assert.equal(lip.hasLauncherVariant(split), false);
  assert.equal(lip.hasLauncherVariant(split, "client", "android-arm64"), true);
});

test("facet options", () => {
  const options = lip.facetOptions(entries);
  assert.deepEqual(plain(options.tags.slice(0, 20)), [
    "library", "utility", "management", "game-mechanics", "adventure", "visual", "economy", "teleport",
    "anticheat", "optimization", "technology", "protocol", "gmlib", "lse", "pland", "backpack", "building",
    "camera-editor", "carry", "collection",
  ]);
  assert.ok(!options.tags.includes("bugfix"), "a featured tag no mod has is not offered");
  assert.deepEqual(plain(options.license), [
    { family: "permissive", ids: ["MIT", "CC0-1.0", "Apache-2.0", "BSD-3-Clause", "Unlicense"] },
    { family: "weak-copyleft", ids: ["LGPL-3.0"] },
    { family: "copyleft", ids: ["GPL-3.0", "AGPL-3.0"] },
    { family: "none", ids: [] },
  ]);
  assert.deepEqual(plain(options.dep.slice(0, 3)), [
    "github.com/LiteLDev/LeviLamina", "github.com/GroupMountain/GMLIB-Release", "github.com/GroupMountain/ModAPI-Release",
  ]);
  assert.deepEqual(plain(options.author.slice(0, 3)), ["MiracleForest", "GroupMountain", "LiteLDev"]);
  assert.deepEqual(plain(options.gv.slice(0, 3)), ["1.26.60", "1.26.51", "1.26.40"]);
  assert.equal(options.gv.length, 26);
  assert.deepEqual(plain(options.platform), ["win-x64", "win-arm64", "linux-x64", "linux-arm64", "android-arm64"]);
});

test("facet counts", () => {
  const { counts, matched } = run();
  assert.equal(matched.length, 111);
  assert.deepEqual(ids(matched.slice(0, 3)), ["github.com/LiteLDev/Arke", "github.com/LiteLDev/LegacyScriptEngine", "github.com/IceBlcokMC/PLand"]);
  assert.deepEqual(
    plain(Object.entries(counts.tags).slice(0, 15)),
    [["library", 8], ["utility", 10], ["management", 2], ["game-mechanics", 6], ["adventure", 2], ["visual", 5], ["economy", 2],
      ["teleport", 2], ["anticheat", 1], ["optimization", 2], ["technology", 2], ["protocol", 1], ["gmlib", 6], ["lse", 5], ["pland", 2]],
  );
  assert.deepEqual(plain(counts.license), {
    permissive: 38, "weak-copyleft": 20, copyleft: 41, other: 0, none: 12,
    MIT: 19, "CC0-1.0": 12, "Apache-2.0": 5, "BSD-3-Clause": 1, Unlicense: 1, "LGPL-3.0": 20, "GPL-3.0": 31, "AGPL-3.0": 10,
  });
});

test("filters", () => {
  assert.deepEqual(ids(run({ tags: { inc: ["economy"], exc: [] } }).matched), ["github.com/LiteLDev/LegacyMoney", "github.com/carefly/RLXMoney"]);
  assert.equal(run({ tags: { inc: [], exc: ["library"] } }).matched.length, 103);
  assert.equal(run({ license: ["permissive"] }).matched.length, 38);
  assert.equal(run({ license: ["GPL-3.0"] }).matched.length, 31);
  assert.equal(run({ license: ["none"] }).matched.length, 12);
  assert.equal(run({ license: ["permissive", "LGPL-3.0"] }).matched.length, 58, "license values are OR");
  assert.deepEqual(ids(run({ dep: ["github.com/liteldev/legacyscriptengine"] }).matched), ["github.com/copper-lamp/LitematicaBE", "github.com/superx101/CALL"]);
  assert.equal(run({ author: ["liteldev"] }).matched.length, 7);
  assert.equal(run({ gv: ["1.21.80"] }).matched.length, 26);
  // An extra filter (the launcher's LeviLamina version) narrows results and counts alike.
  const onlyArke = run({}, "", entries, (e) => e.id === "github.com/LiteLDev/Arke");
  assert.deepEqual(ids(onlyArke.matched), ["github.com/LiteLDev/Arke"]);
  assert.equal(onlyArke.counts.license.none, 1);
  assert.equal(lip.filterCount({ ...lip.DEFAULT_QUERY_STATE, tags: { inc: ["a"], exc: ["b"] }, dep: ["x"] }), 3);
});

test("search", () => {
  assert.deepEqual(ids(run({}, "land").matched), ["github.com/IceBlcokMC/PLand", "github.com/IceBlcokMC/PLand-LegacyRemoteCallApi", "github.com/lwenk/PlandProjectileFix"]);
  assert.deepEqual(plain([...lip.searchCatalog(entries, "land").entries()].sort((a, b) => b[1] - a[1])), [
    ["github.com/IceBlcokMC/PLand", 14], ["github.com/IceBlcokMC/PLand-LegacyRemoteCallApi", 8], ["github.com/lwenk/PlandProjectileFix", 5],
  ]);
  assert.deepEqual(ids(run({}, "领地").matched), ["github.com/IceBlcokMC/PLand"]);
  // Featured labels in every language find their tags' aliases (tp, waystones).
  assert.deepEqual(ids(run({}, "teleport").matched), ["github.com/engsr6982/TeleportSystem", "github.com/MiracleForest/Bedstones"]);
  assert.deepEqual(ids(run({}, "传送").matched), ["github.com/engsr6982/TeleportSystem", "github.com/MiracleForest/Bedstones"]);
  assert.deepEqual(ids(run({}, "lse waila").matched), ["github.com/zimuya4153/LSE_Waila"], "every word is required");
  assert.equal(lip.searchCatalog(entries, "  "), null);
});

test("sorts", () => {
  assert.deepEqual(plain(lip.LIP_SORTS), ["relevance", "stars", "updated", "newest"]);
  const first = (sort) => ids(run({ sort }).matched.slice(0, 3));
  assert.deepEqual(first("stars"), ["github.com/LiteLDev/LegacyScriptEngine", "github.com/GroupMountain/GMLIB-Release", "github.com/LiteLDev/LeviOptimize"]);
  assert.deepEqual(first("updated"), ["github.com/Ieckseyr/HologramLib", "github.com/neverforward/Insight", "github.com/MRUIAW/UtilityRenderDistanceExpansion"]);
  assert.deepEqual(first("newest"), ["github.com/LiteLDev/Arke", "github.com/ch1-k-mc/BetterF3-Bedrock", "github.com/MRUIAW/UtilityRenderDistanceExpansion"]);
});

test("tags", () => {
  assert.equal(lip.normalizeTag(" Category:Utility "), "utility");
  assert.deepEqual(plain(lip.visibleTags(["platform:levilamina", "type:mod", "LeviLamina", "client", "Shader", "shader", "lse", "pland:library"])), ["shader", "lse", "pland:library"]);
  assert.equal(lip.tagKey("money"), "economy");
  assert.equal(lip.tagKey("pland:library"), "library");
  assert.equal(lip.tagKey("type:mod"), null);
  assert.deepEqual(plain(lip.tagFacet(["tp", "-teleport", "money", "mod"])), { inc: ["economy"], exc: ["teleport"] });
  const featured = lip.featuredTagsOf(["tp", "waystones", "money", "custom"]).map(({ tag, raw }) => [tag.id, [...raw]]);
  assert.deepEqual(plain(featured), [["economy", ["money"]], ["teleport", ["tp", "waystones"]]]);
  assert.deepEqual(plain(lip.plainTagsOf(["tp", "custom", "type:mod"])), ["custom"]);
  assert.deepEqual(plain(lip.FEATURED_TAGS.map((f) => f.id)), [
    "library", "utility", "management", "game-mechanics", "adventure", "visual", "economy", "teleport",
    "anticheat", "optimization", "bugfix", "technology", "protocol",
  ]);
  let facet = lip.includeTag(lip.EMPTY_FACET, "lse");
  assert.deepEqual(plain(facet), { inc: ["lse"], exc: [] });
  facet = lip.excludeTag(facet, "lse");
  assert.deepEqual(plain(facet), { inc: [], exc: ["lse"] });
  facet = lip.includeTag(facet, "lse");
  assert.deepEqual(plain(facet), { inc: ["lse"], exc: [] });
  facet = lip.includeTag(facet, "lse");
  assert.deepEqual(plain(facet), { inc: [], exc: [] });
});

test("licenses", () => {
  assert.equal(lip.licenseFamily("mit"), "permissive");
  assert.equal(lip.licenseFamily("BSD-2-Clause"), "permissive");
  assert.equal(lip.licenseFamily("LGPL-2.1"), "weak-copyleft");
  assert.equal(lip.licenseFamily("AGPL-3.0"), "copyleft");
  assert.equal(lip.licenseFamily("NOASSERTION"), "other");
  assert.equal(lip.licenseFamily(undefined), "none");
  assert.equal(lip.licenseOf({ id: "github.com/Copper-Lamp/BSChat", license: "NOASSERTION" }), "AGPL-3.0");
  assert.equal(lip.licenseOf({ id: "github.com/Ieckseyr/HologramLib" }), "MIT");
  assert.equal(lip.licenseOf({ id: "github.com/a/b", license: "" }), undefined);
  // license= as the site reads it: families in any case, SPDX ids as a mod spells them.
  assert.deepEqual(plain(lip.licenseFilterValues(["Permissive", "mit", "MIT", "WTFPL"])), ["permissive", "mit", "WTFPL"]);
  const spdx = lip.facetOptions(entries).license.flatMap((o) => o.ids);
  assert.deepEqual(plain(lip.licenseFilterValues(["Permissive", "mit", "MIT", "WTFPL", "NONE"], spdx)), ["permissive", "MIT", "none"]);
  assert.equal(run({ license: plain(lip.licenseFilterValues(["Permissive"])) }).matched.length, 38);
});

test("variant choice (A2) on the fixture details", () => {
  const arke = detail("github.com/LiteLDev/Arke");
  assert.equal(lip.preferredVariant(arke.versions[0].variants, "client", "win-x64"), "client");
  assert.equal(lip.preferredVariant(arke.versions[0].variants, "server"), undefined);
  assert.equal(arke.latest.variant, "client");

  const lse = detail("github.com/LiteLDev/LegacyScriptEngine");
  const latest = lse.versions[0].variants;
  assert.deepEqual(plain(lip.qualifyingVariants(latest, "client", "win-x64")), ["client", "client_lua", "client_nodejs", "client_python", "client_quickjs"]);
  assert.equal(lip.preferredVariant(latest, "server"), "", "the site's latest.variant");
  assert.equal(lse.latest.variant, "");
  assert.equal(lip.preferredVariant(latest, "client", "android-arm64"), undefined);
  assert.deepEqual(plain(lip.selfVariantAliases(lse)), ["client_lua", "client_quickjs", "lua", "quickjs"]);

  const files = lip.variantFiles(lse, "client", "client", "win-x64");
  assert.equal(files[0].version, "0.23.1");
  assert.deepEqual(plain(files[0].llRanges), ["26.60.*"]);
  assert.ok(!Object.keys(files[0].otherDeps).some(lip.isLeviLaminaKey));

  const pland = detail("github.com/IceBlcokMC/PLand");
  assert.equal(lip.preferredVariant(pland.versions[0].variants, "client", "win-x64"), undefined, "server only");
  assert.equal(lip.preferredVariant(pland.versions[0].variants, "server"), "");
  assert.equal(pland.owner, "IceBlcokMC");
  assert.equal(lip.licenseFamily(lip.licenseOf(pland)), "copyleft");

  assert.equal(lip.compareVariantNames("", "client") > 0, true);
  assert.equal(lip.variantIdentifier("github.com/a/b#x", ""), "github.com/a/b#");
});

test("game versions", () => {
  assert.equal(lip.catalogGameVersion("1.21.50.07"), "1.21.50");
  assert.equal(lip.catalogGameVersion("1.26.051.1"), "1.26.51");
  assert.ok(lip.compareGameVersions("1.26.10", "1.21.132") > 0);
  assert.ok(lip.compareGameVersions("1.0.0-beta", "1.0.0") < 0);
});
