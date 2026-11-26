import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

function loadUtility(path, dependencies) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const exports = {};
  runInNewContext(outputText, {
    exports,
    require(name) {
      assert.ok(name in dependencies, `unexpected dependency: ${name}`);
      return dependencies[name];
    },
  });
  return exports;
}

function setup(extraGlobals = {}) {
  const version = loadUtility("../src/utils/version.ts", {});
  const lipCatalog = loadUtility("../src/utils/lipCatalog.ts", {});
  const contentSource = readFileSync(
    new URL("../src/utils/content.ts", import.meta.url),
    "utf8",
  );
  const { outputText } = ts.transpileModule(contentSource, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const content = {};
  const contentDeps = {
    "./fs": {},
    "./version": version,
    "./lipCatalog": lipCatalog,
    "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice": {},
  };
  runInNewContext(outputText, {
    exports: content,
    AbortController,
    setTimeout,
    clearTimeout,
    ...extraGlobals,
    require(name) {
      assert.ok(name in contentDeps, `unexpected dependency: ${name}`);
      return contentDeps[name];
    },
  });
  const wineBridge = loadUtility("../src/utils/wineBridge.ts", {
    "./content": content,
  });
  return { ...wineBridge, content };
}

const plain = (value) => (value == null ? value : JSON.parse(JSON.stringify(value)));

const samplePkg = {
  identifier: "github.com/wzyyyyyyy/LeviWineBridge",
  preferredVariantKey: "",
  files: [],
  variantDetails: [
    {
      key: "",
      packageIdentifier: "github.com/wzyyyyyyy/LeviWineBridge#",
      versions: ["0.2.0-rc.1", "0.1.3", "0.1.2"],
      llDependencyRanges: [">=0.0.0", "26.60.8"],
      files: [
        {
          version: "0.2.0-rc.1",
          prerelease: true,
          dependencies: { "github.com/LiteLDev/LeviLamina": ">=0.0.0" },
          llDependencyRanges: [">=0.0.0"],
          otherDependencies: {},
        },
        {
          version: "0.1.3",
          prerelease: false,
          dependencies: { "github.com/LiteLDev/LeviLamina": ">=26.60.0" },
          llDependencyRanges: [">=26.60.0"],
          otherDependencies: {},
        },
        {
          version: "0.1.2",
          prerelease: false,
          dependencies: { "github.com/LiteLDev/LeviLamina": "26.60.8" },
          llDependencyRanges: ["26.60.8"],
          otherDependencies: {},
        },
      ],
    },
  ],
};

test("prefers the newest compatible stable release over prereleases", () => {
  const api = setup();
  const picked = api.selectCompatibleWineBridgeRelease(samplePkg, "26.60.8");
  assert.deepEqual(plain(picked), {
    identifier: "github.com/wzyyyyyyy/LeviWineBridge#",
    version: "0.1.3",
    prerelease: false,
  });
});

test("falls back to compatible prerelease when no stable release matches", () => {
  const api = setup();
  const picked = api.selectCompatibleWineBridgeRelease(samplePkg, "26.51.0");
  assert.deepEqual(plain(picked), {
    identifier: "github.com/wzyyyyyyy/LeviWineBridge#",
    version: "0.2.0-rc.1",
    prerelease: true,
  });
});

test("returns null when no release matches the LeviLamina version", () => {
  const api = setup();
  const strictPkg = {
    identifier: "github.com/wzyyyyyyy/LeviWineBridge",
    preferredVariantKey: "",
    files: [
      {
        version: "0.1.2",
        prerelease: false,
        dependencies: { "github.com/LiteLDev/LeviLamina": "26.60.8" },
        llDependencyRanges: ["26.60.8"],
        otherDependencies: {},
      },
    ],
    variantDetails: [],
  };
  assert.equal(api.selectCompatibleWineBridgeRelease(strictPkg, "26.51.0"), null);
  assert.equal(api.selectCompatibleWineBridgeRelease(strictPkg, ""), null);
  assert.equal(api.selectCompatibleWineBridgeRelease(null, "26.60.8"), null);
});

test("resolves option state across loading, unavailable, incompatible, and compatible states", () => {
  const api = setup();

  assert.deepEqual(
    plain(
      api.resolveWineBridgeOptionState({
        catalogStatus: "loading",
        pkg: null,
        llVersion: "26.60.8",
        userEnabled: true,
      }),
    ),
    {
      candidate: null,
      compatible: false,
      checked: false,
      switchDisabled: true,
      statusKind: "loading",
    },
  );

  assert.deepEqual(
    plain(
      api.resolveWineBridgeOptionState({
        catalogStatus: "error",
        pkg: null,
        llVersion: "26.60.8",
        userEnabled: true,
      }),
    ),
    {
      candidate: null,
      compatible: false,
      checked: false,
      switchDisabled: true,
      statusKind: "unavailable",
    },
  );

  const compatibleOn = api.resolveWineBridgeOptionState({
    catalogStatus: "ready",
    pkg: samplePkg,
    llVersion: "26.60.8",
    userEnabled: true,
  });
  assert.equal(compatibleOn.statusKind, "compatible");
  assert.equal(compatibleOn.checked, true);
  assert.equal(compatibleOn.switchDisabled, false);
  assert.equal(compatibleOn.candidate?.version, "0.1.3");

  const compatibleOff = api.resolveWineBridgeOptionState({
    catalogStatus: "ready",
    pkg: samplePkg,
    llVersion: "26.60.8",
    userEnabled: false,
  });
  assert.equal(compatibleOff.statusKind, "compatible");
  assert.equal(compatibleOff.checked, false);
  assert.equal(compatibleOff.switchDisabled, false);

  const incompatible = api.resolveWineBridgeOptionState({
    catalogStatus: "ready",
    pkg: {
      ...samplePkg,
      variantDetails: [
        {
          ...samplePkg.variantDetails[0],
          files: [samplePkg.variantDetails[0].files[2]],
        },
      ],
    },
    llVersion: "26.51.0",
    userEnabled: true,
  });
  assert.equal(incompatible.statusKind, "incompatible");
  assert.equal(incompatible.checked, false);
  assert.equal(incompatible.switchDisabled, true);
});

test("fetchLIPPackageDetailDirect fetches only the target mod detail and caches it", async () => {
  const urls = [];
  const api = setup({
    fetch: async (url) => {
      urls.push(String(url));
      return {
        ok: true,
        status: 200,
        json: async () => ({
          id: "github.com/wzyyyyyyy/LeviWineBridge",
          name: "LeviWineBridge",
          description: "Wine companion plugin",
          owner: "wzyyyyyyy",
          versions: [
            {
              version: "0.1.3",
              prerelease: false,
              variants: {
                "": [
                  {
                    side: "client",
                    platform: "win-x64",
                    deps: { "github.com/LiteLDev/LeviLamina": ">=0.0.0" },
                  },
                ],
              },
            },
          ],
        }),
      };
    },
  });

  const first = await api.content.fetchLIPPackageDetailDirect(
    api.LEVI_WINE_BRIDGE_IDENTIFIER,
  );
  const second = await api.content.fetchLIPPackageDetailDirect(
    api.LEVI_WINE_BRIDGE_IDENTIFIER,
  );

  assert.equal(urls.length, 1);
  assert.equal(
    urls[0],
    "https://levimc.org/api/v4/mods/github.com/wzyyyyyyy/LeviWineBridge/index.json",
  );
  assert.equal(first?.identifier, "github.com/wzyyyyyyy/LeviWineBridge");
  assert.equal(second?.identifier, "github.com/wzyyyyyyy/LeviWineBridge");
  const candidate = api.selectCompatibleWineBridgeRelease(first, "26.60.8");
  assert.deepEqual(plain(candidate), {
    identifier: "github.com/wzyyyyyyy/LeviWineBridge#",
    version: "0.1.3",
    prerelease: false,
  });
});

test("detectWineBridgeInstanceState handles not installed, enabled in LeviClient folder, disabled, manual import, and updates", () => {
  const api = setup();

  assert.deepEqual(
    plain(
      api.detectWineBridgeInstanceState({
        modsInfo: [],
        enabledByFolder: new Map(),
        lipState: { installed: false, installedVersion: "" },
      }),
    ),
    {
      status: "not_installed",
      installedVersion: "",
      folders: [],
      disabledFolders: [],
    },
  );

  // LIP installs LeviWineBridge into mods/LeviClient with manifest name LeviWineBridge.
  assert.deepEqual(
    plain(
      api.detectWineBridgeInstanceState({
        modsInfo: [
          {
            name: "LeviWineBridge",
            entry: "LeviWineBridge.dll",
            version: "0.1.3",
            folder: "LeviClient",
          },
        ],
        enabledByFolder: new Map([["LeviClient", true]]),
        lipState: { installed: true, installedVersion: "0.1.3" },
      }),
    ),
    {
      status: "enabled",
      installedVersion: "0.1.3",
      folders: ["LeviClient"],
      disabledFolders: [],
    },
  );

  // Disabled manually in ModsPage.
  assert.deepEqual(
    plain(
      api.detectWineBridgeInstanceState({
        modsInfo: [
          {
            name: "LeviWineBridge",
            entry: "LeviWineBridge.dll",
            version: "0.1.2",
            folder: "LeviClient",
          },
        ],
        enabledByFolder: new Map([["LeviClient", false]]),
        lipState: { installed: true, installedVersion: "0.1.2" },
      }),
    ),
    {
      status: "disabled",
      installedVersion: "0.1.2",
      folders: ["LeviClient"],
      disabledFolders: ["LeviClient"],
    },
  );

  // Manual zip import (not tracked by LIP).
  assert.deepEqual(
    plain(
      api.detectWineBridgeInstanceState({
        modsInfo: [
          {
            name: "CustomFolderName",
            entry: "LeviWineBridge.dll",
            version: "v0.1.3",
            folder: "CustomFolder",
          },
        ],
        enabledByFolder: new Map([["CustomFolder", true]]),
        lipState: { installed: false, installedVersion: "" },
      }),
    ),
    {
      status: "enabled",
      installedVersion: "0.1.3",
      folders: ["CustomFolder"],
      disabledFolders: [],
    },
  );

  assert.equal(api.hasWineBridgeUpdate("0.1.2", "0.1.3"), true);
  assert.equal(api.hasWineBridgeUpdate("0.1.3", "0.1.3"), false);
  assert.equal(api.hasWineBridgeUpdate("0.1.3", "0.1.2"), false);
  assert.equal(api.hasWineBridgeUpdate("", "0.1.3"), false);
});
