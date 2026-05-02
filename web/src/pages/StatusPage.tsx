import { useQuery } from "@tanstack/react-query";
import { RefreshCcw } from "lucide-react";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { Button, Chip, ErrorState, LoadingState, StatCard } from "../components/ui";
import { useSaveWorkflow } from "../features/useSaveWorkflow";
import { useConfigState } from "../state/config";

export function StatusPage() {
  const { status, isStatusLoading, statusError } = useConfigState();
  const { reloadOnly, isReloading } = useSaveWorkflow();
  const healthQuery = useQuery({
    queryKey: ["healthz"],
    queryFn: api.healthz
  });

  return (
    <div className="page-stack">
      <section className="status-hero">
        <div>
          <span className={healthQuery.isError ? "status-dot error" : "status-dot"} aria-hidden="true" />
          <div>
            <h2>{healthQuery.isError ? "服务健康检查失败" : "服务运行中"}</h2>
            <p>{status?.last_reload?.time ? `最近 reload：${status.last_reload.time}` : "暂无 reload 记录"}</p>
          </div>
        </div>
        <Button variant="secondary" icon={<RefreshCcw size={16} aria-hidden="true" />} loading={isReloading} onClick={() => void reloadOnly()}>
          触发 reload
        </Button>
      </section>

      {isStatusLoading ? <LoadingState message="正在加载系统状态" /> : null}
      {statusError ? <ErrorState message={getErrorMessage(statusError)} /> : null}
      {healthQuery.isError ? <ErrorState title="健康检查失败" message={getErrorMessage(healthQuery.error)} /> : null}

      <div className="stats-grid">
        <StatCard label="healthz" value={healthQuery.isError ? "failed" : healthQuery.data || "OK"} tone={healthQuery.isError ? "error" : "success"} />
        <StatCard label="version" value={status?.version ?? "-"} sub={status?.build_date ?? "build date 未提供"} />
        <StatCard label="config dirty" value={status?.config_dirty ? "true" : "false"} tone={status?.config_dirty ? "warning" : "success"} />
        <StatCard label="last reload" value={status?.last_reload?.success === false ? "failed" : status?.last_reload ? "success" : "-"} tone={status?.last_reload?.success === false ? "error" : "success"} />
      </div>

      <section className="content-panel status-detail-panel">
        <div className="section-heading row">
          <div>
            <h3>配置源</h3>
            <p>配置源、写入能力和 revision 是保存/reload 工作流的边界。</p>
          </div>
          <Chip tone={status?.config_source.writable ? "success" : "warning"}>{status?.config_source.writable ? "writable" : "readonly"}</Chip>
        </div>
        <div className="detail-grid">
          <Detail label="类型" value={status?.config_source.type ?? "-"} />
          <Detail label="位置" value={status?.config_source.location ?? "-"} mono />
          <Detail label="保存能力" value={status?.capabilities.config_write ? "可保存" : "只读"} badge={status?.capabilities.config_write ? "success" : "warning"} />
          <Detail label="reload 能力" value={status?.capabilities.reload ? "可 reload" : "不可 reload"} badge={status?.capabilities.reload ? "success" : "warning"} />
          <Detail label="config_revision" value={status?.config_revision ?? "-"} mono />
          <Detail label="runtime_config_revision" value={status?.runtime_config_revision ?? "-"} mono />
          <Detail label="config_loaded_at" value={status?.config_loaded_at ?? "-"} mono />
          <Detail label="commit" value={status?.commit ?? "-"} mono />
        </div>
      </section>

      <section className="content-panel status-detail-panel">
        <div className="section-heading">
          <h3>最近 reload</h3>
        </div>
        {status?.last_reload ? (
          <div className="detail-grid">
            <Detail label="时间" value={status.last_reload.time} mono />
            <Detail label="结果" value={status.last_reload.success ? "成功" : "失败"} badge={status.last_reload.success ? "success" : "error"} />
            <Detail label="耗时" value={`${status.last_reload.duration_ms} ms`} />
            {status.last_reload.error ? <Detail label="错误" value={status.last_reload.error} /> : null}
          </div>
        ) : (
          <p className="muted">暂无 reload 记录。</p>
        )}
      </section>
    </div>
  );
}

function Detail({ label, value, mono, badge }: { label: string; value: string; mono?: boolean; badge?: "success" | "warning" | "error" }) {
  return (
    <div className="detail-item">
      <span>{label}</span>
      {badge ? <Chip tone={badge}>{value}</Chip> : <strong className={mono ? "mono-cell" : undefined}>{value}</strong>}
    </div>
  );
}
