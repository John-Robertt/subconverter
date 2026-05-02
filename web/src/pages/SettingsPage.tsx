import { ExternalLink, FileCode2 } from "lucide-react";
import { useMemo } from "react";
import { Chip, Field, SelectInput, TextInput } from "../components/ui";
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
          <p>所有规则都不匹配时使用的兜底出口，必须引用 A4 中定义的服务组。</p>
        </div>
        <Field label="Fallback">
          <SelectInput value={fallback} disabled={isReadonly} onChange={(event) => patchConfig({ fallback: event.target.value })}>
            {fallback && !routingOptions.includes(fallback) ? <option value={fallback}>{fallback}（当前配置）</option> : null}
            <option value="">选择服务组</option>
            {routingOptions.map((policy) => (
              <option key={policy} value={policy}>{policy}</option>
            ))}
          </SelectInput>
        </Field>
      </section>

      <section className={focusClassName(activePointer, ["/config/base_url"], "setting-block")}>
        <div className="setting-copy">
          <h3>base_url</h3>
          <p>服务的外部访问地址，用于订阅链接和 Surge Managed Profile。订阅 token 属于服务端运行参数，不在前端编辑。</p>
        </div>
        <Field label="Base URL" hint="只允许 scheme + host，不包含 path、query 或 fragment。" error={baseURLState.valid ? undefined : baseURLState.message}>
          <TextInput className="mono-input" value={baseURL} disabled={isReadonly} onChange={(event) => patchConfig({ base_url: event.target.value })} placeholder="https://sub.example.com" />
        </Field>
        <div className="setting-preview">
          <ExternalLink size={15} aria-hidden="true" />
          <span>订阅链接预览</span>
          <code>{baseURLState.valid && baseURL ? `${baseURL}/generate?format=clash&filename=clash.yaml` : "等待有效 base_url"}</code>
        </div>
      </section>

      <section className={focusClassName(activePointer, ["/config/templates"], "setting-block")}>
        <div className="setting-copy">
          <h3>模板路径</h3>
          <p>可填本地文件路径或 HTTP(S) URL；模板读取和渲染错误由 B3 生成预览或 `/generate` 暴露。</p>
        </div>
        <div className="form-grid two">
          <Field label="Clash 模板">
            <TextInput className="mono-input" value={templates.clash ?? ""} disabled={isReadonly} onChange={(event) => patchTemplate("clash", event.target.value)} placeholder="configs/base_clash.yaml" />
          </Field>
          <Field label="Surge 模板">
            <TextInput className="mono-input" value={templates.surge ?? ""} disabled={isReadonly} onChange={(event) => patchTemplate("surge", event.target.value)} placeholder="https://example.com/base_surge.conf" />
          </Field>
        </div>
        <div className="template-summary">
          <Chip tone={templates.clash ? "success" : "neutral"}><FileCode2 size={12} aria-hidden="true" /> Clash</Chip>
          <Chip tone={templates.surge ? "success" : "neutral"}><FileCode2 size={12} aria-hidden="true" /> Surge</Chip>
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
