// T-INT-RTM — B1 节点预览 / B2 分组预览 / B3 生成下载.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("runtime preview", () => {
  test("T-INT-RTM-001 nodes page renders runtime nodes table", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/nodes");
    await expect(page.getByRole("heading", { name: "节点预览" })).toBeVisible();
    await expect(page.locator("tbody tr")).toHaveCount(9);
    await expect(page.getByText("🇭🇰 香港 IEPL 01")).toBeVisible();
    await expect(page.getByText("Home Relay")).toBeVisible();
    // Format-specific badges
    await expect(page.locator(".chip", { hasText: "Surge" }).first()).toBeVisible();
    await expect(page.locator(".chip", { hasText: "Clash" }).first()).toBeVisible();
  });

  test("T-INT-RTM-002 nodes page filters by kind via category pill", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/nodes");
    await page.locator(".category-pill", { hasText: "Snell" }).click();
    await expect(page.locator("tbody tr")).toHaveCount(1);
    await expect(page.getByText("🇭🇰 Snell HK Premium")).toBeVisible();

    await page.locator(".category-pill", { hasText: "VLESS" }).click();
    await expect(page.locator("tbody tr")).toHaveCount(1);
    await expect(page.getByText("🇩🇪 法兰克福 Reality")).toBeVisible();
  });

  test("T-INT-RTM-003 nodes name search reduces table", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/nodes");
    await page.getByPlaceholder("搜索节点名").fill("香港");
    await expect(page.locator("tbody tr")).toHaveCount(2);
  });

  test("T-INT-RTM-004 group preview page renders node/service expansions", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/preview/groups");
    await expect(page.getByRole("heading", { name: "分组预览" })).toBeVisible();

    await expect(page.locator(".group-preview-card", { hasText: "GRP_HK" })).toBeVisible();
    await expect(page.locator(".group-preview-card", { hasText: "GRP_JP" })).toBeVisible();
    await expect(page.locator(".service-group-row", { hasText: "SVC_PROXY" })).toBeVisible();
  });

  test("T-INT-RTM-005 download page renders dual-format previews from runtime", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/download");

    // Two runtime panels render side-by-side automatically: panel 0 is Clash Meta, panel 1 is Surge.
    const clashPanel = page.locator(".code-preview-panel").nth(0);
    const surgePanel = page.locator(".code-preview-panel").nth(1);
    await expect(clashPanel.getByText(/proxies:/).first()).toBeVisible();
    await expect(clashPanel.getByText(/mixed-port/)).toBeVisible();
    await expect(surgePanel.getByText(/\[Proxy\]/)).toBeVisible();
    await expect(surgePanel.getByText(/loglevel = notify/)).toBeVisible();
  });

  test("T-INT-RTM-006 copy subscription link with token shows confirmation modal", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/download");
    // Trigger the link copy on the Clash panel.
    const clashPanel = page.locator(".code-preview-panel", { hasText: "Clash Meta" });
    await clashPanel.getByRole("button", { name: "复制" }).click();

    const dialog = page.getByRole("dialog", { name: "复制含 token 的订阅链接？" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "确认复制" }).click();

    await expect(page.getByText("订阅链接已复制")).toBeVisible();
    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboard).toContain("token=server-token-123");
  });
});
