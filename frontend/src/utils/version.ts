export function isVersionAtLeast(value: string | undefined, minimum: string): boolean {
  const version = String(value || "").trim();
  return /^\d+(?:\.\d+){0,3}$/.test(version) &&
    version.split(".").every(part => Number(part) <= 0xffffffff) &&
    compareVersions(version, minimum) >= 0;
}

export function compareVersions(a: string, b: string): number {
  const as = String(a || "").split(".");
  const bs = String(b || "").split(".");
  const n = Math.max(as.length, bs.length);
  for (let i = 0; i < n; i++) {
    const ai = i < as.length ? as[i] : "0";
    const bi = i < bs.length ? bs[i] : "0";
    const an = Number(ai);
    const bn = Number(bi);
    const aNum = !Number.isNaN(an) && /^\d+$/.test(ai);
    const bNum = !Number.isNaN(bn) && /^\d+$/.test(bi);
    if (aNum && bNum) {
      if (an !== bn) return an - bn;
    } else {
      if (ai !== bi) return ai.localeCompare(bi);
    }
  }
  return 0;
}
