export const formatBytes = (n?: number) => {
  const v = typeof n === "number" ? n : 0;
  if (v < 1024) return `${v} B`;
  const k = 1024;
  const sizes = ["KB", "MB", "GB", "TB"];
  let i = -1;
  let val = v;
  do {
    val /= k;
    i++;
  } while (val >= k && i < sizes.length - 1);
  return `${val.toFixed(val >= 100 ? 0 : val >= 10 ? 1 : 2)} ${sizes[i]}`;
};

export const formatDate = (ts?: number) => {
  const v = typeof ts === "number" ? ts : 0;
  if (!v) return "";
  const d = new Date(v * 1000);
  return d.toLocaleString();
};

export const formatDateStr = (dateStr: string | undefined) => {
  if (!dateStr) return "-";
  return new Date(dateStr).toLocaleDateString();
};

export const formatNumber = (num: number | undefined) => {
  if (num === undefined) return "0";
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toString();
};

export const formatFileSize = (bytes: number | undefined) => {
  if (bytes === undefined) return "-";
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

export const formatSpeed = (bytesPerSec: number) => {
  return formatBytes(bytesPerSec) + "/s";
};

export const sortGameVersions = (versions: string[] | undefined) => {
  if (!versions) return [];
  const sorted = [...versions].sort((a, b) => {
    const aIsVer = /^\d/.test(a);
    const bIsVer = /^\d/.test(b);

    if (aIsVer && !bIsVer) return -1;
    if (!aIsVer && bIsVer) return 1;
    if (!aIsVer && !bIsVer) return a.localeCompare(b);

    const partsA = a.split(".").map((p) => parseInt(p) || 0);
    const partsB = b.split(".").map((p) => parseInt(p) || 0);

    for (let i = 0; i < Math.max(partsA.length, partsB.length); i++) {
      const valA = partsA[i] || 0;
      const valB = partsB[i] || 0;
      if (valA !== valB) return valB - valA;
    }
    return 0;
  });
  return sorted;
};
