import { readFile, rename, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { join } from "node:path";

const [arch, directory] = process.argv.slice(2);
const runtimeArch = { amd64: "x64", arm64: "arm64" }[arch];
if (!runtimeArch || !directory) {
  throw new Error("Usage: node download-vcredist.mjs <amd64|arm64> <output-directory>");
}

// Always refresh the official redistributable; never package a partial download.
const destination = join(directory, `vc_redist.${runtimeArch}.exe`);
const temporary = `${destination}.download`;
try {
  execFileSync("curl", [
    "--fail", "--location", "--show-error", "--silent",
    "--proto", "=https", "--proto-redir", "=https",
    "--retry", "3", "--connect-timeout", "30", "--max-time", "300",
    "--output", temporary, `https://aka.ms/vc14/vc_redist.${runtimeArch}.exe`,
  ], { stdio: "inherit" });
  const data = await readFile(temporary);
  const peOffset = data.length >= 64 ? data.readUInt32LE(0x3c) : 0;
  if (data.toString("ascii", 0, 2) !== "MZ" || peOffset < 64 ||
      peOffset + 4 > data.length || data.readUInt32LE(peOffset) !== 0x4550) {
    throw new Error("The VC++ Runtime download is not a Windows executable");
  }
  await rename(temporary, destination);
  console.log(`Downloaded ${destination} (${data.length} bytes)`);
} finally {
  await rm(temporary, { force: true });
}
