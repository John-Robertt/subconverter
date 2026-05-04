// T-INT-FLT — A2 过滤器 page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("filters page", () => {
  test("T-INT-FLT-001 renders existing exclude regex from fixture", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/filters");
    await expect(page.getByRole("heading", { name: "排除规则" })).toBeVisible();
    await expect(page.locator("input.mono-input")).toHaveValue("剩余|流量|官网|Expire");
  });

  test("T-INT-FLT-002 invalid regex shows inline error", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/filters");
    const input = page.locator("input.mono-input");
    await input.fill("[unterminated");
    await expect(page.locator(".field-error")).toBeVisible();
  });

  test("T-INT-FLT-003 template chip appends pattern to exclude", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/filters");
    const input = page.locator("input.mono-input");
    await input.fill("");

    await page.getByRole("button", { name: /流量信息/ }).click();
    await expect(input).toHaveValue("剩余|流量|套餐|到期|Expire");

    await page.getByRole("button", { name: /IPv6/ }).click();
    await expect(input).toHaveValue("剩余|流量|套餐|到期|Expire|IPv6|v6");
  });

  test("T-INT-FLT-004 draft preview lists active and filtered nodes", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/filters");
    await page.getByRole("button", { name: "草稿节点预览" }).click();

    const rail = page.locator(".rail-panel");
    await expect(rail.getByText("🇭🇰 香港 IEPL 01")).toBeVisible();
    await expect(rail.getByText(/测试·流量信息/)).toBeVisible();
    await expect(rail.locator(".chip", { hasText: "剔除" }).first()).toBeVisible();
    await expect(rail.locator(".chip", { hasText: "保留" }).first()).toBeVisible();

    // big-stat numbers reflect 9 total / 1 filtered / 8 active
    await expect(page.locator(".big-stat-neutral strong")).toContainText("9");
    await expect(page.locator(".big-stat-error strong")).toContainText("1");
    await expect(page.locator(".big-stat-success strong")).toContainText("8");
  });
});
