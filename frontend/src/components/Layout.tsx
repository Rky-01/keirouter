import { useState, useRef, useEffect, type ReactNode } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  LayoutGrid,
  Boxes,
  GitBranch,
  Wallet,
  Sparkles,
  Search,
  Bell,
  CircleHelp,
  ChevronDown,
  Power,
  LogOut,
  Network,
  BarChart3,
  Clock,
  TerminalSquare,
  Image,
  Waypoints,
  ScrollText,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { api } from "../lib/api";

interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
}

interface NavGroup {
  heading?: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    heading: "Routing",
    items: [
      { to: "/", label: "Overview", icon: LayoutGrid, end: true },
      { to: "/providers", label: "Providers", icon: Boxes },
      { to: "/endpoints", label: "Endpoints", icon: Network },
      { to: "/chains", label: "Combos", icon: GitBranch },
      { to: "/usage", label: "Usage", icon: BarChart3 },
      { to: "/quota", label: "Quota Tracker", icon: Clock },
    ],
  },
  {
    heading: "Tools",
    items: [{ to: "/cli-tools", label: "CLI Tools", icon: TerminalSquare }],
  },
  {
    heading: "System",
    items: [
      { to: "/media", label: "Media Providers", icon: Image },
      { to: "/proxy-pools", label: "Proxy Pools", icon: Waypoints },
      { to: "/skills", label: "Skills", icon: Sparkles },
      { to: "/console", label: "Console Log", icon: ScrollText },
      { to: "/budgets", label: "Budgets", icon: Wallet },
      { to: "/settings", label: "Token Saving", icon: Zap },
    ],
  },
];

export function Layout() {
  return (
    <div className="flex h-full bg-[var(--bg)]">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-6xl px-8 py-8">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}

function Sidebar() {
  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-[var(--border)] bg-[var(--bg-elevated)]">
      <div className="px-5 py-5">
        <img src="/keirouter-logo.png" alt="KeiRouter" className="h-10 w-full object-contain object-left" />
      </div>

      <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-2">
        {navGroups.map((group, gi) => (
          <div key={gi} className="space-y-1">
            {group.heading && (
              <p className="px-3 pb-1 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                {group.heading}
              </p>
            )}
            {group.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                    isActive
                      ? "bg-accent-100 font-medium text-accent-700 dark:bg-accent-800/30 dark:text-accent-200"
                      : "text-[var(--text-muted)] hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
                  }`
                }
              >
                <item.icon className="h-[18px] w-[18px] shrink-0" strokeWidth={2} />
                {item.label}
              </NavLink>
            ))}
          </div>
        ))}
      </nav>

      <div className="border-t border-[var(--border)] p-3">
        <LogoutButton />
      </div>
    </aside>
  );
}

function TopBar() {
  return (
    <header className="flex h-16 shrink-0 items-center gap-4 border-b border-[var(--border)] bg-[var(--bg-elevated)] px-6">
      <div className="relative max-w-md flex-1">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
        <input
          type="text"
          placeholder="Search KeiRouter…"
          className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] py-2 pl-9 pr-12 text-sm placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40"
        />
        <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded border border-[var(--border)] bg-[var(--bg-elevated)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-muted)]">
          ⌘K
        </kbd>
      </div>

      <div className="ml-auto flex items-center gap-1">
        <button className="flex h-9 w-9 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800">
          <CircleHelp className="h-[18px] w-[18px]" strokeWidth={2} />
        </button>
        <button className="flex h-9 w-9 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800">
          <Bell className="h-[18px] w-[18px]" strokeWidth={2} />
        </button>
        <div className="mx-2 h-6 w-px bg-[var(--border)]" />
        <ProfileMenu />
      </div>
    </header>
  );
}

function ProfileMenu() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const qc = useQueryClient();

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-colors hover:bg-ink-100 dark:hover:bg-ink-800"
      >
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent-600 text-xs font-semibold text-white">
          K
        </div>
        <div className="hidden text-left sm:block">
          <p className="text-sm font-medium leading-tight">Kei</p>
          <p className="text-xs leading-tight text-[var(--text-muted)]">Administrator</p>
        </div>
        <ChevronDown className="h-4 w-4 text-[var(--text-muted)]" />
      </button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-2 w-48 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] py-1 shadow-[var(--shadow-float)]">
          <div className="px-4 py-2.5">
            <p className="text-sm font-medium">Kei</p>
            <p className="text-xs text-[var(--text-muted)]">Administrator</p>
          </div>
          <div className="my-1 h-px bg-[var(--border)]" />
          <button
            onClick={async () => {
              await api.logout();
              qc.invalidateQueries({ queryKey: ["auth-status"] });
            }}
            className="flex w-full items-center gap-2.5 px-4 py-2 text-left text-sm text-[var(--text)] transition-colors hover:bg-ink-100 dark:hover:bg-ink-800"
          >
            <LogOut className="h-4 w-4" strokeWidth={2} />
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}

function LogoutButton() {
  const qc = useQueryClient();
  return (
    <button
      onClick={async () => {
        await api.logout();
        qc.invalidateQueries({ queryKey: ["auth-status"] });
      }}
      className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
    >
      <Power className="h-[18px] w-[18px]" strokeWidth={2} />
      Sign out
    </button>
  );
}

export function PageHeader({
  title,
  description,
  icon: Icon,
  action,
}: {
  title: string;
  description?: string;
  icon?: LucideIcon;
  action?: ReactNode;
}) {
  return (
    <header className="mb-7 flex items-start justify-between gap-4">
      <div className="flex items-start gap-3">
        {Icon && (
          <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
            <Icon className="h-5 w-5" strokeWidth={2} />
          </div>
        )}
        <div>
          <h1 className="font-display text-3xl font-semibold tracking-tight">{title}</h1>
          {description && <p className="mt-1 text-sm text-[var(--text-muted)]">{description}</p>}
        </div>
      </div>
      {action}
    </header>
  );
}