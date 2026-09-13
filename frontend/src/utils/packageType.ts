import { isVersionAtLeast } from "./version";

export type PackageType = "gdk" | "uwp";
export type VersionChannel = "Release" | "Beta" | "Preview";

export const normalizePackageType = (value: unknown): PackageType =>
  String(value || "").toLowerCase() === "uwp" ? "uwp" : "gdk";

export const UWP_ISOLATION_MIN_VERSION = "1.19.70.2";

export const EDITOR_RELEASE_MIN_VERSION = "1.21.50";
export const EDITOR_PREVIEW_MIN_VERSION = "1.19.80.20";

export function editorMinVersion(channel?: string): string {
  return String(channel || "").trim().toLowerCase() === "preview"
    ? EDITOR_PREVIEW_MIN_VERSION
    : EDITOR_RELEASE_MIN_VERSION;
}

export function supportsEditorMode(gameVersion?: string, channel?: string): boolean {
  return isVersionAtLeast(gameVersion, editorMinVersion(channel));
}

export function supportsVersionIsolation(packageType?: string, gameVersion?: string): boolean {
  if (normalizePackageType(packageType) !== "uwp") return true;
  return isVersionAtLeast(gameVersion, UWP_ISOLATION_MIN_VERSION);
}

export const normalizeVersionChannel = (value: unknown): VersionChannel => {
  const channel = String(value || "").toLowerCase();
  return channel === "preview" ? "Preview" : channel === "beta" ? "Beta" : "Release";
};

export const versionStatusKey = (
  version: string,
  channel: string,
  packageType?: string,
): string => `${normalizePackageType(packageType)}:${channel.toLowerCase()}:${version}`;

export const isAppxInstaller = (path: string): boolean =>
  /\.(?:appx|msix|appxbundle|msixbundle)$/i.test(path);

export function installerIdentityFromPath(path: string) {
  const fileName = path.split(/[/\\]/).pop() || "";
  const match = fileName.match(/^(?:Minecraft[- ])?(?:(UWP)[- ])?(Preview|Release|Beta)[- ](.+)\.(msixvc|appx|msix|appxbundle|msixbundle)$/i);
  if (!match) return null;
  return {
    version: match[3],
    type: normalizeVersionChannel(match[2]),
    packageType: normalizePackageType(match[1] || match[4].toLowerCase() !== "msixvc" ? "uwp" : "gdk"),
  };
}
