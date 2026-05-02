import { Clock3 } from "lucide-react";
import { PageHeader, StatusBadge } from "../components/ui";

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className="page-stack">
      <PageHeader title={title} description="该页面归属 M10，M9 仅保留受保护路由和统一页面状态框架。" actions={<StatusBadge tone="neutral">M10</StatusBadge>} />
      <section className="content-panel empty-state">
        <Clock3 size={20} aria-hidden="true" />
        <strong>后续里程碑实现</strong>
        <p>当前 M9 先交付 A1-A4、B1、C 与基础工作流；本页不会触发业务写入。</p>
      </section>
    </div>
  );
}
