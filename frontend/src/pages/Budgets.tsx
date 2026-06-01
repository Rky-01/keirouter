import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Wallet, Plus, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import { PageHeader } from "../components/Layout";
import { useToast } from "../components/Toast";
import { Card, SectionHeader, CardHeader, Button, Input, Select, Field, Badge, Spinner, EmptyState, ErrorBanner } from "../components/ui";

const periods = ["daily", "weekly", "monthly", "total"];

export function BudgetsPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const budgets = useQuery({ queryKey: ["budgets"], queryFn: () => api.listBudgets() });

  const [limit, setLimit] = useState("");
  const [period, setPeriod] = useState("monthly");
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () => api.createBudget({ limit_usd: parseFloat(limit), period, scope_kind: "tenant" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["budgets"] });
      setLimit("");
      setError("");
      toast.success("Budget created");
    },
    onError: (e: Error) => {
      setError(e.message);
      toast.error("Couldn't create budget", e.message);
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteBudget(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["budgets"] });
      toast.success("Budget removed");
    },
    onError: (e: Error) => toast.error("Couldn't remove budget", e.message),
  });

  return (
    <>
      <PageHeader
        title="Budgets"
        icon={Wallet}
        description="Hard spend caps. When a budget is exhausted, matching requests are blocked until the period resets."
      />

      <Card className="mb-6">
        <SectionHeader title="Create budget" description="Set a spending cap for a period." icon={Plus} />
        <form
          className="flex flex-wrap items-end gap-3 px-6 pb-6"
          onSubmit={(e) => {
            e.preventDefault();
            if (parseFloat(limit) > 0) create.mutate();
          }}
        >
          <div className="w-40">
            <Field label="Limit (USD)">
              <Input type="number" min="0" step="0.01" value={limit} onChange={(e) => setLimit(e.target.value)} placeholder="50.00" />
            </Field>
          </div>
          <div className="w-40">
            <Field label="Period">
              <Select value={period} onChange={(e) => setPeriod(e.target.value)}>
                {periods.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <Button type="submit" disabled={create.isPending || !(parseFloat(limit) > 0)}>
            <Plus className="h-4 w-4" />
            {create.isPending ? "Creating…" : "Create budget"}
          </Button>
          {error && (
            <div className="w-full">
              <ErrorBanner message={error} />
            </div>
          )}
        </form>
      </Card>

      <Card>
        <CardHeader title="Budgets" />
        {budgets.isLoading ? (
          <Spinner />
        ) : !budgets.data?.budgets.length ? (
          <EmptyState title="No budgets set" hint="Spending is unlimited until you add a budget." />
        ) : (
          <div className="divide-y divide-[var(--border)]">
            {budgets.data.budgets.map((b) => (
              <div key={b.id} className="flex items-center justify-between px-6 py-4">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">${(b.limit_micros / 1_000_000).toFixed(2)}</span>
                    <Badge>{b.period}</Badge>
                    <Badge tone="accent">{b.scope_kind}</Badge>
                    {b.hard_cutoff && <Badge tone="danger">hard cutoff</Badge>}
                  </div>
                  <p className="mt-0.5 text-xs text-[var(--text-muted)]">alert at {b.alert_pct}%</p>
                </div>
                <Button variant="danger" onClick={() => remove.mutate(b.id)}>
                  <Trash2 className="h-4 w-4" />
                  Remove
                </Button>
              </div>
            ))}
          </div>
        )}
      </Card>
    </>
  );
}