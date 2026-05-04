// Route-level mock for the Admin API. Each test installs a `MockState`
// instance, which holds the responses to return per endpoint. Tests can patch
// `state` mid-test to simulate state transitions (login → authed, save → new
// revision, etc.).

import type { Page, Route } from "@playwright/test";
import {
  CLASH_PREVIEW,
  REVISION_BASE,
  SUBSCRIPTION_LINK_NO_TOKEN,
  SUBSCRIPTION_LINK_WITH_TOKEN,
  SURGE_PREVIEW,
  configSnapshot,
  fixtureAuthAuthed,
  fixtureConfig,
  fixtureGroups,
  fixtureNodes,
  fixtureStatus,
  validateOk
} from "./fixtures";
import type {
  AuthStatus,
  Config,
  ConfigSnapshot,
  GroupPreviewResponse,
  NodePreviewResponse,
  StatusResponse,
  ValidateResult
} from "../../../src/api/types";

export interface MockState {
  authStatus: AuthStatus;
  config: ConfigSnapshot;
  status: StatusResponse;
  validate: ValidateResult;
  nodesRuntime: NodePreviewResponse;
  groupsRuntime: GroupPreviewResponse;
  // Per-format generate preview text (runtime + draft branches share state).
  generatePreview: { clash: string; surge: string };
  // Each call to /api/auth/login decrements failures or returns invalid_credentials based on tracker.
  loginFailureMode?: "invalid" | "locked" | null;
  loginRemaining?: number;
  // Track network calls for assertions.
  callLog: { method: string; url: string; body?: unknown }[];
  // Server-side override for /api/reload behaviour.
  reloadMode?: "ok" | "in_progress_then_ok" | "fail";
  reloadAttempts?: number;
  // Save behaviour.
  saveMode?: "ok" | "revision_conflict" | "readonly" | "validation_failed";
  // Generate link behaviour.
  generateLink?: { url: string; token_included: boolean };
}

export function createMockState(overrides: Partial<MockState> = {}): MockState {
  return {
    authStatus: { ...fixtureAuthAuthed },
    config: configSnapshot(fixtureConfig, REVISION_BASE),
    status: { ...fixtureStatus },
    validate: { ...validateOk },
    nodesRuntime: { ...fixtureNodes },
    groupsRuntime: { ...fixtureGroups },
    generatePreview: { clash: CLASH_PREVIEW, surge: SURGE_PREVIEW },
    callLog: [],
    reloadMode: "ok",
    reloadAttempts: 0,
    saveMode: "ok",
    generateLink: { url: SUBSCRIPTION_LINK_WITH_TOKEN, token_included: true },
    ...overrides
  };
}

function jsonResponse(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: { "Cache-Control": "no-store" },
    body: JSON.stringify(body)
  });
}

function textResponse(route: Route, body: string, status = 200, contentType = "text/plain; charset=utf-8") {
  return route.fulfill({ status, contentType, headers: { "Cache-Control": "no-store" }, body });
}

async function logCall(route: Route, state: MockState) {
  const request = route.request();
  let body: unknown;
  try {
    body = request.postDataJSON();
  } catch {
    body = request.postData();
  }
  state.callLog.push({ method: request.method(), url: request.url(), body });
}

export async function installMocks(page: Page, state: MockState): Promise<void> {
  await page.route("**/healthz", async (route) => {
    await logCall(route, state);
    await textResponse(route, "ok");
  });

  await page.route("**/api/auth/status", async (route) => {
    await logCall(route, state);
    await jsonResponse(route, state.authStatus);
  });

  await page.route("**/api/auth/login", async (route) => {
    await logCall(route, state);
    if (state.loginFailureMode === "locked") {
      await jsonResponse(
        route,
        { error: { code: "auth_locked", message: "登录失败次数过多", until: "2099-01-01T00:00:00Z" } },
        423
      );
      return;
    }
    if (state.loginFailureMode === "invalid") {
      const remaining = state.loginRemaining ?? 4;
      await jsonResponse(
        route,
        { error: { code: "invalid_credentials", message: "用户名或密码错误", remaining } },
        401
      );
      return;
    }
    state.authStatus = { ...fixtureAuthAuthed };
    await jsonResponse(route, { redirect: "/sources" });
  });

  await page.route("**/api/auth/setup", async (route) => {
    await logCall(route, state);
    const body = route.request().postDataJSON() as { setup_token?: string } | null;
    if (state.authStatus.setup_token_required && !body?.setup_token) {
      await jsonResponse(route, { error: { code: "setup_token_required", message: "缺少 setup token" } }, 401);
      return;
    }
    if (body?.setup_token === "INVALID-TOKEN") {
      await jsonResponse(route, { error: { code: "setup_token_invalid", message: "setup token 不匹配" } }, 401);
      return;
    }
    state.authStatus = { ...fixtureAuthAuthed };
    await jsonResponse(route, { redirect: "/sources" });
  });

  await page.route("**/api/auth/logout", async (route) => {
    await logCall(route, state);
    state.authStatus = { authed: false, setup_required: false, setup_token_required: false, locked_until: "" };
    await jsonResponse(route, { success: true });
  });

  await page.route("**/api/status", async (route) => {
    await logCall(route, state);
    await jsonResponse(route, state.status);
  });

  await page.route("**/api/config/validate", async (route) => {
    await logCall(route, state);
    await jsonResponse(route, state.validate);
  });

  await page.route("**/api/config", async (route) => {
    await logCall(route, state);
    const method = route.request().method();
    if (method === "GET") {
      await jsonResponse(route, state.config);
      return;
    }
    if (method === "PUT") {
      const body = route.request().postDataJSON() as { config_revision?: string; config?: Config } | null;
      if (state.saveMode === "validation_failed") {
        await jsonResponse(
          route,
          {
            valid: false,
            errors: [
              {
                severity: "error",
                code: "config.regex.invalid",
                message: "正则错误",
                display_path: "groups.GRP_HK.match",
                locator: { section: "groups", index: 0, json_pointer: "/config/groups/0/value/match" }
              }
            ],
            warnings: [],
            infos: []
          },
          400
        );
        return;
      }
      if (state.saveMode === "readonly") {
        await jsonResponse(
          route,
          { error: { code: "config_source_readonly", message: "远程配置只读" } },
          409
        );
        return;
      }
      if (state.saveMode === "revision_conflict" || (body?.config_revision && body.config_revision !== state.config.config_revision)) {
        await jsonResponse(
          route,
          {
            error: {
              code: "config_revision_conflict",
              message: "配置已被外部修改",
              current_config_revision: state.config.config_revision
            }
          },
          409
        );
        return;
      }
      // Successful save: rotate revision.
      const nextRevision = `sha256:${Math.random().toString(16).slice(2).padEnd(64, "0").slice(0, 64)}`;
      state.config = { config_revision: nextRevision, config: body?.config ?? state.config.config };
      state.status = {
        ...state.status,
        config_revision: nextRevision,
        runtime_config_revision: state.status.runtime_config_revision,
        config_dirty: nextRevision !== state.status.runtime_config_revision
      };
      await jsonResponse(route, { config_revision: nextRevision });
      return;
    }
    await route.continue();
  });

  await page.route("**/api/reload", async (route) => {
    await logCall(route, state);
    state.reloadAttempts = (state.reloadAttempts ?? 0) + 1;
    if (state.reloadMode === "fail") {
      await jsonResponse(
        route,
        { error: { code: "config.invalid", message: "Prepare 校验失败" } },
        400
      );
      return;
    }
    if (state.reloadMode === "in_progress_then_ok" && state.reloadAttempts === 1) {
      await jsonResponse(route, { error: { code: "reload_in_progress", message: "Reload 正在执行" } }, 429);
      return;
    }
    state.status = {
      ...state.status,
      runtime_config_revision: state.config.config_revision,
      config_revision: state.config.config_revision,
      config_dirty: false,
      last_reload: { time: new Date().toISOString(), success: true, duration_ms: 142 }
    };
    await jsonResponse(route, { success: true, duration_ms: 142 });
  });

  await page.route("**/api/preview/nodes", async (route) => {
    await logCall(route, state);
    await jsonResponse(route, state.nodesRuntime);
  });

  await page.route("**/api/preview/groups", async (route) => {
    await logCall(route, state);
    await jsonResponse(route, state.groupsRuntime);
  });

  await page.route("**/api/generate/preview**", async (route) => {
    await logCall(route, state);
    const url = new URL(route.request().url());
    const format = url.searchParams.get("format") ?? "clash";
    await textResponse(route, format === "surge" ? state.generatePreview.surge : state.generatePreview.clash);
  });

  await page.route("**/api/generate/link**", async (route) => {
    await logCall(route, state);
    const url = new URL(route.request().url());
    const includeToken = url.searchParams.get("include_token");
    if (includeToken === "false" || !state.generateLink?.token_included) {
      await jsonResponse(route, { url: SUBSCRIPTION_LINK_NO_TOKEN, token_included: false });
      return;
    }
    await jsonResponse(route, state.generateLink ?? { url: SUBSCRIPTION_LINK_WITH_TOKEN, token_included: true });
  });

  await page.route("**/generate**", async (route) => {
    const url = route.request().url();
    if (url.includes("/api/generate")) {
      await route.fallback();
      return;
    }
    await logCall(route, state);
    const u = new URL(url);
    const format = u.searchParams.get("format") ?? "clash";
    const filename = u.searchParams.get("filename") ?? `${format === "surge" ? "surge.conf" : "clash.yaml"}`;
    await route.fulfill({
      status: 200,
      headers: {
        "Content-Type": format === "surge" ? "text/plain; charset=utf-8" : "text/yaml; charset=utf-8",
        "Content-Disposition": `attachment; filename="${filename}"`,
        "Cache-Control": "no-store"
      },
      body: format === "surge" ? state.generatePreview.surge : state.generatePreview.clash
    });
  });
}

/**
 * Helper that completes the first-time-save localStorage flag so save flow
 * tests can either trigger or skip the confirmation modal.
 */
export async function markFirstSaveConfirmed(page: Page): Promise<void> {
  await page.addInitScript(() => {
    window.localStorage.setItem("subconverter.firstSaveConfirmed", "true");
  });
}

export async function clearFirstSaveFlag(page: Page): Promise<void> {
  await page.addInitScript(() => {
    window.localStorage.removeItem("subconverter.firstSaveConfirmed");
  });
}
