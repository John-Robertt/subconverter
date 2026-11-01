import { useMutation } from "@tanstack/react-query";
import { Plus, Search, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import type { Config, Diagnostic, GroupConfig, OrderedEntry, PreviewGroup } from "../api/types";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { DiagnosticList } from "../components/Diagnostics";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, ErrorState, Field, IconButton, LoadingState, RailPanel, SplitWorkbench, TextInput } from "../components/ui";
import { focusClassName, getValidateResult, useDiagnosticPointer } from "../features/diagnostics";
import { useConfigState } from "../state/config";
import { isConfigChanged } from "../features/configModel";
import { useRegexValidation } from "../features/useRegexValidation";
import { useConfirm } from "../state/confirm";

export function GroupsPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [selectedIndex, setSelectedIndex] = useState(0);
  const groups = draft?.groups ?? [];
  const activeIndex = groups.length === 0 ? -1 : Math.min(selectedIndex, groups.length - 1);
  const activeGroup = activeIndex >= 0 ? groups[activeIndex] : undefined;
  const regexState = useRegexValidation(draft, `/config/groups/${activeIndex}/value/match`, activeGroup?.value.match ?? "");
  const previewMutation = useMutation({ mutationFn: (config: Config) => api.previewGroupsDraft(config) });
  const [renameOrigin, setRenameOrigin] = useState<string | null>(null);
  const references = renameOrigin && renameOrigin !== activeGroup?.key ? [
    ...(draft?.routing ?? []).filter((entry) => entry.value.includes(renameOrigin)).map((entry) => `服务组 ${entry.key}`),
    ...(draft?.sources?.custom_proxies ?? []).filter((proxy) => proxy.relay_through?.type === "group" && proxy.relay_through.name === renameOrigin).map((proxy) => `链式代理 ${proxy.name}`)
  ] : [];
  useEffect(() => {
    const match = activePointer?.match(/^\/config\/groups\/(\d+)/);
    if (match) {
      setSelectedIndex(Number(match[1]));
    }
  }, [activePointer]);

  function setGroups(nextGroups: OrderedEntry<GroupConfig>[]) {
    updateDraft((config) => ({ ...config, groups: nextGroups }));
  }

  function reorderGroups(nextGroups: OrderedEntry<GroupConfig>[]) {
    if (activeGroup) setSelectedIndex(nextGroups.indexOf(activeGroup));
    setGroups(nextGroups);
  }

  function selectGroup(index: number) { setSelectedIndex(index); setRenameOrigin(null); }

  function updateReferences() {
    if (!renameOrigin || !activeGroup?.key.trim()) return;
    const name = activeGroup.key;
    updateDraft((config) => ({
      ...config,
      routing: config.routing?.map((entry) => ({ ...entry, value: entry.value.map((member) => member === renameOrigin ? name : member) })),
      sources: { ...config.sources, custom_proxies: config.sources?.custom_proxies?.map((proxy) =>
        proxy.relay_through?.type === "group" && proxy.relay_through.name === renameOrigin
          ? { ...proxy, relay_through: { ...proxy.relay_through, name } } : proxy) }
    }));
    setRenameOrigin(null);
  }

  function patchGroup(index: number, patch: Partial<OrderedEntry<GroupConfig>>) {
    setGroups(groups.map((group, groupIndex) => (groupIndex === index ? { ...group, ...patch } : group)));
  }

  function addGroup() {
    setGroups([...groups, { key: "", value: { match: "", strategy: "select" } }]);
    setSelectedIndex(groups.length);
  }

  async function deleteGroup(index: number) {
    const accepted = await confirm({
      title: "删除节点组？",
      message: `即将删除「${groups[index]?.key || `节点组 #${index + 1}`}」。引用它的路由策略需要另行检查。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    setGroups(groups.filter((_, groupIndex) => groupIndex !== index));
    setSelectedIndex(Math.max(0, index - 1));
  }

  const preview = previewMutation.data;
  const stale = Boolean(preview && isConfigChanged(draft, previewMutation.variables));
  const previewDiagnostics = getValidateResult(previewMutation.error);
  const activePreviewGroup = preview?.node_groups.find((group) => group.name === activeGroup?.key);

  return (
    <SplitWorkbench
      rail={
        <RailPanel eyebrow="Preview" title="草稿分组预览">
          {stale ? <p className="info-panel" role="status">草稿已修改，以下是上次预览结果，请重新预览。</p> : null}
          {previewMutation.isPending ? <LoadingState message="正在拉取订阅并计算草稿分组" /> : null}
          {previewMutation.error ? (
            previewDiagnostics ? (
              <DiagnosticList result={previewDiagnostics} onLocate={(diagnostic) => locateDiagnostic(diagnostic, setSelectedIndex)} />
            ) : (
              <ErrorState message={getErrorMessage(previewMutation.error)} />
            )
          ) : null}
          {preview ? (
            <div className="group-preview-stack">
              <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
                <strong style={{ fontSize: 32, fontWeight: 600, color: activePreviewGroup ? "var(--primary)" : "var(--error)", letterSpacing: "-0.02em" }}>{activePreviewGroup?.members.length ?? 0}</strong>
                <span style={{ fontSize: 13, color: "var(--text-muted)" }}>个节点 / {preview.node_groups.reduce((sum, g) => sum + g.members.length, 0)} 总数</span>
              </div>
              {activePreviewGroup ? (
                <div style={{ height: 4, borderRadius: 2, background: "var(--surface-muted)", overflow: "hidden" }}>
                  <div style={{ height: "100%", width: `${Math.min(100, (activePreviewGroup.members.length / Math.max(1, preview.node_groups.reduce((sum, g) => sum + g.members.length, 0))) * 100)}%`, background: "var(--primary)", transition: "width 0.3s" }} />
                </div>
              ) : null}
              <PreviewGroupList title="节点组" groups={preview.node_groups} />
              <PreviewGroupList title="链式组" groups={preview.chained_groups} />
              <PreviewGroupList title="服务组" groups={preview.service_groups} />
            </div>
          ) : (
            !previewMutation.isPending && !previewMutation.error && <EmptyState title="尚未运行分组预览" message="点击草稿分组预览后，这里会展示 node_groups / chained_groups / service_groups。" />
          )}
        </RailPanel>
      }
    >
      <div className="page-stack">
        <div className="group-toolbar">
          <Button variant="secondary" icon={<Search size={16} aria-hidden="true" />} loading={previewMutation.isPending} disabled={!draft || regexState.pending || regexState.invalid} onClick={() => draft && previewMutation.mutate(draft)}>
            草稿分组预览
          </Button>
          <span>共 {groups.length} 个节点组 · 拖拽调整顺序</span>
        </div>

        {groups.length === 0 ? (
          <div className="group-pill-row">
            <button type="button" className="add-chip" disabled={isReadonly} onClick={addGroup}>
              <Plus size={13} aria-hidden="true" />
              新建分组
            </button>
            <span className="muted" style={{ fontSize: 12 }}>{isReadonly ? "只读模式下不可新增节点组。" : "至少添加一个地区节点组后，路由策略才能引用。"}</span>
          </div>
        ) : (
          <div className="group-pill-row">
            <SortableList
              items={groups}
              getId={(item, index) => `${item.key || "group"}-${index}`}
              disabled={isReadonly}
              onReorder={reorderGroups}
              renderItem={(group, index, handle) => (
                <div
                  className={focusClassName(activePointer, [`/config/groups/${index}`], index === activeIndex ? "group-pill active" : "group-pill")}
                  role="button"
                  tabIndex={0}
                  onClick={() => selectGroup(index)}
                  onKeyDown={(event) => event.target === event.currentTarget && (event.key === "Enter" || event.key === " ") && selectGroup(index)}
                >
                  {handle}
                  <span>{group.key || `节点组 #${index + 1}`}</span>
                  <Chip tone={group.value.strategy === "url-test" ? "success" : "info"}>{group.value.strategy}</Chip>
                </div>
              )}
            />
            <button type="button" className="add-chip-large" disabled={isReadonly} onClick={addGroup}>
              <Plus size={14} aria-hidden="true" />
              新建分组
            </button>
          </div>
        )}

        {activeGroup ? (
          <section className={focusClassName(activePointer, [`/config/groups/${activeIndex}`], "content-panel editor-panel")}>
            <div className="section-heading row">
              <div>
                <h3>编辑分组</h3>
                <p>用正则匹配节点名，组成可被路由策略引用的逻辑分组。</p>
              </div>
              <IconButton label="删除节点组" variant="danger" disabled={isReadonly} onClick={() => void deleteGroup(activeIndex)}>
                <Trash2 size={16} aria-hidden="true" />
              </IconButton>
            </div>
            <div className="form-grid two">
              <div>
              <Field label="分组名称" hint="支持 emoji 前缀">
                <TextInput data-config-pointer={`/config/groups/${activeIndex}/key`} value={activeGroup.key} disabled={isReadonly} onChange={(event) => { setRenameOrigin((current) => current ?? activeGroup.key); patchGroup(activeIndex, { key: event.target.value }); }} />
              </Field>
                {references.length ? <span className="field-error" role="alert">{references.join("、")} 仍引用「{renameOrigin}」，请同步修改引用后再保存。</span> : null}
                {references.length ? <Button disabled={isReadonly || !activeGroup.key.trim()} onClick={updateReferences}>同步修改引用</Button> : null}
              </div>
              <Field label="匹配正则" hint={regexState.pending ? "正在检查正则…" : "使用后端 Go 正则语法检查；点击预览才会拉取订阅。"} error={regexState.message}>
                <TextInput
                  className="text-input mono-input"
                  data-config-pointer={`/config/groups/${activeIndex}/value/match`}
                  value={activeGroup.value.match}
                  disabled={isReadonly}
                  onChange={(event) => patchGroup(activeIndex, { value: { ...activeGroup.value, match: event.target.value } })}
                />
              </Field>
            </div>
            <Field label="路由策略">
              <div className="strategy-grid">
                {(["select", "url-test"] as const).map((strategy) => (
                  <button
                    key={strategy}
                    type="button"
                    className={activeGroup.value.strategy === strategy ? "strategy-card active" : "strategy-card"}
                    disabled={isReadonly}
                    onClick={() => patchGroup(activeIndex, { value: { ...activeGroup.value, strategy } })}
                  >
                    <strong>{strategy}</strong>
                    <span>{strategy === "select" ? "手动从分组中选择节点" : "自动测速选择延迟最低的节点"}</span>
                  </button>
                ))}
              </div>
            </Field>
          </section>
        ) : null}
      </div>
    </SplitWorkbench>
  );
}

function locateDiagnostic(diagnostic: Diagnostic, setSelectedIndex: (index: number) => void) {
  const pointer = diagnostic.locator?.json_pointer ?? "";
  const match = pointer.match(/^\/config\/groups\/(\d+)(?:\/|$)/);
  if (match) {
    setSelectedIndex(Number(match[1]));
  }
}

function PreviewGroupList({ title, groups }: { title: string; groups: PreviewGroup[] }) {
  return (
    <div className="preview-column compact">
      <h4>{title}</h4>
      {groups.length === 0 ? (
        <p className="muted">为空</p>
      ) : (
        <ul>
          {groups.slice(0, 6).map((group) => (
            <li key={group.name}>
              <strong>{group.name}</strong>
              <span>{group.strategy} · {group.members.length}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
