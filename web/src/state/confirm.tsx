import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "../components/ui";

interface ConfirmRequest {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

interface PendingConfirm extends ConfirmRequest {
  resolve: (value: boolean) => void;
}

const ConfirmContext = createContext<((request: ConfirmRequest) => Promise<boolean>) | undefined>(undefined);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<PendingConfirm | null>(null);

  const confirm = useCallback((request: ConfirmRequest) => {
    return new Promise<boolean>((resolve) => {
      setPending({ ...request, resolve });
    });
  }, []);

  const complete = useCallback(
    (value: boolean) => {
      pending?.resolve(value);
      setPending(null);
    },
    [pending]
  );

  const value = useMemo(() => confirm, [confirm]);

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      {pending ? (
        <div className="modal-backdrop" role="presentation">
          <section className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
            <div className={pending.danger ? "confirm-icon danger" : "confirm-icon"}>
              <AlertTriangle size={19} aria-hidden="true" />
            </div>
            <div className="confirm-copy">
              <h2 id="confirm-title">{pending.title}</h2>
              <p>{pending.message}</p>
            </div>
            <div className="dialog-actions">
              <Button variant="ghost" onClick={() => complete(false)}>
                {pending.cancelLabel ?? "取消"}
              </Button>
              <Button variant={pending.danger ? "danger" : "primary"} onClick={() => complete(true)}>
                {pending.confirmLabel ?? "确认"}
              </Button>
            </div>
          </section>
        </div>
      ) : null}
    </ConfirmContext.Provider>
  );
}

export function useConfirm() {
  const value = useContext(ConfirmContext);
  if (!value) {
    throw new Error("useConfirm must be used inside ConfirmProvider");
  }
  return value;
}
