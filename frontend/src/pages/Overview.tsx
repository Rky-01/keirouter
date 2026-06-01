import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  DollarSign,
  Database,
  Zap,
  Sparkles,
  Calendar,
  Rocket,
} from "lucide-react";
import { PieChart, Pie, Cell, ResponsiveContainer } from "recharts";
import { api } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, SectionHeader, Spinner, StatCard, ErrorCard } from "../components/ui";

const periods = [
  { value: "today", label: "Today" },
  { value: "week", label: "Last 7 days" },
  { value: "month", label: "Last 30 days" },
];

export function OverviewPage() {
  const [period, setPeriod] = useState("month");
  const usage = useQuery({ queryKey: ["usage", period], queryFn: () => api.usage(period) });

  return (
    <>
      <PageHeader
        title="Overview"
        icon={Sparkles}
        description="Usage and spending across all providers."
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

      {usage.isLoading ? (
        <Spinner />
      ) : usage.isError ? (
        <ErrorCard message="Failed to load usage. Is the backend running?" />
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              icon={Activity}
              iconTone="accent"
              label="Total Requests"
              value={usage.data!.total_requests.toLocaleString()}
            />
            <StatCard
              icon={DollarSign}
              iconTone="warning"
              label="Estimated Cost"
              value={`$${usage.data!.cost_usd.toFixed(2)}`}
            />
            <StatCard
              icon={Database}
              iconTone="accent"
              label="Tokens in / out"
              value={`${compact(usage.data!.prompt_tokens)} / ${compact(usage.data!.completion_tokens)}`}
            />
            <StatCard
              icon={Zap}
              iconTone="accent"
              label="Cache Hits"
              value={usage.data!.cache_hits.toLocaleString()}
            />
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <Card className="lg:col-span-2">
              <SectionHeader
                title="Token distribution"
                description="How tokens break down across input, output, and cache for this period."
                icon={Database}
              />
              <TokenBreakdown
                prompt={usage.data!.prompt_tokens}
                completion={usage.data!.completion_tokens}
                cached={usage.data!.cached_tokens}
              />
            </Card>

            <Card>
              <SectionHeader title="Getting started" icon={Rocket} />
              <div className="px-6 pb-6">
                <ol className="space-y-3 text-sm text-[var(--text-muted)]">
                  <Step n={1} text="Add a provider account under Accounts." />
                  <Step n={2} text="Create a routing chain to define fallback order." />
                  <Step n={3} text="Create an API key and point your tool at it." />
                </ol>
                <pre className="mt-4 overflow-x-auto rounded-lg bg-[var(--bg-subtle)] p-3 font-mono text-xs leading-relaxed">
{`Base URL: http://localhost:20180/v1
Model:    openai/gpt-4o
       or chain:my-chain`}
                </pre>
              </div>
            </Card>
          </div>
        </div>
      )}
    </>
  );
}

function Step({ n, text }: { n: number; text: string }) {
  return (
    <li className="flex items-start gap-3">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent-100 text-[11px] font-semibold text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
        {n}
      </span>
      <span>{text}</span>
    </li>
  );
}

function TokenBreakdown({
  prompt,
  completion,
  cached,
}: {
  prompt: number;
  completion: number;
  cached: number;
}) {
  const data = [
    { name: "Input", value: prompt, color: "#4a7a6f" },
    { name: "Output", value: completion, color: "#8fb5a8" },
    { name: "Cached", value: cached, color: "#b08a3e" },
  ];
  const total = prompt + completion + cached;

  if (total === 0) {
    return (
      <div className="px-6 py-12 text-center text-sm text-[var(--text-muted)]">
        No token usage recorded yet for this period.
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-6 px-6 pb-6 sm:flex-row sm:gap-8">
      <div className="relative h-40 w-40 shrink-0">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              innerRadius={52}
              outerRadius={72}
              paddingAngle={2}
              startAngle={90}
              endAngle={-270}
              stroke="none"
            >
              {data.map((d) => (
                <Cell key={d.name} fill={d.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-xl font-semibold tracking-tight">{compact(total)}</span>
          <span className="text-xs text-[var(--text-muted)]">tokens</span>
        </div>
      </div>

      <div className="flex-1 space-y-3">
        {data.map((d) => (
          <div key={d.name} className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-2.5">
              <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: d.color }} />
              <span className="text-sm">{d.name}</span>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium tabular-nums">{d.value.toLocaleString()}</span>
              <span className="w-12 text-right text-xs text-[var(--text-muted)]">
                {total ? `${((d.value / total) * 100).toFixed(1)}%` : "0%"}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function compact(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}