import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  esbuild: {
    target: "esnext",
  },
  build: {
    outDir: "dist",
    chunkSizeWarningLimit: 1500,
    target: "esnext",
  },
});
