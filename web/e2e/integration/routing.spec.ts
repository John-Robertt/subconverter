// T-INT-RTE — A4 路由策略 page interactions.
//
// New layout: left side shows read-only service-group cards (members rendered
// as non-removable chips); right rail hosts the edit panel for the currently
// selected card (name input + member palette + member chip with remove).

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("routing page", () => {
  test("T-INT-RTE-001 renders fixture service groups with members on the left", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/routing");
    await expect(page.getByText("共 2 个服务组")).toBeVisible();

    // First card is auto-selected and shows the "已选中" badge; both cards
    // render the key as plain strong text (no inline input on the left).
    const firstCard = page.locator(".routing-card").nth(0);
    const secondCard = page.locator(".routing-card").nth(1);
    await expect(firstCard.getByText("SVC_PROXY", { exact: true })).toBeVisible();
    await expect(secondCard.getByText("SVC_DIRECT", { exact: true })).toBeVisible();
    await expect(firstCard.locator(".chip", { hasText: "已选中" })).toBeVisible();

    // Member chips render in read-only mode (no remove button on left).
    await expect(firstCard.getByText("@auto", { exact: true })).toBeVisible();
    await expect(firstCard.locator(".chip", { hasText: /^@all$/ }).getByRole("button")).toHaveCount(0);

    // Right rail shows the editor for SVC_PROXY.
    const rail = page.locator(".rail-panel");
    await expect(rail.getByRole("heading", { name: "编辑服务组" })).toBeVisible();
    await expect(rail.getByLabel("服务组名称")).toHaveValue("SVC_PROXY");
  });

  test("T-INT-RTE-002 click second card swaps the right-rail editor target", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/routing");
    const secondCard = page.locator(".routing-card").nth(1);
    await secondCard.click();

    const rail = page.locator(".rail-panel");
    await expect(rail.getByLabel("服务组名称")).toHaveValue("SVC_DIRECT");

    // Add @auto and a node group from the rail palette into SVC_DIRECT.
    await rail.getByRole("button", { name: /@auto 自动选择子组/ }).click();
    await rail.getByRole("button", { name: /^GRP_HK/ }).click();

    // Left card now reflects the added members.
    await expect(secondCard.getByText("@auto", { exact: true })).toBeVisible();
    await expect(secondCard.getByText("GRP_HK", { exact: true })).toBeVisible();
  });

  test("T-INT-RTE-003 remove member chip via the right-rail editor", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/routing");
    const firstCard = page.locator(".routing-card").nth(0);
    await expect(firstCard.getByText("@all", { exact: true })).toBeVisible();

    const rail = page.locator(".rail-panel");
    const allChip = rail.locator(".chip", { hasText: /^@all$/ });
    await allChip.getByRole("button", { name: "移除" }).click();

    // Both rail and left card lose @all.
    await expect(rail.locator(".chip", { hasText: /^@all$/ })).toHaveCount(0);
    await expect(firstCard.locator(".chip", { hasText: /^@all$/ })).toHaveCount(0);
  });

  test("T-INT-RTE-004 second click on the same card deselects and hides editor", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/routing");
    const firstCard = page.locator(".routing-card").nth(0);

    const rail = page.locator(".rail-panel");
    await expect(rail.getByLabel("服务组名称")).toBeVisible();

    await firstCard.click();
    await expect(rail.getByText("未选中服务组")).toBeVisible();
    await expect(rail.getByLabel("服务组名称")).toHaveCount(0);
  });

  test("T-INT-RTE-005 rename the active service group via right-rail input", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/routing");
    const rail = page.locator(".rail-panel");
    await rail.getByLabel("服务组名称").fill("SVC_PRIMARY");

    const firstCard = page.locator(".routing-card").nth(0);
    await expect(firstCard.getByText("SVC_PRIMARY", { exact: true })).toBeVisible();
  });
});
