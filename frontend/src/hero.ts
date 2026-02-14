// hero.ts
import { heroui } from "@heroui/react";

export default heroui({
  themes: {
    light: {
      colors: {
        background: "rgba(250, 250, 250, 0.9)",
        foreground: "#27272a",
      },
    },
    dark: {
      colors: {
        background: "#000000",
        content1: "#18181b",
        content2: "#27272a",
        content3: "#3f3f46",
        content4: "#52525b",
      },
    },
  },
});
