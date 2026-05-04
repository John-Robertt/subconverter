import { Plus, Search, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { SortableList } from "../components/SortableList";
import { Button, EmptyState, Field, IconButton, SelectInput, TextArea, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { getPolicyOptions, replaceRulePolicy, splitRulePolicy } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

const TYPE_COLORS: Record<string, string> = {
  "DOMAIN": "#06b6d4",
  "DOMAIN-SUFFIX": "#0891b2",
  "DOMAIN-KEYWORD": "#0e7490",
  "IP-CIDR": "#a855f7",
  "IP-CIDR6": "#a855f7",
  "GEOIP": "#d946ef",
  "PROCESS-NAME": "#f59e0b",
  "MATCH": "#ef4444",
  "FINAL": "#ef4444"
};

const TARGET_COLORS: Record<string, string> = {
  "DIRECT": "#16a34a",
  "REJECT": "#dc2626"
};

export function RulesPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const rules = draft?.rules ?? [];
  const policyOptions = useMemo(() => getPolicyOptions(draft ?? {}), [draft]);
  const activeIndex = rules.length === 0 ? -1 : Math.min(selectedIndex, rules.length - 1);
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
            <TextInput value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索规则…" />
          </Field>
          <div className="group-toolbar">
            <Search size={15} aria-hidden="true" />
            <span>共 {rules.length} 条 · 拖拽调整顺序</span>
          </div>
          <Button variant="primary" icon={<Plus size={16} aria-hidden="true" />} disabled={isReadonly} onClick={addRule}>
            添加规则
          </Button>
        </div>
      </section>

      {rules.length === 0 ? (
        <EmptyState title="暂无内联规则" message={isReadonly ? "只读模式下不可新增规则。" : "添加内联规则后，渲染时会按 YAML 数组顺序直接透传。"} />
      ) : (
        <>
          <div className="rules-list">
            {rules.map((rule, index) => {
              const parsed = splitRulePolicy(rule);
              const visible = filteredIndexes.includes(index);
              if (!visible) return null;
              const typeColor = TYPE_COLORS[parsed.type ?? ""] ?? "var(--text-muted)";
              const targetColor = TARGET_COLORS[parsed.policy ?? ""] ?? "var(--primary)";
              return (
                <div
                  key={`${index}-${rule}`}
                  className={focusClassName(activePointer, [`/config/rules/${index}`], index === activeIndex ? "rules-list-row active" : "rules-list-row")}
                  onClick={() => setSelectedIndex(index)}
                >
                  <span className="drag-handle static" aria-hidden="true">⠿</span>
                  <span className="row-number">{String(index + 1).padStart(2, "0")}</span>
                  <span className="rule-type-badge" style={{ background: `${typeColor}1a`, color: typeColor }}>
                    {parsed.type || "RAW"}
                  </span>
                  <span className="rule-match-text">{parsed.match || rule}</span>
                  <span className="rule-arrow">→</span>
                  <span className="rule-target-badge" style={{ background: `${targetColor}1a`, color: targetColor, fontFamily: parsed.policy === "DIRECT" || parsed.policy === "REJECT" ? '"JetBrains Mono", monospace' : "inherit" }}>
                    {parsed.policy || "—"}
                  </span>
                  <IconButton label="删除规则" variant="danger" disabled={isReadonly} onClick={(event) => { event.stopPropagation(); void deleteRule(index); }}>
                    <Trash2 size={15} aria-hidden="true" />
                  </IconButton>
                </div>
              );
            })}
          </div>

          {activeIndex >= 0 && rules[activeIndex] ? (
            <section className={focusClassName(activePointer, [`/config/rules/${activeIndex}`], "content-panel editor-panel")}>
              <div className="section-heading">
                <h3>编辑规则 #{activeIndex + 1}</h3>
                <p>语义错误由 validate 或生成预览判断。</p>
              </div>
              <div className="form-grid two">
                <Field label="原始规则" hint="渲染时直接透传。">
                  <TextArea className="mono-input" value={rules[activeIndex]} disabled={isReadonly} onChange={(event) => patchRule(activeIndex, event.target.value)} />
                </Field>
                <Field label="Policy 选择器" hint={splitRulePolicy(rules[activeIndex]).parseable ? "替换最后逗号后的策略名。" : "当前规则缺少逗号。"}>
                  <SelectInput
                    value={splitRulePolicy(rules[activeIndex]).policy}
                    disabled={isReadonly || !splitRulePolicy(rules[activeIndex]).parseable}
                    onChange={(event) => patchRule(activeIndex, replaceRulePolicy(rules[activeIndex], event.target.value))}
                  >
                    {splitRulePolicy(rules[activeIndex]).policy && !policyOptions.includes(splitRulePolicy(rules[activeIndex]).policy) ? <option value={splitRulePolicy(rules[activeIndex]).policy}>{splitRulePolicy(rules[activeIndex]).policy}（当前配置）</option> : null}
                    <option value="">选择 Policy</option>
                    {policyOptions.map((policy) => (
                      <option key={policy} value={policy}>{policy}</option>
                    ))}
                  </SelectInput>
                </Field>
              </div>
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}
