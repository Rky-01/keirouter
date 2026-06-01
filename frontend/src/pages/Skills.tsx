import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Sparkles, Plus, Trash2 } from "lucide-react";
import { api, type Skill } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useToast } from "../components/Toast";
import { Card, SectionHeader, CardHeader, Button, Input, Field, Spinner, EmptyState, Toggle, ErrorBanner } from "../components/ui";

export function SkillsPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const skills = useQuery({ queryKey: ["skills"], queryFn: () => api.listSkills() });

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () => api.createSkill({ name, description, prompt }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["skills"] });
      setName("");
      setDescription("");
      setPrompt("");
      setError("");
      toast.success("Skill created");
    },
    onError: (e) => {
      setError((e as Error).message);
      toast.error("Couldn't create skill", (e as Error).message);
    },
  });

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.updateSkill(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["skills"] }),
    onError: (e) => toast.error("Couldn't update skill", (e as Error).message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteSkill(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["skills"] });
      toast.success("Skill removed");
    },
    onError: (e) => toast.error("Couldn't remove skill", (e as Error).message),
  });

  return (
    <>
      <PageHeader
        title="Skills"
        icon={Sparkles}
        description="Reusable system-prompt augmentations applied to matching requests. Enable the ones you want active."
      />

      <div className="space-y-6">
        <Card>
          <SectionHeader
            title="Create skill"
            description="Give the skill a name and the instruction it should inject."
            icon={Plus}
          />
          <div className="space-y-4 border-t border-[var(--border)] px-6 py-5">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Name">
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Concise reviewer"
                />
              </Field>
              <Field label="Description">
                <Input
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Short summary of what it does"
                />
              </Field>
            </div>
            <Field label="Prompt">
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                rows={3}
                placeholder="You are a meticulous code reviewer. Prefer small, safe diffs…"
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-2 text-sm placeholder:text-[var(--text-muted)] focus:border-accent-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400/40"
              />
            </Field>
            {error && <ErrorBanner message={error} />}
            <div className="flex items-center gap-3">
              <Button onClick={() => create.mutate()} disabled={!name.trim() || create.isPending}>
                <Plus className="h-4 w-4" />
                {create.isPending ? "Creating…" : "Create skill"}
              </Button>
            </div>
          </div>
        </Card>

        <Card>
          <CardHeader title="Skills" />
          {skills.isLoading ? (
            <Spinner />
          ) : !skills.data?.skills.length ? (
            <EmptyState title="No skills yet" hint="Create a skill to augment matching requests." />
          ) : (
            <div className="divide-y divide-[var(--border)]">
              {skills.data.skills.map((sk) => (
                <SkillRow
                  key={sk.id}
                  skill={sk}
                  onToggle={(enabled) => toggle.mutate({ id: sk.id, enabled })}
                  onDelete={() => remove.mutate(sk.id)}
                />
              ))}
            </div>
          )}
        </Card>
      </div>
    </>
  );
}

function SkillRow({
  skill,
  onToggle,
  onDelete,
}: {
  skill: Skill;
  onToggle: (enabled: boolean) => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4 px-6 py-4">
      <div className="min-w-0">
        <span className="text-sm font-medium">{skill.name}</span>
        {skill.description && (
          <p className="mt-0.5 text-xs text-[var(--text-muted)]">{skill.description}</p>
        )}
        {skill.prompt && (
          <p className="mt-2 line-clamp-2 rounded-lg bg-[var(--bg-subtle)] px-3 py-2 font-mono text-xs text-[var(--text-muted)]">
            {skill.prompt}
          </p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <Toggle checked={skill.enabled} onChange={onToggle} />
        <Button variant="danger" onClick={onDelete}>
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}