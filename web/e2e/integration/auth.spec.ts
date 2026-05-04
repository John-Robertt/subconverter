// T-INT-AUTH suite — login, setup, lock, logout flows.
//
// All tests run against the Vite dev server with API requests intercepted via
// Playwright route mocking; no Go backend is needed.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";
import { fixtureAuthLocked, fixtureAuthLogout, fixtureAuthSetup } from "./helpers/fixtures";

test.describe("auth", () => {
  test("T-INT-AUTH-001 first-time setup creates admin and lands on sources", async ({ page }) => {
    const state = createMockState({ authStatus: { ...fixtureAuthSetup } });
    await installMocks(page, state);

    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "首次创建管理员" })).toBeVisible();

    await page.locator('input[autocomplete="one-time-code"]').fill("setup-secret");
    await page.locator('input[autocomplete="username"]').fill("admin");
    const passwords = page.locator('input[autocomplete="new-password"]');
    await passwords.nth(0).fill("admin-password-1234");
    await passwords.nth(1).fill("admin-password-1234");

    await page.getByRole("button", { name: "创建管理员并登录" }).click();
    await expect(page).toHaveURL(/\/sources$/);
    await expect(page.getByRole("heading", { name: "订阅来源" })).toBeVisible();
  });

  test("T-INT-AUTH-002 setup token mismatch shows error and stays on login", async ({ page }) => {
    const state = createMockState({ authStatus: { ...fixtureAuthSetup } });
    await installMocks(page, state);

    await page.goto("/login");
    await page.locator('input[autocomplete="one-time-code"]').fill("INVALID-TOKEN");
    await page.locator('input[autocomplete="username"]').fill("admin");
    const passwords = page.locator('input[autocomplete="new-password"]');
    await passwords.nth(0).fill("admin-password-1234");
    await passwords.nth(1).fill("admin-password-1234");

    await page.getByRole("button", { name: "创建管理员并登录" }).click();
    const toast = page.locator(".toast-region .toast");
    await expect(toast).toContainText(/Setup 失败/);
    await expect(toast).toContainText(/setup token 不匹配/);
    await expect(page).toHaveURL(/\/login/);
  });

  test("T-INT-AUTH-003 password mismatch is rejected before submit", async ({ page }) => {
    const state = createMockState({ authStatus: { ...fixtureAuthSetup } });
    await installMocks(page, state);

    await page.goto("/login");
    await page.locator('input[autocomplete="one-time-code"]').fill("setup-secret");
    await page.locator('input[autocomplete="username"]').fill("admin");
    const passwords = page.locator('input[autocomplete="new-password"]');
    await passwords.nth(0).fill("admin-password-1234");
    await passwords.nth(1).fill("DIFFERENT-PASSWORD");

    await expect(page.getByText("两次密码不一致")).toBeVisible();
    await expect(page.getByRole("button", { name: "创建管理员并登录" })).toBeDisabled();
  });

  test("T-INT-AUTH-004 invalid credentials shows remaining attempts", async ({ page }) => {
    const state = createMockState({
      authStatus: { ...fixtureAuthLogout },
      loginFailureMode: "invalid",
      loginRemaining: 3
    });
    await installMocks(page, state);

    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "登录管理后台" })).toBeVisible();
    await page.locator('input[autocomplete="current-password"]').fill("wrong-password");
    await page.getByRole("button", { name: "登录" }).click();

    const toast = page.locator(".toast-region .toast");
    await expect(toast).toContainText("用户名或密码错误");
    await expect(toast).toContainText(/还可尝试 3 次/);
  });

  test("T-INT-AUTH-005 account lockout disables submit", async ({ page }) => {
    const state = createMockState({ authStatus: { ...fixtureAuthLocked } });
    await installMocks(page, state);

    await page.goto("/login");
    // The lock notice now lives in a toast; submit button is disabled.
    await expect(page.getByRole("button", { name: "登录" })).toBeDisabled();
  });

  test("T-INT-AUTH-006 logout invalidates session and redirects to login", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    await expect(page.getByRole("heading", { name: "订阅来源" })).toBeVisible();

    await page.getByRole("button", { name: "注销" }).click();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole("heading", { name: "登录管理后台" })).toBeVisible();
    expect(state.callLog.some((entry) => entry.url.includes("/api/auth/logout"))).toBeTruthy();
  });
});
