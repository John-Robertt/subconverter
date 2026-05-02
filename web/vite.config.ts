import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const apiTarget = "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: apiTarget,
        changeOrigin: true
      },
      "/generate": {
        target: apiTarget,
        changeOrigin: true
      },
      "/healthz": {
        target: apiTarget,
        changeOrigin: true
      }
    }
  }
});
