import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import type { OrderedEntry } from "../api/types";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, Field, IconButton, RailPanel, SplitWorkbench, TextInput } from "../components/ui";
import { getRoutingMemberOptions } from "../features/configModel";
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
  const [selectedIndex, setSelectedIndex] = useState(0);
  const routing = draft?.routing ?? [];
  const groups = draft?.groups ?? [];
  const memberOptions = getRoutingMemberOptions(draft ?? {});
  const activeIndex = routing.length === 0 ? -1 : Math.min(selectedIndex, routing.length - 1);
  const activeRoute = activeIndex >= 0 ? routing[activeIndex] : undefined;

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

  function addMember(member: string) {
    if (!activeRoute || isReadonly) return;
    patchEntry(activeIndex, { value: [...activeRoute.value, member] });
  }

  function removeMember(memberIndex: number) {
    if (!activeRoute || isReadonly) return;
    patchEntry(activeIndex, { value: activeRoute.value.filter((_, index) => index !== memberIndex) });
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
    setSelectedIndex(Math.max(0, index - 1));
  }

  return (
    <SplitWorkbench
      rail={
        <RailPanel eyebrow="Members" title="可选成员">
          <div className="palette-section">
            <span>特殊关键字</span>
            <div className="member-palette-grid">
              {specialMembers.map((member) => (
                <button key={member.value} type="button" disabled={isReadonly || !activeRoute} onClick={() => addMember(member.value)}>
                  <strong>{member.value}</strong>
                  <small>{member.desc}</small>
                </button>
              ))}
            </div>
          </div>
          <div className="palette-section">
            <span>节点分组（{groups.length}）</span>
            <div className="palette-list">
              {groups.map((group) => (
                <button key={group.key} type="button" disabled={isReadonly || !activeRoute} onClick={() => addMember(group.key)}>
                  <span>{group.key}</span>
                  <small>{group.value.strategy}</small>
                </button>
              ))}
            </div>
          </div>
          <div className="palette-section">
            <span>服务组引用</span>
            <div className="palette-list">
              {routing.filter((_, index) => index !== activeIndex).map((route) => (
                <button key={route.key} type="button" disabled={isReadonly || !activeRoute} onClick={() => addMember(route.key)}>
                  <span>{route.key || "未命名服务组"}</span>
                  <small>{route.value.length} 成员</small>
                </button>
              ))}
            </div>
          </div>
          <div className="constraint-hint">
            <strong>约束提示</strong>
            <p>@all 和 @auto 不能同时出现</p>
            <p>@auto 每组最多一个</p>
            <p>REJECT 不会包含在 @auto 里</p>
          </div>
        </RailPanel>
      }
    >
      <div className="page-stack">
        <div className="group-toolbar">
          <Button variant="secondary" icon={<Plus size={16} aria-hidden="true" />} disabled={isReadonly} onClick={addServiceGroup}>
            新增服务组
          </Button>
          <span>共 {routing.length} 个服务组 · 成员可引用节点组、服务组、DIRECT、REJECT、@all、@auto</span>
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
              <article className={index === activeIndex ? "routing-card active" : "routing-card"} onClick={() => setSelectedIndex(index)}>
                <div className="routing-card-header">
                  <div className="row-title">
                    {handle}
                    <code>{String(index + 1).padStart(2, "0")}</code>
                    <strong>{entry.key || `服务组 #${index + 1}`}</strong>
                    <Chip>{entry.value.length} 个成员</Chip>
                  </div>
                  <div className="source-card-actions">
                    <IconButton label="编辑服务组" variant="ghost" disabled={isReadonly} onClick={() => setSelectedIndex(index)}>
                      <Pencil size={15} aria-hidden="true" />
                    </IconButton>
                    <IconButton label="删除服务组" variant="danger" disabled={isReadonly} onClick={() => void deleteRoute(index)}>
                      <Trash2 size={15} aria-hidden="true" />
                    </IconButton>
                  </div>
                </div>
                {index === activeIndex ? (
                  <Field label="服务组名">
                    <TextInput value={entry.key} disabled={isReadonly} onChange={(event) => patchEntry(index, { key: event.target.value })} />
                  </Field>
                ) : null}
                <div className="member-chip-list">
                  {entry.value.length === 0 ? <p className="muted">暂无成员</p> : null}
                  {entry.value.map((member, memberIndex) => (
                    <Chip key={`${member}-${memberIndex}`} tone={memberTone(member)} removable={!isReadonly && index === activeIndex} onRemove={() => removeMember(memberIndex)}>
                      {member}
                    </Chip>
                  ))}
                  {index === activeIndex ? (
                    <button type="button" className="add-chip" disabled={isReadonly || memberOptions.length === 0} onClick={() => addMember(memberOptions[0] ?? "@auto")}>
                      <Plus size={13} aria-hidden="true" />
                      添加成员
                    </button>
                  ) : null}
                </div>
              </article>
            )}
          />
        )}
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
