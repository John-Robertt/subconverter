import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";
import { Loader2, X } from "lucide-react";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
  icon?: ReactNode;
}

export function Button({ children, variant = "secondary", loading = false, icon, disabled, ...props }: ButtonProps) {
  return (
    <button className={`button button-${variant}`} disabled={disabled || loading} {...props}>
      {loading ? <Loader2 className="spin" size={16} aria-hidden="true" /> : icon}
      <span>{children}</span>
    </button>
  );
}

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
  variant?: "default" | "ghost" | "danger";
}

export function IconButton({ label, children, variant = "default", ...props }: IconButtonProps) {
  return (
    <button className={`icon-button ${variant}`} aria-label={label} title={label} {...props}>
      {children}
    </button>
  );
}

interface FieldProps {
  label: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}

export function Field({ label, hint, error, children }: FieldProps) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      {children}
      {hint ? <span className="field-hint">{hint}</span> : null}
      {error ? <span className="field-error">{error}</span> : null}
    </label>
  );
}

export function TextInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={className ? `text-input ${className}` : "text-input"} {...props} />;
}

export function TextArea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={className ? `text-input textarea ${className}` : "text-input textarea"} {...props} />;
}

export function SelectInput({ className, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={className ? `text-input select-input ${className}` : "text-input select-input"} {...props} />;
}

interface StatusBadgeProps {
  tone?: "success" | "warning" | "error" | "info" | "neutral";
  children: ReactNode;
}

export function StatusBadge({ tone = "neutral", children }: StatusBadgeProps) {
  return <span className={`status-badge ${tone}`}>{children}</span>;
}

interface PageHeaderProps {
  title: string;
  description?: string;
  actions?: ReactNode;
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="page-title-row">
      <div>
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </div>
  );
}

export function EmptyState({ title, message, action }: { title: string; message: string; action?: ReactNode }) {
  return (
    <div className="empty-state">
      <strong>{title}</strong>
      <p>{message}</p>
      {action}
    </div>
  );
}

export function ErrorState({ title = "请求失败", message, action }: { title?: string; message: string; action?: ReactNode }) {
  return (
    <div className="error-state">
      <strong>{title}</strong>
      <p>{message}</p>
      {action}
    </div>
  );
}

export function LoadingState({ message = "正在加载" }: { message?: string }) {
  return (
    <div className="loading-state">
      <Loader2 className="spin" size={18} aria-hidden="true" />
      <span>{message}</span>
    </div>
  );
}

interface ModalProps {
  open: boolean;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  width?: "default" | "wide";
}

export function Modal({ open, title, description, children, footer, onClose, width = "default" }: ModalProps) {
  if (!open) return null;

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className={`modal-dialog modal-${width}`} role="dialog" aria-modal="true" aria-labelledby="modal-title">
        <header className="modal-header">
          <div>
            <h2 id="modal-title">{title}</h2>
            {description ? <p>{description}</p> : null}
          </div>
          <IconButton label="关闭" variant="ghost" type="button" onClick={onClose}>
            <X size={17} aria-hidden="true" />
          </IconButton>
        </header>
        <div className="modal-body">{children}</div>
        {footer ? <footer className="modal-footer">{footer}</footer> : null}
      </section>
    </div>
  );
}

export function StatCard({
  label,
  value,
  sub,
  tone = "neutral",
  className
}: {
  label: string;
  value: string | number;
  sub?: string;
  tone?: "neutral" | "success" | "warning" | "error" | "info";
  className?: string;
}) {
  return (
    <div className={`stat-card stat-${tone}${className ? ` ${className}` : ""}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      {sub ? <small>{sub}</small> : null}
    </div>
  );
}

export function SplitWorkbench({ children, rail }: { children: ReactNode; rail: ReactNode }) {
  return (
    <div className="split-workbench">
      <div className="workbench-main">{children}</div>
      {rail}
    </div>
  );
}

export function RailPanel({
  eyebrow,
  title,
  children,
  footer
}: {
  eyebrow?: string;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <aside className="rail-panel">
      <header className="rail-header">
        {eyebrow ? <span>{eyebrow}</span> : null}
        <h3>{title}</h3>
      </header>
      <div className="rail-body">{children}</div>
      {footer ? <footer className="rail-footer">{footer}</footer> : null}
    </aside>
  );
}

export function Chip({
  children,
  tone = "neutral",
  removable = false,
  onRemove,
  className
}: {
  children: ReactNode;
  tone?: "neutral" | "accent" | "success" | "warning" | "error" | "info";
  removable?: boolean;
  onRemove?: () => void;
  className?: string;
}) {
  const cls = className ? `chip chip-${tone} ${className}` : `chip chip-${tone}`;
  return (
    <span className={cls}>
      {children}
      {removable ? (
        <button type="button" aria-label="移除" onClick={onRemove}>
          <X size={12} aria-hidden="true" />
        </button>
      ) : null}
    </span>
  );
}

interface CategoryPillProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
  count?: number;
  active?: boolean;
  tag?: string;
}

export function CategoryPill({ label, count, active = false, tag, ...props }: CategoryPillProps) {
  return (
    <button className={active ? "category-pill active" : "category-pill"} type="button" {...props}>
      <span>{label}</span>
      {typeof count === "number" ? <small>{count}</small> : null}
      {tag ? <em>{tag}</em> : null}
    </button>
  );
}
