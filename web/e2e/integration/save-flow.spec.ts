// T-INT-SVF — Save workflow: validate → first-save confirm → PUT.

import { expect, test } from "playwright/test";
import { createMockState, installMocks, clearFirstSaveFlag, markFirstSaveConfirmed } from "./helpers/mock";
import { REVISION_DRIFT } from "./helpers/fixtures";

async function dirtyDraft(page: import("playwright/test").Page) {
  // Make a tiny change to the draft so the save button enables.
  await page.goto("/filters");
  const input = page.locator("input.mono-input");
  await input.fill("剩余|流量|官网|Expire|Demo");
}

test.describe("save flow", () => {
  test("T-INT-SVF-001 first save shows confirmation modal then succeeds", async ({ page }) => {
    const state = createMockState();
    await clearFirstSaveFlag(page);
    await installMocks(page, state);

    await dirtyDraft(page);
    await page.getByRole("button", { name: "保存" }).click();

    const dialog = page.getByRole("dialog", { name: "将草稿写入 YAML 文件？" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "确认保存" }).click();

    await expect(page.getByText("草稿已写入 YAML 文件")).toBeVisible();
    expect(state.callLog.some((entry) => entry.method === "PUT" && entry.url.endsWith("/api/config"))).toBeTruthy();
    expect(state.callLog.some((entry) => entry.url.endsWith("/api/reload"))).toBeFalsy();
  });

  test("T-INT-SVF-002 save with revision conflict surfaces error toast", async ({ page }) => {
    const state = createMockState({ saveMode: "revision_conflict" });
    state.config.config_revision = REVISION_DRIFT;
    state.status.config_revision = REVISION_DRIFT;
    await markFirstSaveConfirmed(page);
    await installMocks(page, state);

    await dirtyDraft(page);
    await page.getByRole("button", { name: "保存" }).click();

    await expect(page.getByText("配置文件已被外部修改")).toBeVisible();
  });

  test("T-INT-SVF-003 save succeeds and leaves reload as an explicit action", async ({ page }) => {
    const state = createMockState();
    await markFirstSaveConfirmed(page);
    await installMocks(page, state);

    await dirtyDraft(page);
    await page.getByRole("button", { name: "保存" }).click();

    await expect(page.getByText("草稿已写入 YAML 文件")).toBeVisible();
    await expect(page.getByRole("button", { name: "热重载" })).toBeVisible();
    expect(state.callLog.some((entry) => entry.url.endsWith("/api/reload"))).toBeFalsy();
  });

  test("T-INT-SVF-004 readonly config source forces readonly mode after PUT 409", async ({ page }) => {
    const state = createMockState({ saveMode: "readonly" });
    await markFirstSaveConfirmed(page);
    await installMocks(page, state);

    await dirtyDraft(page);
    await page.getByRole("button", { name: "保存" }).click();
    await expect(page.getByText("配置源只读")).toBeVisible();
  });
});
