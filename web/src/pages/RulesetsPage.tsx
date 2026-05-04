import { Plus, Trash2 } from "lucide-react";
import { useMemo } from "react";
import type { OrderedEntry } from "../api/types";
import { Button, Chip, EmptyState, Field, IconButton, SelectInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { getPolicyOptions } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

export function RulesetsPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const rulesets = draft?.rulesets ?? [];
  const policyOptions = useMemo(() => getPolicyOptions(draft ?? {}), [draft]);

  function setRulesets(nextRulesets: OrderedEntry<string[]>[]) {
    updateDraft((config) => ({ ...config, rulesets: nextRulesets }));
  }

  function patchEntry(index: number, patch: Partial<OrderedEntry<string[]>>) {
    setRulesets(rulesets.map((entry, entryIndex) => (entryIndex === index ? { ...entry, ...patch } : entry)));
  }

  function patchUrls(index: number, urls: string[]) {
    patchEntry(index, { value: urls });
  }

  function addRuleset() {
    const used = new Set(rulesets.map((entry) => entry.key));
    const nextPolicy = policyOptions.find((policy) => !used.has(policy)) ?? "";
    setRulesets([...rulesets, { key: nextPolicy, value: [] }]);
  }

  async function deleteRuleset(index: number) {
    const accepted = await confirm({
      title: "删除规则集绑定？",
      message: `即将删除「${rulesets[index]?.key || `规则集 #${index + 1}`}」及其 URL 列表。该操作只影响当前草稿。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    setRulesets(rulesets.filter((_, rulesetIndex) => rulesetIndex !== index));
  }

  async function deleteUrl(entryIndex: number, urlIndex: number) {
    const accepted = await confirm({
      title: "删除规则集 URL？",
      message: `即将删除第 ${urlIndex + 1} 条 URL。规则集 URL 顺序会在保存后写回 YAML。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    patchUrls(entryIndex, rulesets[entryIndex].value.filter((_, index) => index !== urlIndex));
  }

  return (
    <div className="page-stack">
      <div className="group-toolbar">
        <Button variant="secondary" icon={<Plus size={16} aria-hidden="true" />} disabled={isReadonly} onClick={addRuleset}>
          新增规则集
        </Button>
        <span>共 {rulesets.length} 个服务组绑定 · 拖拽保持 policy 与 URL 顺序</span>
      </div>

      {rulesets.length === 0 ? (
        <EmptyState title="暂无规则集绑定" message={isReadonly ? "只读模式下不可新增规则集。" : "为服务组绑定远端规则列表后，生成时会按顺序输出。"} />
      ) : (
        <div className="stack-list" style={{ gap: 14 }}>
          {rulesets.map((entry, index) => (
            <RulesetCard
              key={`${entry.key || "ruleset"}-${index}`}
              entry={entry}
              index={index}
              readonly={isReadonly}
              policyOptions={policyOptions}
              activePointer={activePointer}
              onPatchEntry={(patch) => patchEntry(index, patch)}
              onPatchUrls={(urls) => patchUrls(index, urls)}
              onDelete={() => void deleteRuleset(index)}
              onDeleteUrl={(urlIndex) => void deleteUrl(index, urlIndex)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function RulesetCard({
  entry,
  index,
  readonly,
  policyOptions,
  activePointer,
  onPatchEntry,
  onPatchUrls,
  onDelete,
  onDeleteUrl
}: {
  entry: OrderedEntry<string[]>;
  index: number;
  readonly: boolean;
  policyOptions: string[];
  activePointer?: string;
  onPatchEntry: (patch: Partial<OrderedEntry<string[]>>) => void;
  onPatchUrls: (urls: string[]) => void;
  onDelete: () => void;
  onDeleteUrl: (urlIndex: number) => void;
}) {
  return (
    <article className={focusClassName(activePointer, [`/config/rulesets/${index}`], "content-panel")} style={{ padding: "18px 22px" }}>
      <div className="routing-card-header">
        <div className="row-title">
          <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{entry.key || `规则集 #${index + 1}`}</h3>
          <Chip>{entry.value.length} 条 URL</Chip>
          <span className="muted" style={{ fontSize: 12 }}>{policyOptions.includes(entry.key) ? "已绑定服务组" : "未绑定"}</span>
        </div>
        <div className="source-card-actions">
          <IconButton label="删除规则集" variant="danger" disabled={readonly} onClick={onDelete}>
            <Trash2 size={15} aria-hidden="true" />
          </IconButton>
        </div>
      </div>

      <Field label="绑定 Policy" hint="引用 A4 路由策略中的服务组">
        <SelectInput value={entry.key} disabled={readonly} onChange={(event) => onPatchEntry({ key: event.target.value })}>
          {entry.key && !policyOptions.includes(entry.key) ? <option value={entry.key}>{entry.key}（当前配置）</option> : null}
          <option value="">选择服务组</option>
          {policyOptions.map((policy) => (
            <option key={policy} value={policy}>{policy}</option>
          ))}
        </SelectInput>
      </Field>

      {entry.value.length === 0 ? (
        <EmptyState title="该服务组未挂载任何规则集" message={readonly ? "只读模式下不可新增 URL。" : "添加规则集 URL 后再运行静态校验。"} />
      ) : (
        <div className="stack-list" style={{ gap: 6 }}>
          {entry.value.map((url, urlIndex) => (
            <div
              key={`${urlIndex}-${url}`}
              className={focusClassName(activePointer, [`/config/rulesets/${index}/value/${urlIndex}`], "ruleset-url-row")}
            >
              <span className="drag-handle static" aria-hidden="true">⠿</span>
              <input
                className="ruleset-url-input"
                type="text"
                value={url}
                placeholder="https://example.com/rules.list"
                disabled={readonly}
                onChange={(e) => {
                  const next = [...entry.value];
                  next[urlIndex] = e.target.value;
                  onPatchUrls(next);
                }}
              />
              <IconButton label="删除 URL" variant="danger" disabled={readonly} onClick={() => onDeleteUrl(urlIndex)}>
                <Trash2 size={13} aria-hidden="true" />
              </IconButton>
            </div>
          ))}
        </div>
      )}

      <button className="add-dashed" type="button" disabled={readonly} onClick={() => onPatchUrls([...entry.value, ""])} style={{ marginTop: 10, width: "100%" }}>
        <Plus size={14} aria-hidden="true" />
        添加规则集 URL
      </button>
    </article>
  );
}
