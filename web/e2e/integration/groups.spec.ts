// T-INT-GRP — A3 节点分组 page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("groups page", () => {
  test("T-INT-GRP-001 renders fixture groups as pills", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/groups");
    await expect(page.getByText("共 3 个节点组 · 拖拽调整顺序")).toBeVisible();
    await expect(page.locator(".group-pill", { hasText: "GRP_HK" })).toBeVisible();
    await expect(page.locator(".group-pill", { hasText: "GRP_JP" })).toBeVisible();
    await expect(page.locator(".group-pill", { hasText: "GRP_SG" })).toBeVisible();
  });

  test("T-INT-GRP-002 add a new group and switch strategy", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/groups");
    // The chip-large variant is the only "新建分组" when groups already exist.
    await page.locator(".add-chip-large").click();

    await expect(page.locator(".group-pill")).toHaveCount(4);
    await page.getByLabel("分组名称").fill("GRP_TW");
    await page.getByLabel("匹配正则").fill("(台湾|TW)");

    // Strategy cards are buttons but their accessible name picks up the
    // surrounding <Field label="路由策略">; target by class instead.
    await page.locator(".strategy-card", { hasText: "url-test" }).click();
    await expect(page.locator(".strategy-card.active", { hasText: "url-test" })).toBeVisible();
  });

  test("T-INT-GRP-003 invalid regex surfaces field error", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/groups");
    await page.locator(".group-pill", { hasText: "GRP_HK" }).click();
    await page.getByLabel("匹配正则").fill("[bad-regex");

    await expect(page.locator(".field-error")).toBeVisible();
  });

  test("T-INT-GRP-004 draft preview rail renders node/service group counts", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/groups");
    await page.getByRole("button", { name: "草稿分组预览" }).click();

    const rail = page.locator(".rail-panel");
    await expect(rail.getByRole("heading", { name: "节点组" })).toBeVisible();
    await expect(rail.getByRole("heading", { name: "服务组" })).toBeVisible();
    await expect(rail.getByText(/GRP_HK/)).toBeVisible();
    await expect(rail.getByText(/SVC_PROXY/)).toBeVisible();
  });
});
