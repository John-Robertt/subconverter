import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import type { SubscriptionCheckResult, SubscriptionFormat } from "../api/types";
import { Button, Field, SelectInput, TextArea, TextInput } from "./ui";

export interface SubscriptionEditorValue {
  location: string;
  content?: string;
  format?: SubscriptionFormat;
  inspection?: SubscriptionCheckResult;
}

export const subscriptionFormatLabels = { text: "节点列表", clash: "Clash 配置", surge: "Surge 配置", loon: "Loon 配置" };
const outputLabels = { clash: "Clash Meta", surge: "Surge", loon: "Loon" };

// Checks a subscription independently from choosing whether to keep its link or edit a copy.
export function SubscriptionEditor({ value, fileReference, readonly, onChange }: {
  value: SubscriptionEditorValue;
  fileReference: boolean;
  readonly: boolean;
  onChange: (patch: Partial<SubscriptionEditorValue>) => void;
}) {
  const { location, content, inspection, format = "auto" } = value;
  const inline = content !== undefined;
  const previous = useRef({ location, content: content ?? "" });
  const request = useRef(0);
  const debounce = useRef<number | undefined>(undefined);
  const controller = useRef<AbortController | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [correcting, setCorrecting] = useState(false);

  useEffect(() => () => { request.current++; controller.current?.abort(); }, []);

  function invalidate() {
    request.current++;
    window.clearTimeout(debounce.current);
    controller.current?.abort();
    controller.current = null;
    setLoading(false);
    setError("");
  }

  function edit(patch: Partial<SubscriptionEditorValue>) {
    invalidate();
    onChange({ ...patch, inspection: undefined });
  }

  async function check(selectedFormat = format) {
    window.clearTimeout(debounce.current);
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    const id = ++request.current;
    setLoading(true);
    setError("");
    onChange({ inspection: undefined });
    if (!inline && !fileReference && !/^https?:\/\//.test(location.trim())) {
      active.abort();
      controller.current = null;
      setLoading(false);
      setError("请输入 HTTP(S) 订阅地址；单个节点请粘贴到“节点正文”。");
      return;
    }
    try {
      const result = await api.checkSubscription(inline ? { content, format: selectedFormat } : { location: location.trim(), format: selectedFormat }, active.signal);
      if (request.current !== id) return;
      onChange({ inspection: result, format: result.status === "recognized" ? result.format : selectedFormat });
    } catch (err) {
      if (request.current === id && !active.signal.aborted) setError(err instanceof Error ? err.message : "订阅检查失败");
    } finally {
      if (request.current === id) {
        controller.current = null;
        setLoading(false);
      }
    }
  }

  useEffect(() => {
    if (readonly || !inline || !content.trim() || inspection || controller.current) return;
    const timer = window.setTimeout(() => void check(), 400);
    debounce.current = timer;
    return () => window.clearTimeout(timer);
  }, [content, format, inline, inspection, readonly]);

  function switchMode(nextInline: boolean) {
    if (nextInline === inline) return;
    if (inline) previous.current.content = content;
    else previous.current.location = location;
    edit(nextInline ? { location: "", content: previous.current.content, format: "auto" } : { location: previous.current.location, content: undefined, format: "auto" });
  }

  function chooseFormat(next: SubscriptionFormat) {
    edit({ format: next });
    if (!inline && location.trim()) void check(next);
  }

  return (
    <div className="form-stack subscription-editor">
      <div className="subscription-modes" role="group" aria-label="来源方式">
        <Button variant={inline ? "ghost" : "primary"} aria-pressed={!inline} disabled={readonly} onClick={() => switchMode(false)}>{fileReference ? "文件引用" : "订阅链接"}</Button>
        <Button variant={inline ? "primary" : "ghost"} aria-pressed={inline} disabled={readonly} onClick={() => switchMode(true)}>节点正文</Button>
      </div>
      {inline ? (
        <Field label="节点正文" hint="支持 SS / AnyTLS 节点列表或 Clash、Surge、Loon 配置。正文手动维护，保存为带时间戳的新版本。">
          <TextArea aria-label="节点正文" className="mono-input" rows={12} spellCheck={false} value={content} disabled={readonly}
            onChange={(event) => edit({ content: event.target.value })} />
        </Field>
      ) : (
        <Field label={fileReference ? "正文文件" : "订阅地址"} hint={fileReference ? "可检查现有文件，或改为节点正文修复内容。" : "使用 HTTP(S) 地址，保留链接后按订阅缓存策略拉取更新。"}>
          <TextInput aria-label={fileReference ? "正文文件" : "订阅地址"} className="mono-input" value={location} disabled={readonly}
            onChange={(event) => edit({ location: event.target.value })} />
        </Field>
      )}
      <p className="field-hint">只导入节点，策略组和规则由当前配置生成。</p>
      <div className="subscription-actions">
        <Button loading={loading} disabled={readonly || !(inline ? content.trim() : location.trim())} onClick={() => void check()}>{inline ? "识别并检查" : "检查订阅"}</Button>
        {!inline && inspection?.status === "recognized" ? (
          <Button disabled={readonly} onClick={() => {
            previous.current.location = location;
            invalidate();
            onChange({ location: "", content: inspection.content });
          }}>编辑正文副本</Button>
        ) : null}
      </div>
      {error ? <div className="field-error" role="alert">{error}</div> : null}
      {inspection ? (
        <div className="subscription-result" role="status">
          {inspection.status === "recognized" && inspection.format ? (
            <>
              <strong>已识别：{subscriptionFormatLabels[inspection.format]} · {inspection.node_count} 个节点</strong>
              <span>可完整用于：{inspection.output_formats.map((item) => outputLabels[item]).join("、") || "当前节点未通过输出兼容检查"}</span>
            </>
          ) : (
            <>
              <strong>发现 {inspection.node_count} 条节点声明，需要确认格式</strong>
              <span>这份配置来自哪个客户端？</span>
              <div className="subscription-actions">
                {inspection.candidates?.map((candidate) => <Button key={candidate} disabled={readonly} onClick={() => chooseFormat(candidate)}>{outputLabels[candidate]}</Button>)}
              </div>
            </>
          )}
        </div>
      ) : null}
      <div className="subscription-correction">
        <Button variant="ghost" aria-expanded={correcting} onClick={() => setCorrecting(!correcting)}>更正格式</Button>
        {correcting ? <Field label="按哪种格式解析">
          <SelectInput aria-label="按哪种格式解析" value={format} disabled={readonly} onChange={(event) => chooseFormat(event.target.value as SubscriptionFormat)}>
            <option value="auto">按内容识别</option>
            {Object.entries(subscriptionFormatLabels).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
          </SelectInput>
        </Field> : null}
      </div>
    </div>
  );
}
