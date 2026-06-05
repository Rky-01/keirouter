import { useEffect, useRef, useState, useMemo } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Plus, Trash2, Plug, X, Zap, ArrowUp, ArrowDown, CheckCircle, ToggleLeft, ToggleRight, Search, Route } from "lucide-react";
import { api, type DeviceCode, type OAuthProvider, type Provider, type Account, type ProxyPool, type UpstreamQuota, type ProviderRoutingSettings } from "../lib/api";
import { KiroConnectModal } from "../components/KiroConnectModal";
import { useToast } from "../components/Toast";
import {
  Card,
  CardHeader,
  Button,
  Input,
  Field,
  Badge,
  Spinner,
  EmptyState,
  ErrorBanner,
  SegmentedControl,
} from "../components/ui";

// defaultRedirectURI is the OAuth callback the provider redirects to after sign-in.
// The backend intercepts this path, exchanges the code, and redirects to a
// frontend callback page that notifies this tab via postMessage.
const defaultRedirectURI = "http://localhost:20180/oauth/callback";

function redirectURIForProvider(provider: OAuthProvider): string {
  if (provider.fixed_port && provider.callback_path) {
    const host = provider.loopback_host || "127.0.0.1";
    return `http://${host}:${provider.fixed_port}${provider.callback_path}`;
  }
  return defaultRedirectURI;
}

export function ProviderDetailPage() {
  const { id } = useParams();
  const qc = useQueryClient();
  const toast = useToast();

  const providers = useQuery({ queryKey: ["providers"], queryFn: () => api.providers() });
  const accounts = useQuery({ queryKey: ["accounts"], queryFn: () => api.listAccounts() });
  const oauthProviders = useQuery({ queryKey: ["oauth-providers"], queryFn: () => api.oauthProviders() });
  const pools = useQuery({ queryKey: ["proxy-pools"], queryFn: () => api.listProxyPools() });
  const routing = useQuery({
    queryKey: ["provider-routing", id],
    queryFn: () => api.providerRouting(id!),
    enabled: !!id,
  });
  const disabledModels = useQuery({
    queryKey: ["disabled-models", id],
    queryFn: () => api.listDisabledModels(id!),
    enabled: !!id,
  });
  const models = useQuery({
    queryKey: ["provider-models", id],
    queryFn: () => api.providerModels(id!),
    enabled: !!id,
    staleTime: 60_000,
  });

  const provider = providers.data?.providers.find((p) => p.id === id);
  const oauthProvider = oauthProviders.data?.providers.find((p) => p.provider === id);
  const myAccounts = (accounts.data?.accounts ?? []).filter((a) => a.provider === id);

  const [label, setLabel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [region, setRegion] = useState("");
  const [accountID, setAccountID] = useState("");
  const [azureEndpoint, setAzureEndpoint] = useState("");
  const [azureDeployment, setAzureDeployment] = useState("");
  const [azureAPIVersion, setAzureAPIVersion] = useState("2024-10-01-preview");
  const [azureOrganization, setAzureOrganization] = useState("");
  const [error, setError] = useState("");
  const [oauthOpen, setOauthOpen] = useState(false);
  const [kiroOpen, setKiroOpen] = useState(false);
  const [addKeyOpen, setAddKeyOpen] = useState(false);

  // Model search and pagination
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [modelPage, setModelPage] = useState(1);
  const MODELS_PER_PAGE = 12;

  const filteredModels = useMemo(() => {
    if (!models.data?.models) return [];
    if (!modelSearchQuery.trim()) return models.data.models;
    const lowerQ = modelSearchQuery.toLowerCase();
    return models.data.models.filter(m => 
      m.id.toLowerCase().includes(lowerQ) || 
      (m.name && m.name.toLowerCase().includes(lowerQ)) ||
      (m.kind && m.kind.toLowerCase().includes(lowerQ))
    );
  }, [models.data?.models, modelSearchQuery]);

  useEffect(() => {
    setModelPage(1);
  }, [modelSearchQuery]);

  const totalModelPages = Math.ceil(filteredModels.length / MODELS_PER_PAGE);
  const paginatedModels = filteredModels.slice(
    (modelPage - 1) * MODELS_PER_PAGE, 
    modelPage * MODELS_PER_PAGE
  );

  // Set default region when provider loads.
  useEffect(() => {
    if (provider?.default_region && !region) {
      setRegion(provider.default_region);
    }
  }, [provider, region]);

  const hasRegions = (provider?.regions?.length ?? 0) > 0;

  const create = useMutation({
    mutationFn: () =>
      api.createAccount({
        provider: id!,
        label,
        api_key: apiKey,
        base_url: baseURL || undefined,
        region: hasRegions ? region : undefined,
        account_id: accountID || undefined,
        azure_endpoint: azureEndpoint || undefined,
        azure_deployment: azureDeployment || undefined,
        azure_api_version: azureAPIVersion || undefined,
        azure_organization: azureOrganization || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] });
      setLabel("");
      setApiKey("");
      setBaseURL("");
      setAccountID("");
      setAzureEndpoint("");
      setAzureDeployment("");
      setAzureAPIVersion("2024-10-01-preview");
      setAzureOrganization("");
      setError("");
      setAddKeyOpen(false);
      toast.success("Account connected", `Upstream credentials saved and encrypted. The account is ready for routing.`);
    },
    onError: (e: Error) => {
      setError(e.message);
      toast.error("Account connection failed", e.message);
    },
  });

  const remove = useMutation({
    mutationFn: (accountId: string) => api.deleteAccount(accountId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] });
      toast.success("Account removed", "The upstream credential has been deleted and encrypted secrets purged.");
    },
    onError: (e: Error) => toast.error("Account removal failed", e.message),
  });

  const updateAccount = useMutation({
    mutationFn: ({ id: accId, patch }: { id: string; patch: { label?: string; priority?: number; disabled?: boolean; proxy_pool_id?: string } }) =>
      api.updateAccount(accId, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["accounts"] }),
    onError: (e: Error) => toast.error("Account update failed", e.message),
  });

  const testAccount = useMutation({
    mutationFn: (accountId: string) => api.testAccount(accountId),
    onSuccess: (data) => {
      if (data.status === "ok") {
        toast.success("Credentials verified", "The upstream API key is valid and the provider is reachable.");
      } else {
        toast.error("Credential check failed", data.message);
      }
    },
    onError: (e: Error) => toast.error("Credential check failed", e.message),
  });

  const disableModelsMut = useMutation({
    mutationFn: (ids: string[]) => api.disableModels(id!, ids),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["disabled-models", id] });
      toast.success("Models disabled", "Selected models will be excluded from routing until re-enabled.");
    },
    onError: (e: Error) => toast.error("Model disable failed", e.message),
  });

  const enableModelsMut = useMutation({
    mutationFn: (ids: string[]) => api.enableModels(id!, ids),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["disabled-models", id] });
      toast.success("Models re-enabled", "Selected models are available for routing again.");
    },
    onError: (e: Error) => toast.error("Couldn't enable models", e.message),
  });

  const updateRouting = useMutation({
    mutationFn: (patch: Partial<ProviderRoutingSettings>) => api.updateProviderRouting(id!, patch),
    onSuccess: (data) => {
      qc.setQueryData(["provider-routing", id], data);
      toast.success("Routing updated", "Account routing strategy for this provider was saved.");
    },
    onError: (e: Error) => toast.error("Routing update failed", e.message),
  });

  // Sort accounts by priority for display.
  const sortedAccounts = [...myAccounts].sort((a, b) => a.priority - b.priority);
  const disabledModelIds = new Set(disabledModels.data?.ids ?? []);

  const moveAccount = (accId: string, direction: "up" | "down") => {
    const idx = sortedAccounts.findIndex((a) => a.id === accId);
    if (idx < 0) return;
    const target = direction === "up" ? idx - 1 : idx + 1;
    if (target < 0 || target >= sortedAccounts.length) return;
    const newPriority = sortedAccounts[target].priority;
    updateAccount.mutate({ id: accId, patch: { priority: newPriority } });
  };

  if (providers.isLoading) return <Spinner />;
  if (!provider) {
    return (
      <Card className="px-6 py-12 text-center">
        <p className="text-sm text-[var(--text-muted)]">Provider not found.</p>
        <Link to="/providers" className="mt-3 inline-block text-sm font-medium text-accent-600">
          Back to Providers
        </Link>
      </Card>
    );
  }

  const isKiro = provider.id === "kiro";
  const supportsManualConnect = !isKiro && (
    provider.auth_modes.includes("api_key") ||
    provider.auth_modes.includes("none") ||
    !oauthProvider
  );

  return (
    <>
      <Link
        to="/providers"
        className="mb-5 inline-flex items-center gap-2 text-sm font-medium text-[var(--text-muted)] transition-colors hover:text-[var(--text)]"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to Providers
      </Link>

      <header className="mb-7 flex items-start gap-4">
        <ProviderIcon provider={provider} size={56} />
        <div className="min-w-0 flex-1">
          <h1 className="font-display text-3xl font-semibold tracking-tight">{provider.display_name}</h1>
          <p className="mt-1 text-sm text-[var(--text-muted)]">
            {myAccounts.length} connected {myAccounts.length === 1 ? "account" : "accounts"}
          </p>
          <div className="mt-2 flex flex-wrap gap-1">
            {(provider.service_kinds ?? []).map((k) => (
              <Badge key={k} tone="accent">
                {k}
              </Badge>
            ))}
            {provider.deprecated && <Badge tone="danger">risk</Badge>}
          </div>
        </div>
      </header>

      <div className="space-y-6">
        <Card>
          <CardHeader
            title="Connected accounts"
            action={
              <div className="flex items-center gap-2">
                {isKiro && (
                  <Button variant="ghost" className="h-8 px-3 text-xs" onClick={() => setKiroOpen(true)}>
                    <Plug className="h-3.5 w-3.5" />
                    Connect Kiro
                  </Button>
                )}
                {!isKiro && oauthProvider && (
                  <Button variant="ghost" className="h-8 px-3 text-xs" onClick={() => setOauthOpen(true)}>
                    <Plug className="h-3.5 w-3.5" />
                    Connect {provider.display_name}
                  </Button>
                )}
                {supportsManualConnect && (
                  <Button variant="ghost" className="h-8 px-3 text-xs" onClick={() => setAddKeyOpen(true)}>
                    <Plus className="h-3.5 w-3.5" />
                    {provider.auth_kind === "none" ? "Connect" : "Add API key"}
                  </Button>
                )}
              </div>
            }
          />
          {accounts.isLoading ? (
            <Spinner />
          ) : !myAccounts.length ? (
            <EmptyState
              title="No accounts yet"
              hint="Add an account to start routing through this provider."
            />
          ) : (
            <div className="divide-y divide-[var(--border)]">
              {sortedAccounts.map((a, i) => (
                <AccountRow
                  key={a.id}
                  account={a}
                  index={i}
                  total={sortedAccounts.length}
                  pools={pools.data?.pools ?? []}
                  onDelete={() => remove.mutate(a.id)}
                  onMoveUp={() => moveAccount(a.id, "up")}
                  onMoveDown={() => moveAccount(a.id, "down")}
                  onTest={() => testAccount.mutate(a.id)}
                  onUpdateProxy={(patch) => updateAccount.mutate({ id: a.id, patch })}
                  testing={testAccount.isPending}
                />
              ))}
            </div>
          )}
        </Card>

        <ProviderRoutingCard
          settings={routing.data}
          accountCount={myAccounts.length}
          loading={routing.isLoading}
          saving={updateRouting.isPending}
          onUpdate={(patch) => updateRouting.mutate(patch)}
        />

        {/* Available Models */}
        {models.data?.models && models.data.models.length > 0 && (
          <Card>
            <CardHeader
              title="Available Models"
              description={`${models.data.models.length} model${models.data.models.length === 1 ? "" : "s"} configured for this provider.`}
            />
            <div className="flex flex-col gap-3 border-t border-[var(--border)] bg-[var(--bg-subtle)] px-6 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="relative w-full max-w-sm">
                <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
                <Input
                  placeholder="Search models..."
                  value={modelSearchQuery}
                  onChange={(e) => setModelSearchQuery(e.target.value)}
                  className="pl-9 h-8 text-sm"
                />
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  className="h-8 px-3 text-xs"
                  onClick={() => enableModelsMut.mutate(paginatedModels.map((m) => m.id))}
                  disabled={enableModelsMut.isPending || paginatedModels.length === 0}
                >
                  <ToggleRight className="h-3.5 w-3.5 text-accent-500" />
                  Enable page
                </Button>
                <Button
                  variant="ghost"
                  className="h-8 px-3 text-xs"
                  onClick={() => disableModelsMut.mutate(paginatedModels.map((m) => m.id))}
                  disabled={disableModelsMut.isPending || paginatedModels.length === 0}
                >
                  <ToggleLeft className="h-3.5 w-3.5 text-[var(--text-muted)]" />
                  Disable page
                </Button>
              </div>
            </div>
            {filteredModels.length === 0 ? (
              <div className="px-6 py-12 text-center text-sm text-[var(--text-muted)] border-t border-[var(--border)]">
                No models found matching "{modelSearchQuery}"
              </div>
            ) : (
              <div className={`grid grid-cols-1 gap-px overflow-hidden border-t border-[var(--border)] bg-[var(--border)] sm:grid-cols-2 lg:grid-cols-3 ${totalModelPages <= 1 ? "rounded-b-2xl" : ""}`}>
                {paginatedModels.map((m) => (
                  <ModelCell
                    key={m.id}
                    model={m}
                    provider={provider}
                    disabled={disabledModelIds.has(m.id)}
                    onToggleDisable={() => {
                      if (disabledModelIds.has(m.id)) {
                        enableModelsMut.mutate([m.id]);
                      } else {
                        disableModelsMut.mutate([m.id]);
                      }
                    }}
                  />
                ))}
              </div>
            )}
            {totalModelPages > 0 && (
              <div className="flex items-center justify-between rounded-b-2xl border-t border-[var(--border)] bg-[var(--bg-subtle)] px-6 py-3">
                <span className="text-xs text-[var(--text-muted)]">
                  Showing {(modelPage - 1) * MODELS_PER_PAGE + 1} to {Math.min(modelPage * MODELS_PER_PAGE, filteredModels.length)} of {filteredModels.length} models
                </span>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    className="h-8 px-2 text-xs"
                    disabled={modelPage === 1}
                    onClick={() => setModelPage((p) => p - 1)}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="ghost"
                    className="h-8 px-2 text-xs"
                    disabled={modelPage === totalModelPages}
                    onClick={() => setModelPage((p) => p + 1)}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </Card>
        )}
      </div>

      {oauthOpen && oauthProvider && (
        <ConnectModal provider={oauthProvider} onClose={() => setOauthOpen(false)} />
      )}
      {kiroOpen && <KiroConnectModal onClose={() => setKiroOpen(false)} />}
      {addKeyOpen && (
        <AddApiKeyModal
          provider={provider}
          hasRegions={hasRegions}
          label={label}
          apiKey={apiKey}
          baseURL={baseURL}
          region={region}
          accountID={accountID}
          azureEndpoint={azureEndpoint}
          azureDeployment={azureDeployment}
          azureAPIVersion={azureAPIVersion}
          azureOrganization={azureOrganization}
          error={error}
          pending={create.isPending}
          onLabel={setLabel}
          onApiKey={setApiKey}
          onBaseURL={setBaseURL}
          onRegion={setRegion}
          onAccountID={setAccountID}
          onAzureEndpoint={setAzureEndpoint}
          onAzureDeployment={setAzureDeployment}
          onAzureAPIVersion={setAzureAPIVersion}
          onAzureOrganization={setAzureOrganization}
          onSubmit={() => create.mutate()}
          onClose={() => { setAddKeyOpen(false); setError(""); }}
        />
      )}
    </>
  );
}

const routingOptions = [
  { value: "inherit", label: "Inherit" },
  { value: "fill-first", label: "Fill first" },
  { value: "round-robin", label: "Round robin" },
  { value: "smart-round-robin", label: "Smart" },
];

const routingDescriptions: Record<string, string> = {
  inherit: "Uses the global routing strategy from Settings.",
  "fill-first": "Always starts from the highest priority healthy account.",
  "round-robin": "Cycles accounts after the sticky limit.",
  "smart-round-robin": "Cycles new conversations, then keeps each conversation on the same account.",
};

function ProviderRoutingCard({
  settings,
  accountCount,
  loading,
  saving,
  onUpdate,
}: {
  settings?: ProviderRoutingSettings;
  accountCount: number;
  loading: boolean;
  saving: boolean;
  onUpdate: (patch: Partial<ProviderRoutingSettings>) => void;
}) {
  const mode = settings?.routing_strategy || "inherit";
  const stickyLimit = settings?.sticky_limit || 3;
  const ttlHours = Math.max(1, Math.round((settings?.affinity_ttl_minutes || 1440) / 60));
  const rotatesAccounts = mode === "round-robin" || mode === "smart-round-robin";

  return (
    <Card>
      <CardHeader
        title="Account routing"
        description={`${accountCount} account${accountCount === 1 ? "" : "s"} available for this provider.`}
        action={<Route className="h-4 w-4 text-[var(--text-muted)]" />}
      />
      {loading ? (
        <Spinner />
      ) : (
        <div className="divide-y divide-[var(--border)]">
          <div className="flex flex-col gap-3 px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="min-w-0">
              <p className="text-sm font-medium">Strategy</p>
              <p className="mt-0.5 text-xs text-[var(--text-muted)]">{routingDescriptions[mode] || routingDescriptions.inherit}</p>
            </div>
            <SegmentedControl
              value={mode}
              onChange={(value) => onUpdate({ routing_strategy: value })}
              options={routingOptions}
            />
          </div>

          {rotatesAccounts && (
            <div className="flex flex-wrap items-center justify-between gap-3 px-6 py-4">
              <div>
                <p className="text-sm font-medium">Sticky limit</p>
                <p className="mt-0.5 text-xs text-[var(--text-muted)]">Calls before a new session can move to the next account.</p>
              </div>
              <Input
                type="number"
                min={1}
                max={100}
                value={stickyLimit}
                disabled={saving}
                onChange={(e) => onUpdate({ sticky_limit: parseInt(e.target.value, 10) || 1 })}
                className="w-24 text-center"
              />
            </div>
          )}

          {mode === "smart-round-robin" && (
            <div className="flex flex-wrap items-center justify-between gap-3 px-6 py-4">
              <div>
                <p className="text-sm font-medium">Conversation affinity TTL</p>
                <p className="mt-0.5 text-xs text-[var(--text-muted)]">Hours to keep a conversation pinned to its selected account.</p>
              </div>
              <Input
                type="number"
                min={1}
                max={168}
                value={ttlHours}
                disabled={saving}
                onChange={(e) => onUpdate({ affinity_ttl_minutes: (parseInt(e.target.value, 10) || 1) * 60 })}
                className="w-24 text-center"
              />
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

function AccountRow({
  account: a,
  index,
  total,
  pools,
  onDelete,
  onMoveUp,
  onMoveDown,
  onTest,
  onUpdateProxy,
  testing,
}: {
  account: Account;
  index: number;
  total: number;
  pools: ProxyPool[];
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onTest: () => void;
  onUpdateProxy: (patch: { priority?: number; proxy_pool_id?: string; disabled?: boolean }) => void;
  testing: boolean;
}) {
  const [editPriority, setEditPriority] = useState(false);
  const [priorityVal, setPriorityVal] = useState(String(a.priority));

  const quota = useQuery({
    queryKey: ["account-quota", a.id],
    queryFn: () => api.accountQuota(a.id),
    staleTime: 60_000,
    enabled: !a.disabled,
  });

  const hasQuota = quota.data?.supported && quota.data?.quotas && quota.data.quotas.length > 0;
  const boundPool = pools.find((p) => p.id === a.proxy_pool_id);

  const savePriority = () => {
    const val = parseInt(priorityVal, 10);
    if (!isNaN(val) && val !== a.priority) {
      onUpdateProxy({ priority: val });
    }
    setEditPriority(false);
  };

  return (
    <div className={`px-4 py-3 ${a.disabled ? "opacity-60" : ""}`}>
      {/* Header row */}
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-sm font-medium">{a.label || a.provider}</span>
            <Badge tone="neutral">{a.auth_kind === "oauth" ? "OAuth" : "API Key"}</Badge>
            {a.disabled && <Badge tone="danger">disabled</Badge>}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-0.5">
          <button onClick={onTest} disabled={testing}
            className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text)]" title="Test credentials">
            <CheckCircle className={`h-4 w-4 ${testing ? "animate-pulse" : ""}`} />
          </button>
          <button onClick={() => onUpdateProxy({ disabled: !a.disabled })}
            className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text)]"
            title={a.disabled ? "Enable" : "Disable"}>
            {a.disabled ? <ToggleLeft className="h-4 w-4" /> : <ToggleRight className="h-4 w-4 text-emerald-500 dark:text-emerald-400" />}
          </button>
          <button onClick={onDelete}
            className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-red-500/10 hover:text-red-500" title="Delete">
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Settings row: Priority + Proxy Pool */}
      <div className="mt-2 flex flex-wrap items-center gap-3">
        {/* Priority */}
        <div className="flex items-center gap-1.5">
          <span className="text-[11px] text-[var(--text-muted)]">Priority:</span>
          {editPriority ? (
            <div className="flex items-center gap-1">
              <input
                type="number"
                value={priorityVal}
                onChange={(e) => setPriorityVal(e.target.value)}
                onBlur={savePriority}
                onKeyDown={(e) => e.key === "Enter" && savePriority()}
                className="h-6 w-14 rounded border border-accent-500 bg-[var(--bg)] px-1.5 text-xs text-center focus:outline-none"
                autoFocus
                min={0}
                max={999}
              />
            </div>
          ) : (
            <button onClick={() => { setEditPriority(true); setPriorityVal(String(a.priority)); }}
              className="flex h-6 items-center gap-1 rounded-md border border-[var(--border)] bg-[var(--bg-subtle)] px-2 text-xs font-medium hover:border-accent-500/40 hover:bg-[var(--bg-elevated)]">
              {a.priority}
              <ArrowUp className="h-3 w-3 text-[var(--text-muted)]" />
            </button>
          )}
          <div className="flex items-center gap-0.5">
            <button onClick={onMoveUp} disabled={index === 0}
              className="rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] disabled:opacity-20">
              <ArrowUp className="h-3 w-3" />
            </button>
            <button onClick={onMoveDown} disabled={index === total - 1}
              className="rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] disabled:opacity-20">
              <ArrowDown className="h-3 w-3" />
            </button>
          </div>
        </div>

        {/* Proxy Pool */}
        <div className="flex items-center gap-1.5">
          <span className="text-[11px] text-[var(--text-muted)]">Proxy:</span>
          <select
            value={a.proxy_pool_id || ""}
            onChange={(e) => onUpdateProxy({ proxy_pool_id: e.target.value || "" })}
            className="h-6 rounded-md border border-[var(--border)] bg-[var(--bg-subtle)] pl-1.5 pr-6 text-xs focus:border-accent-500 focus:outline-none"
          >
            <option value="">Direct (no proxy)</option>
            {pools.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}{!p.is_active ? " (inactive)" : ""}
              </option>
            ))}
          </select>
          {boundPool && (
            <span className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-medium ${
              boundPool.test_status === "active"
                ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                : boundPool.test_status === "error"
                  ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                  : "bg-[var(--bg-subtle)] text-[var(--text-muted)]"
            }`}>
              {boundPool.test_status === "active" ? "✓" : boundPool.test_status === "error" ? "✗" : "?"}
              {boundPool.type !== "http" && ` ${boundPool.type}`}
            </span>
          )}
        </div>
      </div>

      {/* Quota / credit info */}
      {hasQuota && quota.data && (
        <div className="mt-2.5 rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] px-3 py-2.5">
          <div className="mb-2 flex items-center gap-2">
            <Zap className="h-3.5 w-3.5 text-[var(--text-muted)]" />
            <span className="text-xs font-medium">
              {quota.data.plan_name ? `${quota.data.plan_name} — Credits` : "Credits & Quota"}
            </span>
          </div>
          {quota.data.quotas && (
            <div className="space-y-2">
              {quota.data.quotas.map((q) => (
                <QuotaBarInline key={q.resource_type} quota={q} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function QuotaBarInline({ quota: q }: { quota: UpstreamQuota }) {
  const pct = q.limit > 0 ? Math.min(100, Math.round((q.used / q.limit) * 100)) : 0;
  const remainingPct = q.limit > 0 ? Math.round((q.remaining / q.limit) * 100) : 0;
  const tone =
    remainingPct < 30
      ? "bg-[color:var(--color-danger)]"
      : remainingPct < 70
        ? "bg-[color:var(--color-warning)]"
        : "bg-accent-500";
  const label = q.resource_type
    .toLowerCase()
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());

  const resetDate = q.reset_at ? new Date(q.reset_at) : null;
  const resetLabel = resetDate && !isNaN(resetDate.getTime())
    ? resetDate.toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
    : null;

  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-[11px]">
        <span className="font-medium text-[var(--text)]">{label}</span>
        <div className="flex items-center gap-2">
          {resetLabel && (
            <span className="text-[10px] text-[var(--text-muted)]">resets {resetLabel}</span>
          )}
          <span className="tabular-nums">
            {q.used.toLocaleString()} / {q.limit.toLocaleString()}
            <span className="ml-1 text-[var(--text-muted)]">({q.remaining.toLocaleString()} left)</span>
          </span>
        </div>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-subtle)]">
        <div className={`h-full rounded-full ${tone}`} style={{ width: `${Math.max(2, pct)}%` }} />
      </div>
    </div>
  );
}

function AddApiKeyModal({
  provider,
  hasRegions,
  label,
  apiKey,
  baseURL,
  region,
  accountID,
  azureEndpoint,
  azureDeployment,
  azureAPIVersion,
  azureOrganization,
  error,
  pending,
  onLabel,
  onApiKey,
  onBaseURL,
  onRegion,
  onAccountID,
  onAzureEndpoint,
  onAzureDeployment,
  onAzureAPIVersion,
  onAzureOrganization,
  onSubmit,
  onClose,
}: {
  provider: Provider;
  hasRegions: boolean;
  label: string;
  apiKey: string;
  baseURL: string;
  region: string;
  accountID: string;
  azureEndpoint: string;
  azureDeployment: string;
  azureAPIVersion: string;
  azureOrganization: string;
  error: string;
  pending: boolean;
  onLabel: (v: string) => void;
  onApiKey: (v: string) => void;
  onBaseURL: (v: string) => void;
  onRegion: (v: string) => void;
  onAccountID: (v: string) => void;
  onAzureEndpoint: (v: string) => void;
  onAzureDeployment: (v: string) => void;
  onAzureAPIVersion: (v: string) => void;
  onAzureOrganization: (v: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const [checkStatus, setCheckStatus] = useState<"idle" | "ok" | "error">("idle");
  const [checkMsg, setCheckMsg] = useState("");
  const [checking, setChecking] = useState(false);
  const isNoAuth = provider.auth_kind === "none" || provider.auth_modes.includes("none");
  const isAzure = provider.id === "azure";
  const isCloudflare = provider.id === "cloudflare-ai";
  const requiresBaseURL = provider.id === "custom-openai" || provider.id === "custom-anthropic";
  const credentialLabel = isNoAuth ? "Connection" : "API key";
  const canSubmit =
    !pending &&
    (isNoAuth || !!apiKey.trim()) &&
    (!isCloudflare || !!accountID.trim()) &&
    (!isAzure || (!!azureEndpoint.trim() && !!azureDeployment.trim())) &&
    (!requiresBaseURL || !!baseURL.trim());

  const handleCheck = async () => {
    if (!canSubmit && !isNoAuth) return;
    setChecking(true);
    setCheckStatus("idle");
    setCheckMsg("");
    try {
      const res = await api.validateKey({
        provider: provider.id,
        label,
        api_key: apiKey || undefined,
        base_url: baseURL || undefined,
        region: hasRegions ? region : undefined,
        account_id: accountID || undefined,
        azure_endpoint: azureEndpoint || undefined,
        azure_deployment: azureDeployment || undefined,
        azure_api_version: azureAPIVersion || undefined,
        azure_organization: azureOrganization || undefined,
      });
      setCheckStatus(res.status === "ok" ? "ok" : "error");
      setCheckMsg(res.message || "");
    } catch (e) {
      setCheckStatus("error");
      setCheckMsg((e as Error).message);
    } finally {
      setChecking(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-2xl border border-[var(--border)] bg-[var(--bg-elevated)] shadow-[var(--shadow-float)] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--border)] px-6 py-4">
          <h2 className="text-sm font-semibold">Add API key — {provider.display_name}</h2>
          <button
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <form
          className="space-y-4 px-6 py-5"
          onSubmit={(e) => {
            e.preventDefault();
            if (canSubmit) onSubmit();
          }}
        >
          <Field label="Label">
            <Input value={label} onChange={(e) => onLabel(e.target.value)} placeholder="personal" />
          </Field>
          {!isNoAuth && (
            <Field label={credentialLabel}>
              <Input
                type="password"
                value={apiKey}
                onChange={(e) => { onApiKey(e.target.value); setCheckStatus("idle"); }}
                placeholder={provider.id === "xai" ? "xai-..." : "sk-..."}
                required
              />
            </Field>
          )}
          {isCloudflare && (
            <Field label="Cloudflare account ID">
              <Input
                value={accountID}
                onChange={(e) => { onAccountID(e.target.value); setCheckStatus("idle"); }}
                placeholder="abc123def456..."
                required
              />
            </Field>
          )}
          {isAzure ? (
            <div className="space-y-3 rounded-xl border border-[var(--border)] bg-[var(--bg-subtle)] p-4">
              <Field label="Azure endpoint">
                <Input
                  value={azureEndpoint}
                  onChange={(e) => { onAzureEndpoint(e.target.value); setCheckStatus("idle"); }}
                  placeholder="https://your-resource.openai.azure.com"
                  required
                />
              </Field>
              <Field label="Deployment name">
                <Input
                  value={azureDeployment}
                  onChange={(e) => { onAzureDeployment(e.target.value); setCheckStatus("idle"); }}
                  placeholder="gpt-4o"
                  required
                />
              </Field>
              <Field label="API version">
                <Input
                  value={azureAPIVersion}
                  onChange={(e) => { onAzureAPIVersion(e.target.value); setCheckStatus("idle"); }}
                  placeholder="2024-10-01-preview"
                />
              </Field>
              <Field label="Organization (optional)">
                <Input
                  value={azureOrganization}
                  onChange={(e) => { onAzureOrganization(e.target.value); setCheckStatus("idle"); }}
                  placeholder="org_..."
                />
              </Field>
            </div>
          ) : hasRegions ? (
            <Field label="Region">
              <select
                value={region}
                onChange={(e) => onRegion(e.target.value)}
                className="w-full rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 text-sm focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40"
              >
                {(provider.regions ?? []).map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.label}
                  </option>
                ))}
              </select>
            </Field>
          ) : (
            <Field label={requiresBaseURL ? "Base URL" : "Base URL (optional)"}>
              <Input
                value={baseURL}
                onChange={(e) => onBaseURL(e.target.value)}
                placeholder="for custom endpoints"
                required={requiresBaseURL}
              />
            </Field>
          )}

          {checkStatus === "ok" && (
            <div className="flex items-center gap-2 rounded-lg border border-accent-300 bg-accent-50 px-3 py-2 text-sm text-accent-700 dark:border-accent-700 dark:bg-accent-900/30 dark:text-accent-200">
              <CheckCircle className="h-4 w-4 shrink-0" />
              Key is valid
            </div>
          )}
          {checkStatus === "error" && (
            <ErrorBanner message={checkMsg || "Key validation failed"} />
          )}
          {error && <ErrorBanner message={error} />}

          <div className="flex gap-3">
            <Button type="button" variant="ghost" onClick={handleCheck} disabled={checking || !canSubmit} className="flex-1">
              <CheckCircle className={`h-4 w-4 ${checking ? "animate-pulse" : ""}`} />
              {checking ? "Checking…" : "Check"}
            </Button>
            <Button type="submit" disabled={!canSubmit} className="flex-1">
              <Plus className="h-4 w-4" />
              {pending ? "Adding…" : isNoAuth ? "Connect" : "Add account"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ---- OAuth connect modal (reused flow) --------------------------------------

function ConnectModal({ provider, onClose }: { provider: OAuthProvider; onClose: () => void }) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-2xl border border-[var(--border)] bg-[var(--bg-elevated)] shadow-[var(--shadow-float)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--border)] px-6 py-4">
          <h2 className="text-sm font-semibold">Connect {provider.display_name}</h2>
          <button
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        {provider.flow === "device_code" ? (
          <DeviceFlow provider={provider} onClose={onClose} />
        ) : (
          <AuthCodeFlow provider={provider} onClose={onClose} />
        )}
      </div>
    </div>
  );
}

function AuthCodeFlow({ provider, onClose }: { provider: OAuthProvider; onClose: () => void }) {
  const qc = useQueryClient();
  const [waiting, setWaiting] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  // Listen for the postMessage sent by the OAuth callback page.
  useEffect(() => {
    if (!waiting) return;
    const handler = (e: MessageEvent) => {
      if (e.data?.type !== "oauth-callback") return;
      if (e.data.provider && e.data.provider !== provider.provider) return;
      if (e.data.status === "success") {
        setDone(true);
        qc.invalidateQueries({ queryKey: ["accounts"] });
        setTimeout(onClose, 1500);
      } else {
        setError(e.data.message || "Connection failed.");
        setWaiting(false);
      }
    };
    window.addEventListener("message", handler);
    return () => window.removeEventListener("message", handler);
  }, [waiting, provider.provider, qc, onClose]);

  const start = async () => {
    setError("");
    try {
      const res = await api.oauthAuthorize(provider.provider, redirectURIForProvider(provider));
      setWaiting(true);
      window.open(res.authorize_url, "_blank", "popup,width=560,height=760");
    } catch (e) {
      setError((e as Error).message);
    }
  };

  if (done) return <div className="px-6 py-6 text-sm">Connected. Refreshing accounts…</div>;

  return (
    <div className="space-y-4 px-6 py-5">
      {!waiting ? (
        <>
          <p className="text-sm text-[var(--text-muted)]">
            Click the button below to sign in with {provider.display_name}. A
            new tab will open for authentication.
          </p>
          <Button onClick={start} className="w-full">
            Open sign-in
          </Button>
        </>
      ) : (
        <div className="flex flex-col items-center gap-3 py-4">
          <Spinner />
          <p className="text-sm text-[var(--text-muted)]">
            Waiting for sign-in to complete…
          </p>
          <p className="text-xs text-[var(--text-muted)]">
            Complete the sign-in in the other tab. This will close
            automatically.
          </p>
        </div>
      )}
      {error && <p className="text-xs text-[color:var(--color-danger)]">{error}</p>}
    </div>
  );
}

function DeviceFlow({ provider, onClose }: { provider: OAuthProvider; onClose: () => void }) {
  const qc = useQueryClient();
  const [dc, setDc] = useState<DeviceCode | null>(null);
  const [status, setStatus] = useState<"idle" | "waiting" | "done" | "error">("idle");
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
    };
  }, []);

  const start = async () => {
    setError("");
    try {
      const res = await api.oauthDeviceCode(provider.provider);
      setDc(res);
      setStatus("waiting");
      poll(res.device_code, res.interval);
    } catch (e) {
      setError((e as Error).message);
      setStatus("error");
    }
  };

  const poll = (deviceCode: string, interval: number) => {
    pollRef.current = setTimeout(async () => {
      try {
        const res = await api.oauthPoll(provider.provider, deviceCode);
        if (res.status === "complete") {
          setStatus("done");
          qc.invalidateQueries({ queryKey: ["accounts"] });
          setTimeout(onClose, 1200);
          return;
        }
        poll(deviceCode, res.slow_down ? interval + 5 : interval);
      } catch (e) {
        setError((e as Error).message);
        setStatus("error");
      }
    }, Math.max(1, interval) * 1000);
  };

  if (status === "done") return <div className="px-6 py-6 text-sm">Connected. Refreshing accounts…</div>;

  return (
    <div className="space-y-4 px-6 py-5">
      {!dc ? (
        <>
          <p className="text-sm text-[var(--text-muted)]">
            A device code will be generated. Enter it on the provider's verification page to authorize.
          </p>
          <Button onClick={start} className="w-full">
            Generate device code
          </Button>
        </>
      ) : (
        <>
          <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-subtle)] px-4 py-4 text-center">
            <p className="text-xs text-[var(--text-muted)]">Your code</p>
            <p className="mt-1 font-mono text-2xl font-bold tracking-widest">{dc.user_code}</p>
          </div>
          <a
            href={dc.verification_uri_complete || dc.verification_uri}
            target="_blank"
            rel="noopener noreferrer"
            className="block w-full rounded-xl bg-accent-600 px-3 py-2 text-center text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-700"
          >
            Open verification page
          </a>
          <p className="text-center text-xs text-[var(--text-muted)]">
            {status === "waiting" ? "Waiting for you to authorize…" : ""}
          </p>
        </>
      )}
      {error && <p className="text-xs text-[color:var(--color-danger)]">{error}</p>}
    </div>
  );
}

// ---- Account quota card -----------------------------------------------------


// ModelCell renders a single model in a structural hairline grid.
function ModelCell({
  model,
  provider,
  disabled,
  onToggleDisable,
}: {
  model: { id: string; name: string; kind: string };
  provider: Provider;
  disabled?: boolean;
  onToggleDisable?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const fullModel = `${provider.alias || provider.id}/${model.id}`;

  const handleCopy = () => {
    navigator.clipboard.writeText(fullModel);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className={`group relative flex flex-col justify-between bg-[var(--bg-elevated)] p-4 transition-all hover:bg-[var(--bg-subtle)] ${disabled ? "opacity-50 grayscale" : ""}`}>
      <div className="mb-3 flex items-start justify-between">
        <div className="flex items-center gap-2">
          <div className={`h-1.5 w-1.5 rounded-full ${disabled ? "bg-ink-400 dark:bg-ink-600" : "bg-accent-500 shadow-[0_0_8px_var(--color-accent-500)]"}`} />
          <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
            {model.kind || "Model"}
          </span>
        </div>
        <div className="flex items-center gap-0.5">
          {onToggleDisable && (
            <button
              onClick={onToggleDisable}
              className="flex h-7 w-7 items-center justify-center rounded bg-transparent text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
              title={disabled ? "Enable model" : "Disable model"}
            >
              {disabled ? <ToggleLeft className="h-4 w-4" /> : <ToggleRight className="h-4 w-4 text-accent-500" />}
            </button>
          )}
          <button
            onClick={handleCopy}
            className="flex h-7 w-7 items-center justify-center rounded bg-transparent text-[var(--text-muted)] opacity-100 transition-all hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800 sm:opacity-0 sm:group-hover:opacity-100"
            title="Copy model path"
          >
            {copied ? (
              <CheckCircle className="h-3.5 w-3.5 text-green-500" />
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2" /><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" /></svg>
            )}
          </button>
        </div>
      </div>
      <div>
        <code className="block truncate font-mono text-xs text-[var(--text)] tracking-tight" title={fullModel}>
          {fullModel}
        </code>
        {model.name && model.name !== model.id && (
          <span className="mt-1 block truncate text-[10px] text-[var(--text-muted)]" title={model.name}>
            {model.name}
          </span>
        )}
      </div>
    </div>
  );
}

// ProviderIcon renders the provider PNG with a colored fallback initial.
function ProviderIcon({ provider: p, size = 40 }: { provider: Provider; size?: number }) {
  const [errored, setErrored] = useState(false);
  const dim = { width: size, height: size };
  if (errored || !p.icon) {
    return (
      <div
        className="flex shrink-0 items-center justify-center rounded-2xl text-lg font-bold text-white"
        style={{ ...dim, backgroundColor: p.color || "var(--text-muted)" }}
      >
        {p.display_name.slice(0, 1).toUpperCase()}
      </div>
    );
  }
  return (
    <img
      src={p.icon}
      alt={p.display_name}
      onError={() => setErrored(true)}
      className="shrink-0 rounded-2xl object-contain"
      style={dim}
    />
  );
}
