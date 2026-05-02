import { useQuery } from "@tanstack/react-query";
import { RefreshCcw } from "lucide-react";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { ExpandedMember, PreviewGroup } from "../api/types";
import { queryKeys } from "../app/queryKeys";
import { DiagnosticList } from "../components/Diagnostics";
import { Button, Chip, EmptyState, ErrorState, LoadingState, StatCard } from "../components/ui";
import { getValidateResult } from "../features/diagnostics";
import { useConfigState } from "../state/config";

export function GroupPreviewPage() {
  const { status } = useConfigState();
  const runtimeRevision = status?.runtime_config_revision;
  const groupsQuery = useQuery({
    queryKey: queryKeys.previewGroups(runtimeRevision),
    queryFn: api.previewGroups,
    enabled: Boolean(runtimeRevision)
  });
  const validation = getValidateResult(groupsQuery.error);
  const result = groupsQuery.data;

  return (
    <div className="page-stack group-runtime-page">
      {status?.config_dirty ? <section className="content-panel info-panel">当前分组预览仍基于运行时配置，不等于已保存但尚未 reload 的草稿。</section> : null}

      <div className="node-toolbar">
        <div className="category-row">
          <Chip tone="info">Runtime revision</Chip>
          <code>{runtimeRevision ?? "-"}</code>
        </div>
        <Button variant="secondary" icon={<RefreshCcw size={16} aria-hidden="true" />} loading={groupsQuery.isFetching} onClick={() => void groupsQuery.refetch()}>
          刷新预览
        </Button>
      </div>

      {groupsQuery.isLoading ? <LoadingState message="正在拉取订阅并计算运行时分组预览" /> : null}
      {groupsQuery.error ? (
        validation ? (
          <section className="content-panel">
            <div className="section-heading">
              <h3>图级校验失败</h3>
              <p>后端没有返回部分成功结果；请根据诊断修复配置后重新 reload。</p>
            </div>
            <DiagnosticList result={validation} />
          </section>
        ) : (
          <ErrorState message={getErrorMessage(groupsQuery.error)} action={<Button variant="secondary" onClick={() => void groupsQuery.refetch()}>重试预览</Button>} />
        )
      ) : null}

      {result ? (
        <>
          <div className="stats-grid">
            <StatCard label="节点组" value={result.node_groups.length} sub="node_groups" />
            <StatCard label="链式组" value={result.chained_groups.length} sub="chained_groups" tone="info" />
            <StatCard label="服务组" value={result.service_groups.length} sub="service_groups" tone="success" />
            <StatCard label="全部代理" value={result.all_proxies.length} sub="@all 展开基础" tone="warning" />
          </div>

          <section className="preview-grid">
            <PreviewGroupSection title="节点组" groups={result.node_groups} />
            <PreviewGroupSection title="链式组" groups={result.chained_groups} />
            <PreviewGroupSection title="服务组" groups={result.service_groups} />
            <section className="content-panel">
              <div className="section-heading">
                <h3>All proxies</h3>
                <p>@all 展开会使用的运行时代理集合。</p>
              </div>
              {result.all_proxies.length === 0 ? <EmptyState title="没有代理" message="来源为空或全部被过滤。" /> : <MemberList members={result.all_proxies.map((value) => ({ value, origin: "all_proxies" }))} />}
            </section>
          </section>
        </>
      ) : null}
    </div>
  );
}

function PreviewGroupSection({ title, groups }: { title: string; groups: PreviewGroup[] }) {
  return (
    <section className="content-panel runtime-group-section">
      <div className="section-heading row">
        <div>
          <h3>{title}</h3>
          <p>树形展示成员和展开来源。</p>
        </div>
        <Chip>{groups.length}</Chip>
      </div>
      {groups.length === 0 ? (
        <EmptyState title={`${title} 为空`} message="当前运行时配置没有生成该类型分组。" />
      ) : (
        <div className="runtime-group-list">
          {groups.map((group) => (
            <article key={group.name} className="runtime-group-card">
              <header>
                <strong>{group.name}</strong>
                <Chip tone={group.strategy === "url-test" ? "success" : "info"}>{group.strategy}</Chip>
                <Chip>{group.members.length} members</Chip>
              </header>
              <MemberList members={(group.expanded_members ?? group.members.map((value) => ({ value, origin: "literal" }))).map((member) => normalizeMember(member))} />
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function MemberList({ members }: { members: ExpandedMember[] }) {
  return (
    <ul className="member-tree">
      {members.map((member, index) => (
        <li key={`${member.value}-${index}`}>
          <span>{member.value}</span>
          <Chip tone={memberOriginTone(member.origin)}>{member.origin}</Chip>
        </li>
      ))}
    </ul>
  );
}

function normalizeMember(member: ExpandedMember | string): ExpandedMember {
  return typeof member === "string" ? { value: member, origin: "literal" } : member;
}

function memberOriginTone(origin: string): "neutral" | "accent" | "success" | "warning" | "info" {
  if (origin === "all_expanded") return "accent";
  if (origin === "auto_expanded") return "success";
  if (origin === "all_proxies") return "warning";
  return "info";
}
