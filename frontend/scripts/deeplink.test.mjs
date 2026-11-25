// levilauncher:// links are parsed and validated by the backend
// (internal/deeplink); the frontend turns the request into the package page
// route that LIPPackagePage reads back (`:id` = `<id>[#<variant>]`,
// `?link=1[&version=]`).
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import ts from "typescript";

const source = readFileSync(new URL("../src/constants/routes.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const routes = {};
runInNewContext(outputText, { exports: routes, URLSearchParams });

const parse = (route) => {
  const url = new URL(route, "http://launcher");
  const prefix = "/lip/package/";
  assert.ok(url.pathname.startsWith(prefix), route);
  const segment = url.pathname.slice(prefix.length);
  assert.ok(!segment.includes("/"), "the id stays one path segment");
  return { id: decodeURIComponent(segment), search: url.searchParams };
};

test("a link with version and variant opens the variant's install dialog", () => {
  const route = routes.routeTo.lipPackageFromLink({
    modulePath: "github.com/LiteLDev/LegacyScriptEngine",
    version: "0.17.0-rc.1+build.5",
    variant: "client/lua",
  });
  const { id, search } = parse(route);
  assert.equal(id, "github.com/LiteLDev/LegacyScriptEngine#client/lua");
  assert.equal(search.get("link"), "1");
  assert.equal(search.get("version"), "0.17.0-rc.1+build.5");
});

test("a link without version or variant asks for the latest of the preferred variant", () => {
  const { id, search } = parse(routes.routeTo.lipPackageFromLink({ modulePath: "github.com/LiteLDev/Arke" }));
  assert.equal(id, "github.com/LiteLDev/Arke");
  assert.equal(search.get("link"), "1");
  assert.equal(search.has("version"), false);
});
