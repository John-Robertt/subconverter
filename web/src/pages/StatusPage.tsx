import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { Chip, ErrorState, LoadingState, StatCard } from "../components/ui";
import { useConfigState } from "../state/config";

export function StatusPage() {
  const { status, draft, isStatusLoading, statusError } = useConfigState();
  const healthQuery = useQuery({
    queryKey: ["healthz"],
    queryFn: api.healthz
  });

  return (
    <div className="page-stack">
      {isStatusLoading ? <LoadingState message="正在加载系统状态" /> : null}
      {statusError ? <ErrorState message={getErrorMessage(statusError)} /> : null}
      {healthQuery.isError ? <ErrorState title="健康检查失败" message={getErrorMessage(healthQuery.error)} /> : null}

      <div className="stats-grid">
        <StatCard
          label="服务状态"
          value={healthQuery.isError ? "异常" : "运行中"}
          sub={formatServiceSub(healthQuery.isError, status)}
          tone={healthQuery.isError ? "error" : "success"}
          className={healthQuery.isError ? undefined : "stat-with-dot"}
        />
        <StatCard
          label="版本"
          value={status?.version ? `v${status.version}` : "-"}
          sub={formatVersionSub(status)}
        />
        <StatCard
          label="配置"
          value={status?.config_dirty ? "待重载" : "已加载"}
          sub={formatConfigSub(draft)}
          tone={status?.config_dirty ? "warning" : "success"}
          className={status?.config_dirty ? undefined : "stat-with-dot"}
        />
        <StatCard
          label="上次热重载"
          value={status?.last_reload?.time ? formatRelativeShort(status.last_reload.time) : "-"}
          sub={formatReloadSub(status)}
          tone={status?.last_reload?.success === false ? "error" : "info"}
        />
      </div>

      <div className="status-two-col">
        <section className="content-panel">
          <div className="section-heading row">
            <div>
              <h3>最近 reload</h3>
              <p>热重载历史记录。</p>
            </div>
            <Chip tone={status?.last_reload?.success === false ? "error" : "success"}>
              {status?.last_reload?.success === false ? "最近失败" : "正常"}
            </Chip>
          </div>
          {status?.last_reload ? (
            <div className="stack-list">
              <div className="reload-history-row">
                <code>{status.last_reload.time ? formatTimeOnly(status.last_reload.time) : "-"}</code>
                <Chip tone={status.last_reload.success ? "success" : "error"}>{status.last_reload.success ? "ok" : "failed"}</Chip>
                <span>{status.last_reload.success ? "配置正常加载" : (status.last_reload.error || "reload 失败，详情见日志")}</span>
                <code style={{ textAlign: "right" }}>{typeof status.last_reload.duration_ms === "number" ? `${status.last_reload.duration_ms}ms` : "-"}</code>
              </div>
            </div>
          ) : (
            <p className="muted">暂无 reload 记录。</p>
          )}
        </section>

        <div className="page-stack">
          <section className="content-panel">
            <div className="section-heading">
              <h3>运行环境</h3>
            </div>
            <div className="stack-list">
              <EnvRow label="配置文件" value={status?.config_source.location ?? "-"} title={status?.config_source.location} />
              <EnvRow label="监听地址" value={status?.runtime_environment?.listen_addr ?? "-"} />
              <EnvRow label="工作目录" value={status?.runtime_environment?.working_dir ?? "-"} title={status?.runtime_environment?.working_dir} />
              <EnvRow label="Go runtime" value={status?.runtime_environment?.go_runtime ?? "-"} />
              <EnvRow label="内存占用" value={status?.runtime_environment ? `${status.runtime_environment.memory_alloc_mb} MB` : "-"} />
              <EnvRow label="请求总数" value={formatRequestCount(status?.runtime_environment?.request_count_24h, status?.runtime_environment?.uptime_seconds)} />
            </div>
          </section>

          <section className="content-panel">
            <div className="section-heading">
              <h3>健康探针</h3>
            </div>
            <div className="stack-list">
              <ProbeRow method="GET" path="/healthz" status={healthQuery.isError ? "err" : "200"} />
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

function EnvRow({ label, value, title }: { label: string; value: string; title?: string }) {
  return (
    <div className="env-row">
      <span>{label}</span>
      <code title={title ?? value}>{value}</code>
    </div>
  );
}

function formatRequestCount(count?: number, uptimeSeconds?: number): string {
  if (count === undefined) return "-";
  const formatted = new Intl.NumberFormat("en-US").format(count);
  if (uptimeSeconds !== undefined && uptimeSeconds < 24 * 3600) {
    return `${formatted}（自启动）`;
  }
  return `${formatted}（过去 24h）`;
}

function ProbeRow({ method, path, status }: { method: string; path: string; status: string }) {
  return (
    <div className="probe-row">
      <span className={`probe-method probe-method-${method.toLowerCase()}`}>{method}</span>
      <code className="probe-path">{path}</code>
      <code className="probe-status" style={{ background: status === "200" ? "var(--success-soft)" : "var(--surface-muted)", color: status === "200" ? "var(--success)" : "var(--text-muted)" }}>{status}</code>
    </div>
  );
}

function formatRelativeShort(value: string) {
  const time = new Date(value).getTime();
  if (Number.isNaN(time)) return value;
  const seconds = Math.max(0, Math.round((Date.now() - time) / 1000));
  if (seconds < 60) return `${seconds} 秒前`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.round(minutes / 60);
  return `${hours} 小时前`;
}

function formatVersionSub(status: ReturnType<typeof useConfigState>["status"]): string {
  const parts: string[] = [];
  if (status?.commit) parts.push(`git ${status.commit.slice(0, 7)}`);
  if (status?.build_date) parts.push(status.build_date);
  return parts.length > 0 ? parts.join(" · ") : "构建信息未提供";
}

function formatServiceSub(unhealthy: boolean, status: ReturnType<typeof useConfigState>["status"]): string {
  if (unhealthy) return "/healthz 不可达";
  if (status?.config_loaded_at) {
    const loadedAt = new Date(status.config_loaded_at).getTime();
    if (!Number.isNaN(loadedAt)) {
      const seconds = Math.max(0, Math.round((Date.now() - loadedAt) / 1000));
      return `已运行 ${formatDuration(seconds)}`;
    }
  }
  return "/healthz 200";
}

function formatConfigSub(draft: ReturnType<typeof useConfigState>["draft"]): string {
  if (!draft) return "配置加载中";
  const sources = draft.sources;
  const fetchSourceCount =
    (sources?.subscriptions?.length ?? 0) + (sources?.snell?.length ?? 0) + (sources?.vless?.length ?? 0) + (sources?.custom_proxies?.length ?? 0);
  return `${fetchSourceCount} 来源 · ${draft.groups?.length ?? 0} 分组 · ${draft.routing?.length ?? 0} 路由`;
}

function formatReloadSub(status: ReturnType<typeof useConfigState>["status"]): string {
  const parts: string[] = [];
  if (typeof status?.last_reload?.duration_ms === "number") parts.push(`${status.last_reload.duration_ms}ms`);
  if (status?.last_reload?.success === false) parts.push("最近失败");
  else if (status?.last_reload) parts.push("成功");
  return parts.length > 0 ? parts.join(" · ") : "无记录";
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function formatTimeOnly(value: string) {
  try {
    const d = new Date(value);
    return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}:${String(d.getSeconds()).padStart(2, "0")}`;
  } catch {
    return value;
  }
}
