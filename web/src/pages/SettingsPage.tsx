import { useMemo } from "react";
import { SelectInput, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { getPolicyOptions } from "../features/configModel";
import { useConfigState } from "../state/config";

export function SettingsPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const activePointer = useDiagnosticPointer();
  const routingOptions = useMemo(() => getPolicyOptions(draft ?? {}).filter((policy) => policy !== "DIRECT" && policy !== "REJECT"), [draft]);
  const fallback = draft?.fallback ?? "";
  const baseURL = draft?.base_url ?? "";
  const templates = draft?.templates ?? {};
  const baseURLState = validateBaseURL(baseURL);

  function patchConfig(patch: Partial<NonNullable<typeof draft>>) {
    updateDraft((config) => ({ ...config, ...patch }));
  }

  function patchTemplate(key: "clash" | "surge", value: string) {
    updateDraft((config) => ({ ...config, templates: { ...config.templates, [key]: value } }));
  }

  return (
    <div className="page-stack settings-page">
      <section className={focusClassName(activePointer, ["/config/fallback"], "setting-block")}>
        <div className="setting-copy">
          <h3>fallback 服务组</h3>
          <p>所有规则都不匹配时使用的兜底。建议指向「全球代理」类的服务组。</p>
        </div>
        <div className="setting-input-row">
          <SelectInput value={fallback} disabled={isReadonly} onChange={(event) => patchConfig({ fallback: event.target.value })}>
            {fallback && !routingOptions.includes(fallback) ? <option value={fallback}>{fallback}（当前配置）</option> : null}
            <option value="">选择服务组</option>
            {routingOptions.map((policy) => (
              <option key={policy} value={policy}>{policy}</option>
            ))}
          </SelectInput>
        </div>
      </section>

      <section className={focusClassName(activePointer, ["/config/base_url"], "setting-block")}>
        <div className="setting-copy">
          <h3>base_url</h3>
          <p>生成订阅链接时使用的基础地址，仅 scheme 和 host。Surge Managed Profile 会用到。</p>
        </div>
        <div className="setting-input-row">
          <TextInput className="mono-input" value={baseURL} disabled={isReadonly} onChange={(event) => patchConfig({ base_url: event.target.value })} placeholder="https://sub.example.com" />
        </div>
        {baseURLState.valid ? null : <div className="field-error">{baseURLState.message}</div>}
        {baseURLState.valid && baseURL ? (
          <div className="setting-preview">
            <span>预览：</span>
            <code style={{ color: "var(--primary)" }}>{baseURL}/generate?format=clash&token=••••</code>
          </div>
        ) : null}
      </section>

      <section className={focusClassName(activePointer, ["/config/templates/clash"], "setting-block")}>
        <div className="setting-copy">
          <h3>Clash 模板</h3>
          <p>生成 Clash Meta 配置时使用的基础模板。可填本地路径或 HTTP URL。</p>
        </div>
        <div className="setting-input-row">
          <TextInput className="mono-input" value={templates.clash ?? ""} disabled={isReadonly} onChange={(event) => patchTemplate("clash", event.target.value)} placeholder="./templates/clash.yaml" />
        </div>
      </section>

      <section className={focusClassName(activePointer, ["/config/templates/surge"], "setting-block")}>
        <div className="setting-copy">
          <h3>Surge 模板</h3>
          <p>生成 Surge 配置时使用的基础模板。</p>
        </div>
        <div className="setting-input-row">
          <TextInput className="mono-input" value={templates.surge ?? ""} disabled={isReadonly} onChange={(event) => patchTemplate("surge", event.target.value)} placeholder="./templates/surge.conf" />
        </div>
      </section>
    </div>
  );
}

function validateBaseURL(value: string): { valid: boolean; message: string } {
  if (!value.trim()) return { valid: true, message: "" };
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return { valid: false, message: "base_url 必须使用 http 或 https" };
    }
    if (parsed.pathname !== "/" || parsed.search || parsed.hash) {
      return { valid: false, message: "base_url 不能包含 path、query 或 fragment" };
    }
    return { valid: true, message: "" };
  } catch {
    return { valid: false, message: "base_url 不是有效 URL" };
  }
}
