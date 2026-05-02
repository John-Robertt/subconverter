import { useQuery } from "@tanstack/react-query";
import { RefreshCcw } from "lucide-react";
import { useMemo, useState } from "react";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { queryKeys } from "../app/queryKeys";
import { Button, CategoryPill, Chip, EmptyState, ErrorState, Field, LoadingState, SelectInput, TextInput } from "../components/ui";
import { useConfigState } from "../state/config";

export function NodesPage() {
  const { status } = useConfigState();
  const [kindFilter, setKindFilter] = useState("all");
  const [typeFilter, setTypeFilter] = useState("all");
  const [nameFilter, setNameFilter] = useState("");
  const runtimeRevision = status?.runtime_config_revision;

  const nodesQuery = useQuery({
    queryKey: queryKeys.previewNodes(runtimeRevision),
    queryFn: api.previewNodes,
    enabled: Boolean(runtimeRevision)
  });

  const nodes = nodesQuery.data?.nodes ?? [];
  const kinds = useMemo(() => uniqueValues(nodes.map((node) => node.kind)), [nodes]);
  const types = useMemo(() => uniqueValues(nodes.map((node) => node.type)), [nodes]);
  const categories = useMemo(
    () => [
      { id: "all", label: "全部", count: nodes.length },
      ...kinds.map((kind) => ({ id: kind, label: kindLabel(kind), count: nodes.filter((node) => node.kind === kind).length }))
    ],
    [kinds, nodes]
  );

  const filteredNodes = useMemo(() => {
    return nodes.filter((node) => {
      const kindMatches = kindFilter === "all" || node.kind === kindFilter;
      const typeMatches = typeFilter === "all" || node.type === typeFilter;
      const nameMatches = node.name.toLowerCase().includes(nameFilter.trim().toLowerCase());
      return kindMatches && typeMatches && nameMatches;
    });
  }, [kindFilter, nameFilter, nodes, typeFilter]);

  return (
    <div className="page-stack">
      {status?.config_dirty ? (
        <section className="content-panel info-panel">
          当前预览仍基于运行时配置，不等于已保存但尚未 reload 的草稿。
        </section>
      ) : null}

      <section className="node-toolbar">
        <div className="category-row">
          {categories.map((category) => (
            <CategoryPill
              key={category.id}
              label={category.label}
              count={category.count}
              tag={category.id === "snell" ? "Surge only" : category.id === "vless" ? "Clash only" : undefined}
              active={kindFilter === category.id}
              onClick={() => setKindFilter(category.id)}
            />
          ))}
        </div>
        <Button variant="secondary" icon={<RefreshCcw size={16} aria-hidden="true" />} loading={nodesQuery.isFetching} onClick={() => void nodesQuery.refetch()}>
          重新拉取
        </Button>
      </section>

      <section className="content-panel dense-panel">
        <div className="filter-row compact">
          <Field label="Type">
            <SelectInput value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
              <option value="all">全部</option>
              {types.map((type) => (
                <option key={type} value={type}>{type}</option>
              ))}
            </SelectInput>
          </Field>
          <Field label="名称">
            <TextInput value={nameFilter} onChange={(event) => setNameFilter(event.target.value)} placeholder="搜索节点名" />
          </Field>
          <div className="node-revision">
            <span>Runtime revision</span>
            <code>{runtimeRevision ?? "-"}</code>
          </div>
        </div>

        {nodesQuery.isLoading ? <LoadingState message="正在拉取订阅并生成运行时节点预览" /> : null}
        {nodesQuery.error ? <ErrorState message={getErrorMessage(nodesQuery.error)} /> : null}
        {nodesQuery.data && filteredNodes.length === 0 ? <EmptyState title="没有匹配节点" message="可能来源为空、全部被过滤，或当前筛选条件过窄。" /> : null}
        {filteredNodes.length > 0 ? (
          <div className="table-wrap node-table">
            <table>
              <thead>
                <tr>
                  <th />
                  <th>名称</th>
                  <th>类型</th>
                  <th>服务器</th>
                  <th>端口</th>
                  <th>来源</th>
                  <th>标签</th>
                </tr>
              </thead>
              <tbody>
                {filteredNodes.map((node) => (
                  <tr key={`${node.kind}-${node.type}-${node.name}`}>
                    <td><span className={node.filtered ? "node-dot filtered" : "node-dot"} aria-hidden="true" /></td>
                    <td>{node.name}</td>
                    <td><code>{node.type}</code></td>
                    <td><code>{node.server ?? "-"}</code></td>
                    <td><code>{node.port ?? "-"}</code></td>
                    <td><Chip>{node.kind}</Chip></td>
                    <td>
                      {node.kind === "snell" ? <Chip tone="warning">Surge</Chip> : null}
                      {node.kind === "vless" ? <Chip tone="info">Clash</Chip> : null}
                      {node.filtered ? <Chip tone="error">filtered</Chip> : <Chip tone="success">active</Chip>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </div>
  );
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values)).sort((a, b) => a.localeCompare(b));
}

function kindLabel(kind: string) {
  if (kind === "subscription" || kind === "sub") return "订阅";
  if (kind === "snell") return "Snell";
  if (kind === "vless") return "VLESS";
  if (kind === "custom" || kind === "custom_proxy") return "自定义";
  return kind;
}
