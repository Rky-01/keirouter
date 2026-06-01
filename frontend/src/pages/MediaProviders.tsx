import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Image, AudioLines, Mic, Search, Globe, Boxes } from "lucide-react";
import { api, type Provider } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, CardHeader, Badge, Spinner, EmptyState } from "../components/ui";

// Media service kinds — everything that isn't a plain chat/LLM provider.
const mediaKinds = [
  { id: "embedding", label: "Embeddings", icon: Boxes },
  { id: "image", label: "Image", icon: Image },
  { id: "tts", label: "Text-to-Speech", icon: AudioLines },
  { id: "stt", label: "Speech-to-Text", icon: Mic },
  { id: "search", label: "Web Search", icon: Search },
  { id: "fetch", label: "Web Fetch", icon: Globe },
];

const kindLabels: Record<string, string> = {
  embedding: "Embed",
  image: "Image",
  tts: "TTS",
  stt: "STT",
  search: "Search",
  fetch: "Fetch",
};

export function MediaProvidersPage() {
  const providers = useQuery({ queryKey: ["providers"], queryFn: () => api.providers() });
  const [filter, setFilter] = useState("embedding");

  const list = useMemo(() => {
    const all = providers.data?.providers ?? [];
    return all
      .filter((p) => !p.hidden)
      .filter((p) => p.service_kinds.includes(filter));
  }, [providers.data, filter]);

  const activeKind = mediaKinds.find((k) => k.id === filter) ?? mediaKinds[0];

  return (
    <>
      <PageHeader
        title="Media Providers"
        icon={Image}
        description="Connect providers for embeddings, image generation, speech, and web access."
      />

      <div className="mb-5 flex flex-wrap gap-1.5">
        {mediaKinds.map((k) => (
          <button
            key={k.id}
            onClick={() => setFilter(k.id)}
            className={`flex items-center gap-1.5 rounded-lg px-3.5 py-1.5 text-xs font-medium transition-colors ${
              filter === k.id
                ? "bg-accent-600 text-white shadow-sm"
                : "border border-[var(--border)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:bg-ink-100 dark:hover:bg-ink-800"
            }`}
          >
            <k.icon className="h-3.5 w-3.5" />
            {k.label}
          </button>
        ))}
      </div>

      <Card>
        <CardHeader
          title={`${activeKind.label} providers`}
          description={`${list.length} available`}
        />
        {providers.isLoading ? (
          <Spinner />
        ) : !list.length ? (
          <EmptyState title="No providers for this capability" />
        ) : (
          <div className="grid grid-cols-1 gap-px overflow-hidden rounded-b-2xl bg-[var(--border)] sm:grid-cols-2">
            {list.map((p) => (
              <MediaRow key={p.id} provider={p} />
            ))}
          </div>
        )}
      </Card>
    </>
  );
}

function MediaRow({ provider: p }: { provider: Provider }) {
  return (
    <div className="flex items-start gap-3 bg-[var(--bg-elevated)] px-5 py-4 transition-colors hover:bg-ink-50 dark:hover:bg-ink-800/40">
      <ProviderIcon provider={p} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-sm font-medium">{p.display_name}</span>
          {!p.drivable && <Badge tone="neutral">soon</Badge>}
        </div>
        <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">{p.id}</p>
        <div className="mt-2 flex flex-wrap gap-1">
          {p.service_kinds.map((k) => (
            <Badge key={k} tone="accent">
              {kindLabels[k] ?? k}
            </Badge>
          ))}
        </div>
      </div>
    </div>
  );
}

function ProviderIcon({ provider: p }: { provider: Provider }) {
  const [errored, setErrored] = useState(false);
  if (errored || !p.icon) {
    return (
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-sm font-bold text-white"
        style={{ backgroundColor: p.color || "#64748b" }}
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