import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CheckCircle2, AlertCircle, Info, X } from "lucide-react";

// Toast system: a lightweight, dependency-free notifier styled with the
// KeiRouter design system. Wrap the app in <ToastProvider> and call useToast()
// to push success / error / info messages from anywhere.

type ToastTone = "success" | "error" | "info";

interface Toast {
  id: number;
  tone: ToastTone;
  title: string;
  description?: string;
}

interface ToastAPI {
  toast: (t: { tone?: ToastTone; title: string; description?: string }) => void;
  success: (title: string, description?: string) => void;
  error: (title: string, description?: string) => void;
  info: (title: string, description?: string) => void;
}

const ToastContext = createContext<ToastAPI | null>(null);

// useToast returns the imperative toast API. Must be used within ToastProvider.
export function useToast(): ToastAPI {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return ctx;
}

const AUTO_DISMISS_MS = 5000;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const idRef = useRef(0);
  const timers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
  }, []);

  const push = useCallback(
    (t: { tone?: ToastTone; title: string; description?: string }) => {
      const id = ++idRef.current;
      const toast: Toast = { id, tone: t.tone ?? "info", title: t.title, description: t.description };
      setToasts((prev) => [...prev, toast]);
      const timer = setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
      timers.current.set(id, timer);
    },
    [dismiss],
  );

  useEffect(() => {
    const map = timers.current;
    return () => {
      map.forEach((t) => clearTimeout(t));
      map.clear();
    };
  }, []);

  const api: ToastAPI = {
    toast: push,
    success: (title, description) => push({ tone: "success", title, description }),
    error: (title, description) => push({ tone: "error", title, description }),
    info: (title, description) => push({ tone: "info", title, description }),
  };

  return (
    <ToastContext.Provider value={api}>
      {children}
      <ToastViewport toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  );
}

function ToastViewport({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-full max-w-sm flex-col gap-2">
      {toasts.map((t) => (
        <ToastCard key={t.id} toast={t} onDismiss={() => onDismiss(t.id)} />
      ))}
    </div>
  );
}

const toneMeta: Record<
  ToastTone,
  { icon: typeof Info; iconClass: string; ring: string }
> = {
  success: {
    icon: CheckCircle2,
    iconClass: "text-accent-600",
    ring: "border-accent-200",
  },
  error: {
    icon: AlertCircle,
    iconClass: "text-[color:var(--color-danger)]",
    ring: "border-[color:var(--color-danger)]/30",
  },
  info: {
    icon: Info,
    iconClass: "text-[color:var(--color-warning)]",
    ring: "border-[var(--border)]",
  },
};

function ToastCard({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const meta = toneMeta[toast.tone];
  const Icon = meta.icon;
  return (
    <div
      role="status"
      className={`pointer-events-auto flex items-start gap-3 rounded-xl border ${meta.ring} bg-[var(--bg-elevated)] px-4 py-3 shadow-[var(--shadow-float)] animate-[toast-in_0.18s_ease-out]`}
    >
      <Icon className={`mt-0.5 h-5 w-5 shrink-0 ${meta.iconClass}`} strokeWidth={2} />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium leading-snug">{toast.title}</p>
        {toast.description && (
          <p className="mt-0.5 break-words text-xs text-[var(--text-muted)]">{toast.description}</p>
        )}
      </div>
      <button
        onClick={onDismiss}
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)]"
        aria-label="Dismiss"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}