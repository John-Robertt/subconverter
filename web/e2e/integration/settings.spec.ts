// T-INT-SET — A7 其他配置 (fallback / base_url / templates) page interactions.

import { expect, test } from "playwright/test";
import { createMockState, installMocks } from "./helpers/mock";

test.describe("settings page", () => {
  test("T-INT-SET-001 renders fixture fallback and base_url", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "fallback 服务组" })).toBeVisible();
    await expect(page.locator("select").first()).toHaveValue("SVC_PROXY");
    await expect(page.locator("input.mono-input").first()).toHaveValue("https://sub.example.com");
  });

  test("T-INT-SET-002 invalid base_url protocol surfaces field error", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/settings");
    const baseInput = page.locator("input.mono-input").first();
    await baseInput.fill("ftp://invalid.example.com");
    await expect(page.locator(".field-error", { hasText: "base_url 必须使用 http 或 https" })).toBeVisible();

    await baseInput.fill("https://sub.example.com/path?q=1");
    await expect(page.locator(".field-error", { hasText: "不能包含 path、query 或 fragment" })).toBeVisible();
  });

  test("T-INT-SET-003 templates are editable", async ({ page }) => {
    const state = createMockState();
    await installMocks(page, state);

    await page.goto("/settings");
    const inputs = page.locator("input.mono-input");
    // [0]=base_url, [1]=clash template, [2]=surge template
    await inputs.nth(1).fill("./templates/clash-custom.yaml");
    await inputs.nth(2).fill("https://templates.example.com/surge.conf");

    await expect(inputs.nth(1)).toHaveValue("./templates/clash-custom.yaml");
    await expect(inputs.nth(2)).toHaveValue("https://templates.example.com/surge.conf");
  });
});
