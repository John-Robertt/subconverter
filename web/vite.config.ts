import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const apiTarget = process.env.SUBCONVERTER_API_TARGET ?? "http://localhost:8080";
const proxyTarget = {
  target: apiTarget,
  changeOrigin: true,
  headers: {
    Origin: apiTarget
  },
  configure(proxy: { on: (event: "proxyReq", handler: (proxyReq: { setHeader: (name: string, value: string) => void }) => void) => void }) {
    proxy.on("proxyReq", (proxyReq) => {
      proxyReq.setHeader("Origin", apiTarget);
    });
  }
};

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        ...proxyTarget
      },
      "/generate": {
        ...proxyTarget
      },
      "/healthz": {
        ...proxyTarget
      }
    }
  }
});
