import { Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import type { OrderedEntry } from "../api/types";
import { SortableList } from "../components/SortableList";
import { Chip, EmptyState, Field, IconButton, RailPanel, SplitWorkbench, TextInput } from "../components/ui";
import { getRoutingMemberOptions } from "../features/configModel";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

const specialMembers = [
  { value: "@all", desc: "所有节点" },
  { value: "@auto", desc: "自动选择子组" },
  { value: "DIRECT", desc: "直连" },
  { value: "REJECT", desc: "拒绝" }
];

export function RoutingPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  // null = no service group selected (toggleable on click)
  const [selectedIndex, setSelectedIndex] = useState<number | null>(0);
  const routing = draft?.routing ?? [];
  const groups = draft?.groups ?? [];
  const memberOptions = getRoutingMemberOptions(draft ?? {});
  const activeIndex =
    selectedIndex === null || routing.length === 0
      ? null
      : Math.min(selectedIndex, routing.length - 1);
  const activeRoute = activeIndex !== null ? routing[activeIndex] : undefined;

  useEffect(() => {
    const match = activePointer?.match(/^\/config\/routing\/(\d+)/);
    if (match) {
      setSelectedIndex(Number(match[1]));
    }
  }, [activePointer]);

  function setRouting(nextRouting: OrderedEntry<string[]>[]) {
    updateDraft((config) => ({ ...config, routing: nextRouting }));
  }

  function patchEntry(index: number, patch: Partial<OrderedEntry<string[]>>) {
    setRouting(routing.map((entry, entryIndex) => (entryIndex === index ? { ...entry, ...patch } : entry)));
  }

  function addServiceGroup() {
    setRouting([...routing, { key: "", value: ["@auto"] }]);
    setSelectedIndex(routing.length);
  }

  function toggleSelection(index: number) {
    setSelectedIndex((current) => (current === index ? null : index));
  }

  function addMember(member: string) {
    if (activeIndex === null || !activeRoute || isReadonly) return;
    patchEntry(activeIndex, { value: [...activeRoute.value, member] });
  }

  function removeMember(memberIndex: number) {
    if (activeIndex === null || !activeRoute || isReadonly) return;
    patchEntry(activeIndex, { value: activeRoute.value.filter((_, index) => index !== memberIndex) });
  }

  function toggleMember(member: string) {
    if (activeIndex === null || !activeRoute || isReadonly) return;
    const index = activeRoute.value.indexOf(member);
    if (index === -1) {
      patchEntry(activeIndex, { value: [...activeRoute.value, member] });
    } else {
      patchEntry(activeIndex, { value: activeRoute.value.filter((_, i) => i !== index) });
    }
  }

  async function deleteRoute(index: number) {
    const accepted = await confirm({
      title: "删除服务组？",
      message: `即将删除「${routing[index]?.key || `服务组 #${index + 1}`}」。引用它的规则集或 fallback 需要另行检查。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    setRouting(routing.filter((_, routingIndex) => routingIndex !== index));
    setSelectedIndex((current) => {
      if (current === null) return null;
      if (current === index) return null;
      return current > index ? current - 1 : current;
    });
  }

  return (
    <SplitWorkbench
      rail={
        <RailPanel eyebrow={activeRoute ? "Editing" : "Members"} title={activeRoute ? "编辑服务组" : "选择服务组"}>
          {activeRoute && activeIndex !== null ? (
            <>
              <Field label="服务组名称" hint="支持 emoji 前缀；保存后写回 YAML 的 routing 段。">
                <TextInput
                  value={activeRoute.key}
                  placeholder={`服务组 #${activeIndex + 1}`}
                  disabled={isReadonly}
                  onChange={(event) => patchEntry(activeIndex, { key: event.target.value })}
                />
              </Field>

              <div className="palette-section">
                <span>当前成员（{activeRoute.value.length}）</span>
                <div className="member-chip-list">
                  {activeRoute.value.length === 0 ? <p className="muted">暂无成员</p> : null}
                  {activeRoute.value.map((member, memberIndex) => (
                    <Chip
                      key={`${member}-${memberIndex}`}
                      tone={memberTone(member)}
                      removable={!isReadonly}
                      onRemove={() => removeMember(memberIndex)}
                    >
                      {member}
                    </Chip>
                  ))}
                </div>
              </div>

              <div className="palette-section">
                <span>特殊关键字</span>
                <div className="member-palette-grid">
                  {specialMembers.map((member) => {
                    const selected = activeRoute.value.includes(member.value);
                    return (
                      <button
                        key={member.value}
                        type="button"
                        className={paletteClass(selected, memberTone(member.value))}
                        aria-pressed={selected}
                        disabled={isReadonly}
                        onClick={() => toggleMember(member.value)}
                      >
                        <strong>{member.value}</strong>
                        <small>{member.desc}</small>
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="palette-section">
                <span>节点分组（{groups.length}）</span>
                <div className="palette-list">
                  {groups.length === 0 ? <p className="muted">尚未配置节点分组</p> : null}
                  {groups.map((group) => {
                    const selected = activeRoute.value.includes(group.key);
                    return (
                      <button
                        key={group.key}
                        type="button"
                        className={paletteClass(selected, memberTone(group.key))}
                        aria-pressed={selected}
                        disabled={isReadonly}
                        onClick={() => toggleMember(group.key)}
                      >
                        <span>{group.key}</span>
                        <small>{group.value.strategy}</small>
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="palette-section">
                <span>服务组引用</span>
                <div className="palette-list">
                  {routing.filter((_, index) => index !== activeIndex).length === 0 ? (
                    <p className="muted">无其它服务组可引用</p>
                  ) : null}
                  {routing
                    .filter((_, index) => index !== activeIndex)
                    .map((route) => {
                      const selected = activeRoute.value.includes(route.key);
                      return (
                        <button
                          key={route.key}
                          type="button"
                          className={paletteClass(selected, memberTone(route.key))}
                          aria-pressed={selected}
                          disabled={isReadonly}
                          onClick={() => toggleMember(route.key)}
                        >
                          <span>{route.key || "未命名服务组"}</span>
                          <small>{route.value.length} 成员</small>
                        </button>
                      );
                    })}
                </div>
              </div>

              <div className="constraint-hint">
                <strong>约束提示</strong>
                <p>@all 和 @auto 不能同时出现</p>
                <p>@auto 每组最多一个</p>
                <p>REJECT 不会包含在 @auto 里</p>
              </div>
            </>
          ) : (
            <EmptyState
              title="未选中服务组"
              message="点击左侧任一服务组卡片进行编辑；再次点击同一卡片可以取消选中。"
            />
          )}
        </RailPanel>
      }
    >
      <div className="page-stack">
        <div className="group-toolbar">
          <span>共 {routing.length} 个服务组 · 点击卡片切换选中，编辑面板在右侧</span>
          {memberOptions.length > 0 ? <span className="muted" style={{ fontSize: 12 }}>{memberOptions.length} 个候选成员</span> : null}
        </div>

        {routing.length === 0 ? (
          <EmptyState title="暂无路由策略" message={isReadonly ? "只读模式下不可新增服务组。" : "添加服务组后，可在 rulesets/rules/fallback 中引用。"} />
        ) : (
          <SortableList
            items={routing}
            getId={(item, index) => `${item.key || "routing"}-${index}`}
            disabled={isReadonly}
            onReorder={setRouting}
            renderItem={(entry, index, handle) => (
              <article
                className={focusClassName(activePointer, [`/config/routing/${index}`], index === activeIndex ? "routing-card active" : "routing-card")}
                onClick={() => toggleSelection(index)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    toggleSelection(index);
                  }
                }}
              >
                <div className="routing-card-header">
                  <div className="row-title">
                    {handle}
                    <code className="routing-card-index">{String(index + 1).padStart(2, "0")}</code>
                    <strong>{entry.key || `服务组 #${index + 1}`}</strong>
                    <Chip>{entry.value.length} 个成员</Chip>
                    {index === activeIndex ? <Chip tone="accent">已选中</Chip> : null}
                  </div>
                  <div className="source-card-actions">
                    <IconButton
                      label="删除服务组"
                      variant="danger"
                      disabled={isReadonly}
                      onClick={(event) => {
                        event.stopPropagation();
                        void deleteRoute(index);
                      }}
                    >
                      <Trash2 size={15} aria-hidden="true" />
                    </IconButton>
                  </div>
                </div>
                <div className="member-chip-list readonly">
                  {entry.value.length === 0 ? <p className="muted">暂无成员</p> : null}
                  {entry.value.map((member, memberIndex) => (
                    <Chip key={`${member}-${memberIndex}`} tone={memberTone(member)}>
                      {member}
                    </Chip>
                  ))}
                </div>
              </article>
            )}
          />
        )}

        <button type="button" className="add-dashed-block" disabled={isReadonly} onClick={addServiceGroup}>
          <Plus size={15} aria-hidden="true" />
          新建服务组
        </button>
      </div>
    </SplitWorkbench>
  );
}

function memberTone(member: string): "neutral" | "accent" | "success" | "error" | "info" {
  if (member === "DIRECT") return "success";
  if (member === "REJECT") return "error";
  if (member.startsWith("@")) return "accent";
  return "info";
}

function paletteClass(selected: boolean, tone: ReturnType<typeof memberTone>): string {
  return [`palette-tone-${tone}`, selected ? "is-selected" : ""].filter(Boolean).join(" ");
}
