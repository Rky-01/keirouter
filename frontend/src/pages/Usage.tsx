import { useState, useMemo, useCallback, useRef, useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ReactFlow, Handle, Position, Controls,
  useNodesState, useEdgesState,
  type Node, type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  Activity, DollarSign, Zap, RefreshCw, TrendingUp, Clock, Search, ArrowUpDown, ArrowUp, ArrowDown, ShieldCheck, Scissors
} from "lucide-react";
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, PieChart, Pie, Cell,
} from "recharts";
import { api, connectUsageStream, type ProviderUsage, type RecentActivity, type SeriesPoint, type ModelUsage, type TokenSavings, type UsageInsights } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, Spinner, ErrorCard } from "../components/ui";
import { useToast } from "../components/Toast";
import { SavingsCardShareButton } from "../components/SavingsCard";

const periods = [
  { value: "today", label: "Today" },
  { value: "24h", label: "24h" },
  { value: "week", label: "7D" },
  { value: "month", label: "30D" },
];

export function UsagePage() {
  const [period, setPeriod] = useState("today");
  const qc = useQueryClient();
  const toast = useToast();

  const insights = useQuery({
    queryKey: ["usage-insights", period],
    queryFn: () => api.usageInsights(period),
    refetchInterval: 10_000, // 10s for fresher recent requests
  });

  const modelUsage = useQuery({
    queryKey: ["usage-models", period],
    queryFn: () => api.modelUsage(period),
    refetchInterval: 10_000, // 10s auto-refresh matching insights
  });

  // Subscribe to SSE usage stream for instant cache invalidation when new
  // requests complete. This makes the Usage page react within ~100ms of a
  // request completing instead of waiting for the 10s polling interval.
  useEffect(() => {
    return connectUsageStream(() => {
      qc.invalidateQueries({ queryKey: ["usage-insights"] });
      qc.invalidateQueries({ queryKey: ["usage-models"] });
    });
  }, [qc]);

  const handleRefresh = () => {
    qc.invalidateQueries({ queryKey: ["usage-insights"] });
    toast.success("Usage data refreshed", "All usage metrics and breakdowns have been re-fetched from the server.");
  };

  return (
    <>
      <PageHeader
        title="Usage"
        icon={Activity}
        description="Monitor request flow and provider distribution."
      />

      <div className="mb-6 flex justify-end">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] p-1">
            {periods.map((p) => (
              <button key={p.value} onClick={() => setPeriod(p.value)}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${period === p.value ? "bg-accent-600 text-white shadow-sm" : "text-[var(--text-muted)] hover:text-[var(--text)]"}`}>
                {p.label}
              </button>
            ))}
          </div>
          <button onClick={handleRefresh}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-elevated)] hover:text-[var(--text)]">
            <RefreshCw className="h-4 w-4" />
          </button>
        </div>
      </div>

      {insights.isLoading ? <Spinner />
        : insights.isError ? <ErrorCard message="Failed to load usage. Is the backend running?" />
        : insights.data ? <UsageContent data={insights.data} models={modelUsage.data?.models ?? []} period={period} /> : null}
    </>
  );
}

function UsageContent({ data, models, period }: { data: any; models: ModelUsage[]; period: string }) {
  const { summary, savings, providers, recent, series } = data;
  const activeProviders = providers.filter((p: ProviderUsage) => p.share_pct > 0);

  return (
    <div className="space-y-5">
      {/* Overview cards */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard label="Total requests" title="REQUESTS" value={fmtNum(summary.total_requests)} icon={Activity} color="accent" />
        <StatCard label="Total input tokens" title="INPUT TOKENS" value={fmtNum(summary.prompt_tokens)} icon={Zap} color="blue" />
        <StatCard label="Total output tokens" title="OUTPUT TOKENS" value={fmtNum(summary.completion_tokens)} icon={TrendingUp} color="green" />
        <StatCard label="Total estimated cost" title="EST. COST" value={`$${summary.cost_usd.toFixed(2)}`} icon={DollarSign} color="amber" />
        
        <Card className="flex flex-col justify-center px-4 py-3 gap-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-[var(--text-muted)]" />
              <div>
                <p className="text-[10px] font-bold tracking-wider text-[var(--text-muted)]">ROUTING EFFICIENCY</p>
                <p className="text-sm font-semibold">{summary.avg_latency_ms > 0 ? `${summary.avg_latency_ms}ms` : summary.total_requests > 0 ? "<1ms" : "—"}</p>
              </div>
            </div>
            <TrendingUp className="h-4 w-4 text-emerald-500 opacity-60" />
          </div>
          <div className="h-px w-full bg-[var(--border)]" />
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-[var(--text-muted)]" />
              <div>
                <p className="text-[10px] font-bold tracking-wider text-[var(--text-muted)]">SUCCESS RATE</p>
                <p className="text-sm font-semibold">{summary.success_rate != null ? (summary.success_rate * 100).toFixed(1) : summary.total_requests > 0 ? "—" : "100"}%</p>
              </div>
            </div>
            <TrendingUp className="h-4 w-4 text-emerald-500 opacity-60" />
          </div>
        </Card>
      </div>

      {/* Main Layout Grid */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,2fr)_minmax(300px,1fr)]">
        {/* Left Column */}
        <div className="flex min-w-0 flex-col space-y-5">
          {/* Topology */}
          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
              <div className="flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-[var(--text-muted)]" />
                <h3 className="text-sm font-semibold">Routing Topology</h3>
              </div>
              <div className="flex items-center gap-2">
                {activeProviders.length > 0 && (
                  <span className="flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                    <span className="relative flex h-2 w-2">
                      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                      <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
                    </span>
                    {activeProviders.length} active
                  </span>
                )}
              </div>
            </div>
            <ProviderTopology providers={providers} />
          </Card>

          {/* Activity chart */}
          <Card>
            <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
              <h3 className="text-sm font-semibold">Usage Trends</h3>
              <span className="text-xs text-[var(--text-muted)]">Input and output tokens over time</span>
            </div>
            <div className="h-52 px-2 pb-3 pt-2">
              <ActivityChart series={series} />
            </div>
          </Card>
        </div>

        {/* Right Column */}
        <div className="flex min-w-0 flex-col space-y-5">
          {/* Insights */}
          <UsageInsightsCard data={data} />
          
          {/* Recent Requests */}
          <Card className="flex max-h-[440px] flex-1 flex-col overflow-hidden">
            <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-[var(--text-muted)]" />
                <h3 className="text-sm font-semibold">Recent Requests</h3>
              </div>
              <span className="text-[10px] text-[var(--text-muted)] hover:text-[var(--text)] cursor-pointer">View all</span>
            </div>
            {!recent.length ? (
              <div className="flex flex-1 flex-col items-center justify-center gap-2 py-10">
                <Activity className="h-8 w-8 text-[var(--text-muted)] opacity-20" />
                <p className="text-xs text-[var(--text-muted)]">No requests yet</p>
              </div>
            ) : (
              <div className="flex-1 overflow-y-auto">
                <table className="w-full border-collapse text-xs">
                  <thead className="sticky top-0 z-10 bg-[var(--bg-elevated)]">
                    <tr className="border-b border-[var(--border)]">
                      <th className="w-6 px-3 py-2" />
                      <th className="px-3 py-2 text-left font-medium text-[var(--text-muted)]">Model</th>
                      <th className="px-3 py-2 text-right font-medium text-[var(--text-muted)]">Tokens</th>
                      <th className="px-3 py-2 text-right font-medium text-[var(--text-muted)]">When</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border)]">
                    {recent.map((r: RecentActivity) => <RecentRow key={r.id} row={r} />)}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </div>
      </div>

      {/* Token Savings breakdown */}
      {savings && savings.rules && savings.rules.length > 0 && (
        <TokenSavingsBreakdown savings={savings} totalRequests={summary.total_requests} insights={data} period={period} />
      )}

      {/* Model usage breakdown */}
      <ModelUsageTable models={models} />

      {/* Provider breakdown */}
      <Card>
        <div className="border-b border-[var(--border)] px-4 py-2.5">
          <h3 className="text-sm font-semibold">Provider Breakdown</h3>
        </div>
        {providers.filter((p: ProviderUsage) => p.total_requests > 0).length === 0 ? (
          <div className="py-8 text-center text-xs text-[var(--text-muted)]">No provider data for this period</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--border)]">
                <th className="px-4 py-2 text-left font-medium text-[var(--text-muted)]">Provider</th>
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">Requests</th>
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">In Tok</th>
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">Out Tok</th>
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">Cost</th>
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">Share</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border)]">
              {providers.filter((p: ProviderUsage) => p.total_requests > 0).map((p: ProviderUsage) => (
                <tr key={p.provider} className="transition-colors hover:bg-[var(--bg-subtle)]">
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      <ProviderIcon provider={p.provider} color={p.color} />
                      <span className="font-medium">{p.display_name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">{p.total_requests.toLocaleString()}</td>
                  <td className="px-4 py-2 text-right tabular-nums text-blue-500 dark:text-blue-400">{fmtNum(p.prompt_tokens)}</td>
                  <td className="px-4 py-2 text-right tabular-nums text-green-500 dark:text-green-400">{fmtNum(p.completion_tokens)}</td>
                  <td className="px-4 py-2 text-right tabular-nums font-medium">${p.cost_usd.toFixed(4)}</td>
                  <td className="px-4 py-2 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-[var(--bg-subtle)]">
                        <div className="h-full rounded-full bg-accent-500" style={{ width: `${Math.max(2, p.share_pct)}%` }} />
                      </div>
                      <span className="w-10 text-right tabular-nums text-[var(--text-muted)]">{p.share_pct.toFixed(1)}%</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        )}
      </Card>
    </div>
  );
}

// ─── Stat Card ────────────────────────────────────────────────────────────────

function StatCard({ label, title, value, icon: Icon, color }: { label: string; title: string; value: string; icon: typeof Activity; color: string }) {
  const colorMap: Record<string, string> = {
    accent: "bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200",
    blue: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300",
    green: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300",
    amber: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300",
    teal: "bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300",
    purple: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300",
    indigo: "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300",
  };
  return (
    <Card className="flex items-center gap-4 px-4 py-4">
      <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl ${colorMap[color] || colorMap.accent}`}>
        <Icon className="h-5 w-5" />
      </div>
      <div className="min-w-0 flex flex-col justify-center">
        <p className="text-[10px] font-bold tracking-wider text-[var(--text-muted)]">{title}</p>
        <p className="truncate text-2xl font-bold tabular-nums leading-none mt-1 mb-0.5">{value}</p>
        <p className="text-[11px] text-[var(--text-muted)]">{label}</p>
      </div>
    </Card>
  );
}

// ─── Insights Component ───────────────────────────────────────────────────────

function UsageInsightsCard({ data }: { data: any }) {
  const { providers, summary } = data;
  const activeProviders = providers.filter((p: any) => p.share_pct > 0);
  
  // For Token Efficiency (Output / Input ratio)
  const tokenRatio = summary.prompt_tokens > 0 
    ? summary.completion_tokens / summary.prompt_tokens
    : 0;

  return (
    <Card className="flex flex-col">
      <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
        <h3 className="text-sm font-semibold">Insights</h3>
        <button className="text-xs font-medium text-[var(--text-muted)] hover:text-[var(--text)] transition-colors">View all</button>
      </div>
      <div className="p-4 flex flex-col gap-6">
        {/* Provider Distribution */}
        <div>
          <div className="flex justify-between items-center mb-3">
            <h4 className="text-xs font-medium text-[var(--text-muted)]">Provider Distribution</h4>
          </div>
          <div className="flex items-center gap-5">
            <div className="h-24 w-24 shrink-0">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={activeProviders}
                    cx="50%"
                    cy="50%"
                    innerRadius={25}
                    outerRadius={45}
                    paddingAngle={2}
                    dataKey="share_pct"
                    stroke="none"
                  >
                    {activeProviders.map((p: any, index: number) => (
                      <Cell key={`cell-${index}`} fill={p.color || "var(--color-chart-1)"} />
                    ))}
                  </Pie>
                </PieChart>
              </ResponsiveContainer>
            </div>
            <div className="flex-1 space-y-2">
              {activeProviders.slice(0, 4).map((p: any) => (
                <div key={p.provider} className="flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <span className="h-2 w-2 rounded-full" style={{ backgroundColor: p.color || "var(--color-chart-1)" }} />
                    <span className="text-[var(--text-muted)]">{p.provider_name || p.provider}</span>
                  </div>
                  <span className="font-medium">{p.share_pct.toFixed(1)}%</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="h-px w-full bg-[var(--border)]" />

        {/* Token Efficiency */}
        <div className="flex gap-4">
          <div className="flex-1">
            <h4 className="text-xs font-medium text-[var(--text-muted)] mb-3">Token Efficiency</h4>
            <div className="flex justify-between items-end">
              <div>
                <p className="text-[10px] text-[var(--text-muted)] mb-1">Output / Input</p>
                <div className="flex items-baseline gap-2">
                  <span className="text-2xl font-bold tabular-nums leading-none">{tokenRatio.toFixed(2)}x</span>
                </div>
                {summary.prompt_tokens > 0 && (
                  <p className="text-[10px] text-[var(--text-muted)] mt-1">
                    {fmtNum(summary.completion_tokens)} out / {fmtNum(summary.prompt_tokens)} in
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}

// ─── Activity Chart ──────────────────────────────────────────────────────────

function ActivityChart({ series }: { series: SeriesPoint[] }) {
  if (!series.length) {
    return <div className="flex h-full items-center justify-center text-sm text-[var(--text-muted)]">No data for this period</div>;
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={series}>
        <defs>
          <linearGradient id="usageFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-chart-1)" stopOpacity={0.25} />
            <stop offset="100%" stopColor="var(--color-chart-1)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" opacity={0.5} />
        <XAxis dataKey="label" tick={{ fontSize: 10, fill: "var(--text-muted)" }} tickLine={false} axisLine={false} />
        <YAxis tick={{ fontSize: 10, fill: "var(--text-muted)" }} tickLine={false} axisLine={false} tickFormatter={(v: number) => fmtNum(v)} width={50} />
        <Tooltip
          contentStyle={{ fontSize: 12, background: "var(--bg)", border: "1px solid var(--border)", borderRadius: 8 }}
          formatter={(value: number) => [fmtNum(value), "Requests"]}
        />
        <Area type="monotone" dataKey="count" stroke="var(--color-chart-1)" strokeWidth={2} fill="url(#usageFill)" />
      </AreaChart>
    </ResponsiveContainer>
  );
}

// ─── React Flow Topology ─────────────────────────────────────────────────────

function RouterNode({ data }: { data: any }) {
  return (
    <div className="flex flex-col items-center">
      <Handle type="source" position={Position.Top} id="top" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="source" position={Position.Bottom} id="bottom" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="source" position={Position.Left} id="left" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="source" position={Position.Right} id="right" className="!bg-transparent !border-0 !w-0 !h-0" />
      <div className="flex items-center justify-center rounded-xl border-2 border-accent-500 bg-accent-50 px-5 py-3 shadow-[var(--shadow-pop)] dark:bg-accent-900/30">
        <img src="/keirouter-logo.png" alt="KeiRouter" className="mr-2 h-6 w-6 object-contain" />
        <span className="text-sm font-bold text-accent-700 dark:text-accent-300">KeiRouter</span>
        {data.activeCount > 0 && (
          <span className="ml-2 rounded-full bg-accent-600 px-1.5 py-0.5 text-xs font-bold text-white">{data.activeCount}</span>
        )}
      </div>
    </div>
  );
}

function ProviderNode({ data }: { data: any }) {
  const { label, color, imageUrl, textIcon, sharePct } = data;
  const [imgError, setImgError] = useState(false);
  const isHigh = sharePct > 20;

  return (
    <div
      className="flex items-center gap-2.5 rounded-lg border-2 bg-[var(--bg-elevated)] px-4 py-2.5 transition-colors"
      style={{
        borderColor: color,
        boxShadow: isHigh ? `0 0 16px ${color}40` : "none",
        minWidth: "150px",
      }}
    >
      <Handle type="target" position={Position.Top} id="top" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="target" position={Position.Bottom} id="bottom" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="target" position={Position.Left} id="left" className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="target" position={Position.Right} id="right" className="!bg-transparent !border-0 !w-0 !h-0" />

      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md" style={{ backgroundColor: `${color}15` }}>
        {!imgError && imageUrl ? (
          <img src={imageUrl} alt={label} className="h-6 w-6 rounded-sm object-contain" onError={() => setImgError(true)} />
        ) : (
          <span className="text-sm font-bold" style={{ color }}>{textIcon}</span>
        )}
      </div>

      <div className="min-w-0">
        <span className="block truncate text-sm font-medium" style={{ color }}>{label}</span>
        <span className="text-[10px] text-[var(--text-muted)]">{sharePct.toFixed(1)}% traffic</span>
      </div>

      {isHigh && (
        <span className="relative flex h-2 w-2 shrink-0">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75" style={{ backgroundColor: color }} />
          <span className="relative inline-flex h-2 w-2 rounded-full" style={{ backgroundColor: color }} />
        </span>
      )}
    </div>
  );
}

const nodeTypes = { router: RouterNode, provider: ProviderNode };

function buildLayout(providers: ProviderUsage[]) {
  // ONLY include providers with actual traffic
  const activeProviders = providers.filter((p) => p.share_pct > 0);
  const count = activeProviders.length;

  if (count === 0) {
    return {
      nodes: [{ id: "router", type: "router", position: { x: 0, y: 0 }, data: { activeCount: 0 } }],
      edges: [],
    };
  }

  const nodeW = 180;
  const nodeGap = 24;
  const minRx = ((nodeW + nodeGap) * count) / (2 * Math.PI);
  const rx = Math.max(280, minRx);
  const ry = Math.max(160, rx * 0.55);

  const nodes: Node[] = [];
  const edges: Edge[] = [];

  nodes.push({
    id: "router",
    type: "router",
    position: { x: -60, y: -22 },
    data: { activeCount: count },
    draggable: false,
  });

  activeProviders.forEach((p, i) => {
    const angle = -Math.PI / 2 + (2 * Math.PI * i) / count;
    const cx = rx * Math.cos(angle);
    const cy = ry * Math.sin(angle);
    const nodeId = `provider-${p.provider}`;
    const color = p.color || "var(--color-ink-400)";
    const isHigh = p.share_pct > 20;

    nodes.push({
      id: nodeId,
      type: "provider",
      position: { x: cx - nodeW / 2, y: cy - 18 },
      data: {
        label: p.display_name,
        color,
        imageUrl: `/providers/${p.provider}.png`,
        textIcon: p.display_name.slice(0, 2).toUpperCase(),
        sharePct: p.share_pct,
      },
      draggable: false,
    });

    // Handle selection based on angle
    let sourceHandle = "right";
    if (Math.abs(angle + Math.PI / 2) < Math.PI / 4) sourceHandle = "top";
    else if (Math.abs(angle - Math.PI / 2) < Math.PI / 4) sourceHandle = "bottom";
    else if (cx > 0) sourceHandle = "right";
    else sourceHandle = "left";
    const targetHandle = sourceHandle === "top" ? "bottom" : sourceHandle === "bottom" ? "top" : sourceHandle === "left" ? "right" : "left";

    edges.push({
      id: `e-${nodeId}`,
      source: "router",
      sourceHandle,
      target: nodeId,
      targetHandle,
      animated: isHigh,
      style: {
        stroke: color,
        strokeWidth: isHigh ? 2.5 : 1.5,
        opacity: isHigh ? 0.9 : 0.5,
      },
    });
  });

  return { nodes, edges };
}

function ProviderTopology({ providers }: { providers: ProviderUsage[] }) {
  const { nodes: initialNodes, edges: initialEdges } = useMemo(() => buildLayout(providers), [providers]);
  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);
  const rfInstance = useRef<any>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (rfInstance.current) {
      const id = setTimeout(() => rfInstance.current?.fitView({ padding: 0.2, duration: 200 }), 50);
      return () => clearTimeout(id);
    }
  }, [nodes.length]);

  const onInit = useCallback((instance: any) => {
    rfInstance.current = instance;
    setTimeout(() => instance.fitView({ padding: 0.2, duration: 200 }), 50);
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => rfInstance.current?.fitView({ padding: 0.2 }));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const activeCount = providers.filter((p) => p.share_pct > 0).length;

  if (activeCount === 0) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-2">
        <TrendingUp className="h-8 w-8 text-[var(--text-muted)] opacity-20" />
        <p className="text-sm text-[var(--text-muted)]">No routing activity yet</p>
        <p className="text-xs text-[var(--text-muted)]">Connections appear when models receive traffic</p>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="h-[400px] w-full min-w-0">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onInit={onInit}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        minZoom={0.3}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
        panOnDrag
        zoomOnScroll={false}
        zoomOnPinch
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
      >
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}

// ─── Recent Row ──────────────────────────────────────────────────────────────

function RecentRow({ row }: { row: RecentActivity }) {
  const success = row.latency_ms > 0;
  const hasSavings = (row.slim_bytes_saved ?? 0) > 0 || row.caveman_active || row.terse_active;
  return (
    <tr className="transition-colors hover:bg-[var(--bg-subtle)]">
      <td className="w-6 px-3 py-1.5">
        <span className={`block h-1.5 w-1.5 rounded-full ${success ? "bg-emerald-500" : "bg-red-500"}`} />
      </td>
      <td className="px-3 py-1.5">
        <div className="flex items-center gap-1.5">
          <span className="font-mono text-[var(--text)]">{row.model || "—"}</span>
          {row.provider && (
            <span className="text-[10px] text-[var(--text-muted)]">{row.provider}</span>
          )}
          {hasSavings && (
            <span className="flex items-center gap-0.5">
              {(row.slim_bytes_saved ?? 0) > 0 && (
                <span className="rounded bg-teal-100 px-1 py-0.5 text-[9px] font-bold text-teal-700 dark:bg-teal-900/30 dark:text-teal-300" title={`RTK saved ${fmtBytes(row.slim_bytes_saved ?? 0)} (${row.slim_rules ?? ""})`}>
                  RTK -{fmtBytes(row.slim_bytes_saved ?? 0)}
                </span>
              )}
              {row.caveman_active && (
                <span className="rounded bg-purple-100 px-1 py-0.5 text-[9px] font-bold text-purple-700 dark:bg-purple-900/30 dark:text-purple-300" title="Caveman output compression active">
                  🦍
                </span>
              )}
              {row.terse_active && (
                <span className="rounded bg-indigo-100 px-1 py-0.5 text-[9px] font-bold text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300" title="Terse output compression active">
                  ✂️
                </span>
              )}
            </span>
          )}
        </div>
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums text-[var(--text-muted)]">
        <span className="text-blue-500 dark:text-blue-400">{fmtNum(Math.round(row.tokens * 0.6))}↑</span>{" "}
        <span className="text-green-500 dark:text-green-400">{fmtNum(Math.round(row.tokens * 0.4))}↓</span>
      </td>
      <td className="px-3 py-1.5 text-right text-[var(--text-muted)]">{relTime(row.created_at)}</td>
    </tr>
  );
}

function ProviderIcon({ provider, color }: { provider: string; color: string }) {
  const [errored, setErrored] = useState(false);
  if (errored) {
    return (
      <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-[8px] font-bold text-white" style={{ backgroundColor: color || "var(--text-muted)" }}>
        {provider.slice(0, 2).toUpperCase()}
      </div>
    );
  }
  return <img src={`/providers/${provider}.png`} alt={provider} onError={() => setErrored(true)} className="h-5 w-5 shrink-0 rounded object-contain" />;
}

// ─── Token Savings Breakdown ────────────────────────────────────────────────

function TokenSavingsBreakdown({ savings, totalRequests, insights, period }: { savings: TokenSavings; totalRequests: number; insights: UsageInsights; period: string }) {
  const maxBytes = Math.max(...savings.rules.map((r) => r.bytes_saved), 1);
  const totalCavemanPct = totalRequests > 0 ? ((savings.caveman_requests / totalRequests) * 100).toFixed(1) : "0";
  const totalTersePct = totalRequests > 0 ? ((savings.terse_requests / totalRequests) * 100).toFixed(0) : "0";

  return (
    <Card>
      <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
        <div className="flex items-center gap-2">
          <Scissors className="h-4 w-4 text-[var(--text-muted)]" />
          <h3 className="text-sm font-semibold">Token Savings Breakdown</h3>
        </div>
        <div className="flex items-center gap-3">
          <SavingsCardShareButton insights={insights} period={period} />
          <div className="flex items-center gap-3 text-[11px] font-medium text-[var(--text-muted)]">
          {savings.caveman_requests > 0 && (
            <span className="flex items-center gap-1">
              <span className="h-1.5 w-1.5 rounded-full bg-purple-500" />
              Caveman {totalCavemanPct}%
            </span>
          )}
          {savings.terse_requests > 0 && (
            <span className="flex items-center gap-1">
              <span className="h-1.5 w-1.5 rounded-full bg-indigo-500" />
              Terse {totalTersePct}%
            </span>
          )}
          </div>
        </div>
      </div>
      <div className="p-4">
        <div className="space-y-3">
          {savings.rules.map((r) => (
            <div key={r.rule} className="flex items-center gap-4">
              <div className="w-36 shrink-0 text-xs font-mono font-medium text-[var(--text)]">{r.rule}</div>
              <div className="flex-1">
                <div className="h-7 overflow-hidden rounded-md bg-[#f4f4f5] dark:bg-white/5">
                  <div
                    className="flex h-full items-center rounded-md bg-[#00c781] px-2 text-[10px] font-bold leading-tight text-white transition-all overflow-hidden"
                    style={{ width: `${Math.max(6, (r.bytes_saved / maxBytes) * 100)}%`, wordBreak: "break-word" }}
                  >
                    {fmtBytes(r.bytes_saved).replace(" ", "\n")}
                  </div>
                </div>
              </div>
              <div className="w-24 text-right text-xs tabular-nums text-[var(--text-muted)]">
                {fmtNum(r.tokens_saved)} tok
              </div>
              <div className="w-12 text-right text-xs tabular-nums text-[var(--text-muted)]">
                {r.count}×
              </div>
            </div>
          ))}
        </div>
        <div className="mt-6 flex items-center justify-between border-t border-[var(--border)] pt-4 text-[13px]">
          <span className="text-[var(--text-muted)] font-medium">
            Total RTK savings: <span className="font-bold text-[#00c781]">{fmtBytes(savings.slim_bytes_saved)}</span> ({fmtNum(savings.slim_tokens_saved)} tokens)
          </span>
          <span className="text-[var(--text-muted)] font-medium">
            Est. cost saved: <span className="font-bold text-[#00c781]">${((savings.slim_tokens_saved / 1_000_000) * 3).toFixed(4)}</span>
          </span>
        </div>
      </div>
    </Card>
  );
}

// ─── Model Usage Table ──────────────────────────────────────────────────────

type SortKey = "provider" | "model" | "requests" | "prompt" | "completion" | "cost";

function ModelUsageTable({ models }: { models: ModelUsage[] }) {
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("requests");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  };

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    let rows = models;
    if (q) {
      rows = rows.filter(
        (m) => m.model.toLowerCase().includes(q) || m.provider_name.toLowerCase().includes(q) || m.provider.toLowerCase().includes(q),
      );
    }
    const sorted = [...rows].sort((a, b) => {
      const dir = sortDir === "asc" ? 1 : -1;
      switch (sortKey) {
        case "provider": return dir * a.provider_name.localeCompare(b.provider_name);
        case "model": return dir * a.model.localeCompare(b.model);
        case "requests": return dir * (a.total_requests - b.total_requests);
        case "prompt": return dir * (a.prompt_tokens - b.prompt_tokens);
        case "completion": return dir * (a.completion_tokens - b.completion_tokens);
        case "cost": return dir * (a.cost_usd - b.cost_usd);
        default: return 0;
      }
    });
    return sorted;
  }, [models, search, sortKey, sortDir]);

  const SortIcon = ({ col }: { col: SortKey }) => {
    if (sortKey !== col) return <ArrowUpDown className="ml-1 inline h-3 w-3 opacity-30" />;
    return sortDir === "asc" ? <ArrowUp className="ml-1 inline h-3 w-3" /> : <ArrowDown className="ml-1 inline h-3 w-3" />;
  };

  const th = (col: SortKey, label: string, align: "left" | "right" = "left") => (
    <th
      className={`cursor-pointer select-none px-4 py-2 font-medium text-[var(--text-muted)] transition-colors hover:text-[var(--text)] ${align === "right" ? "text-right" : "text-left"}`}
      onClick={() => toggleSort(col)}
    >
      {label}
      <SortIcon col={col} />
    </th>
  );

  return (
    <Card>
      <div className="flex flex-col gap-3 border-b border-[var(--border)] px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between">
        <h3 className="text-sm font-semibold">Model Usage</h3>
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search models…"
            className="rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] py-1.5 pl-8 pr-3 text-xs placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40"
          />
        </div>
      </div>
      {filtered.length === 0 ? (
        <div className="py-8 text-center text-xs text-[var(--text-muted)]">
          {models.length === 0 ? "No model data for this period" : "No models match your search"}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--border)]">
                {th("provider", "Provider")}
                {th("model", "Model")}
                {th("requests", "Requests", "right")}
                {th("prompt", "In Tok", "right")}
                {th("completion", "Out Tok", "right")}
                {th("cost", "Cost", "right")}
                <th className="px-4 py-2 text-right font-medium text-[var(--text-muted)]">Rate ($/M)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border)]">
              {filtered.map((m) => (
                <tr key={`${m.provider}/${m.model}`} className="transition-colors hover:bg-[var(--bg-subtle)]">
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      <ProviderIcon provider={m.provider} color="" />
                      <span className="font-medium">{m.provider_name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-2 font-mono">{m.model}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{m.total_requests.toLocaleString()}</td>
                  <td className="px-4 py-2 text-right tabular-nums text-blue-500">{fmtNum(m.prompt_tokens)}</td>
                  <td className="px-4 py-2 text-right tabular-nums text-green-500">{fmtNum(m.completion_tokens)}</td>
                  <td className="px-4 py-2 text-right tabular-nums font-medium">${m.cost_usd.toFixed(4)}</td>
                  <td className="px-4 py-2 text-right tabular-nums text-[var(--text-muted)]">
                    {m.input_per_m != null ? (
                      <span title={`In: $${m.input_per_m}/M · Out: $${m.output_per_m}/M${m.cached_input_per_m ? ` · Cached: $${m.cached_input_per_m}/M` : ""}`}>
                        ${m.input_per_m}/{m.output_per_m}
                      </span>
                    ) : (
                      <span className="opacity-40">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

function fmtBytes(n: number): string {
  if (n >= 1_048_576) return `${(n / 1_048_576).toFixed(1)} MB`;
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${n} B`;
}

function relTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const diff = Date.now() - t;
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}
