import { Link2, Pencil, Plus, Radio, Shield, Trash2 } from "lucide-react";
import { useEffect, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import type { Config, CustomProxy, FetchSource, FetchSourceKind, RelayThrough } from "../api/types";
import { SubscriptionEditor, subscriptionFormatLabels, type SubscriptionEditorValue } from "../components/SubscriptionEditor";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, Field, IconButton, Modal, SelectInput, StatCard, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { ensureSources, fetchSourceKinds, maskUrl } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

const sourceLabels: Record<FetchSourceKind, string> = {
  subscriptions: "节点订阅",
  snell: "Snell 节点池",
  vless: "VLESS 节点池"
};

const sourceDescriptions: Record<FetchSourceKind, string> = {
  subscriptions: "SS / AnyTLS 文本订阅，或 Clash / Surge / Loon 原生配置",
  snell: "Surge 专属代理协议",
  vless: "Reality / Vision 等高级特性"
};

const sourceIcons: Record<FetchSourceKind, ReactNode> = {
  subscriptions: <Link2 size={18} aria-hidden="true" />,
  snell: <Shield size={18} aria-hidden="true" />,
  vless: <Radio size={18} aria-hidden="true" />
};

type SourceEditor =
  | ({ type: "fetch"; kind: FetchSourceKind; index: number | null; reference?: "file" } & SubscriptionEditorValue)
  | { type: "custom"; index: number | null; proxy: CustomProxy }
  | null;

function sourceFromEditor(editor: Extract<NonNullable<SourceEditor>, { type: "fetch" }>): FetchSource {
  const source: FetchSource = editor.content !== undefined ? { content: editor.content }
    : editor.reference === "file" && !/^https?:\/\//.test(editor.location) ? { content_file: editor.location.trim() } : { url: editor.location.trim() };
  const format = editor.inspection?.format ?? editor.format;
  if (editor.kind === "subscriptions" && format) source.format = format;
  return source;
}

function sameFetchSource(left: FetchSource, right: FetchSource | undefined): boolean {
  return right !== undefined && left.url === right.url && left.content_file === right.content_file && left.content === right.content
    && (left.format ?? "auto") === (right.format ?? "auto");
}

export function SourcesPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [editor, setEditor] = useState<SourceEditor>(null);
  const sources = ensureSources(draft?.sources);
  const totalFetch = sources.subscriptions.length + sources.snell.length + sources.vless.length;
  const totalSources = totalFetch + sources.custom_proxies.length;
  useEffect(() => {
    const match = activePointer?.match(/^\/config\/sources\/(subscriptions|snell|vless|custom_proxies)\/(\d+)/);
    if (!match) return;
    const index = Number(match[2]);
    if (match[1] === "custom_proxies") {
      const proxy = sources.custom_proxies[index];
      if (proxy) setEditor({ type: "custom", index, proxy });
    } else {
      const kind = match[1] as FetchSourceKind;
      const source = sources[kind][index];
      if (source) setEditor({ type: "fetch", kind, index, location: source.url ?? source.content_file ?? "", reference: source.content_file ? "file" : undefined, content: source.content, format: source.format });
    }
  }, [activePointer]);

  function patchSources(updater: (sources: ReturnType<typeof ensureSources>) => ReturnType<typeof ensureSources>) {
    updateDraft((config) => ({ ...config, sources: updater(ensureSources(config.sources)) }));
  }

  function saveEditor() {
    if (!editor) return;
    if (editor.type === "fetch") {
      const source = sourceFromEditor(editor);
      patchSources((current) => ({
        ...current,
        [editor.kind]:
          editor.index === null
            ? [...current[editor.kind], source]
            : current[editor.kind].map((item, index) => (index === editor.index ? source : item))
      }));
      setEditor(null);
      return;
    }

    patchSources((current) => ({
      ...current,
      custom_proxies:
        editor.index === null
          ? [...current.custom_proxies, editor.proxy]
          : current.custom_proxies.map((item, index) => (index === editor.index ? editor.proxy : item))
    }));
    setEditor(null);
  }

  async function deleteFetchSource(kind: FetchSourceKind, index: number) {
    const accepted = await confirm({
      title: "删除来源？",
      message: `即将删除 ${sourceLabels[kind]} #${index + 1}。该操作只影响当前草稿，保存后才会写回配置。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    patchSources((current) => ({ ...current, [kind]: current[kind].filter((_, itemIndex) => itemIndex !== index) }));
  }

  async function deleteCustomProxy(index: number) {
    const accepted = await confirm({
      title: "删除自定义代理？",
      message: `即将删除「${sources.custom_proxies[index]?.name || `自定义代理 #${index + 1}`}」。该操作只影响当前草稿。`,
      confirmLabel: "确认删除",
      danger: true
    });
    if (!accepted) return;
    patchSources((current) => ({ ...current, custom_proxies: current.custom_proxies.filter((_, itemIndex) => itemIndex !== index) }));
  }

  function reorderFetchSources(kind: FetchSourceKind, items: FetchSource[]) {
    patchSources((current) => ({ ...current, [kind]: items }));
  }

  function reorderCustomProxies(items: CustomProxy[]) {
    patchSources((current) => ({ ...current, custom_proxies: items }));
  }

  return (
    <div className="page-stack">
      <div className="stats-grid">
        <StatCard label="订阅总数" value={totalSources} sub={`${sources.fetch_order.length} 类拉取顺序`} />
        <StatCard label="订阅来源" value={totalFetch} sub="订阅 / snell / vless" />
        <StatCard label="Surge-only" value={sources.snell.length} sub="Snell" />
        <StatCard label="Clash / Loon" value={sources.vless.length} sub="VLESS" />
      </div>

      <section className={focusClassName(activePointer, ["/config/sources/fetch_order"], "content-panel source-order-panel")}>
        <div className="section-heading row">
          <div>
            <h3>拉取顺序</h3>
            <p>拖拽调整 `subscriptions`、`snell`、`vless` 在 YAML 中的声明顺序。</p>
          </div>
          {isReadonly ? <Chip tone="warning">readonly</Chip> : <Chip tone="accent">draft</Chip>}
        </div>
        <SortableList
          items={sources.fetch_order}
          getId={(item) => item}
          disabled={isReadonly}
          onReorder={(items) => patchSources((current) => ({ ...current, fetch_order: items }))}
          renderItem={(item, _index, handle) => (
            <div className="source-order-row">
              {handle}
              <strong>{sourceLabels[item]}</strong>
              <code>{item}</code>
            </div>
          )}
        />
      </section>

      {fetchSourceKinds.map((kind) => (
        <SourceSection
          key={kind}
          title={sourceLabels[kind]}
          subtitle={sourceDescriptions[kind]}
          count={sources[kind].length}
          tag={kind === "snell" ? "仅 Surge" : kind === "vless" ? "Clash / Loon" : undefined}
          icon={sourceIcons[kind]}
          readonly={isReadonly}
          className={focusClassName(activePointer, [`/config/sources/${kind}`], "source-section")}
          onAdd={() => setEditor({ type: "fetch", kind, index: null, location: "" })}
        >
          <SortableList
            items={sources[kind]}
            getId={(source, index) => `${kind}:${index}:${source.url || "empty"}`}
            disabled={isReadonly}
            onReorder={(items) => reorderFetchSources(kind, items)}
            renderItem={(source, index, handle) => (
              <SourceCard
                title={`${sourceLabels[kind]} #${index + 1}`}
                subtitle={source.content !== undefined ? `正文 · ${source.content.split("\n").length} 行` : source.content_file ? `正文文件 · ${source.content_file}` : maskUrl(source.url ?? "") || "未设置 URL"}
                meta={kind === "subscriptions" ? source.format && source.format !== "auto" ? subscriptionFormatLabels[source.format] : "按内容识别" : kind}
                statusTone={kind === "subscriptions" ? "neutral" : "success"}
                viewable={kind === "subscriptions"}
                tag={source.url || source.content_file || source.content?.trim() ? "已配置" : "待补齐"}
                readonly={isReadonly}
                dragHandle={handle}
                onEdit={() => setEditor({ type: "fetch", kind, index, location: source.url ?? source.content_file ?? "", reference: source.content_file ? "file" : undefined, content: source.content, format: source.format })}
                onDelete={() => void deleteFetchSource(kind, index)}
              />
            )}
          />
        </SourceSection>
      ))}

      <SourceSection
        title="自定义代理"
        subtitle="单节点直连，可链式中转"
        count={sources.custom_proxies.length}
        icon={<Radio size={18} aria-hidden="true" />}
        readonly={isReadonly}
        className={focusClassName(activePointer, ["/config/sources/custom_proxies"], "source-section")}
        onAdd={() => setEditor({ type: "custom", index: null, proxy: { name: "", url: "" } })}
      >
        <SortableList
          items={sources.custom_proxies}
          getId={(proxy, index) => `custom:${index}:${proxy.name || "unnamed"}:${proxy.url || "empty"}`}
          disabled={isReadonly}
          onReorder={reorderCustomProxies}
          renderItem={(proxy, index, handle) => (
            <SourceCard
              title={proxy.name || `自定义代理 #${index + 1}`}
              subtitle={proxy.url ? maskUrl(proxy.url) : "ss://、socks5:// 或 http://"}
              meta={proxy.url.split("://")[0] || "custom"}
              tag={proxy.relay_through ? `中转 · ${proxy.relay_through.strategy}` : "直连"}
              readonly={isReadonly}
              dragHandle={handle}
              onEdit={() => setEditor({ type: "custom", index, proxy })}
              onDelete={() => void deleteCustomProxy(index)}
            />
          )}
        />
      </SourceSection>

      {totalSources === 0 ? <EmptyState title="配置中没有任何来源" message="A1 可先添加订阅或自定义代理；保存前仍可切换其他编辑页继续补齐配置。" /> : null}

      <SourceEditorModal editor={editor} draft={draft} readonly={isReadonly} diagnosticPointer={activePointer} onChange={setEditor} onClose={() => setEditor(null)} onSave={saveEditor} />
    </div>
  );
}

function SourceSection({
  title,
  subtitle,
  count,
  tag,
  icon,
  readonly,
  className = "source-section",
  onAdd,
  children
}: {
  title: string;
  subtitle: string;
  count: number;
  tag?: string;
  icon?: ReactNode;
  readonly: boolean;
  className?: string;
  onAdd: () => void;
  children: ReactNode;
}) {
  return (
    <section className={className}>
      <div className="source-section-header">
        <div className="source-section-title">
          <div>
            <h3>
              {icon ? <span className="source-section-icon" aria-hidden="true">{icon}</span> : null}
              <span>{title}</span>
              <Chip>{count}</Chip>
              {tag ? <Chip tone={tag.includes("Surge") ? "warning" : "info"}>{tag}</Chip> : null}
            </h3>
            <p>{subtitle}</p>
          </div>
        </div>
      </div>
      <div className="source-card-list">
        {count === 0 ? <EmptyState title="暂无来源" message={readonly ? "只读模式下不可新增来源。" : "添加一个来源后再保存生效。"} /> : children}
        <button className="add-dashed" type="button" disabled={readonly} onClick={onAdd}>
          <Plus size={15} aria-hidden="true" />
          <span>添加 {title}</span>
        </button>
      </div>
    </section>
  );
}

function SourceCard({
  title,
  subtitle,
  meta,
  tag,
  readonly,
  dragHandle,
  onEdit,
  onDelete,
  statusTone = "success",
  viewable = false
}: {
  title: string;
  subtitle: string;
  meta: string;
  tag: string;
  readonly: boolean;
  dragHandle: ReactNode;
  onEdit: () => void;
  onDelete: () => void;
  statusTone?: "success" | "warning" | "neutral";
  viewable?: boolean;
}) {
  const ready = !tag.includes("待");
  const status = ready ? meta : "待补齐";
  return (
    <article className="source-card" title={title}>
      {dragHandle}
      <code className="source-card-url">{subtitle}</code>
      <span className={`source-card-meta ${ready ? statusTone : "warning"}`}>{status}</span>
      <div className="source-card-actions">
        <IconButton label={readonly && viewable ? "查看来源" : "编辑来源"} variant="ghost" disabled={readonly && !viewable} onClick={onEdit}>
          <Pencil size={15} aria-hidden="true" />
        </IconButton>
        <IconButton label="删除来源" variant="danger" disabled={readonly} onClick={onDelete}>
          <Trash2 size={15} aria-hidden="true" />
        </IconButton>
      </div>
    </article>
  );
}

function SourceEditorModal({
  editor,
  draft,
  readonly,
  diagnosticPointer,
  onChange,
  onClose,
  onSave
}: {
  editor: SourceEditor;
  draft: Config | undefined;
  readonly: boolean;
  diagnosticPointer?: string;
  onChange: Dispatch<SetStateAction<SourceEditor>>;
  onClose: () => void;
  onSave: () => void;
}) {
  const groupNames = draft?.groups?.map((entry) => entry.key).filter(Boolean) ?? [];
  const isCustom = editor?.type === "custom";
  const customNameError = editor?.type === "custom" && !editor.proxy.name.trim() ? "请填写名称" : undefined;
  const customURLError = editor?.type === "custom" && !editor.proxy.url.trim() ? "请填写代理 URL" : undefined;
  const focusCustomURL = editor?.type === "custom" && diagnosticPointer === `/config/sources/custom_proxies/${editor.index}/url`;
  const canApply = editor?.type === "custom" ? !customNameError && !customURLError : editor?.type === "fetch"
    && Boolean(editor.content !== undefined ? editor.content.trim() : editor.location.trim())
    && (editor.kind !== "subscriptions" || editor.inspection?.status === "recognized");
  const sources = ensureSources(draft?.sources);
  const hasChanges = Boolean(editor && (editor.index === null || (editor.type === "fetch"
    ? !sameFetchSource(sourceFromEditor(editor), sources[editor.kind][editor.index])
    : JSON.stringify(editor.proxy) !== JSON.stringify(sources.custom_proxies[editor.index]))));
  const done = readonly || !hasChanges;
  const title = !editor ? "" : editor.index === null ? (isCustom ? "添加自定义代理" : `添加 ${sourceLabels[editor.kind]}`) : isCustom ? "编辑自定义代理" : `编辑 ${sourceLabels[editor.kind]}`;

  function patchCustom(patch: Partial<CustomProxy>) {
    if (!editor || editor.type !== "custom") return;
    onChange({ ...editor, proxy: { ...editor.proxy, ...patch } });
  }

  function patchRelay(nextRelay: RelayThrough | undefined) {
    if (!editor || editor.type !== "custom") return;
    patchCustom({ relay_through: nextRelay });
  }

  return (
    <Modal
      open={Boolean(editor)}
      title={title}
      description={done ? readonly ? "只读查看，关闭不会修改配置。" : "配置未修改，无需保存。" : isCustom ? "支持单节点 URL 与 relay_through 中转配置。" : "来源修改先进入草稿，保存并热重载后生效。"}
      onClose={onClose}
      footer={
        done ? <Button variant="primary" onClick={onClose}>完成</Button> : <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button variant="primary" disabled={!canApply} onClick={onSave}>应用到草稿</Button>
        </>
      }
    >
      {editor?.type === "fetch" ? (
        <div className="form-stack">
          {editor.kind === "subscriptions" ? (
            <SubscriptionEditor key={`${editor.kind}:${editor.index}`} value={editor} fileReference={editor.reference === "file"} readonly={readonly}
              pointer={`/config/sources/${editor.kind}/${editor.index}`}
              onChange={(patch) => onChange((current) => current?.type === "fetch" ? { ...current, ...patch } : current)} />
          ) : (
            <Field label="订阅 URL" hint="必须为 HTTP(S) URL">
              <TextInput data-config-pointer={`/config/sources/${editor.kind}/${editor.index}/url`} autoFocus aria-label="订阅 URL" value={editor.location} disabled={readonly} onChange={(event) => onChange({ ...editor, location: event.target.value })} />
            </Field>
          )}
        </div>
      ) : editor?.type === "custom" ? (
        <div className="form-stack">
          <Field label="名称" error={customNameError}>
            <TextInput data-config-pointer={`/config/sources/custom_proxies/${editor.index}/name`} value={editor.proxy.name} disabled={readonly} onChange={(event) => patchCustom({ name: event.target.value })} autoFocus={!focusCustomURL} />
          </Field>
          <Field label="URL" error={customURLError} hint={editor.proxy.url ? maskUrl(editor.proxy.url) : "ss://、socks5:// 或 http://"}>
            <TextInput data-config-pointer={`/config/sources/custom_proxies/${editor.index}/url`} autoFocus={focusCustomURL} className="text-input mono-input" value={editor.proxy.url} disabled={readonly} onChange={(event) => patchCustom({ url: event.target.value })} />
          </Field>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={Boolean(editor.proxy.relay_through)}
              disabled={readonly}
              onChange={(event) => patchRelay(event.target.checked ? { type: "group", name: groupNames[0] ?? "", strategy: "select" } : undefined)}
            />
            <span>启用 relay_through</span>
          </label>
          {editor.proxy.relay_through ? <RelayEditor relay={editor.proxy.relay_through} groupNames={groupNames} disabled={readonly} onChange={patchRelay} /> : null}
        </div>
      ) : null}
    </Modal>
  );
}

function RelayEditor({
  relay,
  groupNames,
  disabled,
  onChange
}: {
  relay: RelayThrough;
  groupNames: string[];
  disabled: boolean;
  onChange: (relay: RelayThrough) => void;
}) {
  return (
    <div className="relay-card">
      <div className="relay-card-title">
        <Shield size={15} aria-hidden="true" />
        <strong>链式中转</strong>
      </div>
      <div className="form-grid three">
        <Field label="类型">
          <SelectInput value={relay.type} disabled={disabled} onChange={(event) => onChange({ ...relay, type: event.target.value as RelayThrough["type"] })}>
            <option value="group">group</option>
            <option value="select">select</option>
            <option value="all">all</option>
          </SelectInput>
        </Field>
        <Field label="策略">
          <SelectInput value={relay.strategy} disabled={disabled} onChange={(event) => onChange({ ...relay, strategy: event.target.value as RelayThrough["strategy"] })}>
            <option value="select">select</option>
            <option value="url-test">url-test</option>
          </SelectInput>
        </Field>
        {relay.type === "group" ? (
          <Field label="节点组">
            <SelectInput value={relay.name ?? ""} disabled={disabled} onChange={(event) => onChange({ ...relay, name: event.target.value })}>
              <option value="">选择节点组</option>
              {groupNames.map((name) => (
                <option key={name} value={name}>{name}</option>
              ))}
            </SelectInput>
          </Field>
        ) : relay.type === "select" ? (
          <Field label="匹配正则">
            <TextInput value={relay.match ?? ""} disabled={disabled} onChange={(event) => onChange({ ...relay, match: event.target.value })} />
          </Field>
        ) : null}
      </div>
    </div>
  );
}
