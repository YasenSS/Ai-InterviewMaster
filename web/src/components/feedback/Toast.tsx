"use client";

import { CheckCircle2, X } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

type Toast = { id: number; message: string };
type ToastContextValue = { toast: (message: string) => void };

const ToastContext = createContext<ToastContextValue>({ toast: () => undefined });

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const toast = useCallback((message: string) => {
    const id = Date.now();
    setItems((value) => [...value, { id, message }]);
    window.setTimeout(() => setItems((value) => value.filter((item) => item.id !== id)), 3500);
  }, []);
  const value = useMemo(() => ({ toast }), [toast]);
  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-region" aria-live="polite" aria-label="通知">
        {items.map((item) => (
          <div className="toast" key={item.id}>
            <CheckCircle2 size={17} />
            <span>{item.message}</span>
            <button aria-label="关闭通知" onClick={() => setItems((v) => v.filter((x) => x.id !== item.id))}>
              <X size={16} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export const useToast = () => useContext(ToastContext);
