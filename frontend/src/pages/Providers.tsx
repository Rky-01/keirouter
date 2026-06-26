import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Boxes, Search, X, AlertTriangle, Plus } from "lucide-react";
import { api, type Provider, type Account } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, CardHeader, Badge, Spinner, EmptyState, StatusDot, Button, Modal, Field, Input, Select, ErrorBanner } from "../components/ui";
import { useToast } from "../components/Toast";


// Popularity ranking for default sort order (lower = more popular).
const POPULARITY: Record<string, number> = {
  // Tier 1 — mega platforms
  openai: 1,
  anthropic: 2,
  claude: 3,
  gemini: 4,
  deepseek: 5,
  // Tier 2 — major LLM providers
  xai: 10,
  mistral: 11,
  groq: 12,
  cohere: 13,
  perplexity: 14,
  together: 15,
  fireworks: 16,
  openrouter: 17,
  // Tier 3 — popular regional / coding
  qwen: 20,
  kimi: 21,
  glm: 22,
  minimax: 23,
  "volcengine-ark": 24,
  deepinfra: 25,
  cerebras: 26,
  sambanova: 27,
  nvidia: 28,
  "xiaomi-mimo": 29,
  "mimo-free": 30,
  "xiaomi-tokenplan": 31,
  // Tier 4 — coding tools / wrappers
  cursor: 30,
  codex: 31,
  github: 32,
  cline: 33,
  kiro: 34,
  "gemini-cli": 35,
  commandcode: 36,
  "kimi-coding": 37,
  qoder: 38,
  // Tier 5 — cloud platforms
  azure: 40,
  vertex: 41,
  "vertex-partner": 42,
  "cloudflare-ai": 43,
  "aws-polly": 44,
  "vercel-ai-gateway": 45,
  // Tier 6 — self-hosted / local
  ollama: 50,
  "ollama-local": 51,
  // Tier 7 — media / speech
  elevenlabs: 60,
  deepgram: 61,
  assemblyai: 62,
  cartesia: 63,
  "stability-ai": 64,
  "black-forest-labs": 65,
  runwayml: 66,
  // Tier 8 — search / embeddings / utility
  "voyage-ai": 70,
  "jina-ai": 71,
  "jina-reader": 72,
  tavily: 73,
  "brave-search": 74,
  exa: 75,
  serper: 76,
  firecrawl: 77,
  // Tier 9 — niche / smaller
  huggingface: 80,
  siliconflow: 81,
  hyperbolic: 82,
  nebius: 83,
  ai21: 84,
  reka: 85,
  baseten: 86,
  modal: 87,
  lepton: 88,
};
const DEFAULT_RANK = 999;

function sortByPopularity<T extends { id: string; pinned?: boolean }>(list: T[]): T[] {
  return [...list].sort((a, b) => {
    // Pinned providers always come first.
    if (a.pinned && !b.pinned) return -1;
    if (!a.pinned && b.pinned) return 1;
    const ra = POPULARITY[a.id] ?? DEFAULT_RANK;
    const rb = POPULARITY[b.id] ?? DEFAULT_RANK;
    return ra - rb;
  });
}

// kindFilters are the service-kind tabs shown above the provider grid.
const kindFilters = [
  { id: "all", label: "All" },
  { id: "llm", label: "Chat" },
  { id: "embedding", label: "Embeddings" },
  { id: "image", label: "Image" },
  { id: "stt", label: "Speech-to-Text" },
  { id: "tts", label: "Text-to-Speech" },
  { id: "search", label: "Web Search" },
  { id: "fetch", label: "Web Fetch" },
];

export function ProvidersPage() {
  const providers = useQuery({ queryKey: ["providers"], queryFn: () => api.providers() });
  const accounts = useQuery({ queryKey: ["accounts"], queryFn: () => api.listAccounts() });
  const [filter, setFilter] = useState("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [customOpen, setCustomOpen] = useState(false);


  // Count accounts per provider id so we can split connected vs available.
  const accountsByProvider = useMemo(() => {
    const map = new Map<string, Account[]>();
    for (const a of accounts.data?.accounts ?? []) {
      const list = map.get(a.provider) ?? [];
      list.push(a);
      map.set(a.provider, list);
    }
    return map;
  }, [accounts.data]);

  const visible = useMemo(() => {
    const all = providers.data?.providers ?? [];
    return all
      .filter((p) => !p.hidden)
      .filter((p) => filter === "all" || p.service_kinds.includes(filter))
      .filter((p) => {
        if (!searchQuery.trim()) return true;
        const q = searchQuery.toLowerCase();
        return (
          p.display_name.toLowerCase().includes(q) ||
          p.id.toLowerCase().includes(q) ||
          p.alias.toLowerCase().includes(q)
        );
      });
  }, [providers.data, filter, searchQuery]);

  const connected = sortByPopularity(visible.filter((p) => accountsByProvider.has(p.id)));
  const available = sortByPopularity(visible.filter((p) => !accountsByProvider.has(p.id)));

  return (
    <>
      <PageHeader
        title="Providers"
        icon={Boxes}
        description="Connect and manage AI providers to power your routing."
      />

      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex flex-wrap gap-1.5">
          {kindFilters.map((k) => (
            <button
              key={k.id}
              onClick={() => setFilter(k.id)}
              className={`rounded-xl px-3.5 py-2 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/60 ${
                filter === k.id
                  ? "bg-accent-600 text-white shadow-sm"
                  : "border border-[var(--border)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:bg-ink-100 dark:hover:bg-ink-800"
              }`}
            >
              {k.label}
            </button>
          ))}
        </div>
        <div className="relative max-w-sm flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search providers…"
            className="w-full rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] py-2 pl-9 pr-9 text-sm placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus:ring-2 focus:ring-accent-400/40"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-[var(--text-muted)] hover:text-[var(--text)]"
              aria-label="Clear search"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
        <Button variant="ghost" className="h-9 shrink-0 px-3 text-xs" onClick={() => setCustomOpen(true)}>
          <Plus className="h-3.5 w-3.5" />
          New custom provider
        </Button>
      </div>


      {providers.isLoading ? (
        <Spinner />
      ) : (
        <div className="space-y-6">
          <Card>
            <CardHeader
              title="Connected providers"
              description="These providers have accounts and are ready to use."
              action={<Badge tone="accent">{connected.length}</Badge>}
            />
            {!connected.length ? (
              <EmptyState
                title={searchQuery ? `No connected providers match "${searchQuery}"` : "No connected providers yet"}
                hint={searchQuery ? "Try a different search term." : "Pick a provider below to add your first account."}
              />
            ) : (
              <div className="grid grid-cols-2 gap-px overflow-hidden rounded-b-2xl bg-[var(--border)] sm:grid-cols-3 lg:grid-cols-4">
                {connected.map((p) => (
                  <ProviderCard
                    key={p.id}
                    provider={p}
                    accountCount={accountsByProvider.get(p.id)?.length ?? 0}
                  />
                ))}
              </div>
            )}
          </Card>

          <Card>
            <CardHeader
              title="Available providers"
              description="Add new providers to expand your routing options."
              action={<Badge tone="neutral">{available.length}</Badge>}
            />
            {!available.length ? (
              <EmptyState
                title={searchQuery ? `No providers match "${searchQuery}"` : "No providers for this capability"}
              />
            ) : (
              <div className="grid grid-cols-2 gap-px overflow-hidden rounded-b-2xl bg-[var(--border)] sm:grid-cols-3 lg:grid-cols-4">
                {available.map((p) => (
                  <ProviderCard key={p.id} provider={p} accountCount={0} />
                ))}
              </div>
            )}
          </Card>
        </div>
      )}

      <CreateCustomProviderModal open={customOpen} onClose={() => setCustomOpen(false)} />
    </>
  );
}

// CreateCustomProviderModal creates a new dynamic custom provider instance.
// Each instance gets a unique id so multiple OpenAI-/Anthropic-compatible
// endpoints stay fully isolated (own base URL, accounts, and models).
function CreateCustomProviderModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const [name, setName] = useState("");
  const [dialect, setDialect] = useState("openai");
  const [baseURL, setBaseURL] = useState("");
  const [error, setError] = useState("");

  const reset = () => {
    setName("");
    setDialect("openai");
    setBaseURL("");
    setError("");
  };

  const create = useMutation({
    mutationFn: () =>
      api.createCustomProvider({ display_name: name.trim(), dialect, base_url: baseURL.trim() }),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ["providers"] });
      toast.success("Custom provider created", "Add an account and models to start routing.");
      reset();
      onClose();
      navigate(`/providers/${p.id}`);
    },
    onError: (e: Error) => setError(e.message),
  });

  const canSubmit = name.trim().length > 0 && baseURL.trim().length > 0 && !create.isPending;

  return (
    <Modal
      open={open}
      onClose={() => { reset(); onClose(); }}
      title="New custom provider"
      subtitle="A dedicated instance of an OpenAI- or Anthropic-compatible endpoint. Each instance is isolated with its own base URL, accounts, and models."
    >
      <form
        className="space-y-4 px-6 py-5"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) create.mutate();
        }}
      >
        <Field label="Name (required)">
          <Input
            value={name}
            onChange={(e) => { setName(e.target.value); setError(""); }}
            placeholder="e.g. Local vLLM or Acme Gateway"
            autoFocus
          />
        </Field>
        <Field label="Dialect">
          <Select value={dialect} onChange={(e) => setDialect(e.target.value)}>
            <option value="openai">OpenAI-compatible</option>
            <option value="anthropic">Anthropic-compatible</option>
          </Select>
        </Field>
        <Field label="Base URL (required)">
          <Input
            value={baseURL}
            onChange={(e) => { setBaseURL(e.target.value); setError(""); }}
            placeholder="https://llm.example.com/v1"
          />
        </Field>
        <p className="text-xs text-[var(--text-muted)]">
          Tip: add two separate instances for two endpoints of the same type — they will never share models or credentials.
        </p>
        {error && <ErrorBanner message={error} />}
        <div className="flex items-center justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={() => { reset(); onClose(); }}>
            Cancel
          </Button>
          <Button type="submit" disabled={!canSubmit}>
            <Plus className="h-4 w-4" />
            {create.isPending ? "Creating…" : "Create provider"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function ProviderCard({ provider: p, accountCount }: { provider: Provider; accountCount: number }) {

  const navigate = useNavigate();
  const connected = accountCount > 0;

  return (
    <button
      onClick={() => navigate(`/providers/${p.id}`)}
      className="flex h-full flex-col items-start gap-3 bg-[var(--bg-elevated)] p-5 text-left transition-colors hover:bg-ink-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent-400/60 dark:hover:bg-ink-800/40"
    >
      <div className="flex w-full items-start justify-between gap-2">
        <ProviderIcon provider={p} />
        {connected ? (
          <span className="inline-flex items-center gap-1.5 rounded-md bg-accent-100 px-2 py-0.5 text-xs font-medium text-accent-700 dark:bg-accent-800/40 dark:text-accent-200">
            <StatusDot tone="success" />
            Connected
          </span>
        ) : p.deprecated ? (
          <Badge tone="warning" title={p.notice || "Account may be restricted"}>
            <AlertTriangle className="mr-1 h-3 w-3" />
            unofficial
          </Badge>
        ) : !p.drivable ? (
          <Badge tone="neutral">soon</Badge>
        ) : p.auth_kind === "none" ? (
          <Badge tone="accent">free</Badge>
        ) : null}
      </div>

      <div className="min-w-0">
        <p className="truncate text-sm font-semibold">{p.display_name}</p>
        <p className="mt-0.5 truncate font-mono text-xs text-[var(--text-muted)]">{p.id}</p>
      </div>

      <p className="mt-auto text-xs text-[var(--text-muted)]">
        {connected
          ? `${accountCount} ${accountCount === 1 ? "account" : "accounts"}`
          : "Connect"}
      </p>
    </button>
  );
}

// ProviderIcon renders the provider PNG with a colored fallback initial.
function ProviderIcon({ provider: p }: { provider: Provider }) {
  const [errored, setErrored] = useState(false);
  if (errored || !p.icon) {
    return (
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-sm font-bold text-white"
        style={{ backgroundColor: p.color || "var(--text-muted)" }}
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
      className="h-10 w-10 shrink-0 rounded-xl object-contain"
    />
  );
}