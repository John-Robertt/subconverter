import { expect, test, type Locator, type Page } from "playwright/test";

const setupToken = process.env.E2E_SETUP_TOKEN ?? "setup-e2e-secret";
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? "admin-password-123";

test("T-E2E-015 setup, login, logout and token link confirmation", async ({ page }) => {
  await loginOrSetup(page);

  await page.getByLabel("注销").click();
  await expect(page.getByRole("heading", { name: "登录管理后台" })).toBeVisible();
  await page.locator('input[autocomplete="current-password"]').fill(adminPassword);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "订阅来源" })).toBeVisible();

  await page.goto("/download");
  await page.getByRole("button", { name: "复制订阅链接" }).click();
  await expect(page.getByRole("dialog", { name: "复制含 token 的订阅链接？" })).toBeVisible();
  await page.getByRole("button", { name: "确认复制" }).click();
  await expect(page.getByText("订阅链接已复制")).toBeVisible();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toContain("token=server-token");
});

test("T-E2E-010 writable M10 flow validates, previews groups and downloads", async ({ page }) => {
  await loginOrSetup(page);

  await page.goto("/rulesets");
  await expect(page.getByRole("heading", { name: "规则集" })).toBeVisible();
  await page.getByRole("button", { name: "添加 URL" }).click();
  await page.getByPlaceholder("https://example.com/rules.list").last().fill("http://127.0.0.1:18081/rules.list");
  await saveAndReload(page);

  await page.goto("/validate");
  await page.getByRole("button", { name: "运行静态校验" }).click();
  await expect(page.getByRole("main").getByText("静态校验通过")).toBeVisible();

  await page.goto("/preview/groups");
  await expect(page.getByText("All proxies")).toBeVisible();
  await expect(page.getByText("HK-01").first()).toBeVisible();

  await page.goto("/download");
  await page.getByPlaceholder("clash.yaml").fill("clash-e2e.yaml");
  await page.getByRole("button", { name: "当前运行时预览" }).click();
  await expect(page.getByText(/HK-01/).first()).toBeVisible();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载配置" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("clash-e2e.yaml");
});

test("T-E2E-014 dual format preview keeps subscription token out of API requests", async ({ page }) => {
  const apiRequests: string[] = [];
  page.on("request", (request) => {
    const url = request.url();
    if (url.includes("/api/")) apiRequests.push(url);
  });

  await loginOrSetup(page);
  await page.goto("/download");

  await page.getByRole("radio", { name: /Surge/ }).check();
  await page.getByRole("button", { name: "当前运行时预览" }).click();
  await expect(page.getByText(/HK-01/).first()).toBeVisible();

  await page.getByRole("radio", { name: /Clash Meta/ }).check();
  await page.getByRole("button", { name: "草稿生成预览" }).click();
  await expect(page.getByText(/HK-01/).first()).toBeVisible();

  await page.getByText("复制订阅链接时请求服务端附带 token").click();
  await page.getByRole("button", { name: "复制订阅链接" }).click();
  await expect(page.getByText("订阅链接已复制")).toBeVisible();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).not.toContain("server-token");
  expect(apiRequests.some((url) => url.includes("server-token"))).toBe(false);
});

async function loginOrSetup(page: Page) {
  await page.goto("/sources");
  await expect(page.getByRole("heading", { name: /首次创建管理员|登录管理后台|订阅来源/ })).toBeVisible({ timeout: 10_000 });
  if (await isVisible(page.getByRole("heading", { name: "订阅来源" }), 100)) {
    return;
  }
  if (await isVisible(page.getByRole("heading", { name: "首次创建管理员" }))) {
    await page.locator('input[autocomplete="one-time-code"]').fill(setupToken);
    await page.locator('input[autocomplete="username"]').fill("admin");
    const passwordInputs = page.locator('input[autocomplete="new-password"]');
    await passwordInputs.nth(0).fill(adminPassword);
    await passwordInputs.nth(1).fill(adminPassword);
    await page.getByRole("button", { name: "创建管理员并登录" }).click();
  } else if (await isVisible(page.getByRole("heading", { name: "登录管理后台" }))) {
    await page.locator('input[autocomplete="current-password"]').fill(adminPassword);
    await page.getByRole("button", { name: "登录" }).click();
  }
  await expect(page.getByRole("heading", { name: "订阅来源" })).toBeVisible();
}

async function saveAndReload(page: Page) {
  await page.getByRole("button", { name: "保存并热重载" }).click();
  if (await isVisible(page.getByRole("dialog", { name: "确认首次写回 YAML" }), 5_000)) {
    await page.getByRole("button", { name: "确认保存" }).click();
  }
  await expect(page.getByText(/RuntimeConfig 已重新加载|配置已保存并生效/)).toBeVisible({ timeout: 20_000 });
}

async function isVisible(locator: Locator, timeout = 1_000) {
  return expect(locator).toBeVisible({ timeout }).then(
    () => true,
    () => false
  );
}
