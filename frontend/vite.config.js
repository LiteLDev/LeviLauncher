import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import tailwindcss from "@tailwindcss/vite";
import checker from "vite-plugin-checker";
import { resolve } from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    wails("./bindings"),
    tailwindcss(),
    checker({
      typescript: true,
    }),
  ],
  base: "./",
  resolve: {
    alias: {
      "@": resolve(__dirname, "./src"),
      bindings: resolve(__dirname, "./bindings"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          "vendor-react": ["react", "react-dom", "react-router-dom"],
          "vendor-heroui": ["@heroui/react"],
          "vendor-framer": ["framer-motion"],
          "vendor-icons": ["react-icons"],
          "vendor-utils": ["i18next", "react-i18next", "uuid"],
        },
      },
    },
    chunkSizeWarningLimit: 1500,
  },
});
