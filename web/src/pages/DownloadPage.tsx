import { useMutation } from "@tanstack/react-query";
import { Clipboard, Download, FileDown, RefreshCcw } from "lucide-react";
import { useMemo, useState } from "react";
import { api, buildGeneratePath } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { GenerateFormat } from "../api/types";
import { Button, Chip, EmptyState, ErrorState, Field, LoadingState, StatCard, TextInput } from "../components/ui";
import { useConfigState } from "../state/config";
import { useConfirm } from "../state/confirm";
import { useToast } from "../state/toast";

type PreviewMode = "runtime" | "draft";

interface PreviewState {
  format: GenerateFormat;
  mode: PreviewMode;
  text: string;
}

export function DownloadPage() {
  const { draft, status } = useConfigState();
  const confirm = useConfirm();
  const { pushToast } = useToast();
  const [format, setFormat] = useState<GenerateFormat>("clash");
  const [filename, setFilename] = useState("");
  const [includeToken, setIncludeToken] = useState(true);
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const downloadPath = useMemo(() => buildGeneratePath(format, filename), [filename, format]);

  const previewMutation = useMutation({
    mutationFn: async (mode: PreviewMode) => {
      if (mode === "draft") {
        if (!draft) throw new Error("配置尚未加载");
        return api.generatePreviewDraft(format, draft);
      }
      return api.generatePreview(format);
    },
    onSuccess: (text, mode) => {
      setPreview({ format, mode, text });
    }
  });

  const linkMutation = useMutation({
    mutationFn: () => api.generateLink(format, filename, includeToken),
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

  function startDownload() {
    window.location.assign(downloadPath);
  }

  return (
    <div className="page-stack download-page">
      {status?.config_dirty ? (
        <section className="content-panel info-panel">
          当前运行时预览和下载仍基于旧 RuntimeConfig；可使用“草稿生成预览”检查尚未 reload 的草稿。
        </section>
      ) : null}

      <div className="stats-grid">
        <StatCard label="格式" value={format === "clash" ? "Clash" : "Surge"} sub={format === "clash" ? "text/yaml" : "text/plain"} tone="info" />
        <StatCard label="Runtime revision" value={status?.runtime_config_revision ? "已加载" : "-"} sub={status?.runtime_config_revision ?? "等待状态"} />
        <StatCard label="草稿状态" value={status?.config_dirty ? "dirty" : "clean"} sub="config_revision vs runtime" tone={status?.config_dirty ? "warning" : "success"} />
        <StatCard label="预览来源" value={preview ? (preview.mode === "runtime" ? "运行时" : "草稿") : "-"} sub={preview ? preview.format : "尚未预览"} />
      </div>

      <section className="content-panel download-controls">
        <div className="form-grid three">
          <div className="field">
            <span className="field-label">目标格式</span>
            <div className="format-segmented" role="radiogroup" aria-label="目标格式">
              <FormatOption value="clash" label="Clash Meta" detail="text/yaml" current={format} onChange={setFormat} />
              <FormatOption value="surge" label="Surge" detail="text/plain" current={format} onChange={setFormat} />
            </div>
          </div>
          <Field label="文件名" hint="可选；后端会校验安全 ASCII 文件名。">
            <TextInput className="mono-input" value={filename} onChange={(event) => setFilename(event.target.value)} placeholder={format === "clash" ? "clash.yaml" : "surge.conf"} />
          </Field>
          <label className="checkbox-row token-checkbox">
            <input type="checkbox" checked={includeToken} onChange={(event) => setIncludeToken(event.target.checked)} />
            <span>复制订阅链接时请求服务端附带 token</span>
          </label>
        </div>
        <div className="download-action-row">
          <Button variant="secondary" icon={<RefreshCcw size={16} aria-hidden="true" />} loading={previewMutation.isPending} onClick={() => previewMutation.mutate("runtime")}>
            当前运行时预览
          </Button>
          <Button variant="secondary" icon={<RefreshCcw size={16} aria-hidden="true" />} loading={previewMutation.isPending} disabled={!draft} onClick={() => previewMutation.mutate("draft")}>
            草稿生成预览
          </Button>
          <Button variant="primary" icon={<FileDown size={16} aria-hidden="true" />} onClick={startDownload}>
            下载配置
          </Button>
          <Button variant="secondary" icon={<Clipboard size={16} aria-hidden="true" />} loading={linkMutation.isPending} onClick={() => linkMutation.mutate()}>
            复制订阅链接
          </Button>
        </div>
        <div className="setting-preview">
          <Download size={15} aria-hidden="true" />
          <span>下载路径</span>
          <code>{downloadPath}</code>
        </div>
      </section>

      {previewMutation.isPending ? <LoadingState message={`正在生成 ${format} ${previewMutation.variables === "draft" ? "草稿" : "运行时"}预览`} /> : null}
      {previewMutation.error ? <ErrorState message={getErrorMessage(previewMutation.error)} action={<Button variant="secondary" onClick={() => previewMutation.mutate(previewMutation.variables ?? "runtime")}>重试生成</Button>} /> : null}

      <section className="code-preview-panel">
        {preview ? (
          <>
            <header>
              <div>
                <h3>{preview.format === "clash" ? "Clash Meta" : "Surge"} 生成预览</h3>
                <p>{preview.mode === "runtime" ? "当前 RuntimeConfig" : "前端草稿"} · {preview.text.length.toLocaleString()} bytes</p>
              </div>
              <Chip tone={preview.mode === "runtime" ? "info" : "warning"}>{preview.mode}</Chip>
            </header>
            <CodePreview text={preview.text} />
          </>
        ) : (
          <EmptyState title="尚未生成预览" message="先选择格式，再运行当前预览或草稿预览；下载按钮会直接调用后端 `/generate`。" />
        )}
      </section>
    </div>
  );
}

function FormatOption({
  value,
  label,
  detail,
  current,
  onChange
}: {
  value: GenerateFormat;
  label: string;
  detail: string;
  current: GenerateFormat;
  onChange: (value: GenerateFormat) => void;
}) {
  const checked = current === value;
  return (
    <label className={checked ? "format-option active" : "format-option"}>
      <input type="radio" name="generate-format" value={value} checked={checked} onChange={() => onChange(value)} />
      <span>
        <strong>{label}</strong>
        <small>{detail}</small>
      </span>
    </label>
  );
}

function CodePreview({ text }: { text: string }) {
  const lines = text.split("\n");
  return (
    <pre className="code-preview">
      {lines.map((line, index) => (
        <span key={`${index}-${line}`}>
          <em>{index + 1}</em>
          <code>{line || " "}</code>
        </span>
      ))}
    </pre>
  );
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
