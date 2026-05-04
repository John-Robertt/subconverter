// T-INT-STA — C 系统状态 page interactions.

import { expect, test } from "@playwright/test";
import { createMockState, installMocks } from "./helpers/mock";
import { dirtyStatus } from "./helpers/fixtures";

test.describe("status page", () => {
  test("T-INT-STA-001 renders runtime stats cards", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/status");
    await expect(page.getByRole("heading", { name: "系统状态" })).toBeVisible();

    const stats = page.locator(".stat-card");
    await expect(stats.nth(0)).toContainText("运行中");
    await expect(stats.nth(1)).toContainText("v2.0.0-test");
    await expect(stats.nth(2)).toContainText("已加载");
    await expect(stats.nth(3)).toContainText(/\d+ 秒前|\d+ 分钟前|\d+ 小时前/);
  });

  test("T-INT-STA-002 dirty status surfaces warning indicator", async ({ page }) => {
    const state = createMockState({ status: { ...dirtyStatus } });
    await installMocks(page, state);

    await page.goto("/status");
    const stats = page.locator(".stat-card");
    await expect(stats.nth(2)).toContainText("待重载");
  });

  test("T-INT-STA-003 manual reload triggers /api/reload and shows success toast", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/status");
    await page.getByRole("button", { name: "热重载" }).click();
    await expect(page.getByText("RuntimeConfig 已重新加载")).toBeVisible();

    expect(state.callLog.some((entry) => entry.url.endsWith("/api/reload"))).toBeTruthy();
  });
});
