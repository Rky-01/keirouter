// Reusable UI primitives styled with the KeiRouter design system. Calm,
// generously spaced, soft shadows and rounded surfaces — no gradients or neon.
import { useEffect, type ReactNode } from "react";
import type {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  SelectHTMLAttributes,
} from "react";
import { AlertCircle, X, type LucideIcon } from "lucide-react";

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={`rounded-2xl border border-[var(--border)] bg-[var(--bg-elevated)] shadow-[var(--shadow-card)] ${className}`}
    >
      {children}
    </div>
  );
}

// SectionHeader is the in-card header with an optional rounded icon chip, used
// across Settings/Endpoints-style panels in the attachment.
export function SectionHeader({
  title,
  description,
  icon: Icon,
  iconTone = "accent",
  action,
}: {
  title: ReactNode;
  description?: string;
  icon?: LucideIcon;
  iconTone?: "accent" | "neutral" | "danger" | "secondary";
  action?: ReactNode;
}) {
  const toneClasses: Record<string, string> = {
    accent: "bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200",
    neutral: "bg-ink-100 text-ink-600 dark:bg-ink-800 dark:text-ink-300",
    danger: "bg-[color:var(--color-danger)]/15 text-[color:var(--color-danger)]",
    secondary: "bg-secondary-100 text-secondary-700 dark:bg-secondary-800/40 dark:text-secondary-200",
  };
  return (
    <div className="flex items-start justify-between gap-4 px-6 pt-5 pb-4">
      <div className="flex items-start gap-3">
        {Icon && (
          <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${toneClasses[iconTone]}`}>
            <Icon className="h-[18px] w-[18px]" strokeWidth={2} />
          </div>
        )}
        <div>
          <h2 className="text-base font-semibold tracking-tight">{title}</h2>
          {description && <p className="mt-0.5 text-sm text-[var(--text-muted)]">{description}</p>}
        </div>
      </div>
      {action}
    </div>
  );
}

// CardHeader keeps the lighter divider-style header for list cards.
export function CardHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-[var(--border)] px-6 py-4">
      <div>
        <h2 className="text-sm font-semibold tracking-tight">{title}</h2>
        {description && <p className="mt-0.5 text-xs text-[var(--text-muted)]">{description}</p>}
      </div>
      {action}
    </div>
  );
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
};

export function Button({ variant = "primary", className = "", ...props }: ButtonProps) {
  const base =
    "inline-flex items-center justify-center gap-1.5 rounded-xl px-3.5 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-secondary-400/60";
  const variants = {
    primary: "bg-secondary-600 text-white hover:bg-secondary-700 dark:bg-secondary-500 dark:hover:bg-secondary-400 shadow-sm",
    secondary: "bg-accent-600 text-white hover:bg-accent-700 dark:bg-accent-500 dark:hover:bg-accent-400 shadow-sm",
    ghost:
      "border border-[var(--border)] bg-[var(--bg-elevated)] text-[var(--text)] hover:bg-ink-100 dark:hover:bg-ink-800",
    danger:
      "border border-[color:var(--color-danger)]/30 text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger)]/10",
  };
  return <button className={`${base} ${variants[variant]} ${className}`} {...props} />;
}

export function Input({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`w-full rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 text-sm placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40 ${className}`}
      {...props}
    />
  );
}

export function Select({ className = "", children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={`w-full rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 text-sm focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40 ${className}`}
      {...props}
    >
      {children}
    </select>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-[var(--text-muted)]">{label}</span>
      {children}
    </label>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "accent" | "secondary" | "danger" | "success";
}) {
  const tones = {
    neutral: "bg-ink-100 text-ink-600 dark:bg-ink-800 dark:text-ink-300",
    accent: "bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200",
    secondary: "bg-secondary-100 text-secondary-700 dark:bg-secondary-800/40 dark:text-secondary-200",
    danger: "bg-[color:var(--color-danger)]/15 text-[color:var(--color-danger)]",
    success: "bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200",
  };
  return (
    <span
      className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${tones[tone]}`}
    >
      {children}
    </span>
  );
}

// StatusDot is the small filled circle used next to "Healthy" / "Active" labels.
export function StatusDot({ tone = "success", label }: { tone?: "success" | "danger" | "warning" | "secondary"; label?: string }) {
  const colors = {
    success: "bg-accent-500",
    secondary: "bg-secondary-500",
    danger: "bg-[color:var(--color-danger)]",
    warning: "bg-[color:var(--color-warning)]",
  };
  return <span className={`inline-block h-1.5 w-1.5 rounded-full ${colors[tone]}`} role="img" aria-label={label || tone} />;
}

export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="px-6 py-14 text-center">
      <p className="text-sm text-[var(--text-muted)]">{title}</p>
      {hint && <p className="mt-1 text-xs text-[var(--text-muted)]">{hint}</p>}
    </div>
  );
}

// ErrorBanner is the consistent inline error surface used inside forms and
// cards. For transient feedback prefer a toast; use this for persistent,
// in-context errors (failed loads, validation summaries).
export function ErrorBanner({ message, className = "" }: { message: string; className?: string }) {
  return (
    <div
      role="alert"
      className={`flex items-start gap-2.5 rounded-lg border border-[color:var(--color-danger)]/30 bg-[color:var(--color-danger)]/10 px-3.5 py-2.5 ${className}`}
    >
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-[color:var(--color-danger)]" strokeWidth={2} />
      <p className="text-sm leading-snug break-words overflow-hidden text-[color:var(--color-danger)]">{message}</p>
    </div>
  );
}

// ErrorCard is a full-card error state for failed data loads.
export function ErrorCard({ message }: { message: string }) {
  return (
    <Card className="flex flex-col items-center gap-2 px-6 py-12 text-center">
      <AlertCircle className="h-6 w-6 text-[color:var(--color-danger)]" strokeWidth={2} />
      <p className="text-sm text-[color:var(--color-danger)]">{message}</p>
    </Card>
  );
}

export function Spinner() {
  return (
    <div className="flex items-center justify-center py-10" role="status" aria-label="Loading">
      <div className="h-5 w-5 animate-spin rounded-full border-2 border-ink-300 border-t-accent-500 dark:border-ink-600 dark:border-t-accent-400" />
    </div>
  );
}

export function StatCard({
  icon: Icon,
  iconTone = "accent",
  label,
  value,
  delta,
}: {
  icon: LucideIcon;
  iconTone?: "accent" | "warning" | "danger";
  label: string;
  value: string;
  delta?: { text: string; direction?: "up" | "down" | "flat" };
}) {
  const deltaColor =
    delta?.direction === "up"
      ? "text-accent-600 dark:text-accent-400"
      : delta?.direction === "down"
        ? "text-[color:var(--color-danger)]"
        : "text-[var(--text-muted)]";
  const arrow = delta?.direction === "up" ? "↑" : delta?.direction === "down" ? "↓" : "";
  
  const iconColor = iconTone === "accent" ? "text-accent-500" : iconTone === "warning" ? "text-amber-500" : "text-red-500";

  return (
    <div className="flex flex-col justify-between p-5 rounded-xl border border-[var(--border)] bg-[var(--bg)] shadow-sm transition-colors hover:bg-[var(--bg-subtle)] relative overflow-hidden group">
      {/* Subtle secondary tone gradient glow on hover */}
      <div className="absolute inset-0 bg-gradient-to-br from-accent-500/5 to-transparent opacity-0 transition-opacity duration-500 group-hover:opacity-100 dark:from-accent-500/10" />
      
      <div className="relative flex items-center gap-2 text-[var(--text-muted)] mb-4">
        <Icon className={`h-4 w-4 ${iconColor}`} />
        <span className="text-xs font-medium tracking-wide uppercase">{label}</span>
      </div>
      <div className="relative">
        <div className="text-3xl font-light tracking-tight tabular-nums text-[var(--text)]">
          {value}
        </div>
        {delta && (
          <p className={`mt-1.5 text-[11px] font-medium ${deltaColor}`}>
            {arrow} {delta.text}
          </p>
        )}
      </div>
    </div>
  );
}

// SegmentedControl renders the Gentle / Balanced / Strong style toggle group.
export function SegmentedControl<T extends string>({
  value,
  onChange,
  options,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string }[];
}) {
  return (
    <div className="inline-flex rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] p-0.5" role="radiogroup">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          role="radio"
          aria-checked={value === opt.value}
          onClick={() => onChange(opt.value)}
          className={`rounded-md px-3 py-2 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/60 ${
            value === opt.value
              ? "bg-[var(--bg-elevated)] text-[var(--text)] shadow-sm"
              : "text-[var(--text-muted)] hover:text-[var(--text)]"
          }`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

// Toggle is a small accessible switch.
// Modal is a reusable dialog overlay. Escape and backdrop click close it.
export function Modal({
  open,
  onClose,
  title,
  subtitle,
  children,
  maxWidth = "max-w-lg",
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: string;
  children: ReactNode;
  maxWidth?: string;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={typeof title === "string" ? title : undefined}
    >
      <div
        className={`w-full ${maxWidth} rounded-2xl border border-[var(--border)] bg-[var(--bg-elevated)] shadow-[var(--shadow-float)]`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--border)] px-6 py-4">
          <div>
            <h2 className="text-base font-semibold tracking-tight">{title}</h2>
            {subtitle && <p className="mt-0.5 text-sm text-[var(--text-muted)]">{subtitle}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="flex h-9 w-9 items-center justify-center rounded-xl text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/60"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-7 w-12 shrink-0 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-secondary-400/60 ${
        checked ? "bg-secondary-600" : "bg-ink-300 dark:bg-ink-700"
      }`}
    >
      <span
        className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform ${
          checked ? "translate-x-[22px]" : "translate-x-1"
        }`}
      />
    </button>
  );
}