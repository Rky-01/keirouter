import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Clock, Calendar } from "lucide-react";
import { api, type QuotaAccount } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, CardHeader, Spinner, EmptyState, Badge, StatusDot } from "../components/ui";

const periods = [
  { value: "today", label: "Today" },
  { value: "week", label: "Last 7 days" },
  { value: "month", label: "Last 30 days" },
];

const statusMeta: Record<string, { label: string; tone: "success" | "warning" | "danger" }> = {
  active: { label: "Active", tone: "success" },
  paused: { label: "Paused", tone: "warning" },
  needs_attention: { label: "Needs attention", tone: "danger" },
};

export function QuotaPage() {
  const [period, setPeriod] = useState("month");
  const quota = useQuery({ queryKey: ["quota", period], queryFn: () => api.quota(period) });

  const accounts = quota.data?.accounts ?? [];
  const maxCost = Math.max(0.0001, ...accounts.map((a) => a.cost_usd));

  return (
    <>
      <PageHeader
        title="Quota Tracker"
        icon={Clock}
        description="Monitor consumption per connected account so you can balance load before limits bite."
        action={
          <div className="flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 shadow-[var(--shadow-card)]">
            <Calendar className="h-4 w-4 text-[var(--text-muted)]" />
            <select
              value={period}
              onChange={(e) => setPeriod(e.target.value)}
              className="bg-transparent text-sm font-medium focus:outline-none"
            >
              {periods.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </select>
          </div>
        }
      />

      <Card>
        <CardHeader
          title="Accounts"
          description="Usage is measured from metered requests over the selected period."
        />
        {quota.isLoading ? (
          <Spinner />
        ) : !accounts.length ? (
          <EmptyState
            title="No connected accounts"
            hint="Add a provider account to start tracking quota usage."
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--border)] text-left text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                  <th className="px-6 py-3">Account</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3">Type</th>
                  <th className="px-6 py-3 text-right">Priority</th>
                  <th className="px-6 py-3 text-right">Requests</th>
                  <th className="px-6 py-3">Usage</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
                {accounts.map((a) => (
                  <QuotaRow key={a.id} account={a} maxCost={maxCost} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <p className="mt-4 flex items-center justify-center gap-1.5 text-center text-xs text-[var(--text-muted)]">
        <Clock className="h-3.5 w-3.5" />
        Reorder account priority under Accounts to influence routing preference.
      </p>
    </>
  );
}

function QuotaRow({ account: a, maxCost }: { account: QuotaAccount; maxCost: number }) {
  const meta = statusMeta[a.status] ?? statusMeta.active;
  const pct = Math.min(100, Math.round((a.cost_usd / maxCost) * 100));
  const barTone =
    pct >= 90
      ? "bg-[color:var(--color-danger)]"
      : pct >= 66
        ? "bg-[color:var(--color-warning)]"
        : "bg-accent-500";

  return (
    <tr className="transition-colors hover:bg-ink-50 dark:hover:bg-ink-800/40">
      <td className="px-6 py-4">
        <p className="font-medium">{a.label || a.provider_name}</p>
        <p className="mt-0.5 text-xs text-[var(--text-muted)]">{a.provider_name}</p>
      </td>
      <td className="px-6 py-4">
        <span className="inline-flex items-center gap-1.5">
          <StatusDot tone={meta.tone} />
          <span className="text-xs font-medium">{meta.label}</span>
        </span>
      </td>
      <td className="px-6 py-4">
        <Badge tone="neutral">{a.auth_kind === "oauth" ? "OAuth" : "API key"}</Badge>
      </td>
      <td className="px-6 py-4 text-right tabular-nums text-[var(--text-muted)]">#{a.priority}</td>
      <td className="px-6 py-4 text-right tabular-nums">{a.total_requests.toLocaleString()}</td>
      <td className="px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="h-1.5 w-28 overflow-hidden rounded-full bg-[var(--bg-subtle)]">
            <div className={`h-full rounded-full ${barTone}`} style={{ width: `${Math.max(2, pct)}%` }} />
          </div>
          <span className="w-16 shrink-0 text-right text-xs tabular-nums">${a.cost_usd.toFixed(2)}</span>
        </div>
      </td>
    </tr>
  );
}