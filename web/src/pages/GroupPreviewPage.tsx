import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { PreviewGroup, PreviewMember } from "../api/types";
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
            <StatCard label="全部代理" value={result.all_proxies.length} sub="All proxies · @all 展开基础" tone="warning" />
          </div>

          {result.node_groups.length > 0 ? (
            <div className="group-preview-grid">
              {result.node_groups.map((group) => (
                <GroupPreviewCard key={group.name} group={group} />
              ))}
            </div>
          ) : null}

          <section className="content-panel">
            <div className="section-heading row">
              <div>
                <h3>服务组展开 / Expansion</h3>
                <p>服务组成员展开到最终节点。</p>
              </div>
              <Chip>{result.service_groups.length}</Chip>
            </div>
            {result.service_groups.length === 0 ? (
              <EmptyState title="服务组为空" message="当前运行时配置没有生成服务组。" />
            ) : (
              <div className="stack-list">
                {result.service_groups.map((group) => (
                  <ServiceGroupRow key={group.name} group={group} />
                ))}
              </div>
            )}
          </section>

          {result.chained_groups.length > 0 ? (
            <section className="content-panel runtime-group-section">
              <div className="section-heading row">
                <div>
                  <h3>链式组</h3>
                  <p>relay_through 派生的链式分组。</p>
                </div>
                <Chip>{result.chained_groups.length}</Chip>
              </div>
              <div className="runtime-group-list">
                {result.chained_groups.map((group) => (
                  <article key={group.name} className="runtime-group-card">
                    <header>
                      <strong>{group.name}</strong>
                      <Chip tone={group.strategy === "url-test" ? "success" : "info"}>{group.strategy}</Chip>
                      <Chip>{group.members.length} members</Chip>
                    </header>
                    <MemberList members={group.expanded_members ?? group.members} />
                  </article>
                ))}
              </div>
            </section>
          ) : null}
        </>
      ) : null}
    </div>
  );
}

function GroupPreviewCard({ group }: { group: PreviewGroup }) {
  const members = group.expanded_members ?? group.members;
  return (
    <article className="group-preview-card">
      <header>
        <h3>{group.name}</h3>
        <Chip>{members.length}</Chip>
        <Chip className="group-strategy-chip" tone={group.strategy === "url-test" ? "success" : "info"}>{group.strategy}</Chip>
      </header>
      {group.match ? <code className="group-regex-block">{group.match}</code> : null}
      <div className="group-matched-list">
        {members.map((member, index) => (
          <span key={memberKey(member, index)}>{member.name}</span>
        ))}
        {members.length === 0 ? <span style={{ color: "var(--error)" }}>未匹配到任何节点</span> : null}
      </div>
    </article>
  );
}

function ServiceGroupRow({ group }: { group: PreviewGroup }) {
  const members = group.expanded_members ?? group.members;
  return (
    <div className="service-group-row">
      <div className="service-group-meta">
        <strong>{group.name}</strong>
        <small>{group.members.length} 成员 → {members.length} 节点</small>
      </div>
      <div className="member-chip-list">
        {members.slice(0, 6).map((member, index) => (
          <Chip key={memberKey(member, index)} tone={memberOriginTone(member.origin)}>{member.name}</Chip>
        ))}
        {members.length > 6 ? <Chip>+{members.length - 6}</Chip> : null}
      </div>
    </div>
  );
}

function MemberList({ members }: { members: PreviewMember[] }) {
  return (
    <ul className="member-tree">
      {members.map((member, index) => (
        <li key={memberKey(member, index)}>
          <span>{member.name}</span>
          <Chip tone={memberOriginTone(member.origin)}>{member.origin ?? member.kind}</Chip>
        </li>
      ))}
    </ul>
  );
}

function memberKey(member: PreviewMember, index: number) {
  return member.id ?? `${member.kind}-${member.name}-${index}`;
}

function memberOriginTone(origin?: string): "neutral" | "accent" | "success" | "warning" | "info" {
  if (origin === "all_expanded") return "accent";
  if (origin === "auto_expanded") return "success";
  if (origin === "all_proxies") return "warning";
  return "info";
}
