import { readdir, readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../src");
const violations = [];
let files = 0;
async function scan(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) {
      await scan(path);
      continue;
    }
    if (!/\.tsx?$/.test(path)) continue;
    files++;
    const source = await readFile(path, "utf8");
    const tree = ts.createSourceFile(
      path,
      source,
      ts.ScriptTarget.Latest,
      true,
    );
    const visit = (node) => {
      if (
        ts.isStringLiteralLike(node) ||
        ts.isTemplateHead(node) ||
        ts.isTemplateMiddle(node) ||
        ts.isTemplateTail(node)
      ) {
        // These dots describe the selected color swatch, not an app surface.
        const isSwatchMarker =
          path.endsWith("SettingsPage.tsx") &&
          /w-2\.5 h-2\.5 bg-white rounded-full/.test(node.text);
        const rawFill = node.text.match(
          /\bbg-(?:white|(?:zinc|gray|slate|neutral)-\d+)\b|dark:bg-black\b/,
        );
        if (rawFill && !isSwatchMarker) {
          const line =
            tree.getLineAndCharacterOfPosition(node.getStart(tree)).line + 1;
          violations.push(
            `${path}:${line}: ${rawFill[0]} bypasses material tokens; use bg-surface, bg-default, bg-segment or bg-overlay.`,
          );
        }
      }
      ts.forEachChild(node, visit);
    };
    visit(tree);
  }
}
await scan(sourceRoot);
if (violations.length) {
  console.error(violations.join("\n"));
  process.exitCode = 1;
} else {
  console.log(
    `Material audit: ${files} source files use semantic neutral fills (color swatch markers excluded).`,
  );
}
