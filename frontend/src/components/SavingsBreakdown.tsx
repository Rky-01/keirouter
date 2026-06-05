import { Scissors } from "lucide-react";
import type { TokenSavings, UsageInsights } from "../lib/api";
import { SavingsCardShareButton } from "./SavingsCard";

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

export function TokenSavingsBreakdown({ savings, totalRequests, insights, period }: { savings: TokenSavings; totalRequests: number; insights: UsageInsights; period: string }) {
  const rules = savings.rules || [];
  const maxBytes = Math.max(...rules.map((r) => r.bytes_saved), 1);
  const totalCavemanPct = totalRequests > 0 ? ((savings.caveman_requests / totalRequests) * 100).toFixed(1) : "0";
  const totalTersePct = totalRequests > 0 ? ((savings.terse_requests / totalRequests) * 100).toFixed(0) : "0";
  const hasSavings = savings.slim_bytes_saved > 0 || savings.caveman_requests > 0 || savings.terse_requests > 0 || rules.length > 0;

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--bg)] shadow-sm overflow-hidden">
      <div className="flex items-center justify-between border-b border-[var(--border)] px-5 py-3 bg-[var(--bg-subtle)]">
        <div className="flex items-center gap-2">
          <Scissors className="h-4 w-4 text-[var(--text-muted)]" />
          <h3 className="text-sm font-semibold tracking-tight">Optimization Engine</h3>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
          {savings.caveman_requests > 0 && (
            <span className="flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-purple-500" />
              CVMN {totalCavemanPct}%
            </span>
          )}
          {savings.terse_requests > 0 && (
            <span className="flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-indigo-500" />
              TRSE {totalTersePct}%
            </span>
          )}
          </div>
          <SavingsCardShareButton insights={insights} period={period} />
        </div>
      </div>
      <div className="p-5">
        <div className="space-y-4">
          {rules.length === 0 ? (
            <div className="flex items-center justify-center py-6 text-xs font-medium text-[var(--text-muted)]">
              {!hasSavings 
                ? "No optimizations active for this period" 
                : "Output optimizations active (no prompt savings yet)"}
            </div>
          ) : (
            rules.map((r) => (
              <div key={r.rule} className="flex items-center gap-4">
                <div className="w-32 shrink-0 text-xs font-mono font-medium text-[var(--text)] truncate" title={r.rule}>{r.rule}</div>
                <div className="flex-1">
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-subtle)]">
                    <div
                      className="h-full rounded-full bg-[var(--text)] transition-all"
                      style={{ width: `${Math.max(2, (r.bytes_saved / maxBytes) * 100)}%` }}
                    />
                  </div>
                </div>
                <div className="w-24 text-right text-xs font-medium tabular-nums text-[var(--text)]">
                  {fmtBytes(r.bytes_saved)}
                </div>
                <div className="w-20 text-right text-[10px] font-medium tabular-nums text-[var(--text-muted)] uppercase">
                  {fmtNum(r.tokens_saved)} tok
                </div>
                <div className="w-12 text-right text-[10px] font-medium tabular-nums text-[var(--text-muted)]">
                  {r.count}×
                </div>
              </div>
            ))
          )}
        </div>
        <div className="mt-6 flex items-center justify-between border-t border-[var(--border)] pt-4">
          <div className="flex flex-col">
            <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">Total Savings</span>
            <span className="text-lg font-light text-[var(--text)] tabular-nums">{fmtBytes(savings.slim_bytes_saved)} <span className="text-xs text-[var(--text-muted)] font-medium ml-1">({fmtNum(savings.slim_tokens_saved)} tokens)</span></span>
          </div>
          <div className="flex flex-col text-right">
            <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">Est. Value Saved</span>
            <span className="text-lg font-light text-[var(--text)] tabular-nums">${((savings.slim_tokens_saved / 1_000_000) * 3).toFixed(4)}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
