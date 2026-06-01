import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  DollarSign,
  ShieldCheck,
  Calendar,
  Workflow,
  Clock,
  Lightbulb,
  ArrowRight,
} from "lucide-react";
import { AreaChart, Area, ResponsiveContainer } from "recharts";
import { api, type ProviderUsage, type RecentActivity } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, SectionHeader, CardHeader, Spinner, StatCard, Badge, ErrorCard } from "../components/ui";

const periods = [
  { value: "today", label: "Today" },
  { value: "week", label: "Last 7 days" },
  { value: "month", label: "Last 30 days" },
];

export function UsagePage() {
  const [period, setPeriod] = useState("today");
  const insights = useQuery({
    queryKey: ["usage-insights", period],
    queryFn: () => api.usageInsights(period),
  });

  return (
    <>
      <PageHeader
        title="Usage"
        icon={Workflow}
        description="Understand how your requests flow across providers."
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

      {insights.isLoading ? (
        <Spinner />
      ) : insights.isError ? (
        <ErrorCard message="Failed to load usage. Is the backend running?" />
      ) : (
        (() => {
          const data = insights.data!;
          const total = data.summary.total_requests;
          return (
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
              {/* Left column: activity map + recent activity (spans 2). */}
              <div className="space-y-6 lg:col-span-2">
                <Card>
                  <SectionHeader
                    title="Routing Activity Map"
                    description="Live view of request flow and provider distribution."
                    icon={Workflow}
                  />
                  <RoutingMap providers={data.providers} total={total} />
                </Card>

                <Card>
                  <CardHeader
                    title="Recent Activity"
                    action={
                      <span className="flex items-center gap-1 text-xs font-medium text-accent-600 dark:text-accent-300">
                        View all activity <ArrowRight className="h-3.5 w-3.5" />
                      </span>
                    }
                  />
                  {data.recent.length === 0 ? (
                    <div className="px-6 py-12 text-center text-sm text-[var(--text-muted)]">
                      No requests recorded yet for this period.
                    </div>
                  ) : (
                    <div className="divide-y divide-[var(--border)]">
                      {data.recent.map((r) => (
                        <ActivityRow key={r.id} row={r} />
                      ))}
                    </div>
                  )}
                </Card>
              </div>

              {/* Right column: headline metrics + activity sparkline + insights. */}
              <div className="space-y-4">
                <StatCard
                  icon={Activity}
                  iconTone="accent"
                  label="Total Requests"
                  value={total.toLocaleString()}
                />
                <StatCard
                  icon={DollarSign}
                  iconTone="warning"
                  label="Estimated Cost"
                  value={`$${data.summary.cost_usd.toFixed(2)}`}
                />
                <StatCard
                  icon={ShieldCheck}
                  iconTone="accent"
                  label="Success Rate"
                  value={`${data.summary.success_rate.toFixed(1)}%`}
                />

                <Card>
                  <div className="flex items-center justify-between px-5 pt-4">
                    <p className="text-sm font-medium">Activity over time</p>
                    <span className="text-xs text-[var(--text-muted)]">
                      avg {data.summary.avg_latency_ms}ms
                    </span>
                  </div>
                  <div className="h-28 px-2 pb-3 pt-2">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={data.series}>
                        <defs>
                          <linearGradient id="usageFill" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="#4a7a6f" stopOpacity={0.35} />
                            <stop offset="100%" stopColor="#4a7a6f" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <Area
                          type="monotone"
                          dataKey="count"
                          stroke="#4a7a6f"
                          strokeWidth={2}
                          fill="url(#usageFill)"
                        />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                </Card>

                <Card>
                  <SectionHeader title="Key insights" icon={Lightbulb} iconTone="neutral" />
                  <div className="space-y-4 px-6 pb-6">
                    {data.providers[0] && (
                      <Insight
                        icon={Workflow}
                        title={`${data.providers[0].display_name} handled the most traffic`}
                        detail={`${data.providers[0].share_pct.toFixed(1)}% of total requests`}
                      />
                    )}
                    {data.busiest && (
                      <Insight
                        icon={Clock}
                        title="Busiest period"
                        detail={`Peak activity around ${data.busiest}`}
                      />
                    )}
                    <Insight
                      icon={ShieldCheck}
                      title={
                        data.summary.success_rate >= 99
                          ? "All providers healthy"
                          : "Some requests need attention"
                      }
                      detail={`${data.summary.success_rate.toFixed(1)}% success rate`}
                    />
                  </div>
                </Card>
              </div>
            </div>
          );
        })()
      )}
    </>
  );
}

// RoutingMap renders the hub-and-spoke list of providers with their share of
// total traffic. A literal node graph is overkill for a local dashboard, so we
// present the same information as a ranked, proportional list.
function RoutingMap({ providers, total }: { providers: ProviderUsage[]; total: number }) {
  if (!providers.length) {
    return (
      <div className="px-6 py-12 text-center text-sm text-[var(--text-muted)]">
        No routing activity yet. Send a request through KeiRouter to see the flow here.
      </div>
    );
  }
  return (
    <div className="space-y-4 px-6 pb-6">
      <div className="flex items-center justify-between rounded-xl border border-[var(--border)] bg-[var(--bg-subtle)] px-4 py-3">
        <div className="flex items-center gap-2.5">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent-600 text-white">
            <Workflow className="h-4 w-4" />
          </span>
          <span className="text-sm font-semibold">KeiRouter</span>
        </div>
        <span className="text-sm font-medium tabular-nums">
          {total.toLocaleString()} requests
        </span>
      </div>

      <div className="space-y-3">
        {providers.map((p) => (
          <div key={p.provider} className="flex items-center gap-3">
            <ProviderDot color={p.color} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-3">
                <span className="truncate text-sm font-medium">{p.display_name}</span>
                <span className="shrink-0 text-xs text-[var(--text-muted)] tabular-nums">
                  {p.total_requests.toLocaleString()} req · {p.share_pct.toFixed(1)}%
                </span>
              </div>
              <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-subtle)]">
                <div
                  className="h-full rounded-full"
                  style={{
                    width: `${Math.max(2, p.share_pct)}%`,
                    backgroundColor: p.color || "#4a7a6f",
                  }}
                />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ProviderDot({ color }: { color: string }) {
  return (
    <span
      className="h-2.5 w-2.5 shrink-0 rounded-full"
      style={{ backgroundColor: color || "#4a7a6f" }}
    />
  );
}

function ActivityRow({ row }: { row: RecentActivity }) {
  return (
    <div className="flex items-center gap-3 px-6 py-3 text-sm">
      <span className="w-16 shrink-0 text-xs text-[var(--text-muted)]">{relTime(row.created_at)}</span>
      <span className="min-w-0 flex-1 truncate font-mono text-xs">{row.model || "—"}</span>
      <span className="hidden shrink-0 text-xs text-[var(--text-muted)] sm:block">{row.provider}</span>
      <Badge tone={row.latency_ms > 0 ? "success" : "danger"}>
        {row.latency_ms > 0 ? "Success" : "Failed"}
      </Badge>
      <span className="w-20 shrink-0 text-right text-xs tabular-nums text-[var(--text-muted)]">
        {row.tokens.toLocaleString()} tok
      </span>
      <span className="w-14 shrink-0 text-right text-xs tabular-nums">${row.cost_usd.toFixed(3)}</span>
    </div>
  );
}

function Insight({
  icon: Icon,
  title,
  detail,
}: {
  icon: typeof Activity;
  title: string;
  detail: string;
}) {
  return (
    <div className="flex items-start gap-3">
      <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
        <Icon className="h-4 w-4" />
      </span>
      <div className="min-w-0">
        <p className="text-sm font-medium leading-snug">{title}</p>
        <p className="mt-0.5 text-xs text-[var(--text-muted)]">{detail}</p>
      </div>
    </div>
  );
}

function relTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const diff = Date.now() - t;
  const m = Math.floor(diff / 60000);
  if (m < 1) return "now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}