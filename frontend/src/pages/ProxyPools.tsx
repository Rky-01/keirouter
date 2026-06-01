import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Network, Plus, Trash2 } from "lucide-react";
import { api, type ProxyPool } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useToast } from "../components/Toast";
import { Card, SectionHeader, CardHeader, Button, Input, Field, Badge, Spinner, EmptyState, ErrorBanner } from "../components/ui";

export function ProxyPoolsPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const pools = useQuery({ queryKey: ["proxy-pools"], queryFn: () => api.listProxyPools() });

  const [name, setName] = useState("");
  const [proxies, setProxies] = useState("");
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.createProxyPool({
        name,
        proxies: proxies
          .split("\n")
          .map((p) => p.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["proxy-pools"] });
      setName("");
      setProxies("");
      setError("");
      toast.success("Proxy pool created");
    },
    onError: (e) => {
      setError((e as Error).message);
      toast.error("Couldn't create proxy pool", (e as Error).message);
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteProxyPool(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["proxy-pools"] });
      toast.success("Proxy pool removed");
    },
    onError: (e) => toast.error("Couldn't remove proxy pool", (e as Error).message),
  });

  return (
    <>
      <PageHeader
        title="Proxy Pools"
        icon={Network}
        description="Route upstream provider traffic through pools of egress proxies for resilience and geo-distribution."
      />

      <div className="space-y-6">
        <Card>
          <SectionHeader
            title="Create proxy pool"
            description="Add one proxy URL per line (http(s) or socks5)."
            icon={Plus}
          />
          <div className="space-y-4 border-t border-[var(--border)] px-6 py-5">
            <Field label="Pool name">
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="us-east-residential"
                className="max-w-md"
              />
            </Field>
            <Field label="Proxy URLs">
              <textarea
                value={proxies}
                onChange={(e) => setProxies(e.target.value)}
                rows={4}
                placeholder={"http://user:pass@host:port\nsocks5://host:port"}
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 font-mono text-xs placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40"
              />
            </Field>
            {error && <ErrorBanner message={error} />}
            <div className="flex items-center gap-3">
              <Button onClick={() => create.mutate()} disabled={!name.trim() || create.isPending}>
                <Plus className="h-4 w-4" />
                {create.isPending ? "Creating…" : "Create pool"}
              </Button>
            </div>
          </div>
        </Card>

        <Card>
          <CardHeader title="Proxy pools" />
          {pools.isLoading ? (
            <Spinner />
          ) : !pools.data?.pools.length ? (
            <EmptyState
              title="No proxy pools yet"
              hint="Traffic goes direct until you add a pool."
            />
          ) : (
            <div className="divide-y divide-[var(--border)]">
              {pools.data.pools.map((p) => (
                <PoolRow key={p.id} pool={p} onDelete={() => remove.mutate(p.id)} />
              ))}
            </div>
          )}
        </Card>
      </div>
    </>
  );
}

function PoolRow({ pool, onDelete }: { pool: ProxyPool; onDelete: () => void }) {
  return (
    <div className="flex items-start justify-between gap-4 px-6 py-4">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{pool.name}</span>
          <Badge tone={pool.enabled ? "success" : "neutral"}>
            {pool.enabled ? "Enabled" : "Disabled"}
          </Badge>
        </div>
        <p className="mt-1 text-xs text-[var(--text-muted)]">
          {pool.proxies.length} {pool.proxies.length === 1 ? "proxy" : "proxies"}
        </p>
        {pool.proxies.length > 0 && (
          <ul className="mt-2 space-y-0.5">
            {pool.proxies.slice(0, 3).map((proxy, i) => (
              <li key={i} className="truncate font-mono text-xs text-[var(--text-muted)]">
                {maskProxy(proxy)}
              </li>
            ))}
            {pool.proxies.length > 3 && (
              <li className="text-xs text-[var(--text-muted)]">+{pool.proxies.length - 3} more</li>
            )}
          </ul>
        )}
      </div>
      <Button variant="danger" onClick={onDelete} className="shrink-0">
        <Trash2 className="h-4 w-4" />
      </Button>
    </div>
  );
}

// maskProxy hides credentials embedded in a proxy URL before display.
function maskProxy(url: string): string {
  return url.replace(/\/\/[^@/]+@/, "//••••@");
}