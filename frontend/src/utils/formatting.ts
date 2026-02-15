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
