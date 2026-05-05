// T-INT-SRC — A1 订阅来源 page interactions.

import { expect, type Locator, type Page, test } from "@playwright/test";
import { configSnapshot } from "./helpers/fixtures";
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

  test("T-INT-SRC-006 reorders all source pools with drag handles", async ({ page }) => {
    const baseState = createMockState();
    const state = createMockState({
      config: configSnapshot({
        ...baseState.config.config,
        sources: {
          ...baseState.config.config.sources,
          subscriptions: [
            { url: "https://provider-a.example.com/sub?token=alpha" },
            { url: "https://provider-b.example.com/sub?token=beta" }
          ],
          snell: [
            { url: "https://snell-a.example.com/list.txt" },
            { url: "https://snell-b.example.com/list.txt" }
          ],
          vless: [
            { url: "https://vless-a.example.com/sub" },
            { url: "https://vless-b.example.com/sub" }
          ],
          custom_proxies: [
            { name: "Proxy A", url: "ss://alpha@example.com:8388" },
            { name: "Proxy B", url: "socks5://beta@example.com:1080" }
          ]
        }
      })
    });
    await installMocks(page, state);

    await page.goto("/sources");
    await dragFirstCardAfterSecond(page, sourceSection(page, /SS 订阅/));
    await dragFirstCardAfterSecond(page, sourceSection(page, /Snell 节点池/));
    await dragFirstCardAfterSecond(page, sourceSection(page, /VLESS 节点池/));
    await dragFirstCardAfterSecond(page, sourceSection(page, /自定义代理/));

    await expect(sourceSection(page, /SS 订阅/).locator(".source-card").first()).toContainText("provider-b.example.com");
    await expect(sourceSection(page, /Snell 节点池/).locator(".source-card").first()).toContainText("snell-b.example.com");
    await expect(sourceSection(page, /VLESS 节点池/).locator(".source-card").first()).toContainText("vless-b.example.com");
    await expect(sourceSection(page, /自定义代理/).locator(".source-card").first()).toHaveAttribute("title", "Proxy B");
  });
});

function sourceSection(page: Page, heading: RegExp) {
  return page.locator(".source-section", { has: page.getByRole("heading", { name: heading }) });
}

async function dragFirstCardAfterSecond(page: Page, section: Locator) {
  const cards = section.locator(".source-card");
  await cards.nth(1).scrollIntoViewIfNeeded();
  const firstHandle = cards.first().getByRole("button", { name: "拖拽排序" });
  const handleBox = await firstHandle.boundingBox();
  const secondBox = await cards.nth(1).boundingBox();
  expect(handleBox).not.toBeNull();
  expect(secondBox).not.toBeNull();
  if (!handleBox || !secondBox) return;

  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(secondBox.x + secondBox.width / 2, secondBox.y + secondBox.height - 2, { steps: 12 });
  await page.mouse.up();
  await page.waitForTimeout(120);
}
