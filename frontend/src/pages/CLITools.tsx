import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { TerminalSquare, Copy, Check } from "lucide-react";
import { api, type CLITool } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { Card, SectionHeader, Input, Field, Spinner } from "../components/ui";

export function CLIToolsPage() {
  const [model, setModel] = useState("");
  const tools = useQuery({
    queryKey: ["cli-tools", model],
    queryFn: () => api.cliTools(model || undefined),
  });

  return (
    <>
      <PageHeader
        title="CLI Tools"
        icon={TerminalSquare}
        description="Copy-paste configuration for coding tools, wired to this KeiRouter instance."
      />

      <Card className="mb-6 p-5">
        <Field label="Model to embed in snippets">
          <Input
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="openai/gpt-4o or chain:my-chain"
            className="max-w-md"
          />
        </Field>
        {tools.data && (
          <p className="mt-3 text-xs text-[var(--text-muted)]">
            Base URL: <span className="font-mono">{tools.data.base_url}</span>
          </p>
        )}
      </Card>

      {tools.isLoading ? (
        <Spinner />
      ) : tools.isError ? (
        <Card className="px-6 py-10 text-center text-sm text-[color:var(--color-danger)]">
          Failed to load CLI tools.
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {tools.data!.tools.map((t) => (
            <ToolCard key={t.id} tool={t} />
          ))}
        </div>
      )}
    </>
  );
}

function ToolCard({ tool }: { tool: CLITool }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(tool.snippet);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard unavailable; no-op
    }
  };

  return (
    <Card>
      <SectionHeader
        title={tool.name}
        description={tool.instructions}
        icon={TerminalSquare}
        iconTone="neutral"
        action={
          <button
            onClick={copy}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
            title="Copy snippet"
          >
            {copied ? (
              <Check className="h-4 w-4 text-accent-600 dark:text-accent-300" />
            ) : (
              <Copy className="h-4 w-4" />
            )}
          </button>
        }
      />
      <div className="px-6 pb-6">
        <pre className="overflow-x-auto rounded-lg bg-[var(--bg-subtle)] p-3 font-mono text-xs leading-relaxed">
          {tool.snippet}
        </pre>
      </div>
    </Card>
  );
}