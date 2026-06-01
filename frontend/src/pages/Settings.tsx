import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Sparkles, Zap, MessageSquare, Layers } from "lucide-react";
import { api, type EndpointSettings } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useToast } from "../components/Toast";
import { Card, SectionHeader, Spinner, Toggle, SegmentedControl, ErrorBanner } from "../components/ui";

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
    onError: (e) => toast.error("Couldn't save settings", (e as Error).message),
  });

  const update = (patch: Partial<EndpointSettings>) => {
    if (local) setLocal({ ...local, ...patch });
    save.mutate(patch);
  };

  return (
    <>
      <PageHeader
        title="Token Saving"
        icon={Sparkles}
        description="Cut token usage on both sides of every request. Changes apply immediately to new requests."
      />

      {settings.isLoading || !local ? (
        <Spinner />
      ) : (
        <div className="space-y-6">
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
              <Toggle checked={local.caveman_enabled} onChange={(v) => update({ caveman_enabled: v })} />
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
              <Toggle checked={local.terse_enabled} onChange={(v) => update({ terse_enabled: v })} />
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

          {save.isError && <ErrorBanner message={`Failed to save: ${(save.error as Error).message}`} />}
        </div>
      )}
    </>
  );
}