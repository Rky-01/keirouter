import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Plus, Copy, Check } from "lucide-react";
import { api, type CreatedKey } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useToast } from "../components/Toast";
import { Card, SectionHeader, CardHeader, Button, Input, Field, Badge, Spinner, EmptyState } from "../components/ui";

export function KeysPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api.listKeys() });

  const [name, setName] = useState("");
  const [created, setCreated] = useState<CreatedKey | null>(null);
  const [copied, setCopied] = useState(false);

  const create = useMutation({
    mutationFn: () => api.createKey(name),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ["keys"] });
      setCreated(data);
      setName("");
      toast.success("API key created", "Copy it now — it won't be shown again.");
    },
    onError: (e: Error) => toast.error("Couldn't create key", e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteKey(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys"] });
      toast.success("Key revoked");
    },
    onError: (e: Error) => toast.error("Couldn't revoke key", e.message),
  });

  return (
    <>
      <PageHeader
        title="API Keys"
        icon={KeyRound}
        description="Keys your tools use to authenticate. Stored hashed; shown once."
      />

      {created && (
        <Card className="mb-6 border-accent-300 dark:border-accent-700">
          <div className="p-6">
            <p className="text-sm font-medium">Copy your new key now — it won't be shown again.</p>
            <div className="mt-3 flex items-center gap-2">
              <code className="flex-1 overflow-x-auto rounded-lg bg-[var(--bg-subtle)] px-3 py-2.5 font-mono text-sm">
                {created.key}
              </code>
              <Button
                onClick={() => {
                  navigator.clipboard.writeText(created.key);
                  setCopied(true);
                  setTimeout(() => setCopied(false), 1500);
                }}
              >
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                {copied ? "Copied" : "Copy"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  setCreated(null);
                  setCopied(false);
                }}
              >
                Done
              </Button>
            </div>
          </div>
        </Card>
      )}

      <Card className="mb-6">
        <SectionHeader title="Create key" description="Generate a new API key for a tool or device." icon={Plus} />
        <form
          className="flex items-end gap-3 px-6 pb-6"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create.mutate();
          }}
        >
          <div className="flex-1">
            <Field label="Key name">
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="laptop" />
            </Field>
          </div>
          <Button type="submit" disabled={create.isPending || !name.trim()}>
            <Plus className="h-4 w-4" />
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
        </form>
      </Card>

      <Card>
        <CardHeader title="Keys" />
        {keys.isLoading ? (
          <Spinner />
        ) : !keys.data?.keys.length ? (
          <EmptyState title="No keys yet" />
        ) : (
          <div className="divide-y divide-[var(--border)]">
            {keys.data.keys.map((k) => (
              <div key={k.id} className="flex items-center justify-between px-6 py-4">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{k.name}</span>
                    {k.disabled ? <Badge tone="danger">disabled</Badge> : <Badge tone="success">active</Badge>}
                  </div>
                  <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">{k.display}</p>
                </div>
                <Button variant="danger" onClick={() => remove.mutate(k.id)}>
                  Revoke
                </Button>
              </div>
            ))}
          </div>
        )}
      </Card>
    </>
  );
}