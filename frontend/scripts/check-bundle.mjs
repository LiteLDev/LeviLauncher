import { gzipSync } from "node:zlib";
import { readdir, readFile, stat } from "node:fs/promises";
import { extname, join, relative, resolve } from "node:path";

const distDir = resolve("dist");
const limits = {
  total: 4_500_000,
  javascript: 3_800_000,
  css: 370_000,
  fonts: 0,
  largestJavaScript: 850_000,
};
const forbiddenFiles = new Set(["stats.html", ".vite/manifest.json"]);
const fontExtensions = new Set([".woff", ".woff2", ".ttf", ".otf", ".eot"]);

const files = [];
const visit = async (directory) => {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const absolutePath = join(directory, entry.name);
    if (entry.isDirectory()) {
      await visit(absolutePath);
      continue;
    }
    if (!entry.isFile()) continue;

    const info = await stat(absolutePath);
    const relativePath = relative(distDir, absolutePath).replaceAll("\\", "/");
    files.push({
      absolutePath,
      relativePath,
      size: info.size,
      extension: extname(entry.name).toLowerCase(),
    });
  }
};

await visit(distDir);

const forbidden = files
  .filter(
    (file) =>
      forbiddenFiles.has(file.relativePath) ||
      file.relativePath.endsWith(".map"),
  )
  .map((file) => file.relativePath);
const javascriptFiles = files.filter((file) => file.extension === ".js");
const cssFiles = files.filter((file) => file.extension === ".css");
const fontFiles = files.filter((file) => fontExtensions.has(file.extension));
const sum = (items) => items.reduce((total, item) => total + item.size, 0);
const total = sum(files);
const javascript = sum(javascriptFiles);
const css = sum(cssFiles);
const fonts = sum(fontFiles);
const largestJavaScript = javascriptFiles.reduce(
  (largest, file) => (file.size > largest.size ? file : largest),
  { relativePath: "(none)", size: 0 },
);
let gzipJavaScript = 0;
for (const file of javascriptFiles) {
  gzipJavaScript += gzipSync(await readFile(file.absolutePath)).byteLength;
}

const metrics = [
  ["total", total, limits.total],
  ["javascript", javascript, limits.javascript],
  ["css", css, limits.css],
  ["fonts", fonts, limits.fonts],
  ["largestJavaScript", largestJavaScript.size, limits.largestJavaScript],
];

console.log("Production bundle budget:");
for (const [name, actual, limit] of metrics) {
  console.log(`  ${name}: ${actual.toLocaleString()} / ${limit.toLocaleString()} bytes`);
}
console.log(
  `  javascript gzip (report only): ${gzipJavaScript.toLocaleString()} bytes`,
);
console.log(
  `  largest JavaScript chunk: ${largestJavaScript.relativePath}`,
);

const exceeded = metrics.filter(([, actual, limit]) => actual > limit);
if (forbidden.length > 0 || exceeded.length > 0) {
  if (forbidden.length > 0) {
    console.error(`Forbidden production artifacts: ${forbidden.join(", ")}`);
  }
  for (const [name, actual, limit] of exceeded) {
    console.error(
      `Bundle budget exceeded for ${name}: ${actual.toLocaleString()} > ${limit.toLocaleString()} bytes`,
    );
  }
  process.exitCode = 1;
}
