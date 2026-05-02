import type { Diagnostic, ValidateResult } from "../api/types";

export function DiagnosticList({ result, onLocate }: { result: ValidateResult; onLocate?: (diagnostic: Diagnostic) => void }) {
  const diagnostics: Diagnostic[] = [...result.errors, ...result.warnings, ...result.infos];

  if (diagnostics.length === 0) {
    return <p className="diagnostic-empty">静态校验通过，没有诊断项。</p>;
  }

  return (
    <ul className="diagnostic-list">
      {diagnostics.map((item, index) => (
        <li key={`${item.code}-${index}`} className={`diagnostic diagnostic-${item.severity}`}>
          <strong>{item.code}</strong>
          <span>{item.message}</span>
          <code>{item.locator?.json_pointer ?? item.display_path ?? "未定位"}</code>
          {onLocate && item.locator?.json_pointer ? (
            <button type="button" className="diagnostic-action" onClick={() => onLocate(item)}>
              定位字段
            </button>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
