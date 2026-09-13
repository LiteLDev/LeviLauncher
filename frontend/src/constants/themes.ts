export const hexToRgb = (hex: string): string => {
  if (!hex || !/^#[0-9A-Fa-f]{6}$/.test(hex)) return "0, 0, 0";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `${r}, ${g}, ${b}`;
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

const luminance = (hex: string) => {
  const rgb = parseHex(hex);
  const linear = [rgb.r, rgb.g, rgb.b].map((value) => {
    const channel = value / 255;
    return channel <= 0.04045
      ? channel / 12.92
      : Math.pow((channel + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
};

const contrast = (a: string, b: string) => {
  const values = [luminance(a), luminance(b)];
  return (Math.max(...values) + 0.05) / (Math.min(...values) + 0.05);
};

const mixColor = (base: string, target: string, amount: number) => {
  const a = parseHex(base);
  const b = parseHex(target);
  return rgbToHex(
    a.r + (b.r - a.r) * amount,
    a.g + (b.g - a.g) * amount,
    a.b + (b.b - a.b) * amount,
  );
};

// Preserve the selected fill. Adapt the label instead of darkening pastels.
export const getAccentForeground = (base: string): string => {
  if (contrast(base, "#18181b") >= 4.5) return "#18181b";
  return contrast(base, "#ffffff") >= 4.5 ? "#ffffff" : "#000000";
};

export const getAccentHover = (base: string): string =>
  mixColor(
    base,
    getAccentForeground(base) === "#ffffff" ? "#000000" : "#ffffff",
    0.08,
  );

// Adjust text only as far as its surface requires. RGB mixing retains the soft
// character of pastels; lowering HSL lightness made pink turn saturated red.
export const getReadableAccent = (
  base: string,
  surface: string,
  ratio = 4.6,
): string => {
  const target = luminance(surface) > 0.5 ? "#000000" : "#ffffff";
  for (let step = 0; step <= 200; step++) {
    const color = mixColor(base, target, step / 200);
    if (contrast(color, surface) >= ratio) return color;
  }
  return target;
};

export const generateTheme = (baseColor: string): Record<number, string> => {
  // Mix toward white/black without boosting saturation or clipping pale shades.
  return {
    50: mixColor(baseColor, "#ffffff", 0.95),
    100: mixColor(baseColor, "#ffffff", 0.9),
    200: mixColor(baseColor, "#ffffff", 0.75),
    300: mixColor(baseColor, "#ffffff", 0.5),
    400: mixColor(baseColor, "#ffffff", 0.25),
    500: baseColor,
    600: mixColor(baseColor, "#000000", 0.15),
    700: mixColor(baseColor, "#000000", 0.3),
    800: mixColor(baseColor, "#000000", 0.5),
    900: mixColor(baseColor, "#000000", 0.7),
    950: mixColor(baseColor, "#000000", 0.85),
  };
};

export const THEME_GROUPS = {
  preset: [
    "emerald",
    "blue",
    "violet",
    "amber",
    "pink",
    "cyan",
    "win_yellow",
    "win_orange",
    "win_red",
    "win_blue",
    "win_green",
    "win_purple",
    "win_teal",
    "win_pink_red",
  ],
  generated: [
    "win_light_orange",
    "win_orange_red",
    "win_red_orange",
    "win_light_red",
    "win_bright_red",
    "win_deep_red",
    "win_rose",
    "win_dark_rose",
    "win_magenta",
    "win_dark_magenta",
    "win_orchid",
    "win_dark_orchid",
    "win_dark_blue",
    "win_light_purple",
    "win_medium_purple",
    "win_dark_purple",
    "win_light_magenta",
    "win_deep_purple",
    "win_dark_teal",
    "win_cyan",
    "win_dark_cyan",
    "win_green_blue",
    "win_dark_green_blue",
    "win_light_green",
    "win_gray",
    "win_dark_gray",
    "win_blue_gray",
    "win_dark_blue_gray",
    "win_green_gray",
    "win_dark_green_gray",
    "win_olive",
    "win_dark_green",
    "win_medium_gray",
    "win_darker_gray",
    "win_slate_gray",
  ],
};

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

  win_yellow: {
    50: "#ffff00",
    100: "#ffff00",
    200: "#ffff00",
    300: "#fffc00",
    400: "#ffda00",
    500: "#ffb900",
    600: "#d69b00",
    700: "#ad7e00",
    800: "#856000",
    900: "#5c4300",
    950: "#473400",
  },
  win_orange: {
    50: "#fffd00",
    100: "#fff100",
    200: "#ffd800",
    300: "#ffbe00",
    400: "#ffa500",
    500: "#ff8c00",
    600: "#d67600",
    700: "#ad5f00",
    800: "#854900",
    900: "#5c3200",
    950: "#472700",
  },
  win_red: {
    50: "#ff5e65",
    100: "#ff5960",
    200: "#ff5056",
    300: "#ff474c",
    400: "#f73d42",
    500: "#d13438",
    600: "#b02c2f",
    700: "#8e2326",
    800: "#6d1b1d",
    900: "#4b1314",
    950: "#3b0f10",
  },
  win_blue: {
    50: "#00d9ff",
    100: "#00ceff",
    200: "#00b9ff",
    300: "#00a3ff",
    400: "#008efe",
    500: "#0078d7",
    600: "#0065b5",
    700: "#005292",
    800: "#003e70",
    900: "#002b4d",
    950: "#00223c",
  },
  win_green: {
    50: "#1df870",
    100: "#1cec6b",
    200: "#19d35f",
    300: "#16ba54",
    400: "#13a249",
    500: "#10893e",
    600: "#0d7334",
    700: "#0b5d2a",
    800: "#084720",
    900: "#063116",
    950: "#042611",
  },
  win_purple: {
    50: "#c2beff",
    100: "#b8b5ff",
    200: "#a5a2ff",
    300: "#928fff",
    400: "#7e7cfd",
    500: "#6b69d6",
    600: "#5a58b4",
    700: "#494792",
    800: "#38376f",
    900: "#27264d",
    950: "#1e1d3c",
  },
  win_teal: {
    50: "#00ffff",
    100: "#00ffff",
    200: "#00ecff",
    300: "#00d0ff",
    400: "#00b5de",
    500: "#0099bc",
    600: "#00819e",
    700: "#006880",
    800: "#005062",
    900: "#003744",
    950: "#002b35",
  },
  win_pink_red: {
    50: "#ff829c",
    100: "#ff7c94",
    200: "#ff6f84",
    300: "#ff6275",
    400: "#ff5565",
    500: "#e74856",
    600: "#c23c48",
    700: "#9d313a",
    800: "#78252d",
    900: "#531a1f",
    950: "#411418",
  },

  win_light_orange: generateTheme("#F7630C"),
  win_orange_red: generateTheme("#CA5010"),
  win_red_orange: generateTheme("#DA3B01"),
  win_light_red: generateTheme("#EF6950"),
  win_bright_red: generateTheme("#FF4343"),
  win_deep_red: generateTheme("#E81123"),
  win_rose: generateTheme("#EA005E"),
  win_dark_rose: generateTheme("#C30052"),
  win_magenta: generateTheme("#E3008C"),
  win_dark_magenta: generateTheme("#BF0077"),
  win_orchid: generateTheme("#C239B3"),
  win_dark_orchid: generateTheme("#9A0089"),
  win_dark_blue: generateTheme("#0063B1"),
  win_light_purple: generateTheme("#8E8CD8"),
  win_medium_purple: generateTheme("#8764B8"),
  win_dark_purple: generateTheme("#744DA9"),
  win_light_magenta: generateTheme("#B146C2"),
  win_deep_purple: generateTheme("#881798"),
  win_dark_teal: generateTheme("#2D7D9A"),
  win_cyan: generateTheme("#00B7C3"),
  win_dark_cyan: generateTheme("#038387"),
  win_green_blue: generateTheme("#00B294"),
  win_dark_green_blue: generateTheme("#018574"),
  win_light_green: generateTheme("#00CC6A"),
  win_gray: generateTheme("#7A7574"),
  win_dark_gray: generateTheme("#5D5A58"),
  win_blue_gray: generateTheme("#68768A"),
  win_dark_blue_gray: generateTheme("#515C6B"),
  win_green_gray: generateTheme("#567C73"),
  win_dark_green_gray: generateTheme("#486860"),
  win_olive: generateTheme("#498205"),
  win_dark_green: generateTheme("#107C10"),
  win_medium_gray: generateTheme("#767676"),
  win_darker_gray: generateTheme("#4C4A48"),
  win_slate_gray: generateTheme("#69797E"),
};
