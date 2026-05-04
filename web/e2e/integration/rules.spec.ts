// T-INT-RUL — A6 内联规则 page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("rules page", () => {
  test("T-INT-RUL-001 renders fixture rules and parses badges", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rules");
    await expect(page.getByText("共 4 条 · 拖拽调整顺序")).toBeVisible();
    await expect(page.locator(".rules-list-row")).toHaveCount(4);
    await expect(page.locator(".rules-list-row", { hasText: "DOMAIN-SUFFIX" }).first()).toBeVisible();
    await expect(page.locator(".rules-list-row", { hasText: "MATCH" })).toBeVisible();
  });

  test("T-INT-RUL-002 search filters rules to matching subset", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rules");
    await page.getByPlaceholder("搜索规则…").fill("github");

    await expect(page.locator(".rules-list-row", { hasText: "github" })).toBeVisible();
    await expect(page.locator(".rules-list-row")).toHaveCount(1);
  });

  test("T-INT-RUL-003 add new rule using fallback policy", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rules");
    await page.getByRole("button", { name: /添加规则/ }).click();

    await expect(page.locator(".rules-list-row")).toHaveCount(5);
    const lastRow = page.locator(".rules-list-row").last();
    await expect(lastRow).toContainText("example.com");
    await expect(lastRow).toContainText("SVC_PROXY");
  });

  test("T-INT-RUL-004 swap policy via select replaces last segment", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rules");
    // Click on the github rule to select it.
    await page.locator(".rules-list-row", { hasText: "github" }).click();
    await expect(page.getByRole("heading", { name: /编辑规则/ })).toBeVisible();

    await page.getByLabel("Policy 选择器").selectOption("SVC_DIRECT");
    await expect(page.locator(".rules-list-row", { hasText: "github" })).toContainText("SVC_DIRECT");
  });
});
