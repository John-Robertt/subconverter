// T-INT-RST — A5 规则集 page interactions.

import { expect, test } from "@playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("rulesets page", () => {
  test("T-INT-RST-001 fixture ruleset is rendered with bound URLs", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rulesets");
    await expect(page.getByText("共 1 个服务组绑定")).toBeVisible();
    await expect(
      page.locator(".ruleset-url-input[value='https://ruleset.example.com/proxy.list']")
    ).toBeVisible();
  });

  test("T-INT-RST-002 add new ruleset URL and edit value inline", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rulesets");
    await page.getByRole("button", { name: /添加规则集 URL/ }).click();

    const input = page.locator(".ruleset-url-input").last();
    await input.fill("https://ruleset.example.com/streaming.list");
    await expect(input).toHaveValue("https://ruleset.example.com/streaming.list");
  });

  test("T-INT-RST-003 deleting ruleset URL prompts confirmation", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/rulesets");
    // Find the row that wraps the input matching proxy.list URL.
    const row = page
      .locator(".ruleset-url-row")
      .filter({ has: page.locator(".ruleset-url-input[value*='proxy.list']") });
    await row.getByRole("button", { name: "删除 URL" }).click();

    const dialog = page.getByRole("dialog", { name: "删除规则集 URL？" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "确认删除" }).click();

    await expect(page.locator(".ruleset-url-input[value*='proxy.list']")).toHaveCount(0);
  });
});
