import { useMutation, useQuery } from "@tanstack/react-query";
import { Clipboard, FileDown } from "lucide-react";
import { useMemo, type ReactNode } from "react";
import { api, buildGeneratePath } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { GenerateFormat } from "../api/types";
import { Button, Chip, ErrorState, LoadingState, StatCard } from "../components/ui";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";
import { useToast } from "../state/toast";

export function DownloadPage() {
  const { draft, status } = useConfigState();
  const confirm = useConfirm();
  const { pushToast } = useToast();
  const runtimeRevision = status?.runtime_config_revision;

  const nodesQuery = useQuery({
    queryKey: ["previewNodes", runtimeRevision],
    queryFn: api.previewNodes,
    enabled: Boolean(runtimeRevision)
  });

  const clashPreviewQuery = useQuery({
    queryKey: ["generatePreview", "clash", runtimeRevision],
    queryFn: () => api.generatePreview("clash"),
    enabled: Boolean(runtimeRevision)
  });

  const surgePreviewQuery = useQuery({
    queryKey: ["generatePreview", "surge", runtimeRevision],
    queryFn: () => api.generatePreview("surge"),
    enabled: Boolean(runtimeRevision)
  });

  const clashLinkQuery = useQuery({
    queryKey: ["generateLink", "clash", runtimeRevision],
    queryFn: () => api.generateLink("clash", "", false),
    enabled: Boolean(runtimeRevision)
  });

  const surgeLinkQuery = useQuery({
    queryKey: ["generateLink", "surge", runtimeRevision],
    queryFn: () => api.generateLink("surge", "", false),
    enabled: Boolean(runtimeRevision)
  });

  const linkMutation = useMutation({
    mutationFn: (format: GenerateFormat) => api.generateLink(format, "", true),
    onSuccess: async (result) => {
      if (result.token_included) {
        const accepted = await confirm({
          title: "复制含 token 的订阅链接？",
          message: "token 会进入 URL，可能出现在客户端配置、浏览器历史或代理日志中。",
          confirmLabel: "确认复制"
        });
        if (!accepted) return;
      }
      await copyToClipboard(result.url);
      pushToast({
        kind: "success",
        title: "订阅链接已复制",
        message: result.token_included ? "链接包含服务端订阅访问 token。" : "链接未包含订阅访问 token。"
      });
    },
    onError: (error) => {
      pushToast({ kind: "error", title: "订阅链接生成失败", message: getErrorMessage(error), persistent: true });
    }
  });

  const stats = useMemo(() => deriveStats(draft, nodesQuery.data), [draft, nodesQuery.data]);
  const hasBaseURL = Boolean(draft?.base_url?.trim());
  const previewError = clashPreviewQuery.error ?? surgePreviewQuery.error;
  const previewLoading = clashPreviewQuery.isLoading || surgePreviewQuery.isLoading;

  return (
    <div className="page-stack download-page">
      {status?.config_dirty ? (
        <section className="content-panel info-panel">
          当前预览基于运行时配置，已保存草稿尚未 reload；如需查看草稿生成结果，请先点击"热重载"。
        </section>
      ) : null}

      <div className="stats-grid">
        <StatCard label="节点（过滤后）" value={stats.activeNodes} sub={`原始 ${stats.totalNodes}`} />
        <StatCard label="分组" value={stats.groupsTotal} sub={`${stats.groupsSelect} select · ${stats.groupsUrlTest} url-test`} />
        <StatCard label="服务组" value={stats.serviceTotal} sub={`${stats.serviceWithRuleset} 带规则集`} />
        <StatCard label="规则总数" value={stats.totalRules} sub={`${stats.rulesetUrls} 含规则集`} />
      </div>

      {previewError ? (
        <ErrorState message={getErrorMessage(previewError)} action={<Button variant="secondary" onClick={() => { void clashPreviewQuery.refetch(); void surgePreviewQuery.refetch(); }}>重试生成</Button>} />
      ) : null}

      {previewLoading && !previewError ? <LoadingState message="正在生成双格式预览" /> : null}

      <div className="preview-grid">
        <PreviewCard
          title="Clash Meta"
          badge="YAML"
          filename="config.yaml"
          format="clash"
          previewText={clashPreviewQuery.data ?? null}
          downloadPath={buildGeneratePath("clash", "config.yaml")}
          subUrl={clashLinkQuery.data?.url ?? buildGeneratePath("clash", "config.yaml")}
          subUrlChip="SUB URL"
          onCopyLink={() => linkMutation.mutate("clash")}
          linkLoading={linkMutation.isPending && linkMutation.variables === "clash"}
        />
        <PreviewCard
          title="Surge"
          badge="conf"
          filename="config.conf"
          format="surge"
          previewText={surgePreviewQuery.data ?? null}
          downloadPath={buildGeneratePath("surge", "config.conf")}
          subUrl={surgeLinkQuery.data?.url ?? buildGeneratePath("surge", "config.conf")}
          subUrlChip={hasBaseURL ? "MANAGED" : "SUB URL"}
          onCopyLink={() => linkMutation.mutate("surge")}
          linkLoading={linkMutation.isPending && linkMutation.variables === "surge"}
        />
      </div>
    </div>
  );
}

interface PreviewCardProps {
  title: string;
  badge: string;
  filename: string;
  format: GenerateFormat;
  previewText: string | null;
  downloadPath: string;
  subUrl: string;
  subUrlChip: string;
  onCopyLink: () => void;
  linkLoading: boolean;
}

function PreviewCard({ title, badge, filename, format, previewText, downloadPath, subUrl, subUrlChip, onCopyLink, linkLoading }: PreviewCardProps) {
  const sizeKb = previewText ? `${(previewText.length / 1024).toFixed(1)} KB` : "—";
  return (
    <section className="code-preview-panel">
      <header>
        <div className="preview-header-meta">
          <Chip tone="neutral">{badge}</Chip>
          <div>
            <h3>{title}</h3>
            <p>{filename} · {sizeKb}</p>
          </div>
        </div>
        <div className="page-actions">
          <Button variant="primary" icon={<FileDown size={15} aria-hidden="true" />} onClick={() => window.location.assign(downloadPath)}>
            下载
          </Button>
        </div>
      </header>
      <div className="sub-url-row">
        <span className="sub-url-chip">{subUrlChip}</span>
        <code>{subUrl}</code>
        <Button variant="secondary" icon={<Clipboard size={13} aria-hidden="true" />} loading={linkLoading} onClick={onCopyLink}>
          复制
        </Button>
      </div>
      {previewText ? <CodePreview text={previewText} format={format} /> : <div className="code-preview placeholder">正在加载预览…</div>}
    </section>
  );
}

function CodePreview({ text, format }: { text: string; format: GenerateFormat }) {
  const lines = text.split("\n");
  return (
    <pre className="code-preview">
      {lines.map((line, index) => (
        <span key={`${index}-${line}`}>
          <em>{index + 1}</em>
          <code>{highlightLine(line, format) ?? " "}</code>
        </span>
      ))}
    </pre>
  );
}

function highlightLine(line: string, format: GenerateFormat): ReactNode {
  if (line.length === 0) return null;
  return format === "clash" ? highlightYAML(line) : highlightSurge(line);
}

function highlightYAML(line: string): ReactNode {
  if (/^\s*#/.test(line)) return <span className="hl-comment">{line}</span>;
  const kv = line.match(/^(\s*-?\s*)([\w./-]+)(:)(.*)$/);
  if (kv) {
    return (
      <>
        <span>{kv[1]}</span>
        <span className="hl-key">{kv[2]}</span>
        <span className="hl-punct">{kv[3]}</span>
        {kv[4] ? <span className="hl-string">{kv[4]}</span> : null}
      </>
    );
  }
  return <span>{line}</span>;
}

function highlightSurge(line: string): ReactNode {
  if (/^\s*(#|\/\/)/.test(line)) return <span className="hl-comment">{line}</span>;
  const section = line.match(/^(\s*\[)([^\]]+)(\])\s*$/);
  if (section) {
    return (
      <>
        <span className="hl-punct">{section[1]}</span>
        <span className="hl-key">{section[2]}</span>
        <span className="hl-punct">{section[3]}</span>
      </>
    );
  }
  const kv = line.match(/^(\s*)([^=,\s][^=,]*?)(\s*=\s*)(.*)$/);
  if (kv) {
    return (
      <>
        <span>{kv[1]}</span>
        <span className="hl-key">{kv[2]}</span>
        <span className="hl-punct">{kv[3]}</span>
        <span className="hl-string">{kv[4]}</span>
      </>
    );
  }
  return <span>{line}</span>;
}

interface DownloadStats {
  activeNodes: number | string;
  totalNodes: number | string;
  groupsTotal: number;
  groupsSelect: number;
  groupsUrlTest: number;
  serviceTotal: number;
  serviceWithRuleset: number;
  totalRules: number;
  rulesetUrls: number;
}

function deriveStats(
  draft: ReturnType<typeof useConfigState>["draft"],
  nodes: { total: number; active_count: number } | undefined
): DownloadStats {
  const groups = draft?.groups ?? [];
  const routing = draft?.routing ?? [];
  const rulesets = draft?.rulesets ?? [];
  const rules = draft?.rules ?? [];

  const groupsSelect = groups.filter((g) => g.value?.strategy === "select").length;
  const groupsUrlTest = groups.filter((g) => g.value?.strategy === "url-test").length;
  const rulesetKeys = new Set(rulesets.map((entry) => entry.key));
  const serviceWithRuleset = routing.filter((entry) => rulesetKeys.has(entry.key)).length;
  const rulesetUrls = rulesets.reduce((sum, entry) => sum + (entry.value?.length ?? 0), 0);

  return {
    activeNodes: nodes?.active_count ?? "—",
    totalNodes: nodes?.total ?? "—",
    groupsTotal: groups.length,
    groupsSelect,
    groupsUrlTest,
    serviceTotal: routing.length,
    serviceWithRuleset,
    totalRules: rules.length + rulesetUrls,
    rulesetUrls
  };
}

async function copyToClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}
