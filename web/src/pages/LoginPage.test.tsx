import { QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { api } from "../api/client";
import { ApiError } from "../api/errors";
import type { AuthStatus } from "../api/types";
import { createAppQueryClient } from "../app/queryClient";
import { queryKeys } from "../app/queryKeys";
import { ThemeProvider } from "../state/theme";
import { ToastProvider } from "../state/toast";
import { LoginPage } from "./LoginPage";

const start = new Date("2026-11-08T00:00:00Z");
const unlocked: AuthStatus = { authed: false, setup_required: false, setup_token_required: false, locked_until: "" };
let client: ReturnType<typeof createAppQueryClient>;

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(start);
  localStorage.clear();
  Object.defineProperty(window, "matchMedia", { writable: true, value: () => ({ matches: false, addEventListener() {}, removeEventListener() {} }) });
  client = createAppQueryClient();
});

afterEach(() => {
  cleanup();
  client.clear();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

async function renderLogin(lockedUntil = "") {
  vi.spyOn(api, "authStatus").mockResolvedValue({ ...unlocked, locked_until: lockedUntil });
  render(
    <QueryClientProvider client={client}>
      <ThemeProvider><ToastProvider><MemoryRouter><LoginPage /></MemoryRouter></ToastProvider></ThemeProvider>
    </QueryClientProvider>
  );
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  return screen.getByRole("button", { name: "登录" });
}

// T-WEB-AUTH-LOCK-001: cached expired deadlines cannot disable login.
it("已过期的锁定时间不再禁用登录", async () => {
  const button = await renderLogin(new Date(start.getTime() - 1000).toISOString());
  expect(button.hasAttribute("disabled")).toBe(false);
});

// T-WEB-AUTH-LOCK-002: an initial status lock expires without refreshing or editing the form.
it("锁定截止时自动恢复登录按钮", async () => {
  const button = await renderLogin(new Date(start.getTime() + 60_000).toISOString());
  expect(button.hasAttribute("disabled")).toBe(true);
  await act(async () => { await vi.advanceTimersByTimeAsync(59_998); });
  expect(button.hasAttribute("disabled")).toBe(true);
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(button.hasAttribute("disabled")).toBe(false);
});

// T-WEB-AUTH-LOCK-003: a 423 response temporarily disables login and expiry permits a new request.
it("登录响应触发锁定，到期后可再次提交", async () => {
  const until = new Date(start.getTime() + 60_000).toISOString();
  const login = vi.spyOn(api, "login")
    .mockRejectedValueOnce(new ApiError(423, "auth_locked", "登录失败次数过多", undefined, { until }))
    .mockRejectedValueOnce(new ApiError(401, "invalid_credentials", "用户名或密码错误", undefined, { remaining: 4 }));
  const button = await renderLogin();
  fireEvent.change(screen.getByLabelText("密码"), { target: { value: "wrong-password" } });
  fireEvent.click(button);
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(screen.getByText("账号已临时锁定")).toBeTruthy();
  expect(button.hasAttribute("disabled")).toBe(true);
  await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
  expect(button.hasAttribute("disabled")).toBe(false);
  expect(screen.queryByText("账号已临时锁定")).toBeNull();
  fireEvent.click(button);
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(login).toHaveBeenCalledTimes(2);
  expect(screen.getByText("还可尝试 4 次。")).toBeTruthy();
});

// T-WEB-AUTH-LOCK-004: a newer status deadline replaces the previous unlock timer.
it("锁定时间更新后按新的截止时间解锁", async () => {
  const button = await renderLogin(new Date(start.getTime() + 60_000).toISOString());
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  await act(async () => {
    client.setQueryData(queryKeys.authStatus, { ...unlocked, locked_until: new Date(start.getTime() + 120_000).toISOString() });
    await vi.advanceTimersByTimeAsync(1);
  });
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(button.hasAttribute("disabled")).toBe(true);
  await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
  expect(button.hasAttribute("disabled")).toBe(false);
});
