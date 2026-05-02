import { Link2, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { OrderedEntry } from "../api/types";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, Field, IconButton, SelectInput, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { getPolicyOptions } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

export function RulesetsPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [selectedIndex, setSelectedIndex] = useState(0);
  const rulesets = draft?.rulesets ?? [];
  const policyOptions = useMemo(() => getPolicyOptions(draft ?? {}), [draft]);
  const activeIndex = rulesets.length === 0 ? -1 : Math.min(selectedIndex, rulesets.length - 1);
  const activeRuleset = activeIndex >= 0 ? rulesets[activeIndex] : undefined;

  useEffect(() => {
    const match = activePointer?.match(/^\/config\/rulesets\/(\d+)/);
    if (match) {
      setSelectedIndex(Number(match[1]));
    }
  }, [activePointer]);

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
    setSelectedIndex(rulesets.length);
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
    setSelectedIndex(Math.max(0, index - 1));
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
        <SortableList
          items={rulesets}
          getId={(item, index) => `${item.key || "ruleset"}-${index}`}
          disabled={isReadonly}
          onReorder={setRulesets}
          renderItem={(entry, index, handle) => (
            <article
              className={focusClassName(activePointer, [`/config/rulesets/${index}`], index === activeIndex ? "routing-card active" : "routing-card")}
              onClick={() => setSelectedIndex(index)}
            >
              <div className="routing-card-header">
                <div className="row-title">
                  {handle}
                  <code>{String(index + 1).padStart(2, "0")}</code>
                  <strong>{entry.key || `规则集 #${index + 1}`}</strong>
                  <Chip>{entry.value.length} 条 URL</Chip>
                </div>
                <IconButton label="删除规则集" variant="danger" disabled={isReadonly} onClick={() => void deleteRuleset(index)}>
                  <Trash2 size={15} aria-hidden="true" />
                </IconButton>
              </div>
              {index === activeIndex ? (
                <div className="form-stack">
                  <Field label="绑定 Policy" hint="应引用 A4 路由策略中的服务组；未知 key 会保留并交给 validate 报错。">
                    <SelectInput value={entry.key} disabled={isReadonly} onChange={(event) => patchEntry(index, { key: event.target.value })}>
                      {entry.key && !policyOptions.includes(entry.key) ? <option value={entry.key}>{entry.key}（当前配置）</option> : null}
                      <option value="">选择服务组</option>
                      {policyOptions.map((policy) => (
                        <option key={policy} value={policy}>{policy}</option>
                      ))}
                    </SelectInput>
                  </Field>
                  <RulesetUrlEditor
                    entryIndex={index}
                    urls={entry.value}
                    readonly={isReadonly}
                    activePointer={activePointer}
                    onChange={(urls) => patchUrls(index, urls)}
                    onDelete={(urlIndex) => void deleteUrl(index, urlIndex)}
                  />
                </div>
              ) : null}
            </article>
          )}
        />
      )}
    </div>
  );
}

function RulesetUrlEditor({
  entryIndex,
  urls,
  readonly,
  activePointer,
  onChange,
  onDelete
}: {
  entryIndex: number;
  urls: string[];
  readonly: boolean;
  activePointer?: string;
  onChange: (urls: string[]) => void;
  onDelete: (index: number) => void;
}) {
  function patchUrl(index: number, value: string) {
    onChange(urls.map((url, urlIndex) => (urlIndex === index ? value : url)));
  }

  return (
    <div className="editor-block">
      <div className="editor-block-header">
        <div>
          <strong>URL 列表</strong>
          <p className="muted">多条 URL 会合并匹配同一服务组，顺序按当前列表写回。</p>
        </div>
        <Button variant="secondary" icon={<Link2 size={15} aria-hidden="true" />} disabled={readonly} onClick={() => onChange([...urls, ""])}>
          添加 URL
        </Button>
      </div>
      {urls.length === 0 ? <EmptyState title="该服务组未挂载任何规则集" message={readonly ? "只读模式下不可新增 URL。" : "添加一条 HTTP(S) URL 后再运行静态校验。"} /> : null}
      <SortableList
        items={urls}
        getId={(url, index) => `${index}-${url}`}
        disabled={readonly}
        onReorder={onChange}
        renderItem={(url, index, handle) => (
          <div className={focusClassName(activePointer, [`/config/rulesets/${entryIndex}/value/${index}`], "inline-editor-row")}>
            {handle}
            <span className="row-number">{String(index + 1).padStart(2, "0")}</span>
            <TextInput className="mono-input" value={url} disabled={readonly} onChange={(event) => patchUrl(index, event.target.value)} placeholder="https://example.com/rules.list" />
            <IconButton label="删除规则集 URL" variant="danger" disabled={readonly} onClick={() => onDelete(index)}>
              <Trash2 size={15} aria-hidden="true" />
            </IconButton>
          </div>
        )}
      />
    </div>
  );
}
