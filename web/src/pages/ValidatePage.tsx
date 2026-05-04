import { useMutation } from "@tanstack/react-query";
import { ArrowRight, CheckCircle2, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import type { Diagnostic, ValidateResult } from "../api/types";
import { Button, EmptyState, ErrorState, LoadingState, StatusBadge } from "../components/ui";
import { diagnosticTarget, diagnosticsFromResult } from "../features/diagnostics";
import { useConfigState } from "../state/config";
import { useToast } from "../state/toast";

export function ValidatePage() {
  const { draft, status } = useConfigState();
  const navigate = useNavigate();
  const location = useLocation();
  const { pushToast } = useToast();
  const [selectedDiagnostic, setSelectedDiagnostic] = useState<Diagnostic | null>(null);
  const initialResult = (location.state as { validateResult?: ValidateResult } | null)?.validateResult ?? null;
  const [result, setResult] = useState<ValidateResult | null>(initialResult);
  const validateMutation = useMutation({
    mutationFn: () => {
      if (!draft) throw new Error("配置尚未加载");
      return api.validateConfig(draft);
    },
    onSuccess: (data) => {
      setResult(data);
      const total = data.errors.length + data.warnings.length + data.infos.length;
      pushToast({
        kind: data.valid ? "success" : "warning",
        title: data.valid ? "静态校验通过" : "静态校验发现问题",
        message: data.valid ? "当前草稿没有静态诊断项。" : `发现 ${total} 个诊断项。`
      });
    }
  });

  const diagnostics = diagnosticsFromResult(result ?? undefined);

  function locateDiagnostic(diagnostic: Diagnostic) {
    const target = diagnosticTarget(diagnostic);
    navigate(target.path, { state: { diagnosticPointer: target.pointer } });
  }

  return (
    <div className="page-stack validate-page">
      {status?.config_dirty ? <section className="content-panel info-panel">当前校验对象是前端草稿；已保存配置尚未 reload 时，运行时预览仍使用旧 RuntimeConfig。</section> : null}

      <div className="stats-grid three">
        <div className="summary-stat summary-stat-error">
          <div className="summary-stat-row">
            {result ? <strong>{result.errors.length}</strong> : null}
            <span>错误</span>
          </div>
          <small>必须修复才能保存</small>
        </div>
        <div className="summary-stat summary-stat-warning">
          <div className="summary-stat-row">
            {result ? <strong>{result.warnings.length}</strong> : null}
            <span>警告</span>
          </div>
          <small>建议修复但不阻塞</small>
        </div>
        <div className="summary-stat summary-stat-info">
          <div className="summary-stat-row">
            {result ? <strong>{result.infos.length}</strong> : null}
            <span>提示</span>
          </div>
          <small>可选优化建议</small>
        </div>
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
        <span className="nav-section-label">{title} · {items.length}</span>
      </div>
      <div className="content-panel" style={{ padding: 0, overflow: "hidden" }}>
        {items.map((item, index) => (
          <div key={`${item.code}-${index}`} className={`diagnostic-row diagnostic-row-${tone}`} style={{ display: "flex", alignItems: "flex-start", gap: 14, padding: "14px 18px", borderBottom: index < items.length - 1 ? "1px solid var(--border)" : "none", cursor: "pointer", gridTemplateColumns: "unset" }} onClick={() => onOpen(item)}>
            <span className="diagnostic-dot" style={{ marginTop: 7, flex: "0 0 8px" }} aria-hidden="true" />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
                <strong style={{ fontSize: 14, fontWeight: 600 }}>{item.message}</strong>
                <code style={{ fontSize: 11, padding: "2px 8px", borderRadius: 4, background: "var(--surface-muted)", color: "var(--text-muted)" }}>{item.code}</code>
              </div>
              {item.locator?.json_pointer ? <div style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 4 }}>{item.locator.json_pointer}</div> : null}
            </div>
            <Button variant="secondary" onClick={(event) => { event.stopPropagation(); onOpen(item); }}>跳转</Button>
          </div>
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
