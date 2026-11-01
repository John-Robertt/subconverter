import { useId, type ReactNode } from "react";
import { Button, TextArea } from "./ui";

// Only the textarea follows the expanded state; actions remain visible.
export function ResourceBody({ label, content, readOnly, expanded, onToggle, onChange, actions }: {
  label: string;
  content: string | undefined;
  readOnly: boolean;
  expanded: boolean;
  onToggle: () => void;
  onChange: (content: string) => void;
  actions: ReactNode;
}) {
  const id = useId();
  return (
    <div className="resource-body">
      <div className="resource-body-toolbar">
        <span className="field-label">{label}{readOnly ? " · 只读" : ""}</span>
        <div className="subscription-actions">
          <Button aria-label={`${expanded ? "收起" : "展开"}${label}`} aria-controls={id} aria-expanded={expanded}
            disabled={content === undefined} onClick={onToggle}>{expanded ? "收起正文" : "展开正文"}</Button>
          {actions}
        </div>
      </div>
      <div id={id} hidden={!expanded}>
        <TextArea aria-label={label} className="mono-input" rows={12} spellCheck={false} value={content ?? ""}
          readOnly={readOnly} onChange={(event) => onChange(event.target.value)} />
      </div>
    </div>
  );
}
