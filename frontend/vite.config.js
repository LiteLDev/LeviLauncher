import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import tailwindcss from "@tailwindcss/vite";
import checker from "vite-plugin-checker";
import { visualizer } from "rollup-plugin-visualizer";
import { resolve } from "path";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isAnalyzeMode = mode === "analyze";

  return {
    plugins: [
      react(),
      wails("./bindings"),
      tailwindcss(),
      checker({
        typescript: true,
      }),
      isAnalyzeMode &&
        visualizer({
          filename: "dist/stats.html",
          template: "treemap",
          gzipSize: true,
          brotliSize: true,
          open: false,
        }),
    ].filter(Boolean),
    base: "./",
    resolve: {
      alias: {
        "@": resolve(__dirname, "./src"),
        bindings: resolve(__dirname, "./bindings"),
        "@heroui/react": resolve(__dirname, "./src/shims/heroui.ts"),
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            const normalizedId = id.split("\\").join("/");

            if (!normalizedId.includes("/node_modules/")) {
              return undefined;
            }

            if (
              normalizedId.includes("/node_modules/react/") ||
              normalizedId.includes("/node_modules/react-dom/") ||
              normalizedId.includes("/node_modules/react-router-dom/")
            ) {
              return "vendor-react";
            }

            if (normalizedId.includes("/node_modules/framer-motion/")) {
              return "vendor-framer";
            }

            if (normalizedId.includes("/node_modules/react-icons/")) {
              return "vendor-icons";
            }

            if (
              normalizedId.includes("/node_modules/i18next/") ||
              normalizedId.includes("/node_modules/react-i18next/") ||
              normalizedId.includes("/node_modules/uuid/")
            ) {
              return "vendor-utils";
            }

            return undefined;
          },
        },
      },
      chunkSizeWarningLimit: 1500,
    },
  };
});
