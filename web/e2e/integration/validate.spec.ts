// T-INT-VAL — A8 静态校验 page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";
import { validateOk, validateWithErrors } from "./helpers/fixtures";

test.describe("validate page", () => {
  test("T-INT-VAL-001 successful validation shows success state", async ({ page }) => {
    const state = createMockState({ validate: { ...validateOk } });
    await installMocks(page, state);

    await page.goto("/validate");
    await page.getByRole("button", { name: "运行静态校验" }).click();
    await expect(page.getByRole("main").getByText("静态校验通过")).toBeVisible();
  });

  test("T-INT-VAL-002 failing validation surfaces errors / warnings / infos", async ({ page }) => {
    const state = createMockState({ validate: { ...validateWithErrors } });
    await installMocks(page, state);

    await page.goto("/validate");
    await page.getByRole("button", { name: "运行静态校验" }).click();

    await expect(page.locator(".summary-stat-error strong")).toContainText("1");
    await expect(page.locator(".summary-stat-warning strong")).toContainText("1");
    await expect(page.locator(".summary-stat-info strong")).toContainText("1");

    await expect(page.getByText("正则表达式语法错误")).toBeVisible();
    await expect(page.getByText("分组未匹配到任何节点")).toBeVisible();
    await expect(page.getByText("fallback 未设置")).toBeVisible();
  });

  test("T-INT-VAL-003 jump from diagnostic navigates to groups page with focused field", async ({ page }) => {
    const state = createMockState({ validate: { ...validateWithErrors } });
    await installMocks(page, state);

    await page.goto("/validate");
    await page.getByRole("button", { name: "运行静态校验" }).click();

    // Open the regex error row, then click 跳转字段 in the drawer.
    await page.getByText("正则表达式语法错误").click();
    await expect(page.getByRole("dialog", { name: /正则表达式语法错误/ })).toBeVisible();
    await page.getByRole("button", { name: "跳转字段" }).click();

    await expect(page).toHaveURL(/\/groups/);
    await expect(page.getByRole("heading", { name: "节点分组" })).toBeVisible();
  });
});
