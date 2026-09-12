export type PackageType = "gdk" | "uwp";
export type VersionChannel = "Release" | "Beta" | "Preview";

export const normalizePackageType = (value: unknown): PackageType =>
  String(value || "").toLowerCase() === "uwp" ? "uwp" : "gdk";

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
