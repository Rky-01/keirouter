import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Network,
  Copy,
  Check,
  Monitor,
  Lock,
  Radar,
  Zap,
  MessageSquare,
  KeyRound,
  Plus,
  Trash2,
} from "lucide-react";
import { api, type AccessSettings, type EndpointSettings, type APIKey, type CreatedKey } from "../lib/api";
import { PageHeader } from "../components/Layout";
import {
  Card,
  SectionHeader,
  CardHeader,
  Button,
  Input,
  Field,
  Badge,
  Spinner,
  EmptyState,
  Toggle,
  SegmentedControl,
} from "../components/ui";

const cavemanOptions = [
  { value: "lite", label: "Gentle" },
  { value: "full", label: "Balanced" },
  { value: "ultra", label: "Strong" },
];

export function EndpointsPage() {
  return (
    <>
      <PageHeader
        title="Endpoints"
        icon={Network}
        description="Configure how KeiRouter connects to your application."
      />
      <div className="space-y-6">
        <PrimaryEndpoint />
        <AccessOptions />
        <ResponseOptimization />
        <APIKeys />
      </div>
    </>
  );
}

// ---- primary endpoint -------------------------------------------------------

function PrimaryEndpoint() {
  const access = useQuery({ queryKey: ["access-settings"], queryFn: () => api.accessSettings() });
  const [copied, setCopied] = useState(false);
  const url = access.data?.endpoint_url ?? "";

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // no-op
    }
  };

  return (
    <Card>
      <SectionHeader
        title="Primary endpoint"
        description="The main API endpoint your app uses to connect to KeiRouter."
        icon={Network}
      />
      <div className="border-t border-[var(--border)] px-6 py-5">
        <Field label="Endpoint URL">
          <div className="flex items-center gap-2">
            <Input value={url} readOnly className="font-mono" />
            <Button variant="ghost" onClick={copy} className="shrink-0">
              {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
        </Field>
      </div>
    </Card>
  );
}

// ---- access options ---------------------------------------------------------

function AccessOptions() {
  const qc = useQueryClient();
  const access = useQuery({ queryKey: ["access-settings"], queryFn: () => api.accessSettings() });
  const [local, setLocal] = useState<AccessSettings | null>(null);

  useEffect(() => {
    if (access.data) setLocal(access.data);
  }, [access.data]);

  const save = useMutation({
    mutationFn: (patch: Partial<Omit<AccessSettings, "endpoint_url">>) =>
      api.updateAccessSettings(patch),
    onSuccess: (data) => {
      setLocal(data);
      qc.setQueryData(["access-settings"], data);
    },
  });

  const update = (patch: Partial<Omit<AccessSettings, "endpoint_url">>) => {
    if (local) setLocal({ ...local, ...patch });
    save.mutate(patch);
  };

  return (
    <Card>
      <SectionHeader
        title="Access options"
        description="Choose how you'd like to reach the KeiRouter API. You can enable one or more options."
        icon={Lock}
      />
      {!local ? (
        <Spinner />
      ) : (
        <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
          <AccessRow
            icon={Monitor}
            title="Local access"
            description="Reach KeiRouter over your local network."
            checked={local.local_enabled}
            onChange={(v) => update({ local_enabled: v })}
          />
          <AccessRow
            icon={Lock}
            title="Secure tunnel"
            description="Route traffic through a secure tunnel for added privacy."
            checked={local.tunnel_enabled}
            onChange={(v) => update({ tunnel_enabled: v })}
          />
          <AccessRow
            icon={Radar}
            title="Tailscale"
            description="Access KeiRouter over your private Tailscale network."
            checked={local.tailscale_enabled}
            onChange={(v) => update({ tailscale_enabled: v })}
          />
        </div>
      )}
    </Card>
  );
}

function AccessRow({
  icon: Icon,
  title,
  description,
  checked,
  onChange,
}: {
  icon: typeof Monitor;
  title: string;
  description: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-4 px-6 py-4">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-ink-100 text-ink-600 dark:bg-ink-800 dark:text-ink-300">
          <Icon className="h-[18px] w-[18px]" />
        </span>
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-0.5 text-xs text-[var(--text-muted)]">{description}</p>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-xs text-[var(--text-muted)]">{checked ? "On" : "Off"}</span>
        <Toggle checked={checked} onChange={onChange} />
      </div>
    </div>
  );
}

// ---- response optimization --------------------------------------------------

function ResponseOptimization() {
  const qc = useQueryClient();
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
  });

  const update = (patch: Partial<EndpointSettings>) => {
    if (local) setLocal({ ...local, ...patch });
    save.mutate(patch);
  };

  return (
    <Card>
      <SectionHeader
        title="Response optimization"
        description="Tune how KeiRouter formats responses to reduce size and improve performance."
        icon={Zap}
      />
      {!local ? (
        <Spinner />
      ) : (
        <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
          <div className="flex items-center justify-between gap-4 px-6 py-4">
            <div className="flex items-start gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
                <Zap className="h-[18px] w-[18px]" />
              </span>
              <div>
                <p className="text-sm font-medium">Compact tool output</p>
                <p className="mt-0.5 text-xs text-[var(--text-muted)]">
                  Reduce the size of tool results and logs (RTK).
                </p>
              </div>
            </div>
            <Toggle checked={local.rtk_enabled} onChange={(v) => update({ rtk_enabled: v })} />
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 px-6 py-4">
            <div className="flex items-start gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-accent-100 text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
                <MessageSquare className="h-[18px] w-[18px]" />
              </span>
              <div>
                <p className="text-sm font-medium">Compact AI responses</p>
                <p className="mt-0.5 text-xs text-[var(--text-muted)]">
                  Shorten AI responses while preserving meaning (caveman).
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Toggle
                checked={local.caveman_enabled}
                onChange={(v) => update({ caveman_enabled: v })}
              />
              {local.caveman_enabled && (
                <SegmentedControl
                  value={local.caveman_level}
                  onChange={(v) => update({ caveman_level: v })}
                  options={cavemanOptions}
                />
              )}
            </div>
          </div>
        </div>
      )}
    </Card>
  );
}

// ---- API keys ---------------------------------------------------------------

function APIKeys() {
  const qc = useQueryClient();
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api.listKeys() });
  const [name, setName] = useState("");
  const [created, setCreated] = useState<CreatedKey | null>(null);
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () => api.createKey(name),
    onSuccess: (data) => {
      setCreated(data);
      setName("");
      setError("");
      qc.invalidateQueries({ queryKey: ["keys"] });
    },
    onError: (e) => setError((e as Error).message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["keys"] }),
  });

  return (
    <Card>
      <CardHeader
        title="API keys"
        description="Create and manage API keys for authenticating your applications."
      />
      <div className="space-y-4 px-6 py-5">
        <div className="flex flex-wrap items-end gap-3">
          <Field label="Key name">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Production key"
              className="w-56"
            />
          </Field>
          <Button onClick={() => create.mutate()} disabled={!name.trim() || create.isPending}>
            <Plus className="h-4 w-4" />
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
          {error && <span className="text-xs text-[color:var(--color-danger)]">{error}</span>}
        </div>

        {created && (
          <div className="rounded-xl border border-accent-200 bg-accent-50 px-4 py-3 dark:border-accent-800/50 dark:bg-accent-800/20">
            <p className="text-xs font-medium text-accent-700 dark:text-accent-200">
              Copy this key now — it won't be shown again.
            </p>
            <code className="mt-1.5 block break-all font-mono text-sm">{created.key}</code>
          </div>
        )}
      </div>

      {keys.isLoading ? (
        <Spinner />
      ) : !keys.data?.keys.length ? (
        <EmptyState title="No API keys yet" hint="Create a key to authenticate your app." />
      ) : (
        <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
          {keys.data.keys.map((k) => (
            <KeyRow key={k.id} k={k} onDelete={() => remove.mutate(k.id)} />
          ))}
        </div>
      )}
    </Card>
  );
}

function KeyRow({ k, onDelete }: { k: APIKey; onDelete: () => void }) {
  return (
    <div className="flex items-center justify-between gap-4 px-6 py-4">
      <div className="flex items-center gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-ink-100 text-ink-600 dark:bg-ink-800 dark:text-ink-300">
          <KeyRound className="h-[18px] w-[18px]" />
        </span>
        <div>
          <p className="text-sm font-medium">{k.name}</p>
          <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">{k.display}</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <Badge tone={k.disabled ? "neutral" : "success"}>{k.disabled ? "Disabled" : "Active"}</Badge>
        <Button variant="danger" onClick={onDelete}>
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}