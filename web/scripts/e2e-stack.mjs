import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = path.resolve(webDir, "..");
const apiPort = Number(process.env.E2E_API_PORT ?? "18080");
const webPort = Number(process.env.E2E_WEB_PORT ?? "15173");
const fakePort = Number(process.env.E2E_FAKE_PORT ?? "18081");
const setupToken = process.env.E2E_SETUP_TOKEN ?? "setup-e2e-secret";
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? "admin-password-123";
const tempRoot = path.join(tmpdir(), `subconverter-e2e-${Date.now()}`);

const children = [];
let fakeServer;

try {
  await mkdir(tempRoot, { recursive: true });
  fakeServer = await startFakeUpstream(fakePort);
  await writeFixtureConfig(path.join(tempRoot, "config.yaml"));

  const api = spawnManaged(
    "go",
    [
      "run",
      "./cmd/subconverter",
      "-config",
      path.join(tempRoot, "config.yaml"),
      "-listen",
      `127.0.0.1:${apiPort}`,
      "-auth-state",
      path.join(tempRoot, "auth.json"),
      "-setup-token",
      setupToken,
      "-access-token",
      "server-token"
    ],
    { cwd: repoRoot, name: "api" }
  );
  children.push(api);
  await waitForHTTP(`http://127.0.0.1:${apiPort}/healthz`, "");

  const vite = spawnManaged(
    process.execPath,
    [path.join(webDir, "node_modules/vite/bin/vite.js"), "--host", "127.0.0.1", "--port", String(webPort)],
    {
      cwd: webDir,
      name: "vite",
      env: { SUBCONVERTER_API_TARGET: `http://127.0.0.1:${apiPort}` }
    }
  );
  children.push(vite);
  await waitForHTTP(`http://127.0.0.1:${webPort}/`, "<!doctype html>");

  const playwrightCli = resolvePlaywrightCli();
  const result = await runCommand(process.execPath, [playwrightCli, "test", "--config", path.join(webDir, "playwright.config.ts")], {
    cwd: webDir,
    env: {
      E2E_BASE_URL: `http://127.0.0.1:${webPort}`,
      E2E_SETUP_TOKEN: setupToken,
      E2E_ADMIN_PASSWORD: adminPassword
    }
  });
  process.exitCode = result;
} finally {
  for (const child of children.reverse()) {
    killProcessGroup(child);
  }
  if (fakeServer) {
    await new Promise((resolve) => fakeServer.close(resolve));
  }
  await rm(tempRoot, { recursive: true, force: true });
}

function startFakeUpstream(port) {
  const subscription = Buffer.from("ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@hk.example.com:8388#HK-01\n").toString("base64");
  const rules = "DOMAIN-SUFFIX,example.net\n";
  const clashTemplate = "mixed-port: 7890\nmode: rule\nlog-level: info\n";
  const surgeTemplate = "[General]\nloglevel = notify\n";

  const server = createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://127.0.0.1:${port}`);
    const routes = {
      "/subscription.txt": ["text/plain", subscription],
      "/rules.list": ["text/plain", rules],
      "/base_clash.yaml": ["text/yaml", clashTemplate],
      "/base_surge.conf": ["text/plain", surgeTemplate]
    };
    const route = routes[url.pathname];
    if (!route) {
      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("not found");
      return;
    }
    res.writeHead(200, { "Content-Type": route[0], "Cache-Control": "no-store" });
    res.end(route[1]);
  });

  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", () => resolve(server));
  });
}

async function writeFixtureConfig(configPath) {
  const config = `base_url: "http://127.0.0.1:${apiPort}"
templates:
  clash: "http://127.0.0.1:${fakePort}/base_clash.yaml"
  surge: "http://127.0.0.1:${fakePort}/base_surge.conf"
sources:
  subscriptions:
    - url: "http://127.0.0.1:${fakePort}/subscription.txt?token=upstream-secret"
  snell: []
  vless: []
  custom_proxies:
    - name: "LOCAL-DIRECT"
      url: "http://user:pass@127.0.0.1:8081"
filters:
  exclude: "(Expire|Traffic)"
groups:
  HK: { match: "(HK|Hong Kong)", strategy: select }
routing:
  Proxy: ["HK", "DIRECT"]
  Direct: ["DIRECT"]
rulesets:
  Proxy:
    - "http://127.0.0.1:${fakePort}/rules.list"
rules:
  - "DOMAIN-SUFFIX,example.org,Proxy"
fallback: Proxy
`;
  await writeFile(configPath, config, "utf8");
}

function spawnManaged(command, args, options) {
  const child = spawn(command, args, {
    cwd: options.cwd,
    env: { ...process.env, ...(options.env ?? {}) },
    detached: process.platform !== "win32",
    stdio: ["ignore", "pipe", "pipe"]
  });
  child.stdout.on("data", (chunk) => process.stdout.write(`[${options.name}] ${chunk}`));
  child.stderr.on("data", (chunk) => process.stderr.write(`[${options.name}] ${chunk}`));
  child.on("exit", (code, signal) => {
    if (code && code !== 0) {
      process.stderr.write(`[${options.name}] exited with code ${code}${signal ? ` signal ${signal}` : ""}\n`);
    }
  });
  return child;
}

function killProcessGroup(child) {
  if (!child.pid) return;
  try {
    if (process.platform === "win32") {
      child.kill("SIGTERM");
    } else {
      process.kill(-child.pid, "SIGTERM");
    }
  } catch {
    try {
      child.kill("SIGTERM");
    } catch {
      // Already exited.
    }
  }
}

function runCommand(command, args, options) {
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: { ...process.env, ...(options.env ?? {}) },
      stdio: "inherit"
    });
    child.on("exit", (code) => resolve(code ?? 1));
  });
}

async function waitForHTTP(url, expectedText) {
  const deadline = Date.now() + 60_000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      const body = await response.text();
      if (response.ok && (!expectedText || body.includes(expectedText))) return;
      lastError = new Error(`${url} returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 300));
  }
  throw lastError ?? new Error(`${url} did not become ready`);
}

function resolvePlaywrightCli() {
  const direct = path.join(webDir, "node_modules/@playwright/test/cli.js");
  if (existsSync(direct)) return direct;
  const local = path.join(webDir, "node_modules/playwright/cli.js");
  if (existsSync(local)) return local;
  throw new Error("Playwright CLI not found. Run `pnpm install` at the repository root first.");
}
