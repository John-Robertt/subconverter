import { Plus, Search, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, Field, IconButton, SelectInput, TextArea, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { getPolicyOptions, replaceRulePolicy, splitRulePolicy } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

export function RulesPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const rules = draft?.rules ?? [];
  const policyOptions = useMemo(() => getPolicyOptions(draft ?? {}), [draft]);
  const activeIndex = rules.length === 0 ? -1 : Math.min(selectedIndex, rules.length - 1);
  const activeRule = activeIndex >= 0 ? rules[activeIndex] : undefined;
  const filteredIndexes = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return rules.map((_, index) => index);
    return rules.flatMap((rule, index) => (rule.toLowerCase().includes(needle) ? [index] : []));
  }, [query, rules]);

  useEffect(() => {
    const match = activePointer?.match(/^\/config\/rules\/(\d+)/);
    if (match) {
      setSelectedIndex(Number(match[1]));
    }
  }, [activePointer]);

  function setRules(nextRules: string[]) {
    updateDraft((config) => ({ ...config, rules: nextRules }));
  }

  function patchRule(index: number, value: string) {
    setRules(rules.map((rule, ruleIndex) => (ruleIndex === index ? value : rule)));
  }

  function addRule() {
    const policy = draft?.fallback || policyOptions[0] || "DIRECT";
    setRules([...rules, `DOMAIN-SUFFIX,example.com,${policy}`]);
    setSelectedIndex(rules.length);
  }

  async function deleteRule(index: number) {
    const accepted = await confirm({
      title: "删除内联规则？",
      message: `即将删除第 ${index + 1} 条内联规则。规则顺序会在保存后写回 YAML。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    setRules(rules.filter((_, ruleIndex) => ruleIndex !== index));
    setSelectedIndex(Math.max(0, index - 1));
  }

  return (
    <div className="page-stack">
      <section className="content-panel dense-panel">
        <div className="rules-toolbar">
          <Field label="搜索规则">
            <TextInput value={query} onChange={(event) => setQuery(event.target.value)} placeholder="DOMAIN-SUFFIX / policy / keyword" />
          </Field>
          <div className="group-toolbar">
            <Search size={15} aria-hidden="true" />
            <span>显示 {filteredIndexes.length} / {rules.length} 条 · 拖拽调整原始数组顺序</span>
          </div>
          <Button variant="secondary" icon={<Plus size={16} aria-hidden="true" />} disabled={isReadonly} onClick={addRule}>
            添加规则
          </Button>
        </div>
      </section>

      {rules.length === 0 ? (
        <EmptyState title="暂无内联规则" message={isReadonly ? "只读模式下不可新增规则。" : "添加内联规则后，渲染时会按 YAML 数组顺序直接透传。"} />
      ) : (
        <SortableList
          items={rules}
          getId={(rule, index) => `${index}-${rule}`}
          disabled={isReadonly}
          onReorder={setRules}
          renderItem={(rule, index, handle) => {
            const parsed = splitRulePolicy(rule);
            const visible = filteredIndexes.includes(index);
            return (
              <article
                className={focusClassName(activePointer, [`/config/rules/${index}`], index === activeIndex ? "rule-card active" : "rule-card")}
                style={{ display: visible ? undefined : "none" }}
                onClick={() => setSelectedIndex(index)}
              >
                <div className="rule-card-meta">
                  {handle}
                  <span className="row-number">{String(index + 1).padStart(2, "0")}</span>
                  <Chip tone={parsed.parseable ? "info" : "warning"}>{parsed.parseable ? "parseable" : "raw"}</Chip>
                  {parsed.policy ? <Chip tone={reservedPolicyTone(parsed.policy)}>{parsed.policy}</Chip> : <Chip tone="warning">无 policy</Chip>}
                  <IconButton label="删除规则" variant="danger" disabled={isReadonly} onClick={() => void deleteRule(index)}>
                    <Trash2 size={15} aria-hidden="true" />
                  </IconButton>
                </div>
                {index === activeIndex ? (
                  <div className="form-grid two">
                    <Field label="原始规则" hint="渲染时直接透传；语义错误由 validate 或生成预览判断。">
                      <TextArea className="mono-input" value={rule} disabled={isReadonly} onChange={(event) => patchRule(index, event.target.value)} />
                    </Field>
                    <Field label="Policy 选择器" hint={parsed.parseable ? "只替换最后一个逗号后的策略名。" : "当前规则缺少逗号，选择器不会改写原文。"}>
                      <SelectInput
                        value={parsed.policy}
                        disabled={isReadonly || !parsed.parseable}
                        onChange={(event) => patchRule(index, replaceRulePolicy(rule, event.target.value))}
                      >
                        {parsed.policy && !policyOptions.includes(parsed.policy) ? <option value={parsed.policy}>{parsed.policy}（当前配置）</option> : null}
                        <option value="">选择 Policy</option>
                        {policyOptions.map((policy) => (
                          <option key={policy} value={policy}>{policy}</option>
                        ))}
                      </SelectInput>
                    </Field>
                  </div>
                ) : (
                  <code className="rule-preview">{rule}</code>
                )}
              </article>
            );
          }}
        />
      )}
    </div>
  );
}

function reservedPolicyTone(policy: string): "neutral" | "accent" | "success" | "error" | "info" {
  if (policy === "DIRECT") return "success";
  if (policy === "REJECT") return "error";
  return "accent";
}
