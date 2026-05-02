import { Link2, Pencil, Plus, Radio, RefreshCw, Shield, Trash2 } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { Config, CustomProxy, FetchSourceKind, RelayThrough } from "../api/types";
import { SortableList } from "../components/SortableList";
import { Button, Chip, EmptyState, Field, IconButton, Modal, SelectInput, StatCard, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { ensureSources, fetchSourceKinds, maskUrl } from "../features/configModel";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";

const sourceLabels: Record<FetchSourceKind, string> = {
  subscriptions: "SS 订阅",
  snell: "Snell 节点池",
  vless: "VLESS 节点池"
};

const sourceDescriptions: Record<FetchSourceKind, string> = {
  subscriptions: "Shadowsocks · 同时输出到 Clash 和 Surge",
  snell: "Surge 专属代理协议",
  vless: "Reality / Vision 等高级特性"
};

const sourceIcons: Record<FetchSourceKind, ReactNode> = {
  subscriptions: <Link2 size={18} aria-hidden="true" />,
  snell: <Shield size={18} aria-hidden="true" />,
  vless: <Radio size={18} aria-hidden="true" />
};

type SourceEditor =
  | { type: "fetch"; kind: FetchSourceKind; index: number | null; url: string }
  | { type: "custom"; index: number | null; proxy: CustomProxy }
  | null;

export function SourcesPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const confirm = useConfirm();
  const activePointer = useDiagnosticPointer();
  const [editor, setEditor] = useState<SourceEditor>(null);
  const sources = ensureSources(draft?.sources);
  const totalFetch = sources.subscriptions.length + sources.snell.length + sources.vless.length;
  const totalSources = totalFetch + sources.custom_proxies.length;

  function patchSources(updater: (sources: ReturnType<typeof ensureSources>) => ReturnType<typeof ensureSources>) {
    updateDraft((config) => ({ ...config, sources: updater(ensureSources(config.sources)) }));
  }

  function saveEditor() {
    if (!editor) return;
    if (editor.type === "fetch") {
      patchSources((current) => ({
        ...current,
        [editor.kind]:
          editor.index === null
            ? [...current[editor.kind], { url: editor.url }]
            : current[editor.kind].map((item, index) => (index === editor.index ? { url: editor.url } : item))
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

  return (
    <div className="page-stack">
      <div className="stats-grid">
        <StatCard label="订阅总数" value={totalSources} sub={`${sources.fetch_order.length} 类拉取顺序`} />
        <StatCard label="订阅来源" value={totalFetch} sub="subscriptions / snell / vless" tone="info" />
        <StatCard label="Surge-only" value={sources.snell.length} sub="Snell" tone="warning" />
        <StatCard label="Clash-only" value={sources.vless.length} sub="VLESS" tone="success" />
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
          tag={kind === "snell" ? "仅 Surge" : kind === "vless" ? "仅 Clash" : undefined}
          icon={sourceIcons[kind]}
          readonly={isReadonly}
          className={focusClassName(activePointer, [`/config/sources/${kind}`], "source-section")}
          onAdd={() => setEditor({ type: "fetch", kind, index: null, url: "" })}
        >
          {sources[kind].map((source, index) => (
            <SourceCard
              key={`${kind}-${index}`}
              title={`${sourceLabels[kind]} #${index + 1}`}
              subtitle={maskUrl(source.url) || "未设置 URL"}
              meta={kind}
              tag={source.url ? "已配置" : "待补齐"}
              readonly={isReadonly}
              onEdit={() => setEditor({ type: "fetch", kind, index, url: source.url })}
              onDelete={() => void deleteFetchSource(kind, index)}
            />
          ))}
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
        {sources.custom_proxies.map((proxy, index) => (
          <SourceCard
            key={`${proxy.name}-${index}`}
            title={proxy.name || `自定义代理 #${index + 1}`}
            subtitle={proxy.url ? maskUrl(proxy.url) : "ss://、socks5:// 或 http://"}
            meta={proxy.url.split("://")[0] || "custom"}
            tag={proxy.relay_through ? `中转 · ${proxy.relay_through.strategy}` : "直连"}
            readonly={isReadonly}
            onEdit={() => setEditor({ type: "custom", index, proxy })}
            onDelete={() => void deleteCustomProxy(index)}
          />
        ))}
      </SourceSection>

      {totalSources === 0 ? <EmptyState title="配置中没有任何来源" message="A1 可先添加订阅或自定义代理；保存前仍可切换其他编辑页继续补齐配置。" /> : null}

      <SourceEditorModal editor={editor} draft={draft} readonly={isReadonly} onChange={setEditor} onClose={() => setEditor(null)} onSave={saveEditor} />
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
          {icon ? <span className="source-section-icon" aria-hidden="true">{icon}</span> : null}
          <div>
            <h3>{title}</h3>
            <p>{subtitle}</p>
          </div>
        </div>
        <Chip>{count}</Chip>
        {tag ? <Chip tone={tag.includes("Surge") ? "warning" : "info"}>{tag}</Chip> : null}
      </div>
      <div className="source-card-list">
        {count === 0 ? <EmptyState title="暂无来源" message={readonly ? "只读模式下不可新增来源。" : "添加一个来源后再保存生效。"} /> : children}
        <button className="add-dashed" type="button" disabled={readonly} onClick={onAdd}>
          <Plus size={15} aria-hidden="true" />
          添加{title}
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
  onEdit,
  onDelete
}: {
  title: string;
  subtitle: string;
  meta: string;
  tag: string;
  readonly: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <article className="source-card">
      <span className="drag-handle static" aria-hidden="true">⠿</span>
      <div className="source-card-main">
        <code>{subtitle}</code>
        <div className="source-card-meta">
          <strong>{title}</strong>
          <Chip tone="neutral">{meta}</Chip>
          <Chip tone={tag.includes("待") ? "warning" : "success"}>{tag}</Chip>
        </div>
      </div>
      <div className="source-card-actions">
        <IconButton label="刷新来源" variant="ghost" disabled>
          <RefreshCw size={15} aria-hidden="true" />
        </IconButton>
        <IconButton label="编辑来源" variant="ghost" disabled={readonly} onClick={onEdit}>
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
  onChange,
  onClose,
  onSave
}: {
  editor: SourceEditor;
  draft: Config | undefined;
  readonly: boolean;
  onChange: (editor: SourceEditor) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const groupNames = draft?.groups?.map((entry) => entry.key).filter(Boolean) ?? [];
  const isCustom = editor?.type === "custom";
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
      description={isCustom ? "支持单节点 URL 与 relay_through 中转配置。" : "URL 会在列表中脱敏展示，原始值只保存在草稿里。"}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button variant="primary" disabled={readonly || !editor} onClick={onSave}>保存来源</Button>
        </>
      }
    >
      {editor?.type === "fetch" ? (
        <Field label="订阅 URL" hint={editor.url ? maskUrl(editor.url) : "必须为 HTTP(S) URL"}>
          <TextInput className="text-input mono-input" value={editor.url} disabled={readonly} onChange={(event) => onChange({ ...editor, url: event.target.value })} autoFocus />
        </Field>
      ) : editor?.type === "custom" ? (
        <div className="form-stack">
          <div className="form-grid two">
            <Field label="名称">
              <TextInput value={editor.proxy.name} disabled={readonly} onChange={(event) => patchCustom({ name: event.target.value })} autoFocus />
            </Field>
            <Field label="URL" hint={editor.proxy.url ? maskUrl(editor.proxy.url) : "ss://、socks5:// 或 http://"}>
              <TextInput className="text-input mono-input" value={editor.proxy.url} disabled={readonly} onChange={(event) => patchCustom({ url: event.target.value })} />
            </Field>
          </div>
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
