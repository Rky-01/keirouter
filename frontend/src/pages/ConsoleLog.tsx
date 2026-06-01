import { useQuery } from "@tanstack/react-query";
import { ScrollText, RefreshCw } from "lucide-react";
import { api, type ConsoleEntry } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, CardHeader, Button, Spinner, EmptyState } from "../components/ui";

const levelMeta: Record<string, { label: string; className: string }> = {
  info: { label: "INFO", className: "text-accent-600 dark:text-accent-300" },
  warn: { label: "WARN", className: "text-[color:var(--color-warning)]" },
  error: { label: "ERR ", className: "text-[color:var(--color-danger)]" },
};

export function ConsoleLogPage() {
  const log = useQuery({ queryKey: ["console-log"], queryFn: () => api.consoleLog() });
  const entries = log.data?.entries ?? [];

  return (
    <>
      <PageHeader
        title="Console Log"
        icon={ScrollText}
        description="A live feed of the requests KeiRouter has handled, newest first."
        action={
          <Button variant="ghost" onClick={() => log.refetch()} disabled={log.isFetching}>
            <RefreshCw className={`h-4 w-4 ${log.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        }
      />

      <Card>
        <CardHeader title={`${entries.length} entries`} />
        {log.isLoading ? (
          <Spinner />
        ) : !entries.length ? (
          <EmptyState
            title="No log entries yet"
            hint="Requests will appear here once traffic flows through KeiRouter."
          />
        ) : (
          <div className="max-h-[60vh] overflow-y-auto rounded-b-2xl bg-[var(--bg-subtle)] p-4 font-mono text-xs leading-relaxed">
            {entries.map((e) => (
              <LogLine key={e.id} entry={e} />
            ))}
          </div>
        )}
      </Card>
    </>
  );
}

function LogLine({ entry: e }: { entry: ConsoleEntry }) {
  const meta = levelMeta[e.level] ?? levelMeta.info;
  return (
    <div className="flex gap-3 py-0.5">
      <span className="shrink-0 text-[var(--text-muted)]">{fmtTime(e.created_at)}</span>
      <span className={`shrink-0 font-semibold ${meta.className}`}>{meta.label}</span>
      <span className="min-w-0 flex-1 truncate">
        <span className="text-[var(--text)]">{e.provider}</span>
        <span className="text-[var(--text-muted)]"> · {e.model || "—"}</span>
        <span className="text-[var(--text-muted)]">
          {" "}
          · {e.tokens.toLocaleString()} tok · ${e.cost_usd.toFixed(3)} · {e.latency_ms}ms
          {e.cache_hit ? " · cache" : ""}
        </span>
      </span>
    </div>
  );
}

function fmtTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "--:--:--";
  return d.toLocaleTimeString([], { hour12: false });
}