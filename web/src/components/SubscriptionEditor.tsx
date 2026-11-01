import { useEffect, useEffectEvent, useRef, useState } from "react";
import { api } from "../api/client";
import type { SubscriptionCheckResult, SubscriptionFormat } from "../api/types";
import { useResourceContent } from "../features/useResourceContent";
import { ResourceBody } from "./ResourceBody";
import { Button, Field, SelectInput, TextInput } from "./ui";

export interface SubscriptionEditorValue {
  location: string;
  content?: string;
  format?: SubscriptionFormat;
  inspection?: SubscriptionCheckResult;
}

export const subscriptionFormatLabels = { text: "节点列表", clash: "Clash 配置", surge: "Surge 配置", loon: "Loon 配置" };
const outputLabels = { clash: "Clash Meta", surge: "Surge", loon: "Loon" };

export function SubscriptionEditor({ value, fileReference, readonly, onChange, pointer }: {
  value: SubscriptionEditorValue;
  fileReference: boolean;
  readonly: boolean;
  pointer?: string;
  onChange: (patch: Partial<SubscriptionEditorValue>) => void;
}) {
  const { location, content, inspection, format = "auto" } = value;
  const inline = content !== undefined;
  const address = location.trim();
  const validAddress = /^https?:\/\//.test(address) || (fileReference && !address.includes("://"));
  const resource = useResourceContent(!inline && validAddress ? address : "");
  const body = inline ? content : resource.content;
  const previous = useRef({ location, content, format, bodyFormat: format });
  const [expanded, setExpanded] = useState(false);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const readError = !inline && address && !validAddress
    ? "请输入 HTTP(S) 订阅地址；单个节点请粘贴到“本地正文”。"
    : resource.error;
  const publish = useEffectEvent((result?: SubscriptionCheckResult) => onChange({ inspection: result }));

  // Inspect downloaded bytes without re-fetching or discarding invalid content.
  useEffect(() => {
    setError("");
    if (body === undefined || !body.trim()) {
      setChecking(false);
      publish();
      return;
    }
    let current = true;
    const controller = new AbortController();
    setChecking(true);
    publish();
    const timer = window.setTimeout(async () => {
      try {
        const result = await api.checkSubscription({ content: body, format, source_location: inline ? undefined : address }, controller.signal);
        if (current) publish(result);
      } catch (err) {
        if (current) {
          setError(err instanceof Error ? err.message : "订阅检查失败");
          publish();
        }
      } finally {
        if (current) setChecking(false);
      }
    }, inline ? 400 : 0);
    return () => {
      current = false;
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [body, format, inline, address, attempt, resource.loading, readError]);

  function edit(patch: Partial<SubscriptionEditorValue>) {
    setError("");
    onChange({ ...patch, inspection: undefined });
  }

  function switchMode(nextInline: boolean) {
    if (inline === nextInline) return;
    if (nextInline) {
      previous.current.location = location;
      previous.current.format = format;
      edit({ location: "", content: previous.current.content ?? body ?? "", format: previous.current.content !== undefined ? previous.current.bodyFormat : inspection?.format ?? format });
      setExpanded(true);
    } else {
      previous.current.content = content;
      previous.current.bodyFormat = format;
      edit({ location: previous.current.location, content: undefined, format: previous.current.format });
    }
  }

  function retry() {
    edit({});
    if (inline) setAttempt((current) => current + 1);
    else resource.reload();
  }

  return (
    <div className="form-stack subscription-editor">
      <div className="subscription-modes" role="group" aria-label="来源方式">
        <Button variant={inline ? "ghost" : "primary"} aria-pressed={!inline} disabled={readonly} onClick={() => switchMode(false)}>{fileReference ? "文件引用" : "订阅链接"}</Button>
        <Button variant={inline ? "primary" : "ghost"} aria-pressed={inline} disabled={readonly || (!inline && resource.loading)} onClick={() => switchMode(true)}>本地正文</Button>
      </div>
      {!inline ? <Field label={fileReference ? "正文文件" : "订阅地址"} hint="按现有缓存策略读取；查看正文不会改变来源。">
        <TextInput data-config-pointer={pointer ? `${pointer}/${fileReference ? "content_file" : "url"}` : undefined} autoFocus aria-label={fileReference ? "正文文件" : "订阅地址"} className="mono-input" value={location} disabled={readonly}
          onChange={(event) => { previous.current.content = undefined; edit({ location: event.target.value }); }} />
      </Field> : <p className="field-hint">当前使用本地正文，由你手动维护，不再跟随原地址更新；原资源保持不变。</p>}
      <p className="field-hint">只导入节点，策略组和规则由当前配置生成。</p>
      <p className="field-hint" role="status">{resource.loading ? "正在读取订阅…" : readError ? "订阅读取失败" : checking ? "正文已读取，正在检查格式…" : body !== undefined ? "正文已就绪" : "请填写订阅地址或粘贴本地正文。"}</p>
      {readError || error ? <div className="field-error" role="alert">{readError || error}</div> : null}
      {inspection ? <div className="subscription-result" role="status">
        {inspection.status === "recognized" && inspection.format ? <>
          <strong>已识别：{subscriptionFormatLabels[inspection.format]} · {inspection.node_count} 个节点</strong>
          <span>可完整用于：{inspection.output_formats.map((item) => outputLabels[item]).join("、") || "当前节点未通过输出兼容检查"}</span>
        </> : <>
          <strong>发现 {inspection.node_count} 条节点声明，需要确认格式</strong>
          <span>这份配置来自哪个客户端？正文仍可展开查看或编辑。</span>
          <div className="subscription-actions">{inspection.candidates?.map((candidate) =>
            <Button key={candidate} disabled={readonly} onClick={() => edit({ format: candidate })}>{outputLabels[candidate]}</Button>)}</div>
        </>}
      </div> : null}
      <Field label="按哪种格式解析">
        <SelectInput data-config-pointer={pointer ? `${pointer}/format` : undefined} aria-label="按哪种格式解析" value={format} disabled={readonly} onChange={(event) => edit({ format: event.target.value as SubscriptionFormat })}>
          <option value="auto">按内容识别</option>
          {Object.entries(subscriptionFormatLabels).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
        </SelectInput>
      </Field>
      <ResourceBody label="节点正文" content={body} readOnly={readonly || !inline} expanded={expanded}
        onToggle={() => setExpanded((current) => !current)} onChange={(text) => edit({ content: text })}
        actions={<>
          <Button loading={resource.loading || checking} disabled={inline ? !body?.trim() : !address || !validAddress} onClick={retry}>{inline ? "重新检查" : "重新读取"}</Button>
          {!inline ? <Button disabled={readonly || body === undefined || resource.loading} onClick={() => switchMode(true)}>编辑为本地副本</Button> : null}
        </>} />
    </div>
  );
}
