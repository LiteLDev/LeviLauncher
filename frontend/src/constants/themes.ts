export const THEMES: Record<string, Record<number, string>> = {
  emerald: {
    50: "#ecfdf5",
    100: "#d1fae5",
    200: "#a7f3d0",
    300: "#6ee7b7",
    400: "#34d399",
    500: "#10b981",
    600: "#059669",
    700: "#047857",
    800: "#065f46",
    900: "#064e3b",
    950: "#022c22",
  },
  blue: {
    50: "#eff6ff",
    100: "#dbeafe",
    200: "#bfdbfe",
    300: "#93c5fd",
    400: "#60a5fa",
    500: "#3b82f6",
    600: "#2563eb",
    700: "#1d4ed8",
    800: "#1e40af",
    900: "#1e3a8a",
    950: "#172554",
  },
  violet: {
    50: "#f5f3ff",
    100: "#ede9fe",
    200: "#ddd6fe",
    300: "#c4b5fd",
    400: "#a78bfa",
    500: "#8b5cf6",
    600: "#7c3aed",
    700: "#6d28d9",
    800: "#5b21b6",
    900: "#4c1d95",
    950: "#2e1065",
  },
  amber: {
    50: "#fffbeb",
    100: "#fef3c7",
    200: "#fde68a",
    300: "#fcd34d",
    400: "#fbbf24",
    500: "#f59e0b",
    600: "#d97706",
    700: "#b45309",
    800: "#92400e",
    900: "#78350f",
    950: "#451a03",
  },
  pink: {
    50: "#fff2f5",
    100: "#ffe6ea",
    200: "#ffcdd6",
    300: "#ffaec0",
    400: "#ff8fa8",
    500: "#ff6b8b",
    600: "#e64a6f",
    700: "#c23355",
    800: "#a32e4a",
    900: "#8a2a41",
    950: "#4d1020",
  },
  cyan: {
    50: "#ecfeff",
    100: "#cffafe",
    200: "#a5f3fc",
    300: "#67e8f9",
    400: "#22d3ee",
    500: "#06b6d4",
    600: "#0891b2",
    700: "#0e7490",
    800: "#155e75",
    900: "#164e63",
    950: "#083344",
  },
};

export const hexToRgb = (hex: string): string => {
  if (!hex || !/^#[0-9A-Fa-f]{6}$/.test(hex)) return "0 0 0";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `${r} ${g} ${b}`;
};

const rgbToHex = (r: number, g: number, b: number): string => {
  const toHex = (n: number) => {
    const hex = Math.max(0, Math.min(255, Math.round(n))).toString(16);
    return hex.length === 1 ? "0" + hex : hex;
  };
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
};

const parseHex = (hex: string) => {
  if (!hex || !/^#[0-9A-Fa-f]{6}$/.test(hex)) return { r: 0, g: 0, b: 0 };
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return { r, g, b };
};

const mix = (color1: string, color2: string, weight: number) => {
  const c1 = parseHex(color1);
  const c2 = parseHex(color2);
  const r = c1.r * (1 - weight) + c2.r * weight;
  const g = c1.g * (1 - weight) + c2.g * weight;
  const b = c1.b * (1 - weight) + c2.b * weight;
  return rgbToHex(r, g, b);
};

export const generateTheme = (baseColor: string): Record<number, string> => {
  const safeColor =
    baseColor && /^#[0-9A-Fa-f]{6}$/.test(baseColor) ? baseColor : "#10b981";
  return {
    50: mix(safeColor, "#ffffff", 0.9),
    100: mix(safeColor, "#ffffff", 0.8),
    200: mix(safeColor, "#ffffff", 0.7),
    300: mix(safeColor, "#ffffff", 0.5),
    400: mix(safeColor, "#ffffff", 0.3),
    500: safeColor,
    600: mix(safeColor, "#000000", 0.1),
    700: mix(safeColor, "#000000", 0.25),
    800: mix(safeColor, "#000000", 0.4),
    900: mix(safeColor, "#000000", 0.55),
    950: mix(safeColor, "#000000", 0.7),
  };
};
