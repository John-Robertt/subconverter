import { defineConfig, devices } from "playwright/test";

const port = Number(process.env.E2E_INT_PORT ?? "5273");
const baseURL = process.env.E2E_INT_BASE_URL ?? `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./e2e/integration",
  timeout: 30_000,
  expect: { timeout: 6_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: process.env.CI ? [["list"], ["html", { outputFolder: "playwright-report-int", open: "never" }]] : [["list"]],
  use: {
    baseURL,
    viewport: { width: 1280, height: 800 },
    permissions: ["clipboard-read", "clipboard-write"],
    trace: "retain-on-failure",
    screenshot: "only-on-failure"
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] }
    }
  ],
  webServer: process.env.E2E_INT_BASE_URL
    ? undefined
    : {
        command: `node node_modules/vite/bin/vite.js --host 127.0.0.1 --port ${port} --strictPort`,
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 60_000
      }
});
