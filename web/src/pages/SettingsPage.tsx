import { useMemo } from "react";
import { ResourceEditor } from "../components/ResourceEditor";
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

  function patchTemplate(key: "clash" | "surge" | "loon", location: string, content?: string) {
    updateDraft((config) => {
      const inline = { ...config.templates?.inline };
      if (content === undefined) delete inline[key];
      else inline[key] = content;
      return { ...config, templates: { ...config.templates, [key]: location || undefined, inline } };
    });
  }

  return (
    <div className="page-stack settings-page">
      <section className={focusClassName(activePointer, ["/config/fallback"], "setting-block")}>
        <div className="setting-copy">
          <h3>fallback 服务组</h3>
          <p>所有规则都不匹配时使用的兜底。建议指向「全球代理」类的服务组。</p>
        </div>
        <div className="setting-input-row">
          <SelectInput aria-label="fallback 服务组" data-config-pointer="/config/fallback" value={fallback} disabled={isReadonly} onChange={(event) => patchConfig({ fallback: event.target.value })}>
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
          <TextInput aria-label="base_url" data-config-pointer="/config/base_url" className="mono-input" value={baseURL} disabled={isReadonly} onChange={(event) => patchConfig({ base_url: event.target.value })} placeholder="https://sub.example.com" />
        </div>
        {baseURLState.valid ? null : <div className="field-error">{baseURLState.message}</div>}
        {baseURLState.valid && baseURL ? (
          <div className="setting-preview">
            <span>预览：</span>
            <code style={{ color: "var(--primary)" }}>{baseURL}/generate?format=clash&token=••••</code>
          </div>
        ) : null}
      </section>

      {(["clash", "surge", "loon"] as const).map((format) => {
        const label = `${format === "clash" ? "Clash" : format === "surge" ? "Surge" : "Loon"} 模板`;
        return (
          <section key={format} className={focusClassName(activePointer, [`/config/templates/${format}`, `/config/templates/inline/${format}`], "setting-block")}>
            <div className="setting-copy">
              <h3>{label}</h3>
              <p>编辑基础配置。节点、策略组和规则由当前配置生成。</p>
            </div>
            <ResourceEditor label={label} location={templates[format] ?? ""} content={templates.inline?.[format]} readonly={isReadonly}
              onChange={({ location, content }) => patchTemplate(format, location, content)} />
          </section>
        );
      })}
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
