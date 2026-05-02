import { useMutation } from "@tanstack/react-query";
import { Plus, Search, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { Diagnostic, GroupConfig, OrderedEntry } from "../api/types";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { DiagnosticList } from "../components/Diagnostics";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, ErrorState, Field, IconButton, LoadingState, RailPanel, SelectInput, SplitWorkbench, StatCard, TextInput } from "../components/ui";
import { focusClassName, getValidateResult, useDiagnosticPointer } from "../features/diagnostics";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

export function GroupsPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [selectedIndex, setSelectedIndex] = useState(0);
  const groups = draft?.groups ?? [];
  const activeIndex = groups.length === 0 ? -1 : Math.min(selectedIndex, groups.length - 1);
  const activeGroup = activeIndex >= 0 ? groups[activeIndex] : undefined;
  const regexState = useMemo(() => {
    const match = activeGroup?.value.match ?? "";
    if (!match) return { valid: true, message: "" };
    try {
      new RegExp(match);
      return { valid: true, message: "" };
    } catch (error) {
      return { valid: false, message: error instanceof Error ? error.message : "正则语法错误" };
    }
  }, [activeGroup?.value.match]);
  const previewMutation = useMutation({
    mutationFn: () => {
      if (!draft) throw new Error("配置尚未加载");
      return api.previewGroupsDraft(draft);
    }
  });

  useEffect(() => {
    const match = activePointer?.match(/^\/config\/groups\/(\d+)/);
    if (match) {
      setSelectedIndex(Number(match[1]));
    }
  }, [activePointer]);

  function setGroups(nextGroups: OrderedEntry<GroupConfig>[]) {
    updateDraft((config) => ({ ...config, groups: nextGroups }));
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
  const previewDiagnostics = getValidateResult(previewMutation.error);
  const activePreviewGroup = preview?.node_groups.find((group) => group.name === activeGroup?.key);

  return (
    <SplitWorkbench
      rail={
        <RailPanel eyebrow="Preview" title="草稿分组预览">
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
              <StatCard label="当前匹配" value={activePreviewGroup?.members.length ?? 0} sub={activeGroup?.key || "未选择"} tone={activePreviewGroup ? "success" : "warning"} />
              <PreviewGroupList title="节点组" groups={preview.node_groups} />
              <PreviewGroupList title="链式组" groups={preview.chained_groups} />
              <PreviewGroupList title="服务组" groups={preview.service_groups} />
            </div>
          ) : (
            !previewMutation.isPending && <EmptyState title="尚未运行分组预览" message="点击草稿分组预览后，这里会展示 node_groups / chained_groups / service_groups。" />
          )}
        </RailPanel>
      }
    >
      <div className="page-stack">
        <div className="group-toolbar">
          <Button variant="secondary" icon={<Plus size={16} aria-hidden="true" />} disabled={isReadonly} onClick={addGroup}>
            新增组
          </Button>
          <Button variant="secondary" icon={<Search size={16} aria-hidden="true" />} loading={previewMutation.isPending} disabled={!draft} onClick={() => previewMutation.mutate()}>
            草稿分组预览
          </Button>
          <span>共 {groups.length} 个节点组 · 拖拽调整顺序</span>
        </div>

        {groups.length === 0 ? (
          <EmptyState title="暂无节点组" message={isReadonly ? "只读模式下不可新增节点组。" : "至少添加一个地区节点组后，路由策略才能引用。"} />
        ) : (
          <SortableList
            items={groups}
            getId={(item, index) => `${item.key || "group"}-${index}`}
            disabled={isReadonly}
            onReorder={setGroups}
            renderItem={(group, index, handle) => (
              <div
                className={focusClassName(activePointer, [`/config/groups/${index}`], index === activeIndex ? "group-pill active" : "group-pill")}
                role="button"
                tabIndex={0}
                onClick={() => setSelectedIndex(index)}
                onKeyDown={(event) => (event.key === "Enter" || event.key === " ") && setSelectedIndex(index)}
              >
                {handle}
                <span>{group.key || `节点组 #${index + 1}`}</span>
                <Chip tone={group.value.strategy === "url-test" ? "success" : "info"}>{group.value.strategy}</Chip>
              </div>
            )}
          />
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
              <Field label="分组名称" hint="支持 emoji 前缀">
                <TextInput value={activeGroup.key} disabled={isReadonly} onChange={(event) => patchGroup(activeIndex, { key: event.target.value })} />
              </Field>
              <Field label="匹配正则" hint="本地校验语法；点击预览才会拉取订阅。" error={regexState.valid ? undefined : regexState.message}>
                <TextInput
                  className="text-input mono-input"
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

function PreviewGroupList({ title, groups }: { title: string; groups: { name: string; strategy: string; members: string[] }[] }) {
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
