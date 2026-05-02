import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, Check, Info, X } from "lucide-react";

export type ToastKind = "success" | "error" | "warning" | "info";

export interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  message?: string;
  persistent?: boolean;
  action?: {
    label: string;
    onClick: () => void;
  };
}

interface ToastContextValue {
  pushToast: (toast: Omit<Toast, "id">) => void;
  dismissToast: (id: number) => void;
}

const ToastContext = createContext<ToastContextValue | undefined>(undefined);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const dismissToast = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const pushToast = useCallback(
    (toast: Omit<Toast, "id">) => {
      const id = Date.now() + Math.floor(Math.random() * 1000);
      const nextToast = { ...toast, id };
      setToasts((current) => [nextToast, ...current].slice(0, 5));
      if (!toast.persistent && toast.kind === "success") {
        window.setTimeout(() => dismissToast(id), 3600);
      }
    },
    [dismissToast]
  );

  const value = useMemo(() => ({ pushToast, dismissToast }), [dismissToast, pushToast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-region" aria-live="polite" aria-label="操作反馈">
        {toasts.map((toast) => (
          <article key={toast.id} className={`toast toast-${toast.kind}`}>
            <span className="toast-icon" aria-hidden="true">
              {toast.kind === "success" ? <Check size={15} /> : toast.kind === "info" ? <Info size={15} /> : <AlertTriangle size={15} />}
            </span>
            <div>
              <strong>{toast.title}</strong>
              {toast.message ? <p>{toast.message}</p> : null}
              {toast.action ? (
                <button
                  type="button"
                  className="toast-action"
                  onClick={() => {
                    toast.action?.onClick();
                    dismissToast(toast.id);
                  }}
                >
                  {toast.action.label}
                </button>
              ) : null}
            </div>
            <button type="button" className="icon-button ghost small" aria-label="关闭提示" onClick={() => dismissToast(toast.id)}>
              <X size={15} aria-hidden="true" />
            </button>
          </article>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const value = useContext(ToastContext);
  if (!value) {
    throw new Error("useToast must be used inside ToastProvider");
  }
  return value;
}
