import React from "react";
import { useTheme } from "next-themes";

const colorMap: Record<string, string> = {
  "0": "#000000",
  "1": "#0000AA",
  "2": "#00AA00",
  "3": "#00AAAA",
  "4": "#AA0000",
  "5": "#AA00AA",
  "6": "#FFAA00",
  "7": "#AAAAAA",
  "8": "#555555",
  "9": "#5555FF",
  a: "#55FF55",
  b: "#55FFFF",
  c: "#FF5555",
  d: "#FF55FF",
  e: "#FFFF55",
  f: "#FFFFFF",
  g: "#DDD605",
  h: "#E3D4D1",
  i: "#CECACA",
  j: "#443A3B",
  m: "#971607",
  n: "#B4684D",
  p: "#DEB12D",
  q: "#47A036",
  s: "#2CBAA8",
  t: "#21497B",
  u: "#9A5CC6",
  v: "#EB7114",
};

const lightModeColorMap: Record<string, string> = {
  ...colorMap,
  e: "#D4A500",
  g: "#B89F00",
  h: "#8B7355",
  i: "#666666",
};

type StyleState = { color?: string; bold?: boolean; italic?: boolean };

type McTextProps = {
  text: string;
};

export function McText({ text }: McTextProps): React.ReactElement {
  const { theme } = useTheme();
  const isLight = theme === "light";
  const activeColorMap = isLight ? lightModeColorMap : colorMap;
  const parts: React.ReactNode[] = [];
  let i = 0;
  let style: StyleState = {};
  const pushText = (s: string) => {
    if (!s) return;
    const key = `${parts.length}-${style.color || ""}-${style.bold ? 1 : 0}-${
      style.italic ? 1 : 0
    }`;
    const children = s
      .split("\n")
      .flatMap((seg, idx) =>
        idx > 0 ? [<br key={`${key}-br-${idx}`} />, seg] : [seg]
      );
    parts.push(
      <span
        key={key}
        style={{
          color: style.color,
          fontWeight: style.bold ? 700 : undefined,
          fontStyle: style.italic ? "italic" : undefined,
        }}
      >
        {children}
      </span>
    );
  };
  while (i < text.length) {
    const ch = text[i];
    if (ch === "§" && i + 1 < text.length) {
      const code = text[i + 1].toLowerCase();
      i += 2;
      if (code === "r") {
        style = {};
        continue;
      }
      if (code === "l") {
        style = { ...style, bold: true };
        continue;
      }
      if (code === "o") {
        style = { ...style, italic: true };
        continue;
      }
      const col = activeColorMap[code];
      if (col) {
        style = { ...style, color: col };
        continue;
      }
      continue;
    }
    let j = i;
    while (j < text.length && !(text[j] === "§" && j + 1 < text.length)) j++;
    pushText(text.slice(i, j));
    i = j;
  }
  return <>{parts.length ? parts : text}</>;
}

export function renderMcText(text: string): React.ReactNode {
  return <McText text={text} />;
}
