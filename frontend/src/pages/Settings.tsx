import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Sparkles, Zap, MessageSquare, Layers, Route, Wifi, Monitor, Database, Clock,
  ArrowUpCircle, CheckCircle2, ExternalLink,
} from "lucide-react";
import { api, type EndpointSettings } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useUpdateInfo } from "../components/UpdateNotification";
import { useToast } from "../components/Toast";
import {
  Card, SectionHeader, Spinner, Toggle, SegmentedControl, ErrorBanner, Button, Input, Field,
} from "../components/ui";

// Caveman compression maps to a Gentle / Balanced / Strong segmented control.
const cavemanOptions = [
  { value: "lite", label: "Gentle" },
  { value: "full", label: "Balanced" },
  { value: "ultra", label: "Strong" },
];
const cavemanHints: Record<string, string> = {
  lite: "Drop filler, keep full sentences.",
  full: "Terse caveman style, fragments OK.",
  ultra: "Maximum compression, telegraphic.",
};

const terseOptions = [
  { value: "light", label: "Gentle" },
  { value: "medium", label: "Balanced" },
  { value: "aggressive", label: "Strong" },
];
const terseHints: Record<string, string> = {
  light: "Trim pleasantries.",
  medium: "Bullets, minimal prose.",
  aggressive: "Bare technical minimum.",
};

const isRoundRobin = (strategy: string) =>
  strategy === "round-robin" || strategy === "round_robin" || strategy === "smart-round-robin" || strategy === "smart_round_robin";

export function SettingsPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const settings = useQuery({ queryKey: ["endpoint-settings"], queryFn: () => api.endpointSettings() });
  const [local, setLocal] = useState<EndpointSettings | null>(null);

  useEffect(() => {
    if (settings.data) setLocal(settings.data);
  }, [settings.data]);

  const save = useMutation({
    mutationFn: (patch: Partial<EndpointSettings>) => api.updateEndpointSettings(patch),
    onSuccess: (data) => {
      setLocal(data);
      qc.setQueryData(["endpoint-settings"], data);
    },
    onError: (e) => toast.error("Settings save failed", (e as Error).message),
  });

  const update = (patch: Partial<EndpointSettings>) => {
    if (local) setLocal({ ...local, ...patch });
    save.mutate(patch);
  };

  return (
    <>
      <PageHeader
        title="Settings"
        icon={Sparkles}
        description="Configure token saving, routing strategy, network, and more."
      />

      {settings.isLoading || !local ? (
        <Spinner />
      ) : (
        <div className="space-y-6">
          {/* Token Saving */}
          <Card>
            <SectionHeader
              title="RTK input compression"
              description="Compresses bulky tool outputs (diffs, greps, listings, build logs) before they reach the model. Saves input tokens. Safe by design — never corrupts content."
              icon={Zap}
            />
            <div className="flex items-center justify-between border-t border-[var(--border)] px-6 py-4">
              <span className="text-sm font-medium">Enable RTK token saver</span>
              <Toggle checked={local.rtk_enabled} onChange={(v) => update({ rtk_enabled: v })} />
            </div>
          </Card>

          <Card>
            <SectionHeader
              title="Caveman output compression"
              description="Instructs the model to answer tersely (caveman style) — keeps all technical substance, drops filler. Cuts output tokens 65-75%."
              icon={MessageSquare}
            />
            <div className="flex items-center justify-between border-t border-[var(--border)] px-6 py-4">
              <span className="text-sm font-medium">Enable caveman mode</span>
              <Toggle checked={local.caveman_enabled} onChange={(v) => update({ caveman_enabled: v, ...(v ? { terse_enabled: false } : {}) })} />
            </div>
            {local.caveman_enabled && (
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--border)] px-6 py-4">
                <div>
                  <p className="text-sm font-medium">Compression level</p>
                  <p className="mt-0.5 text-xs text-[var(--text-muted)]">{cavemanHints[local.caveman_level]}</p>
                </div>
                <SegmentedControl
                  value={local.caveman_level}
                  onChange={(v) => update({ caveman_level: v })}
                  options={cavemanOptions}
                />
              </div>
            )}
          </Card>

          <Card>
            <SectionHeader
              title="Terse mode (alternative)"
              description="KeiRouter's own concise-output directive. An alternative to caveman; both inject a system instruction, so pick one."
              icon={Layers}
              iconTone="neutral"
            />
            <div className="flex items-center justify-between border-t border-[var(--border)] px-6 py-4">
              <span className="text-sm font-medium">Enable terse mode</span>
              <Toggle checked={local.terse_enabled} onChange={(v) => update({ terse_enabled: v, ...(v ? { caveman_enabled: false } : {}) })} />
            </div>
            {local.terse_enabled && (
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--border)] px-6 py-4">
                <div>
                  <p className="text-sm font-medium">Terse level</p>
                  <p className="mt-0.5 text-xs text-[var(--text-muted)]">{terseHints[local.terse_level]}</p>
                </div>
                <SegmentedControl
                  value={local.terse_level}
                  onChange={(v) => update({ terse_level: v })}
                  options={terseOptions}
                />
              </div>
            )}
          </Card>

          {/* Routing Strategy */}
          <RoutingStrategy local={local} update={update} />

          {/* Timeouts */}
          <TimeoutSettings local={local} update={update} />

          {/* Network */}
          <NetworkSettings local={local} update={update} />

          {/* Observability */}
          <Card>
            <SectionHeader
              title="Observability"
              description="Record request details for inspection in the logs view."
              icon={Monitor}
            />
            <div className="flex items-center justify-between border-t border-[var(--border)] px-6 py-4">
              <span className="text-sm font-medium">Enable request detail recording</span>
              <Toggle
                checked={local.observability_enabled !== false}
                onChange={(v) => update({ observability_enabled: v })}
              />
            </div>
          </Card>

          {/* Updates */}
          <UpdatesSettings />

          {/* Database */}
          <DatabaseSettings />

          {save.isError && <ErrorBanner message={`Failed to save: ${(save.error as Error)?.message ?? "unknown error"}`} />}
        </div>
      )}
    </>
  );
}

function RoutingStrategy({
  local,
  update,
}: {
  local: EndpointSettings;
  update: (patch: Partial<EndpointSettings>) => void;
}) {
  return (
    <Card>
      <SectionHeader
        title="Routing Strategy"
        description="Control how requests are distributed across accounts and chains."
        icon={Route}
      />
      <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
        <div className="flex items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-sm font-medium">Provider group round robin</p>
            <p className="mt-0.5 text-xs text-[var(--text-muted)]">Cycle through accounts in the same provider/model group</p>
          </div>
          <Toggle
            checked={isRoundRobin(local.routing_strategy)}
            onChange={() => update({ routing_strategy: isRoundRobin(local.routing_strategy) ? "fill-first" : "round-robin" })}
          />
        </div>

        {isRoundRobin(local.routing_strategy) && (
          <div className="flex items-center justify-between gap-4 px-6 py-4">
            <div>
              <p className="text-sm font-medium">Provider sticky limit</p>
              <p className="mt-0.5 text-xs text-[var(--text-muted)]">Calls per account before switching</p>
            </div>
            <Input
              type="number"
              min={1}
              max={10}
              value={local.sticky_limit || 3}
              onChange={(e) => update({ sticky_limit: parseInt(e.target.value) || 3 })}
              className="w-20 text-center"
            />
          </div>
        )}

        <div className="flex items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-sm font-medium">Chain round robin</p>
            <p className="mt-0.5 text-xs text-[var(--text-muted)]">Cycle through providers in chains instead of always starting with first</p>
          </div>
          <Toggle
            checked={isRoundRobin(local.combo_strategy)}
            onChange={() => update({ combo_strategy: isRoundRobin(local.combo_strategy) ? "fallback" : "round-robin" })}
          />
        </div>

        {isRoundRobin(local.combo_strategy) && (
          <div className="flex items-center justify-between gap-4 px-6 py-4">
            <div>
              <p className="text-sm font-medium">Chain sticky limit</p>
              <p className="mt-0.5 text-xs text-[var(--text-muted)]">Calls per chain model before switching</p>
            </div>
            <Input
              type="number"
              min={1}
              max={100}
              value={local.combo_sticky_limit || 1}
              onChange={(e) => update({ combo_sticky_limit: parseInt(e.target.value) || 1 })}
              className="w-20 text-center"
            />
          </div>
        )}

        <div className="px-6 py-3">
          <p className="text-xs text-[var(--text-muted)] italic">
            {isRoundRobin(local.routing_strategy)
              ? `Distributing requests across all available accounts with ${local.sticky_limit || 3} calls per account.`
              : "Using accounts in priority order (Fill First)."}
            {isRoundRobin(local.combo_strategy)
              ? ` Chains rotate after ${local.combo_sticky_limit || 1} call${(local.combo_sticky_limit || 1) === 1 ? "" : "s"} per model.`
              : " Chains always start with their first model."}
          </p>
        </div>
      </div>
    </Card>
  );
}

function TimeoutSettings({
  local,
  update,
}: {
  local: EndpointSettings;
  update: (patch: Partial<EndpointSettings>) => void;
}) {
  return (
    <Card>
      <SectionHeader
        title="Timeouts"
        description="Fine-tune upstream connection and streaming timeouts. Increase for slow providers or reasoning models (Deepseek, GLM) that think before streaming."
        icon={Clock}
      />
      <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
        <div className="px-6 py-4">
          <Field label="Connect timeout (seconds)">
            <Input
              type="number"
              min={5}
              max={300}
              value={Math.round((local.response_header_timeout_ms || 60000) / 1000)}
              onChange={(e) => {
                const sec = parseInt(e.target.value) || 60;
                update({ response_header_timeout_ms: sec * 1000 });
              }}
              placeholder="60"
              className="w-24 text-center"
            />
            <p className="mt-1 text-xs text-[var(--text-muted)]">
              Max time waiting for upstream to send response headers. Default: 60s. Increase for slow providers.
            </p>
          </Field>
        </div>
        <div className="px-6 py-4">
          <Field label="Stream stall timeout (seconds)">
            <Input
              type="number"
              min={10}
              max={600}
              value={Math.round((local.stream_stall_timeout_ms || 120000) / 1000)}
              onChange={(e) => {
                const sec = parseInt(e.target.value) || 120;
                update({ stream_stall_timeout_ms: sec * 1000 });
              }}
              placeholder="120"
              className="w-24 text-center"
            />
            <p className="mt-1 text-xs text-[var(--text-muted)]">
              Abort stream if no data received for this long. Default: 120s. Increase for reasoning models that think before streaming.
            </p>
          </Field>
        </div>
        <div className="px-6 py-4">
          <Field label="Request timeout (seconds)">
            <Input
              type="number"
              min={30}
              max={3600}
              value={Math.round((local.request_timeout_ms || 300000) / 1000)}
              onChange={(e) => {
                const sec = parseInt(e.target.value) || 300;
                update({ request_timeout_ms: sec * 1000 });
              }}
              placeholder="300"
              className="w-24 text-center"
            />
            <p className="mt-1 text-xs text-[var(--text-muted)]">
              Upper bound for non-streaming requests. Default: 300s (5 min).
            </p>
          </Field>
        </div>
      </div>
    </Card>
  );
}

function NetworkSettings({
  local,
  update,
}: {
  local: EndpointSettings;
  update: (patch: Partial<EndpointSettings>) => void;
}) {
  const [testResult, setTestResult] = useState<string>("");
  const [testing, setTesting] = useState(false);

  const testProxy = async () => {
    if (!local.outbound_proxy_url) return;
    setTesting(true);
    setTestResult("");
    try {
      const res = await api.testProxy(local.outbound_proxy_url);
      if (res.ok) {
        setTestResult(`Proxy test OK (${res.status}) in ${res.elapsedMs}ms`);
      } else {
        setTestResult(`Proxy test failed: ${res.error}`);
      }
    } catch (e) {
      setTestResult(`Error: ${(e as Error).message}`);
    } finally {
      setTesting(false);
    }
  };

  return (
    <Card>
      <SectionHeader
        title="Network"
        description="Configure outbound proxy for provider requests."
        icon={Wifi}
      />
      <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
        <div className="flex items-center justify-between gap-4 px-6 py-4">
          <div>
            <p className="text-sm font-medium">Outbound Proxy</p>
            <p className="mt-0.5 text-xs text-[var(--text-muted)]">Enable proxy for OAuth + provider outbound requests.</p>
          </div>
          <Toggle
            checked={local.outbound_proxy_enabled}
            onChange={(v) => update({ outbound_proxy_enabled: v })}
          />
        </div>

        {local.outbound_proxy_enabled && (
          <>
            <div className="px-6 py-4">
              <Field label="Proxy URL">
                <Input
                  placeholder="http://127.0.0.1:7897"
                  value={local.outbound_proxy_url}
                  onChange={(e) => update({ outbound_proxy_url: e.target.value })}
                />
                <p className="mt-1 text-xs text-[var(--text-muted)]">Leave empty to inherit existing env proxy (if any).</p>
              </Field>
            </div>
            <div className="px-6 py-4">
              <Field label="No Proxy">
                <Input
                  placeholder="localhost,127.0.0.1"
                  value={local.outbound_no_proxy}
                  onChange={(e) => update({ outbound_no_proxy: e.target.value })}
                />
                <p className="mt-1 text-xs text-[var(--text-muted)]">Comma-separated hostnames/domains to bypass the proxy.</p>
              </Field>
            </div>
            <div className="flex items-center gap-3 px-6 py-4">
              <Button variant="ghost" onClick={testProxy} disabled={testing || !local.outbound_proxy_url}>
                {testing ? "Testing…" : "Test proxy URL"}
              </Button>
              {testResult && (
                <span className={`text-xs ${testResult.startsWith("Proxy test OK") ? "text-green-600 dark:text-green-400" : "text-[color:var(--color-danger)]"}`}>
                  {testResult}
                </span>
              )}
            </div>
          </>
        )}
      </div>
    </Card>
  );
}

function DatabaseSettings() {
  const toast = useToast();
  const importRef = useRef<HTMLInputElement>(null);
  const [loading, setLoading] = useState(false);

  const handleExport = async () => {
    setLoading(true);
    try {
      const data = await api.exportDatabase();
      const content = JSON.stringify(data, null, 2);
      const blob = new Blob([content], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      const stamp = new Date().toISOString().replace(/[.:]/g, "-");
      a.href = url;
      a.download = `keirouter-backup-${stamp}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success("Backup downloaded", "Full database snapshot saved as JSON. Store it safely for disaster recovery.");
    } catch (e) {
      toast.error("Export failed", (e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    setLoading(true);
    try {
      const raw = await file.text();
      const payload = JSON.parse(raw);
      const result = await api.importDatabase(payload);
      toast.success("Import complete", `${result.imported} records restored. Existing data was merged or updated.`);
    } catch (e) {
      toast.error("Import failed", (e as Error).message);
    } finally {
      if (importRef.current) importRef.current.value = "";
      setLoading(false);
    }
  };

  return (
    <Card>
      <SectionHeader
        title="Database"
        description="Export or import your KeiRouter configuration."
        icon={Database}
      />
      <div className="flex items-center gap-3 border-t border-[var(--border)] px-6 py-4">
        <Button variant="ghost" onClick={handleExport} disabled={loading}>
          <Database className="h-4 w-4" />
          Download Backup
        </Button>
        <Button variant="ghost" onClick={() => importRef.current?.click()} disabled={loading}>
          <Database className="h-4 w-4" />
          Import Backup
        </Button>
        <input
          ref={importRef}
          type="file"
          accept="application/json,.json"
          className="hidden"
          onChange={handleImport}
        />
      </div>
    </Card>
  );
}

function UpdatesSettings() {
  const { data, isLoading, isError } = useUpdateInfo();

  const publishedLabel = data?.published_at
    ? new Date(data.published_at).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      })
    : "";

  return (
    <Card>
      <SectionHeader
        title="Updates"
        description="Check for new KeiRouter releases and read the latest changelog."
        icon={ArrowUpCircle}
      />
      <div className="border-t border-[var(--border)] px-6 py-4">
        {isLoading ? (
          <Spinner />
        ) : isError || !data || !data.checked ? (
          <p className="text-sm text-[var(--text-muted)]">
            Could not reach GitHub to check for updates. Current version:{" "}
            <span className="font-mono text-[var(--text)]">{data?.current ?? "dev"}</span>
          </p>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                  Current
                </p>
                <p className="font-mono text-sm">{data.current}</p>
              </div>
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                  Latest
                </p>
                <p className="font-mono text-sm">{data.latest || "—"}</p>
              </div>
              <div className="ml-auto">
                {data.update_available ? (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-accent-100 px-3 py-1 text-xs font-medium text-accent-700 dark:bg-accent-900/40 dark:text-accent-200">
                    <ArrowUpCircle className="h-3.5 w-3.5" />
                    Update available
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300">
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    Up to date
                  </span>
                )}
              </div>
            </div>

            {data.update_available && data.changelog && (
              <div>
                <div className="mb-1 flex items-center justify-between">
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                    Changelog{publishedLabel ? ` · ${publishedLabel}` : ""}
                  </p>
                  {data.html_url && (
                    <a
                      href={data.html_url}
                      target="_blank"
                      rel="noreferrer noopener"
                      className="flex items-center gap-1 text-xs text-accent-600 hover:underline dark:text-accent-300"
                    >
                      View on GitHub
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  )}
                </div>
                <div className="max-h-80 overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--bg)] p-4">
                  <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-relaxed text-[var(--text)]">
                    {data.changelog}
                  </pre>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}
