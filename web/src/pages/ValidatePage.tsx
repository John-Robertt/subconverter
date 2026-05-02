import { useMutation } from "@tanstack/react-query";
import { ArrowRight, CheckCircle2, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { Diagnostic, ValidateResult } from "../api/types";
import { Button, EmptyState, ErrorState, LoadingState, StatCard, StatusBadge } from "../components/ui";
import { diagnosticTarget, diagnosticsFromResult } from "../features/diagnostics";
import { useConfigState } from "../state/config";
import { useToast } from "../state/toast";

export function ValidatePage() {
  const { draft, status } = useConfigState();
  const navigate = useNavigate();
  const { pushToast } = useToast();
  const [selectedDiagnostic, setSelectedDiagnostic] = useState<Diagnostic | null>(null);
  const validateMutation = useMutation({
    mutationFn: () => {
      if (!draft) throw new Error("配置尚未加载");
      return api.validateConfig(draft);
    },
    onSuccess: (result) => {
      const total = result.errors.length + result.warnings.length + result.infos.length;
      pushToast({
        kind: result.valid ? "success" : "warning",
        title: result.valid ? "静态校验通过" : "静态校验发现问题",
        message: result.valid ? "当前草稿没有静态诊断项。" : `发现 ${total} 个诊断项。`
      });
    }
  });

  const result = validateMutation.data;
  const diagnostics = diagnosticsFromResult(result);

  function locateDiagnostic(diagnostic: Diagnostic) {
    const target = diagnosticTarget(diagnostic);
    navigate(target.path, { state: { diagnosticPointer: target.pointer } });
  }

  return (
    <div className="page-stack validate-page">
      {status?.config_dirty ? <section className="content-panel info-panel">当前校验对象是前端草稿；已保存配置尚未 reload 时，运行时预览仍使用旧 RuntimeConfig。</section> : null}

      <div className="stats-grid three">
        <StatCard label="错误" value={result?.errors.length ?? "-"} sub="阻塞保存 / reload" tone={result?.errors.length ? "error" : "success"} />
        <StatCard label="警告" value={result?.warnings.length ?? "-"} sub="建议修复" tone={result?.warnings.length ? "warning" : "neutral"} />
        <StatCard label="提示" value={result?.infos.length ?? "-"} sub="可选优化" tone={result?.infos.length ? "info" : "neutral"} />
      </div>

      <section className="content-panel">
        <div className="section-heading row">
          <div>
            <h3>静态配置校验</h3>
            <p>调用后端 Prepare 校验，不拉取订阅、不生成目标配置。</p>
          </div>
          <Button variant="primary" icon={<ShieldCheck size={16} aria-hidden="true" />} loading={validateMutation.isPending} disabled={!draft} onClick={() => validateMutation.mutate()}>
            运行静态校验
          </Button>
        </div>
        {validateMutation.isPending ? <LoadingState message="正在运行静态配置校验" /> : null}
        {validateMutation.error ? <ErrorState message={getErrorMessage(validateMutation.error)} action={<Button variant="secondary" onClick={() => validateMutation.mutate()}>重试校验</Button>} /> : null}
        {!result && !validateMutation.isPending && !validateMutation.error ? <EmptyState title="尚未运行校验" message="运行后会按 errors / warnings / infos 分级展示诊断，并可跳转到字段。" /> : null}
        {result ? <ValidationResult result={result} onOpen={setSelectedDiagnostic} /> : null}
      </section>

      {selectedDiagnostic ? <DiagnosticDrawer diagnostic={selectedDiagnostic} onClose={() => setSelectedDiagnostic(null)} onLocate={locateDiagnostic} /> : null}
    </div>
  );
}

function ValidationResult({ result, onOpen }: { result: ValidateResult; onOpen: (diagnostic: Diagnostic) => void }) {
  if (result.valid && diagnosticsFromResult(result).length === 0) {
    return (
      <div className="validate-success">
        <CheckCircle2 size={20} aria-hidden="true" />
        <strong>静态校验通过</strong>
        <p>当前草稿通过 Prepare 阶段校验；生成可用性仍需通过 B2/B3 预览确认。</p>
      </div>
    );
  }

  return (
    <div className="validate-groups">
      <DiagnosticSection title="错误" tone="error" items={result.errors} onOpen={onOpen} />
      <DiagnosticSection title="警告" tone="warning" items={result.warnings} onOpen={onOpen} />
      <DiagnosticSection title="提示" tone="info" items={result.infos} onOpen={onOpen} />
    </div>
  );
}

function DiagnosticSection({
  title,
  tone,
  items,
  onOpen
}: {
  title: string;
  tone: "error" | "warning" | "info";
  items: Diagnostic[];
  onOpen: (diagnostic: Diagnostic) => void;
}) {
  if (items.length === 0) return null;
  return (
    <section className="diagnostic-section">
      <div className="section-heading row">
        <h3>{title}</h3>
        <StatusBadge tone={tone}>{items.length}</StatusBadge>
      </div>
      <div className="diagnostic-table">
        {items.map((item, index) => (
          <button key={`${item.code}-${index}`} type="button" className={`diagnostic-row diagnostic-row-${tone}`} onClick={() => onOpen(item)}>
            <span className="diagnostic-dot" aria-hidden="true" />
            <strong>{item.code}</strong>
            <span>{item.message}</span>
            <code>{item.locator?.json_pointer ?? "未定位"}</code>
            <ArrowRight size={15} aria-hidden="true" />
          </button>
        ))}
      </div>
    </section>
  );
}

function DiagnosticDrawer({
  diagnostic,
  onClose,
  onLocate
}: {
  diagnostic: Diagnostic;
  onClose: () => void;
  onLocate: (diagnostic: Diagnostic) => void;
}) {
  const target = diagnosticTarget(diagnostic);
  return (
    <aside className="diagnostic-drawer" role="dialog" aria-modal="true" aria-labelledby="diagnostic-drawer-title">
      <header className="diagnostic-drawer-header">
        <div>
          <code>{target.pointer}</code>
          <h3 id="diagnostic-drawer-title">{diagnostic.message}</h3>
        </div>
        <button type="button" className="icon-button ghost" aria-label="关闭诊断详情" onClick={onClose}>
          <X size={16} aria-hidden="true" />
        </button>
      </header>
      <div className="diagnostic-drawer-body">
        <StatusBadge tone={diagnostic.severity === "error" ? "error" : diagnostic.severity === "warning" ? "warning" : "info"}>{diagnostic.severity}</StatusBadge>
        <dl>
          <div>
            <dt>Code</dt>
            <dd><code>{diagnostic.code}</code></dd>
          </div>
          <div>
            <dt>Display path</dt>
            <dd>{diagnostic.display_path ?? "未提供"}</dd>
          </div>
          <div>
            <dt>Section</dt>
            <dd>{diagnostic.locator?.section ?? "未提供"}</dd>
          </div>
        </dl>
        <p>跳转时仅使用 `locator.json_pointer` 映射页面和字段，`display_path` 只作为可读文案。</p>
      </div>
      <footer className="diagnostic-drawer-footer">
        <Button variant="ghost" onClick={onClose}>关闭</Button>
        <Button variant="primary" icon={<ArrowRight size={15} aria-hidden="true" />} onClick={() => onLocate(diagnostic)}>
          跳转字段
        </Button>
      </footer>
    </aside>
  );
}
