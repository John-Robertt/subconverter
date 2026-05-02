import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, FileText, LogOut, Moon, Save, Sun, SunMoon, TriangleAlert } from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { queryKeys } from "../app/queryKeys";
import { findPage, pages, sectionLabels } from "../app/pages";
import { useSaveWorkflow } from "../features/useSaveWorkflow";
import { useConfigState } from "../state/config";
import { useTheme, type ThemePreference } from "../state/theme";
import { useToast } from "../state/toast";
import { Button, IconButton, LoadingState, StatusBadge } from "../components/ui";

export function AppShell() {
  const location = useLocation();
  const currentPage = findPage(location.pathname);
  const { draft, status, isLoading, isDraftDirty, isReadonly, externalRevisionChanged, resetDraft } = useConfigState();
  const { preference, setPreference } = useTheme();
  const { saveDraft, isSaving, validateDraft, isValidating } = useSaveWorkflow();
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

  async function runValidation() {
    try {
      const result = await validateDraft();
      const total = result.errors.length + result.warnings.length + result.infos.length;
      pushToast({
        kind: result.valid ? "success" : "warning",
        title: result.valid ? "静态校验通过" : "静态校验发现问题",
        message: result.valid ? "当前草稿可以进入保存流程。" : `发现 ${total} 个诊断项，请在保存前检查。`
      });
    } catch (error) {
      pushToast({ kind: "error", title: "校验失败", message: error instanceof Error ? error.message : "请求失败", persistent: true });
    }
  }

  const navCount = {
    sources: (draft?.sources?.subscriptions?.length ?? 0) + (draft?.sources?.snell?.length ?? 0) + (draft?.sources?.vless?.length ?? 0) + (draft?.sources?.custom_proxies?.length ?? 0),
    filters: draft?.filters?.exclude ? 1 : 0,
    groups: draft?.groups?.length ?? 0,
    routing: draft?.routing?.length ?? 0
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
            <div className="brand-subtitle">{status?.version ? `v${status.version}` : "Admin"}</div>
          </div>
        </div>
        <div className="nav-section-label">配置</div>
        <nav className="nav-list">
          {pages.map((page) => {
            const Icon = page.icon;
            const pageKey = page.path.replace("/", "") as keyof typeof navCount;
            const count = navCount[pageKey];
            return (
              <NavLink key={page.path} to={page.path} className={({ isActive }) => (isActive ? "nav-link active" : "nav-link")}>
                <Icon size={17} aria-hidden="true" />
                <span>{page.label}</span>
                <small>{typeof count === "number" && count > 0 ? count : page.milestone}</small>
              </NavLink>
            );
          })}
        </nav>
        <div className="sidebar-status-card">
          <span className={status?.config_dirty ? "status-dot warning" : "status-dot"} aria-hidden="true" />
          <div>
            <strong>{status?.config_dirty ? "待热重载" : "服务运行中"}</strong>
            <small>{status?.last_reload?.time ? `上次 reload ${formatRelative(status.last_reload.time)}` : status?.runtime_config_revision ?? "等待状态"}</small>
          </div>
        </div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div className="topbar-title">
            <p className="eyebrow">{sectionLabels[currentPage.section]}</p>
            <h1>{currentPage.title}</h1>
            <p>{currentPage.subtitle}</p>
          </div>

          <div className="topbar-status">
            <StatusBadge tone={status?.config_dirty ? "warning" : "success"}>config.yaml</StatusBadge>
            {isReadonly ? <StatusBadge tone="warning">readonly</StatusBadge> : <StatusBadge tone="info">writable</StatusBadge>}
            {isDraftDirty ? <StatusBadge tone="warning">draft changed</StatusBadge> : <StatusBadge>draft clean</StatusBadge>}
          </div>

          <div className="topbar-actions">
            <IconButton label={`主题：${preference}`} variant="ghost" onClick={cycleTheme}>
              {preference === "system" ? <SunMoon size={18} aria-hidden="true" /> : preference === "dark" ? <Moon size={18} aria-hidden="true" /> : <Sun size={18} aria-hidden="true" />}
            </IconButton>
            <Button variant="secondary" icon={<CheckCircle2 size={16} aria-hidden="true" />} loading={isValidating} disabled={!draft} onClick={() => void runValidation()}>
              校验
            </Button>
            <Button
              variant="primary"
              icon={<Save size={16} aria-hidden="true" />}
              loading={isSaving}
              disabled={isReadonly || !isDraftDirty}
              onClick={() => void saveDraft().catch(() => undefined)}
            >
              保存并热重载
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
