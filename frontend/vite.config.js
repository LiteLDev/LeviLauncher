import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), wails("./bindings"), tailwindcss()],
  base: "./",
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
      'bindings': resolve(__dirname, './bindings'),
    },
  },
});