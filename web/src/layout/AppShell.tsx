import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileText, LogOut, Moon, RefreshCcw, Save, Sun, SunMoon, TriangleAlert } from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { queryKeys } from "../app/queryKeys";
import { findPage, pages, type PageSection } from "../app/pages";
import { useSaveWorkflow } from "../features/useSaveWorkflow";
import { useConfigState } from "../state/config";
import { useTheme, type ThemePreference } from "../state/theme";
import { useToast } from "../state/toast";
import { Button, IconButton, LoadingState } from "../components/ui";

const sidebarNavGroups: Array<{ label: string; sections: PageSection[] }> = [
  { label: "配置", sections: ["config"] },
  { label: "运行时", sections: ["runtime", "system"] }
];

export function AppShell() {
  const location = useLocation();
  const currentPage = findPage(location.pathname);
  const { draft, status, isLoading, isDraftDirty, isReadonly, externalRevisionChanged, resetDraft, statusError } = useConfigState();
  const { preference, setPreference } = useTheme();
  const { saveDraft, isSaving, reloadOnly, isReloading } = useSaveWorkflow();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { pushToast } = useToast();

  const logoutMutation = useMutation({
    mutationFn: api.logout,
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.authStatus });
      navigate("/login", { replace: true });
    }
  });

  function cycleTheme() {
    const next: Record<ThemePreference, ThemePreference> = {
      system: "light",
      light: "dark",
      dark: "system"
    };
    setPreference(next[preference]);
  }

  const navCount = {
    sources: (draft?.sources?.subscriptions?.length ?? 0) + (draft?.sources?.snell?.length ?? 0) + (draft?.sources?.vless?.length ?? 0) + (draft?.sources?.custom_proxies?.length ?? 0),
    filters: draft?.filters?.exclude ? 1 : 0,
    groups: draft?.groups?.length ?? 0,
    routing: draft?.routing?.length ?? 0,
    rulesets: draft?.rulesets?.length ?? 0,
    rules: draft?.rules?.length ?? 0,
    settings: [draft?.fallback, draft?.base_url, draft?.templates?.clash, draft?.templates?.surge].filter(Boolean).length
  };

  if (isLoading) {
    return (
      <div className="app-shell">
        <aside className="sidebar skeleton-sidebar" />
        <main className="workspace">
          <LoadingState message="正在加载后台状态与配置" />
        </main>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="主导航">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">S</span>
          <div>
            <div className="brand-name">subconverter</div>
            <div className="brand-version">{status?.version ? `v${status.version}` : ""}</div>
          </div>
        </div>

        <nav className="nav-groups" aria-label="主导航">
          {sidebarNavGroups.map((group) => {
            const sectionPages = pages.filter((page) => group.sections.includes(page.section));
            if (sectionPages.length === 0) return null;
            return (
              <section key={group.label} className="nav-section" aria-labelledby={`nav-section-${group.label}`}>
                <div id={`nav-section-${group.label}`} className="nav-section-label">
                  {group.label}
                </div>
                <div className="nav-list">
                  {sectionPages.map((page) => {
                    const Icon = page.icon;
                    const pageKey = page.path.replace("/", "") as keyof typeof navCount;
                    const count = navCount[pageKey];
                    const badge = typeof count === "number" && count > 0 ? count : null;
                    return (
                      <NavLink key={page.path} to={page.path} className={({ isActive }) => (isActive ? "nav-link active" : "nav-link")}>
                        <Icon size={17} aria-hidden="true" />
                        <span>{page.label}</span>
                        {badge ? <small>{badge}</small> : null}
                      </NavLink>
                    );
                  })}
                </div>
              </section>
            );
          })}
        </nav>

        <SidebarStatusCard status={status} statusError={statusError} />

      </aside>

      <main className="workspace">
        <header className="topbar">
          <div className="topbar-title">
            <h1>{currentPage.title}</h1>
            <p>{currentPage.subtitle}</p>
          </div>

          <div className="topbar-actions">
            <span className="topbar-dim-text">config.yaml</span>
            <IconButton label={`主题：${preference}`} variant="ghost" onClick={cycleTheme}>
              {preference === "system" ? <SunMoon size={18} aria-hidden="true" /> : preference === "dark" ? <Moon size={18} aria-hidden="true" /> : <Sun size={18} aria-hidden="true" />}
            </IconButton>
            <Button
              variant="primary"
              icon={<Save size={16} aria-hidden="true" />}
              loading={isSaving}
              disabled={isReadonly || !isDraftDirty}
              onClick={() => void saveDraft().catch(() => undefined)}
            >
              保存
            </Button>
            <Button
              variant="secondary"
              icon={<RefreshCcw size={16} aria-hidden="true" />}
              loading={isReloading}
              onClick={() => void reloadOnly()}
            >
              热重载
            </Button>
            <IconButton label="注销" variant="ghost" onClick={() => void logoutMutation.mutateAsync()}>
              <LogOut size={18} aria-hidden="true" />
            </IconButton>
          </div>
        </header>

        {status?.config_dirty || externalRevisionChanged ? (
          <div className="warning-strip">
            {externalRevisionChanged ? <TriangleAlert size={17} aria-hidden="true" /> : <FileText size={17} aria-hidden="true" />}
            <span>
              {externalRevisionChanged ? "检测到配置 revision 已变化，当前草稿不会被自动覆盖。" : "已保存配置尚未生效，当前运行时仍使用旧 RuntimeConfig。"}
            </span>
            {externalRevisionChanged ? (
              <button
                type="button"
                onClick={() => {
                  resetDraft();
                  pushToast({ kind: "info", title: "已回到当前已加载草稿基线" });
                }}
              >
                丢弃草稿
              </button>
            ) : null}
          </div>
        ) : null}

        <section className="workspace-content">
          <Outlet />
        </section>
      </main>
    </div>
  );
}

function formatRelative(value: string) {
  const time = new Date(value).getTime();
  if (Number.isNaN(time)) return value;
  const seconds = Math.max(0, Math.round((Date.now() - time) / 1000));
  if (seconds < 60) return `${seconds} 秒前`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.round(minutes / 60);
  return `${hours} 小时前`;
}

type SidebarTone = "success" | "warning" | "error";

interface SidebarStatusView {
  tone: SidebarTone;
  title: string;
  detail: string;
}

function deriveSidebarStatus(
  status: ReturnType<typeof useConfigState>["status"],
  statusError: unknown
): SidebarStatusView {
  if (statusError) {
    return { tone: "error", title: "后端不可达", detail: "请求 /api/status 失败" };
  }
  if (status?.last_reload?.success === false) {
    return {
      tone: "error",
      title: "上次热重载失败",
      detail: status.last_reload.time ? `${formatRelative(status.last_reload.time)}失败` : "请检查日志"
    };
  }
  if (status?.config_dirty) {
    return { tone: "warning", title: "配置待重载", detail: "草稿已保存但未生效" };
  }
  if (status?.last_reload?.time) {
    return { tone: "success", title: "服务运行中", detail: `${formatRelative(status.last_reload.time)}热重载` };
  }
  if (status?.config_loaded_at) {
    return { tone: "success", title: "服务运行中", detail: `${formatRelative(status.config_loaded_at)}加载` };
  }
  return { tone: "success", title: "服务运行中", detail: "等待状态信息" };
}

function SidebarStatusCard({
  status,
  statusError
}: {
  status: ReturnType<typeof useConfigState>["status"];
  statusError: unknown;
}) {
  const view = deriveSidebarStatus(status, statusError);
  const dotClass = view.tone === "success" ? "status-dot" : `status-dot ${view.tone}`;
  return (
    <div className="sidebar-status-card">
      <span className={dotClass} aria-hidden="true" />
      <div>
        <strong>{view.title}</strong>
        <small>{view.detail}</small>
      </div>
    </div>
  );
}
