// T-INT-EDGE — Cross-cutting edge cases that aren't tied to a single page:
//   * 401 session_expired during a protected request triggers global redirect
//   * /api/reload 429 in_progress causes a single retry that succeeds
//   * Login page shows a connection-error banner when /api/auth/status fails
//   * Drag-and-drop reordering (here: rules deletion preserves remaining order)

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("edge cases", () => {
  test("T-INT-EDGE-001 401 session_expired on /api/status redirects to /login", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    // Override /api/status to fail with session_expired AFTER also flipping
    // authStatus to logged-out — this mirrors the real flow where the cookie
    // became invalid, so /login won't bounce the user back via Navigate.
    state.authStatus = { authed: false, setup_required: false, setup_token_required: false, locked_until: "" };
    await page.unroute("**/api/status");
    await page.route("**/api/status", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "session_expired", message: "登录已过期" } })
      })
    );

    await page.goto("/sources");
    await expect(page).toHaveURL(/\/login(\?next=)?/);
    await expect(page.getByRole("heading", { name: "登录管理后台" })).toBeVisible();
  });

  test("T-INT-EDGE-002 reload retries once after 429 in_progress", async ({ page }) => {
    const state = createMockState({ reloadMode: "in_progress_then_ok" });
    await installMocks(page, state);

    await page.goto("/status");
    await page.getByRole("button", { name: "热重载" }).click();

    await expect(page.getByText(/Reload 正在执行/)).toBeVisible();
    await expect(page.getByText("RuntimeConfig 已重新加载")).toBeVisible();

    const reloadCalls = state.callLog.filter((entry) => entry.url.endsWith("/api/reload"));
    expect(reloadCalls.length).toBeGreaterThanOrEqual(2);
  });

  test("T-INT-EDGE-003 backend error on /api/auth/status surfaces network toast with retry action", async ({ page }) => {
    // Make /api/auth/status fail with a 503 — the api client normalizes it
    // into an ApiError, react-query sets isError=true, and the LoginPage
    // shows a persistent error toast in the bottom-right toast region.
    await page.route("**/api/auth/status", (route) =>
      route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "unavailable", message: "后端暂不可用" } })
      })
    );

    await page.goto("/login");
    const toast = page.locator(".toast-region .toast");
    await expect(toast).toContainText("后端不可达");
    await expect(toast.locator("button.toast-action", { hasText: "重试" })).toBeVisible();
  });

  test("T-INT-EDGE-004 deleting a middle rule preserves order of remaining rules", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rules");
    await expect(page.locator(".rules-list-row")).toHaveCount(4);

    // Delete the second rule (DOMAIN-KEYWORD,github,SVC_PROXY).
    const secondRow = page.locator(".rules-list-row").nth(1);
    await secondRow.getByRole("button", { name: "删除规则" }).click();
    await page.getByRole("dialog", { name: "删除内联规则？" }).getByRole("button", { name: "确认删除" }).click();

    await expect(page.locator(".rules-list-row")).toHaveCount(3);
    // Remaining rules keep their relative order: DOMAIN-SUFFIX,cn → GEOIP,CN → MATCH.
    await expect(page.locator(".rules-list-row").nth(0)).toContainText("cn");
    await expect(page.locator(".rules-list-row").nth(1)).toContainText("GEOIP");
    await expect(page.locator(".rules-list-row").nth(2)).toContainText("MATCH");
  });

  test("T-INT-EDGE-005 readonly status disables edit buttons across pages", async ({ page }) => {
    const state = createMockState();
    state.status = {
      ...state.status,
      config_source: { ...state.status.config_source, type: "remote", writable: false },
      capabilities: { ...state.status.capabilities, config_write: false }
    };
    await installMocks(page, state);

    await page.goto("/sources");
    // Save button in the topbar should be disabled in readonly mode.
    await expect(page.getByRole("button", { name: "保存" })).toBeDisabled();
    // The dashed "添加 SS 订阅" button should also be disabled.
    await expect(page.getByRole("button", { name: "添加 SS 订阅" })).toBeDisabled();
  });
});
