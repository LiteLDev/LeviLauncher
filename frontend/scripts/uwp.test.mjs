import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const { outputText } = ts.transpileModule(
  readFileSync(new URL("../src/utils/packageType.ts", import.meta.url), "utf8"),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } },
);
const exports = {};
const versionExports = {};
runInNewContext(ts.transpileModule(
  readFileSync(new URL("../src/utils/version.ts", import.meta.url), "utf8"),
  { compilerOptions: { module: ts.ModuleKind.CommonJS } },
).outputText, { exports: versionExports });
runInNewContext(outputText, { exports, require(name) {
  assert.equal(name, "./version");
  return versionExports;
} });
const { installerIdentityFromPath, isAppxInstaller, normalizePackageType, normalizeVersionChannel, versionStatusKey } = exports;

test("UWP isolation starts at 1.19.70.2 and rejects unknown or malformed versions", () => {
  for (const [version, supported] of [
    ["1.19.70.1", false], ["1.19.70.2", true], ["1.19.70.02", true],
    ["1.19.70.3", true], ["1.19.69.99", false], ["1.19.70", false],
    ["1.20", true], ["1.9.100.0", false], ["1.21.100.0", true],
    [" 1.19.70.2 ", true], ["", false], ["unknown", false],
    ["1.19.70.2-preview", false], ["1.19.70.2.0", false], ["1.19.70.4294967296", false],
  ]) {
    assert.equal(exports.supportsVersionIsolation("uwp", version), supported, version);
    assert.equal(exports.supportsVersionIsolation("gdk", version), true, `GDK ${version}`);
  }
});

test("version status separates package types and all three channels", () => {
  const keys = ["gdk", "uwp"].flatMap((packageType) =>
    ["Release", "Beta", "Preview"].map((channel) => versionStatusKey("1.21.100.0", channel, packageType)),
  );
  assert.equal(new Set(keys).size, 6);
  assert.equal(versionStatusKey("1.21.100.0", "Release"), keys[0]);
  assert.equal(normalizePackageType(undefined), "gdk");
  assert.equal(normalizeVersionChannel("beta"), "Beta");
});

test("concurrent completed downloads derive their own package and channel identity", () => {
  const files = [
    ["C:\\installers\\Minecraft-UWP-Beta-1.21.100.0.appx", "uwp", "Beta"],
    ["C:\\installers\\Minecraft-Release-1.21.100.0.msixvc", "gdk", "Release"],
    ["C:/installers/Preview 1.21.100.0.msixvc", "gdk", "Preview"],
    ["C:/installers/Minecraft-UWP-Preview-1.21.100.0.APPX", "uwp", "Preview"],
  ];
  for (const [path, packageType, type] of files) {
    const actual = installerIdentityFromPath(path);
    assert.equal(actual.packageType, packageType);
    assert.equal(actual.type, type);
    assert.equal(actual.version, "1.21.100.0");
  }
  assert.equal(installerIdentityFromPath("C:\\downloads\\unknown.zip"), null);
});

test("all supported local UWP containers use the Appx installer", () => {
  for (const extension of ["appx", "msix", "appxbundle", "msixbundle", "APPX"]) {
    assert.equal(isAppxInstaller(`C:\\packages\\Minecraft.${extension}`), true);
  }
  assert.equal(isAppxInstaller("C:\\packages\\Minecraft.msixvc"), false);
  assert.equal(isAppxInstaller("C:\\packages\\Minecraft.appx.backup"), false);
});
