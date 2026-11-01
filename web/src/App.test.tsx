import { QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { api } from "./api/client";
import { isApiError } from "./api/errors";
import { createAppQueryClient } from "./app/queryClient";
import { ThemeProvider } from "./state/theme";
import { ToastProvider } from "./state/toast";
import { ConfirmProvider } from "./state/confirm";

const sampleStatus = {
  version: "2.0.0",
  commit: "abc1234",
  build_date: "2026-05-03",
  config_source: { location: "/config/config.yaml", type: "local", writable: true },
  config_revision: "sha256:saved",
  runtime_config_revision: "sha256:runtime",
  config_loaded_at: "2026-05-03T00:00:00Z",
  config_dirty: false,
  capabilities: { config_write: true, reload: true },
  last_reload: { time: "2026-05-03T00:00:01Z", success: true, duration_ms: 12 }
};

const sampleConfig = {
  config_revision: "sha256:saved",
  config: {
    sources: {
      subscriptions: [{ url: "https://sub.example.com/api?token=secret", format: "auto" }],
      snell: [],
      vless: [],
      custom_proxies: [],
      fetch_order: ["subscriptions", "snell", "vless"]
    },
    filters: { exclude: "" },
    groups: [{ key: "HK", value: { match: "(HK)", strategy: "select" } }],
    routing: [{ key: "Proxy", value: ["HK", "@auto"] }],
    rulesets: [{ key: "Proxy", value: ["https://rules.example.com/a.list", "https://rules.example.com/b.list"] }],
    rules: ["DOMAIN-SUFFIX,example.com,Proxy", "DOMAIN-KEYWORD,internal,DIRECT"],
    fallback: "Proxy",
    base_url: "https://sub.example.com",
    templates: {
      clash: "configs/base_clash.yaml",
      surge: "configs/base_surge.conf",
      loon: "configs/base_loon.conf"
    }
  }
};

function installMatchMedia() {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  });
}

function json(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" }
  });
}

function text(data: string, status = 200) {
  return new Response(data, {
    status,
    headers: { "Content-Type": "text/plain" }
  });
}

interface MockBackendOptions {
  authed?: boolean;
  resourceReadError?: boolean;
  resourceRead?: () => Promise<Response>;
  subscriptionCheck?: (input: { location?: string; content?: string; format?: string }) => Promise<Response>;
  config?: unknown;
  setupRequired?: boolean;
  readonly?: boolean;
  saveError?: { status: number; code: string; message: string };
  validateResult?: unknown;
  reloadErrors?: { status: number; code: string; message: string }[];
  groupPreviewValidationError?: boolean;
}

function mockBackend(options: MockBackendOptions = {}) {
  const calls: { path: string; init?: RequestInit }[] = [];
  const authed = options.authed ?? true;
  const setupRequired = options.setupRequired ?? false;
  let configSnapshot = clone(sampleConfig);
  let configOverride = options.config;
  let status = options.readonly
    ? {
        ...sampleStatus,
        config_source: { ...sampleStatus.config_source, writable: false },
        capabilities: { ...sampleStatus.capabilities, config_write: false }
      }
    : sampleStatus;
  const reloadErrors = [...(options.reloadErrors ?? [])];

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = input.toString();
    calls.push({ path, init });

    if (path === "/api/auth/status") {
      return json({ authed, setup_required: setupRequired, setup_token_required: setupRequired, locked_until: "" });
    }
    if (path === "/api/auth/login") return json({ redirect: "/sources" });
    if (path === "/api/auth/setup") return json({ redirect: "/sources" });
    if (path === "/api/auth/logout") return json({ success: true });
    if (path === "/api/status") return json(status);
    if (path === "/api/subscriptions/check") {
      const body = JSON.parse(String(init?.body)) as { location?: string; content?: string; format?: string };
      if (options.subscriptionCheck) return options.subscriptionCheck(body);
      const format = body.format && body.format !== "auto" ? body.format : "text";
      return json({ status: "recognized", format, node_count: 1, output_formats: format === "text" ? ["clash", "surge", "loon"] : [format], content: body.content ?? "anytls://pw@hk.example.com:443#HK" });
    }
    if (path === "/api/resources/read" && options.resourceRead) return options.resourceRead();
    if (path === "/api/resources/read" && options.resourceReadError) return json({ error: { code: "resource_local_read_failed", message: "模板文件不存在" } }, 400);
    if (path === "/api/resources/read") return json({ content: "[General]\ndns-server = system\n" });
    if (path === "/api/config") {
      if (init?.method === "PUT") {
        if (options.saveError) {
          return json({ error: { code: options.saveError.code, message: options.saveError.message } }, options.saveError.status);
        }
        const body = JSON.parse(String(init.body)) as { config: typeof sampleConfig.config };
        configSnapshot = { config_revision: "sha256:next", config: body.config };
        configOverride = undefined;
        status = { ...status, config_revision: "sha256:next", config_dirty: true };
        return json({ config_revision: "sha256:next" });
      }
      return json(configOverride === undefined ? configSnapshot : { ...configSnapshot, config: configOverride });
    }
    if (path === "/api/config/effective.yaml") return text("sources: {}\nfallback: Proxy\n");
    if (path === "/api/config/effective.zip") return text("zip");
    if (path === "/api/config/import") {
      return json({
        config: {
          ...sampleConfig.config,
          base_url: "https://import.example.com",
          groups: [{ key: "IMPORTED", value: { match: "(IMPORTED)", strategy: "select" } }]
        }
      });
    }
    if (path === "/api/config/validate") return json(options.validateResult ?? { valid: true, errors: [], warnings: [], infos: [] });
    if (path === "/api/reload") {
      const reloadError = reloadErrors.shift();
      if (reloadError) {
        return json({ error: { code: reloadError.code, message: reloadError.message } }, reloadError.status);
      }
      status = { ...status, runtime_config_revision: status.config_revision, config_dirty: false };
      return json({ success: true, duration_ms: 12 });
    }
    if (path === "/api/preview/nodes") {
      return json({
        nodes: [{ id: "proxy-1", name: "HK-01", type: "ss", kind: "subscription", server: "hk.example.com", port: 8388, filtered: false }],
        total: 1,
        active_count: 1,
        filtered_count: 0
      });
    }
    if (path === "/api/preview/groups") {
      if (options.groupPreviewValidationError) {
        return json(
          {
            valid: false,
            errors: [
              {
                severity: "error",
                code: "empty_group",
                message: "节点组没有匹配到任何节点",
                display_path: "groups[0].match",
                locator: { index: 0, json_pointer: "/config/groups/0/value/match" }
              }
            ],
            warnings: [],
            infos: []
          },
          400
        );
      }
      return json({
        node_groups: [{
          name: "HK",
          strategy: "select",
          members: ["HK-01"],
          member_details: [{ kind: "proxy", id: "proxy-1", name: "HK-01" }]
        }],
        chained_groups: [],
        service_groups: [{
          name: "Proxy",
          strategy: "select",
          members: ["HK"],
          member_details: [{ kind: "node_group", name: "HK" }],
          expanded_members: [{ value: "HK", origin: "literal" }],
          expanded_member_details: [{ kind: "node_group", name: "HK", origin: "literal" }]
        }],
        all_proxies: ["HK-01"],
        all_proxy_details: [{ kind: "proxy", id: "proxy-1", name: "HK-01" }]
      });
    }
    if (path.startsWith("/api/generate/preview")) {
      return text(init?.method === "POST" ? "proxies:\n  - name: draft-proxy\n" : "proxies:\n  - name: runtime-proxy\n");
    }
    if (path.startsWith("/api/generate/link")) {
      const url = new URL(path, "http://localhost");
      const includeToken = url.searchParams.get("include_token") !== "false";
      const format = url.searchParams.get("format") ?? "clash";
      const filename = url.searchParams.get("filename") ?? `${format}.yaml`;
      return json({
        url: `https://sub.example.com/generate?format=${format}${includeToken ? "&token=server-token" : ""}&filename=${filename}`,
        token_included: includeToken
      });
    }
    if (path === "/healthz") return text("OK");
    return json({ error: { code: "not_found", message: path } }, 404);
  });
  vi.stubGlobal("fetch", fetchMock);
  return { fetchMock, calls };
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function lastSavedConfig(calls: { path: string; init?: RequestInit }[]) {
  const saveCall = calls.filter((item) => item.path === "/api/config" && item.init?.method === "PUT").at(-1);
  if (!saveCall?.init?.body) return null;
  return (JSON.parse(String(saveCall.init.body)) as { config: typeof sampleConfig.config }).config;
}

function renderApp(initialPath = "/sources") {
  const queryClient = createAppQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <ToastProvider>
          <ConfirmProvider>
            <MemoryRouter initialEntries={[initialPath]}>
              <App />
            </MemoryRouter>
          </ConfirmProvider>
        </ToastProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  installMatchMedia();
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("M9 app shell", () => {
  it("protects routes and renders the M9 sources page", async () => {
    mockBackend();
    renderApp("/sources");

    expect(await screen.findByRole("heading", { level: 1, name: "订阅来源" })).toBeTruthy();
    expect(screen.getByRole("link", { name: /节点预览/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(true);
  });

  it("keeps M10 routes inside the protected SPA", async () => {
    const backend = mockBackend();
    renderApp("/download");

    expect(await screen.findByRole("heading", { level: 1, name: "生成下载" })).toBeTruthy();
    await waitFor(() => expect(backend.calls.some((c) => c.path === "/api/generate/preview?format=clash")).toBe(true));
  });
});

describe("M10 frontend workflows", () => {
  it("T-WEB-011 edits ruleset policy and URL list while preserving order", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/rulesets");

    expect(await screen.findByRole("heading", { level: 1, name: "规则集" })).toBeTruthy();
    expect(await screen.findByDisplayValue("https://rules.example.com/a.list")).toBeTruthy();
    fireEvent.change(screen.getAllByRole("combobox")[0], { target: { value: "DIRECT" } });
    const urlInputs = screen.getAllByPlaceholderText("https://example.com/rules.list");
    fireEvent.change(urlInputs[1], { target: { value: "https://rules.example.com/c.list" } });
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => {
      const saved = lastSavedConfig(backend.calls);
      expect(saved?.rulesets?.[0]).toEqual({
        key: "DIRECT",
        value: ["https://rules.example.com/a.list", "https://rules.example.com/c.list"]
      });
    });
  });

  it("T-WEB-012 keeps raw inline rules as source of truth and replaces only the trailing policy", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/rules");

    expect(await screen.findByRole("heading", { level: 1, name: "内联规则" })).toBeTruthy();
    const ruleInput = await screen.findByDisplayValue("DOMAIN-SUFFIX,example.com,Proxy");
    const policySelector = await screen.findByDisplayValue("Proxy");
    expect((policySelector as HTMLSelectElement).disabled).toBe(false);
    fireEvent.change(ruleInput, { target: { value: "DOMAIN-SUFFIX,example.com,DIRECT" } });
    expect(await screen.findByDisplayValue("DOMAIN-SUFFIX,example.com,DIRECT")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => {
      const saved = lastSavedConfig(backend.calls);
      expect(saved?.rules).toEqual(["DOMAIN-SUFFIX,example.com,DIRECT", "DOMAIN-KEYWORD,internal,DIRECT"]);
    });
  });

  it("T-WEB-013 edits settings and disables fields for readonly config sources", async () => {
    const writableBackend = mockBackend();
    renderApp("/settings");

    expect(await screen.findByRole("heading", { level: 1, name: "其他配置" })).toBeTruthy();
    fireEvent.change(await screen.findByDisplayValue("https://sub.example.com"), { target: { value: "https://edge.example.com" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByRole("dialog", { name: "将草稿写入 YAML 文件？" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "确认保存" }));

    await waitFor(() => expect(lastSavedConfig(writableBackend.calls)?.base_url).toBe("https://edge.example.com"));
    cleanup();
    vi.restoreAllMocks();
    installMatchMedia();
    localStorage.clear();

    mockBackend({ readonly: true });
    renderApp("/settings");

    expect((await screen.findByDisplayValue("https://sub.example.com") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByDisplayValue("Proxy") as HTMLSelectElement).disabled).toBe(true);
    expect((screen.getByDisplayValue("configs/base_clash.yaml") as HTMLInputElement).disabled).toBe(true);
  });

  it("T-WEB-014 shows validation diagnostics in a drawer and jumps by json pointer", async () => {
    mockBackend({
      validateResult: {
        valid: false,
        errors: [
          {
            severity: "error",
            code: "invalid_group_regex",
            message: "分组正则无效",
            display_path: "groups[0].match",
            locator: { json_pointer: "/config/groups/0/value/match", index: 0 }
          }
        ],
        warnings: [
          {
            severity: "warning",
            code: "base_url_empty",
            message: "base_url 为空",
            display_path: "base_url",
            locator: { json_pointer: "/config/base_url" }
          }
        ],
        infos: [
          {
            severity: "info",
            code: "templates_optional",
            message: "模板可选",
            display_path: "templates",
            locator: { json_pointer: "/config/templates" }
          }
        ]
      }
    });
    renderApp("/validate");

    expect(await screen.findByRole("heading", { level: 1, name: "静态校验" })).toBeTruthy();
    await waitFor(() => expect(screen.getByRole("button", { name: "运行静态校验" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "运行静态校验" }));
    fireEvent.click(await screen.findByText("invalid_group_regex"));

    expect(await screen.findByRole("dialog", { name: "分组正则无效" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "跳转字段" }));
    expect(await screen.findByRole("heading", { level: 1, name: "节点分组" })).toBeTruthy();
    expect(screen.getByDisplayValue("(HK)")).toBeTruthy();
  });

  it("T-WEB-015 renders runtime group preview success and graph diagnostics failure", async () => {
    mockBackend();
    renderApp("/preview/groups");

    expect(await screen.findByRole("heading", { level: 1, name: "分组预览" })).toBeTruthy();
    expect(await screen.findByText(/All proxies/)).toBeTruthy();
    expect(screen.getAllByText("HK-01").length).toBeGreaterThan(0);

    cleanup();
    vi.restoreAllMocks();
    installMatchMedia();
    localStorage.clear();

    mockBackend({ groupPreviewValidationError: true });
    renderApp("/preview/groups");

    expect(await screen.findByText("图级校验失败", {}, { timeout: 3000 })).toBeTruthy();
    expect(screen.getByText("empty_group")).toBeTruthy();
    expect(screen.queryByText(/All proxies/)).toBeNull();
  });

  it("T-WEB-016 previews generated config and confirms copying token-included links", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const backend = mockBackend();
    renderApp("/download");

    expect(await screen.findByRole("heading", { level: 1, name: "生成下载" })).toBeTruthy();
    expect((await screen.findAllByText(/runtime-proxy/)).length).toBeGreaterThan(0);

    fireEvent.click(screen.getAllByRole("button", { name: /复制/ })[0]);
    expect(await screen.findByRole("dialog", { name: "复制含 token 的订阅链接？" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "确认复制" }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(expect.stringContaining("token=server-token")));
    expect(backend.calls.some((item) => item.path.startsWith("/api/generate/link"))).toBe(true);
    expect(backend.calls.some((item) => item.path === "/api/generate/preview?format=clash")).toBe(true);
  });

  it("T-WEB-022 imports YAML into the draft and exports the effective config package", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    const { container } = renderApp("/download");

    expect(await screen.findByRole("heading", { level: 1, name: "生成下载" })).toBeTruthy();
    const exportLink = await screen.findByRole("link", { name: "导出生效配置" });
    expect(exportLink.getAttribute("href")).toBe("/api/config/effective.zip");
    expect(exportLink.getAttribute("download")).toBe("subconverter-config.zip");

    const input = container.querySelector('input[type="file"]') as HTMLInputElement | null;
    expect(input).toBeTruthy();
    const file = new File(["base_url: https://import.example.com\n"], "config.yaml", { type: "application/x-yaml" });
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => {
      const saved = lastSavedConfig(backend.calls);
      expect(saved?.base_url).toBe("https://import.example.com");
      expect(saved?.groups?.[0]?.key).toBe("IMPORTED");
    });
  });

  it("T-WEB-024 imports a config package as ZIP and confirms template copy", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    const { container } = renderApp("/download");

    expect(await screen.findByRole("heading", { level: 1, name: "生成下载" })).toBeTruthy();
    const input = container.querySelector('input[type="file"]') as HTMLInputElement | null;
    expect(input).toBeTruthy();
    const file = new File(["zip-bytes"], "subconverter-config.zip", { type: "application/zip" });
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    expect(await screen.findByRole("dialog", { name: "导入配置包并写入模板副本？" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "确认导入" }));

    await waitFor(() => {
      const importCall = backend.calls.find((item) => item.path === "/api/config/import" && item.init?.method === "POST");
      expect(importCall?.init?.headers).toEqual({ "Content-Type": "application/zip" });
    });
    await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
  });

  it("T-WEB-023 disables import for readonly config sources but keeps effective export", async () => {
    mockBackend({ readonly: true });
    renderApp("/download");

    expect(await screen.findByRole("heading", { level: 1, name: "生成下载" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "导入配置" }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("link", { name: "导出生效配置" }).getAttribute("href")).toBe("/api/config/effective.zip");
  });
});

describe("auth", () => {
  it("renders setup mode with bootstrap token", async () => {
    mockBackend({ authed: false, setupRequired: true });
    renderApp("/login");

    expect(await screen.findByRole("heading", { name: "首次创建管理员" })).toBeTruthy();
    expect(screen.getAllByText(/服务日志/).length).toBeGreaterThan(0);
  });
});

describe("preview workflows", () => {
  it("uses POST draft preview on A2 filters", async () => {
    const backend = mockBackend();
    renderApp("/filters");

    await screen.findByRole("heading", { level: 1, name: "过滤器" });
    await waitFor(() => expect(screen.getByRole("button", { name: "草稿节点预览" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "草稿节点预览" }));

    await waitFor(() => {
      const call = backend.calls.find((item) => item.path === "/api/preview/nodes" && item.init?.method === "POST");
      expect(call).toBeTruthy();
      expect(String(call?.init?.body)).toContain('"config"');
    });
  });

  it("uses GET runtime preview on B1 nodes", async () => {
    const backend = mockBackend();
    renderApp("/nodes");

    expect(await screen.findByText("HK-01")).toBeTruthy();
    const getCall = backend.calls.find((item) => item.path === "/api/preview/nodes" && !item.init?.method);
    expect(getCall).toBeTruthy();
  });
});

describe("save workflow", () => {
  it("confirms YAML formatting loss before the first PUT and does not auto-reload", async () => {
    const backend = mockBackend();
    renderApp("/sources");

    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getByRole("button", { name: "添加 节点订阅" }));
    expect(await screen.findByRole("dialog", { name: "添加 节点订阅" })).toBeTruthy();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "https://sub.example.com/new?token=secret" } });
    await finishSourceEdit();
    await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByRole("dialog", { name: "将草稿写入 YAML 文件？" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "确认保存" }));

    await waitFor(() => {
      expect(backend.calls.some((item) => item.path === "/api/config/validate" && item.init?.method === "POST")).toBe(true);
      expect(backend.calls.some((item) => item.path === "/api/config" && item.init?.method === "PUT")).toBe(true);
    });
    expect(backend.calls.some((item) => item.path === "/api/reload" && item.init?.method === "POST")).toBe(false);
  });

  it("navigates to /validate when static validation fails on save", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend({
      validateResult: {
        valid: false,
        errors: [
          {
            severity: "error",
            code: "empty_group",
            message: "节点组 HK 为空",
            display_path: "groups[HK].match",
            locator: { section: "groups", json_pointer: "/config/groups/0/value/match" }
          }
        ],
        warnings: [],
        infos: []
      }
    });
    renderApp("/sources");

    await addSubscriptionDraft();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByRole("heading", { level: 1, name: "静态校验" })).toBeTruthy();
    expect(await screen.findByText("节点组 HK 为空")).toBeTruthy();
    expect(backend.calls.some((item) => item.path === "/api/config" && item.init?.method === "PUT")).toBe(false);
  });

  it.each([
    {
      code: "config_revision_conflict",
      title: "配置文件已被外部修改",
      action: "重新加载配置"
    },
    {
      code: "config_source_readonly",
      title: "配置源只读",
      action: ""
    },
    {
      code: "config_file_not_writable",
      title: "配置文件不可写",
      action: ""
    },
    {
      code: "resource_content_write_failed",
      title: "正文目录无法写入",
      action: ""
    },
    {
      code: "unexpected_conflict",
      title: "未知保存冲突",
      action: ""
    }
  ])("branches 409 save error $code", async ({ code, title, action }) => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    mockBackend({ saveError: { status: 409, code, message: "保存冲突" } });
    renderApp("/sources");

    await addSubscriptionDraft();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByText(title)).toBeTruthy();
    if (action) {
      expect(screen.getByRole("button", { name: action })).toBeTruthy();
    }
  });
});

describe("high fidelity interactions", () => {
  it("adds a routing member from the A4 palette", async () => {
    mockBackend();
    renderApp("/routing");

    await screen.findByRole("heading", { level: 1, name: "路由策略" });
    expect(screen.getAllByRole("button", { name: "拖拽成员排序" })).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: /@all/ }));

    await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
    expect(screen.getAllByText("@all").length).toBeGreaterThan(0);
  });

  it("shows a reload backoff hint on 429 and retries once", async () => {
    mockBackend({ reloadErrors: [{ status: 429, code: "reload_in_progress", message: "已有 reload 正在执行" }] });
    renderApp("/status");

    await screen.findByRole("heading", { level: 1, name: "系统状态" });
    fireEvent.click(screen.getByRole("button", { name: "热重载" }));

    expect(await screen.findByText("Reload 正在执行")).toBeTruthy();
    expect(await screen.findByText("RuntimeConfig 已重新加载", {}, { timeout: 3000 })).toBeTruthy();
  }, 7000);

  it("renders A3 draft preview validation diagnostics with json pointer location", async () => {
    mockBackend({ groupPreviewValidationError: true });
    renderApp("/groups");

    await screen.findByRole("heading", { level: 1, name: "节点分组" });
    await waitFor(() => expect(screen.getByRole("button", { name: "草稿分组预览" }).hasAttribute("disabled")).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "草稿分组预览" }));

    expect(await screen.findByText("empty_group")).toBeTruthy();
    expect(screen.getByText("/config/groups/0/value/match")).toBeTruthy();
    expect(screen.getByRole("button", { name: "定位字段" })).toBeTruthy();
  });
});

describe("API client", () => {
  it("normalizes 409 config errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        json({ error: { code: "config_revision_conflict", message: "配置文件已被其他来源修改", current_config_revision: "sha256:new" } }, 409)
      )
    );

    await expect(api.saveConfig("sha256:old", {})).rejects.toMatchObject({
      status: 409,
      code: "config_revision_conflict"
    });

    try {
      await api.saveConfig("sha256:old", {});
    } catch (error) {
      expect(isApiError(error)).toBe(true);
    }
  });
});


describe("Loon support", () => {
  // T-WEB-LOON-001: editing a subscription keeps its explicit format through the save workflow.
  it("saves the selected subscription format", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "更正格式" }));
    const select = screen.getByLabelText("按哪种格式解析");
    fireEvent.change(select, { target: { value: "loon" } });
    await finishSourceEdit();
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "更正格式" }));
    expect((screen.getByLabelText("按哪种格式解析") as HTMLSelectElement).value).toBe("loon");
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "https://example.com/loon.conf" } });
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.sources.subscriptions[0]).toEqual({ url: "https://example.com/loon.conf", format: "loon" }));
  });

  // T-WEB-LOON-002: template editing survives config JSON writeback.
  it("saves the Loon template", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/settings");
    const input = await screen.findByLabelText("Loon 模板");
    fireEvent.change(input, { target: { value: "configs/custom_loon.conf" } });
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.templates.loon).toBe("configs/custom_loon.conf"));
  });

  // T-WEB-LOON-003: runtime previews expose the Loon output alongside existing formats.
  it("loads the Loon preview and subscription link", async () => {
    const backend = mockBackend();
    renderApp("/download");
    expect(await screen.findByRole("heading", { name: "Loon", level: 3 })).toBeTruthy();
    await waitFor(() => {
      expect(backend.calls.some((call) => call.path === "/api/generate/preview?format=loon")).toBe(true);
      expect(backend.calls.some((call) => call.path.startsWith("/api/generate/link?format=loon"))).toBe(true);
    });
  });
});

async function addSubscriptionDraft() {
  await screen.findByRole("heading", { level: 1, name: "订阅来源" });
  fireEvent.click(screen.getByRole("button", { name: "添加 节点订阅" }));
  expect(await screen.findByRole("dialog", { name: "添加 节点订阅" })).toBeTruthy();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "https://sub.example.com/new?token=secret" } });
  await finishSourceEdit();
  await waitFor(() => expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(false));
}


describe("inline content editing", () => {
  // T-WEB-CHECK-001: checking a link keeps automatic updates and exposes a separate copy action.
  it("checks a link without replacing it with a body", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const content = '[Proxy]\nHK = anytls, hk.example.com, 443, "pw"';
    const backend = mockBackend({ subscriptionCheck: async () => json({ status: "recognized", format: "loon", node_count: 4, output_formats: ["loon"], content }) });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    expect(screen.queryByLabelText("按哪种格式解析")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    expect(await screen.findByText("已识别：Loon 配置 · 4 个节点")).toBeTruthy();
    expect(screen.queryByLabelText("节点正文")).toBeNull();
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.sources.subscriptions[0]).toEqual({ url: sampleConfig.config.sources.subscriptions[0].url, format: "loon" }));
    expect(backend.calls.some((call) => call.path === "/api/resources/read")).toBe(false);
  });

  // T-WEB-CHECK-002: shared INI syntax prompts for its original client and rechecks the selection.
  it("asks for the client only when the INI source is ambiguous", async () => {
    const backend = mockBackend({ subscriptionCheck: async (input) => json(input.format === "loon"
      ? { status: "recognized", format: "loon", node_count: 1, output_formats: ["loon"], content: "[Proxy]\nHK = http, hk.example.com, 80" }
      : { status: "ambiguous", node_count: 1, output_formats: [], candidates: ["surge", "loon"], content: "[Proxy]\nHK = http, hk.example.com, 80" }) });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    await screen.findByText("这份配置来自哪个客户端？");
    expect(screen.getByRole("button", { name: "保存来源" }).hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Loon" }));
    await screen.findByText("已识别：Loon 配置 · 1 个节点");
    expect(JSON.parse(String(backend.calls.filter((call) => call.path === "/api/subscriptions/check").at(-1)?.init?.body)).format).toBe("loon");
  });

  // T-WEB-CHECK-003: an outdated response cannot overwrite a later correction or new input.
  it("ignores an earlier check after the user corrects the format", async () => {
    let finish!: (response: Response) => void;
    const pending = new Promise<Response>((resolve) => { finish = resolve; });
    let calls = 0;
    mockBackend({ subscriptionCheck: async () => ++calls === 1 ? pending : json({ status: "recognized", format: "loon", node_count: 2, output_formats: ["loon"], content: "[Proxy]" }) });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    fireEvent.click(screen.getByRole("button", { name: "更正格式" }));
    fireEvent.change(screen.getByLabelText("按哪种格式解析"), { target: { value: "loon" } });
    await screen.findByText("已识别：Loon 配置 · 2 个节点");
    finish(json({ status: "recognized", format: "surge", node_count: 9, output_formats: ["surge"], content: "old" }));
    await waitFor(() => expect(screen.queryByText("已识别：Surge 配置 · 9 个节点")).toBeNull());
    expect((screen.getByLabelText("按哪种格式解析") as HTMLSelectElement).value).toBe("loon");
    fireEvent.change(screen.getByLabelText("订阅地址"), { target: { value: "https://example.com/changed" } });
    expect(screen.queryByText("已识别：Loon 配置 · 2 个节点")).toBeNull();
    expect(screen.getByRole("button", { name: "保存来源" }).hasAttribute("disabled")).toBe(true);
  });

  // T-WEB-CHECK-004: copying an inspected resource retains the checked bytes and format.
  it("edits an inspected body copy explicitly", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const content = '[Proxy]\nHK = anytls, hk.example.com, 443, "pw"';
    const backend = mockBackend({ subscriptionCheck: async () => json({ status: "recognized", format: "loon", node_count: 1, output_formats: ["loon"], content }) });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    fireEvent.click(await screen.findByRole("button", { name: "编辑正文副本" }));
    expect((screen.getByLabelText("节点正文") as HTMLTextAreaElement).value).toBe(content);
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.sources.subscriptions[0]).toEqual({ content, format: "loon" }));
  });

  // T-WEB-CHECK-005: parse failures keep input editable and prevent a successful result.
  it("keeps failed body checks editable", async () => {
    mockBackend({ subscriptionCheck: async () => json({ error: { code: "fetch_subscription_empty", message: "订阅未产生任何有效节点" } }, 400) });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    fireEvent.change(screen.getByLabelText("节点正文"), { target: { value: "invalid body" } });
    await screen.findByRole("alert");
    expect((screen.getByLabelText("节点正文") as HTMLTextAreaElement).value).toBe("invalid body");
    expect(screen.getByRole("button", { name: "保存来源" }).hasAttribute("disabled")).toBe(true);
    expect(screen.queryByText(/已识别：/)).toBeNull();
  });

  // T-WEB-CHECK-009: a node URI in the address field receives guidance instead of a local-file error.
  it("directs node URIs to the body input", async () => {
    const backend = mockBackend();
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.change(screen.getByLabelText("订阅地址"), { target: { value: "anytls://pw@hk.example.com:443#HK" } });
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    expect(await screen.findByText("请输入 HTTP(S) 订阅地址；单个节点请粘贴到“节点正文”。")).toBeTruthy();
    expect(backend.calls.some((call) => call.path === "/api/subscriptions/check")).toBe(false);
  });

  // T-WEB-CHECK-008: refreshing clears the previous result until the new check succeeds.
  it("blocks copying or saving an earlier result during a refresh", async () => {
    let finish!: (response: Response) => void;
    const pending = new Promise<Response>((resolve) => { finish = resolve; });
    let count = 0;
    mockBackend({ subscriptionCheck: async () => ++count === 1
      ? json({ status: "recognized", format: "loon", node_count: 1, output_formats: ["loon"], content: "[Proxy]" }) : pending });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    await screen.findByRole("button", { name: "编辑正文副本" });
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    expect(screen.queryByRole("button", { name: "编辑正文副本" })).toBeNull();
    expect(screen.getByRole("button", { name: "保存来源" }).hasAttribute("disabled")).toBe(true);
    finish(json({ status: "recognized", format: "loon", node_count: 3, output_formats: ["loon"], content: "[Proxy]" }));
    await screen.findByText("已识别：Loon 配置 · 3 个节点");
  });

  // T-WEB-CHECK-006: a file that cannot be checked can still be replaced with pasted nodes.
  it("repairs a missing subscription file without blocking other fields", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend({ config: { ...sampleConfig.config, sources: { ...sampleConfig.config.sources, subscriptions: [{ content_file: "/config/missing.txt", format: "loon" }] } },
      subscriptionCheck: async (input) => input.location
        ? json({ error: { code: "resource_local_read_failed", message: "正文文件不存在" } }, 500)
        : json({ status: "recognized", format: "loon", node_count: 1, output_formats: ["loon"], content: input.content })
    });
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    expect((screen.getByLabelText("正文文件") as HTMLInputElement).value).toBe("/config/missing.txt");
    fireEvent.click(screen.getByRole("button", { name: "检查订阅" }));
    await screen.findByText("正文文件不存在");
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    const content = 'HK = anytls, hk.example.com, 443, "pw"';
    fireEvent.change(screen.getByLabelText("节点正文"), { target: { value: content } });
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.sources.subscriptions[0]).toEqual({ content, format: "loon" }));
  });

  // T-WEB-CHECK-007: body editing is checked automatically without a second user action.
  it("checks pasted text automatically and preserves it when modes are switched", async () => {
    const backend = mockBackend();
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    const content = "anytls://pw@hk.example.com:443#HK";
    fireEvent.change(screen.getByLabelText("节点正文"), { target: { value: content } });
    await screen.findByText("已识别：节点列表 · 1 个节点");
    expect(backend.calls.filter((call) => call.path === "/api/subscriptions/check")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "订阅链接" }));
    expect((screen.getByLabelText("订阅地址") as HTMLInputElement).value).toBe(sampleConfig.config.sources.subscriptions[0].url);
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    expect((screen.getByLabelText("节点正文") as HTMLTextAreaElement).value).toBe(content);
  });

  // T-WEB-STORE-001: a storage error keeps body edits and permits retry after permissions are fixed.
  it("preserves body drafts on directory write errors and allows retry", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const options: MockBackendOptions = { saveError: { status: 409, code: "resource_content_write_failed", message: "正文保存失败：目录 /config 无法写入，请检查目录权限" } };
    const backend = mockBackend(options);
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    const content = "anytls://pw@hk.example.com:443#HK-EDITED";
    fireEvent.change(screen.getByLabelText("节点正文"), { target: { value: content } });
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByText("正文目录无法写入")).toBeTruthy();
    expect(screen.getByText("正文保存失败：目录 /config 无法写入，请检查目录权限")).toBeTruthy();
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    expect((screen.getByLabelText("节点正文") as HTMLTextAreaElement).value).toBe(content);
    await finishSourceEdit();
    options.saveError = undefined;
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(backend.calls.filter((call) => call.path === "/api/config" && call.init?.method === "PUT")).toHaveLength(2));
    await screen.findByText("草稿已写入 YAML 文件");
  });

  // T-WEB-INLINE-005: failed reads leave the reference and draft unchanged.
  it("shows read failures without replacing the template", async () => {
    mockBackend({ resourceReadError: true });
    renderApp("/settings");
    await screen.findByLabelText("Loon 模板");
    fireEvent.click(screen.getAllByRole("button", { name: "读取并编辑正文" })[2]);
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect((screen.getByLabelText("Loon 模板") as HTMLInputElement).value).toBe("configs/base_loon.conf");
    expect(screen.queryByLabelText("Loon 模板正文")).toBeNull();
    expect(screen.getByRole("button", { name: "保存" }).hasAttribute("disabled")).toBe(true);
  });

  // T-WEB-INLINE-001: switching a URL source to pasted nodes clears the old URL on save.
  it("saves pasted subscription nodes", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/sources");
    await screen.findByRole("heading", { level: 1, name: "订阅来源" });
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "节点正文" }));
    const content = 'HK = anytls, hk.example.com, 443, "pw"';
    fireEvent.change(screen.getByLabelText("节点正文"), { target: { value: content } });
    fireEvent.click(screen.getByRole("button", { name: "更正格式" }));
    fireEvent.change(screen.getByLabelText("按哪种格式解析"), { target: { value: "loon" } });
    await finishSourceEdit();
    fireEvent.click(screen.getAllByRole("button", { name: "编辑来源" })[0]);
    expect((screen.getByLabelText("节点正文") as HTMLTextAreaElement).value).toBe(content);
    await finishSourceEdit();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(lastSavedConfig(backend.calls)?.sources.subscriptions[0]).toEqual({ content, format: "loon" }));
  });

  // T-WEB-INLINE-002: reading a referenced template creates an editable draft body.
  it("reads and saves a template body without its old path", async () => {
    localStorage.setItem("subconverter.firstSaveConfirmed", "true");
    const backend = mockBackend();
    renderApp("/settings");
    await screen.findByLabelText("Loon 模板");
    fireEvent.click(screen.getAllByRole("button", { name: "读取并编辑正文" })[2]);
    const input = await screen.findByLabelText("Loon 模板正文");
    expect((input as HTMLTextAreaElement).value).toBe("[General]\ndns-server = system\n");
    const content = "[General]\ndns-server = 1.1.1.1\n";
    fireEvent.change(input, { target: { value: content } });
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => {
      const saved = lastSavedConfig(backend.calls) as { templates: { loon?: string; inline?: { loon?: string } } } | undefined;
      expect(saved?.templates.inline?.loon).toBe(content);
      expect(saved?.templates.loon).toBeUndefined();
    });
    expect(JSON.parse(String(backend.calls.find((call) => call.path === "/api/resources/read")?.init?.body))).toEqual({ location: "configs/base_loon.conf" });
  });

  // T-WEB-INLINE-003: mode switching preserves text until the draft is applied.
  it("keeps unsaved template text while switching modes", async () => {
    mockBackend();
    renderApp("/settings");
    const select = await screen.findByLabelText("Loon 模板来源");
    fireEvent.change(select, { target: { value: "content" } });
    fireEvent.change(screen.getByLabelText("Loon 模板正文"), { target: { value: "[General]\ndns-server = system" } });
    fireEvent.change(select, { target: { value: "location" } });
    expect((screen.getByLabelText("Loon 模板") as HTMLInputElement).value).toBe("configs/base_loon.conf");
    fireEvent.change(select, { target: { value: "content" } });
    expect((screen.getByLabelText("Loon 模板正文") as HTMLTextAreaElement).value).toBe("[General]\ndns-server = system");
  });

  // T-WEB-INLINE-004: readonly capability prevents both mode changes and resource imports.
  it("keeps content editors readonly", async () => {
    mockBackend({ readonly: true });
    renderApp("/settings");
    expect((await screen.findByLabelText("Loon 模板来源")).hasAttribute("disabled")).toBe(true);
    for (const button of screen.getAllByRole("button", { name: "读取并编辑正文" })) expect(button.hasAttribute("disabled")).toBe(true);
  });
});


async function finishSourceEdit() {
  const save = screen.getByRole("button", { name: "保存来源" });
  if (save.hasAttribute("disabled")) {
    const check = screen.queryByRole("button", { name: "检查订阅" }) ?? screen.getByRole("button", { name: "识别并检查" });
    fireEvent.click(check);
    await waitFor(() => expect(save.hasAttribute("disabled")).toBe(false));
  }
  fireEvent.click(save);
}
