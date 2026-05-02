import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import App from "./App";

function renderApp(initialPath = "/sources") {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false
      }
    }
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("App", () => {
  it("renders the M8 shell route", () => {
    renderApp("/sources");

    expect(screen.getByRole("heading", { name: "Web 镜像与 Compose 集成" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "订阅来源" })).toBeTruthy();
  });

  it("keeps /download inside the SPA", () => {
    renderApp("/download");

    expect(screen.getByRole("heading", { name: "生成下载" })).toBeTruthy();
  });

  it("renders the login route without the app shell", () => {
    renderApp("/login");

    expect(screen.getByRole("heading", { name: "管理员登录" })).toBeTruthy();
  });
});
