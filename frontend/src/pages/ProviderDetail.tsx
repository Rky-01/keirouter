import { useEffect, useRef, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Plus, Trash2, Plug, KeyRound, X } from "lucide-react";
import { api, type DeviceCode, type OAuthProvider, type Provider, type Account } from "../lib/api";
import { KiroConnectModal } from "../components/KiroConnectModal";
import { useToast } from "../components/Toast";
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
  ErrorBanner,
} from "../components/ui";

// redirectURI is the OAuth callback the provider returns to (out-of-band flow
// suited to a local dashboard).
const redirectURI = "http://localhost:20180/oauth/callback";

export function ProviderDetailPage() {
  const { id } = useParams();
  const qc = useQueryClient();
  const toast = useToast();

  const providers = useQuery({ queryKey: ["providers"], queryFn: () => api.providers() });
  const accounts = useQuery({ queryKey: ["accounts"], queryFn: () => api.listAccounts() });
  const oauthProviders = useQuery({ queryKey: ["oauth-providers"], queryFn: () => api.oauthProviders() });

  const provider = providers.data?.providers.find((p) => p.id === id);
  const oauthProvider = oauthProviders.data?.providers.find((p) => p.provider === id);
  const myAccounts = (accounts.data?.accounts ?? []).filter((a) => a.provider === id);

  const [label, setLabel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [error, setError] = useState("");
  const [oauthOpen, setOauthOpen] = useState(false);
  const [kiroOpen, setKiroOpen] = useState(false);

  const create = useMutation({
    mutationFn: () =>
      api.createAccount({
        provider: id!,
        label,
        api_key: apiKey,
        base_url: baseURL || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] });
      setLabel("");
      setApiKey("");
      setBaseURL("");
      setError("");
      toast.success("Account added", "Provider account connected.");
    },
    onError: (e: Error) => {
      setError(e.message);
      toast.error("Couldn't add account", e.message);
    },
  });

  const remove = useMutation({
    mutationFn: (accountId: string) => api.deleteAccount(accountId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] });
      toast.success("Account removed");
    },
    onError: (e: Error) => toast.error("Couldn't remove account", e.message),
  });

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
  const supportsApiKey = !isKiro && (provider.auth_modes.includes("api_key") || !oauthProvider);

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
            {provider.service_kinds.map((k) => (
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
          <CardHeader title="Connected accounts" />
          {accounts.isLoading ? (
            <Spinner />
          ) : !myAccounts.length ? (
            <EmptyState
              title="No accounts yet"
              hint="Add an account below to start routing through this provider."
            />
          ) : (
            <div className="divide-y divide-[var(--border)]">
              {myAccounts.map((a) => (
                <AccountRow key={a.id} account={a} onDelete={() => remove.mutate(a.id)} />
              ))}
            </div>
          )}
        </Card>

        {isKiro && (
          <Card>
            <SectionHeader
              title="Connect with sign-in"
              description="Authenticate with AWS Builder ID, IAM Identity Center, or import a Kiro IDE token."
              icon={Plug}
            />
            <div className="border-t border-[var(--border)] px-6 py-5">
              <Button onClick={() => setKiroOpen(true)}>
                <Plug className="h-4 w-4" />
                Connect Kiro
              </Button>
            </div>
          </Card>
        )}

        {!isKiro && oauthProvider && (
          <Card>
            <SectionHeader
              title="Connect with sign-in"
              description={flowLabel(oauthProvider.flow)}
              icon={Plug}
            />
            <div className="border-t border-[var(--border)] px-6 py-5">
              <Button onClick={() => setOauthOpen(true)}>
                <Plug className="h-4 w-4" />
                Connect {provider.display_name}
              </Button>
            </div>
          </Card>
        )}

        {supportsApiKey && (
          <Card>
            <SectionHeader
              title="Add account with API key"
              description="API keys are encrypted at rest and never shown again."
              icon={KeyRound}
            />
            <form
              className="grid grid-cols-1 gap-4 border-t border-[var(--border)] px-6 py-5 sm:grid-cols-2"
              onSubmit={(e) => {
                e.preventDefault();
                if (apiKey) create.mutate();
              }}
            >
              <Field label="Label">
                <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="personal" />
              </Field>
              <Field label="API key">
                <Input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder="sk-..."
                  required
                />
              </Field>
              <Field label="Base URL (optional)">
                <Input
                  value={baseURL}
                  onChange={(e) => setBaseURL(e.target.value)}
                  placeholder="for custom endpoints"
                />
              </Field>
              {error && (
                <div className="sm:col-span-2">
                  <ErrorBanner message={error} />
                </div>
              )}
              <div className="flex items-end justify-end gap-3 sm:col-span-2">
                <Button type="submit" disabled={create.isPending || !apiKey}>
                  <Plus className="h-4 w-4" />
                  {create.isPending ? "Adding…" : "Add account"}
                </Button>
              </div>
            </form>
          </Card>
        )}
      </div>

      {oauthOpen && oauthProvider && (
        <ConnectModal provider={oauthProvider} onClose={() => setOauthOpen(false)} />
      )}
      {kiroOpen && <KiroConnectModal onClose={() => setKiroOpen(false)} />}
    </>
  );
}

function AccountRow({ account: a, onDelete }: { account: Account; onDelete: () => void }) {
  return (
    <div className="flex items-center justify-between px-6 py-4">
      <div>
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{a.label || a.provider}</span>
          <Badge tone="neutral">{a.auth_kind === "oauth" ? "OAuth" : "API key"}</Badge>
          {a.disabled && <Badge tone="danger">disabled</Badge>}
        </div>
        <p className="mt-0.5 text-xs text-[var(--text-muted)]">priority {a.priority}</p>
      </div>
      <Button variant="danger" onClick={onDelete}>
        <Trash2 className="h-4 w-4" />
        Remove
      </Button>
    </div>
  );
}

function flowLabel(flow: string): string {
  switch (flow) {
    case "device_code":
      return "Device code — enter a code on the provider site.";
    case "authorization_code_pkce":
      return "Browser sign-in (PKCE).";
    default:
      return "Browser sign-in.";
  }
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
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)]"
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
  const [state, setState] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  const start = async () => {
    setError("");
    setBusy(true);
    try {
      const res = await api.oauthAuthorize(provider.provider, redirectURI);
      setState(res.state);
      window.open(res.authorize_url, "_blank", "noopener");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const finish = async () => {
    setError("");
    setBusy(true);
    try {
      await api.oauthExchange(provider.provider, { code: code.trim(), state });
      setDone(true);
      qc.invalidateQueries({ queryKey: ["accounts"] });
      setTimeout(onClose, 1200);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  if (done) return <div className="px-6 py-6 text-sm">Connected. Refreshing accounts…</div>;

  return (
    <div className="space-y-4 px-6 py-5">
      <ol className="list-decimal space-y-2 pl-5 text-sm text-[var(--text-muted)]">
        <li>Click "Open sign-in" to authorize in your browser.</li>
        <li>After approving, copy the code from the redirect URL.</li>
        <li>Paste it below and finish.</li>
      </ol>
      {!state ? (
        <Button onClick={start} disabled={busy} className="w-full">
          {busy ? "Starting…" : "Open sign-in"}
        </Button>
      ) : (
        <>
          <Field label="Authorization code">
            <Input value={code} onChange={(e) => setCode(e.target.value)} placeholder="paste code here" />
          </Field>
          <Button onClick={finish} disabled={busy || !code.trim()} className="w-full">
            {busy ? "Connecting…" : "Finish connection"}
          </Button>
        </>
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
            className="block w-full rounded-lg bg-accent-600 px-3 py-2 text-center text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-700"
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

// ProviderIcon renders the provider PNG with a colored fallback initial.
function ProviderIcon({ provider: p, size = 40 }: { provider: Provider; size?: number }) {
  const [errored, setErrored] = useState(false);
  const dim = { width: size, height: size };
  if (errored || !p.icon) {
    return (
      <div
        className="flex shrink-0 items-center justify-center rounded-2xl text-lg font-bold text-white"
        style={{ ...dim, backgroundColor: p.color || "#64748b" }}
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