// T-INT-SRC — A1 订阅来源 page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("sources page", () => {
  test("T-INT-SRC-001 stats grid reflects fixture counts", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    await expect(page.getByRole("heading", { name: "订阅来源" })).toBeVisible();

    // total = 2 subs + 1 snell + 1 vless + 1 custom = 5
    const stats = page.locator(".stat-card");
    await expect(stats.nth(0)).toContainText("5");
    await expect(stats.nth(1)).toContainText("4"); // fetch sources only
    await expect(stats.nth(2)).toContainText("1"); // snell
    await expect(stats.nth(3)).toContainText("1"); // vless

    // Section headings render once per kind.
    await expect(page.getByRole("heading", { name: /Snell 节点池/ })).toBeVisible();
    await expect(page.getByRole("heading", { name: /VLESS 节点池/ })).toBeVisible();
    await expect(page.getByRole("heading", { name: /自定义代理/ })).toBeVisible();
  });

  test("T-INT-SRC-002 add SS subscription appends a card and updates stats", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    await page.getByRole("button", { name: "添加 SS 订阅" }).click();

    const modal = page.getByRole("dialog");
    await expect(modal.getByRole("heading", { name: "添加 SS 订阅" })).toBeVisible();
    await modal.locator(".text-input").fill("https://provider-c.example.com/sub?token=new-token");
    await modal.getByRole("button", { name: "保存来源" }).click();

    await expect(page.getByText(/provider-c\.example\.com/)).toBeVisible();
    // SS 订阅 section heading carries chip with the count.
    await expect(page.getByRole("heading", { name: /SS 订阅 3/ })).toBeVisible();
  });

  test("T-INT-SRC-003 deleting a custom proxy requires confirmation", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    // Custom proxy card only exposes the proxy name via the article's title attribute.
    const customCard = page.locator('.source-card[title="Home Relay"]');
    await expect(customCard).toBeVisible();

    await customCard.getByRole("button", { name: "删除来源" }).click();

    const dialog = page.getByRole("dialog", { name: "删除自定义代理？" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "确认删除" }).click();

    await expect(page.locator('.source-card[title="Home Relay"]')).toHaveCount(0);
  });

  test("T-INT-SRC-004 cancel delete keeps the subscription", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    const subscriptionCard = page.locator(".source-card", { hasText: /provider-b\.net/ });
    await expect(subscriptionCard).toBeVisible();

    await subscriptionCard.getByRole("button", { name: "删除来源" }).click();
    const dialog = page.getByRole("dialog", { name: "删除来源？" });
    await dialog.getByRole("button", { name: "取消" }).click();

    await expect(page.locator(".source-card", { hasText: /provider-b\.net/ })).toBeVisible();
  });

  test("T-INT-SRC-005 enable relay_through reveals link-relay editor", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/sources");
    await page.getByRole("button", { name: "添加 自定义代理" }).click();

    const modal = page.getByRole("dialog", { name: "添加自定义代理" });
    await modal.getByLabel("名称").fill("Office Relay");
    await modal.getByLabel("URL").fill("ss://chacha20:secret@office.example.com:8388");

    await modal.getByText("启用 relay_through").click();
    await expect(modal.getByText("链式中转")).toBeVisible();
    await expect(modal.getByLabel("类型")).toBeVisible();

    await modal.getByLabel("类型").selectOption("select");
    await expect(modal.getByLabel("匹配正则")).toBeVisible();

    await modal.getByRole("button", { name: "保存来源" }).click();
    await expect(page.locator('.source-card[title="Office Relay"]')).toBeVisible();
  });
});
