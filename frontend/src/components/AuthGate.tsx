import { useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { Card, Button, Input, Field, Spinner } from "./ui";

// AuthGate gates the dashboard behind a login, and surfaces a one-time
// onboarding step that nudges the operator off the default password.
export function AuthGate({ children }: { children: ReactNode }) {
  const status = useQuery({ queryKey: ["auth-status"], queryFn: () => api.authStatus() });

  if (status.isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner />
      </div>
    );
  }
  if (status.isError) {
    return <CenteredCard title="Cannot reach KeiRouter" body="Is the backend running on :20180?" />;
  }

  const s = status.data!;
  if (!s.authenticated) {
    return <LoginScreen />;
  }
  if (s.using_default && !s.onboarding_complete) {
    return <OnboardingScreen />;
  }
  return <>{children}</>;
}

function CenteredCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="flex h-full items-center justify-center px-4">
      <Card className="w-full max-w-sm p-6 text-center">
        <h1 className="text-base font-semibold tracking-tight">{title}</h1>
        <p className="mt-2 text-sm text-[var(--text-muted)]">{body}</p>
      </Card>
    </div>
  );
}

function LoginScreen() {
  const qc = useQueryClient();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const login = useMutation({
    mutationFn: () => api.login(password),
    onSuccess: () => {
      setError("");
      qc.invalidateQueries({ queryKey: ["auth-status"] });
    },
    onError: () => setError("Incorrect password"),
  });

  return (
    <div className="flex h-full items-center justify-center px-4">
      <Card className="w-full max-w-sm p-8 shadow-[var(--shadow-pop)]">
        <div className="mb-6 flex flex-col items-center text-center">
          <img src="/keirouter-logo.png" alt="KeiRouter" className="h-12 object-contain" />
          <h1 className="mt-4 text-lg font-semibold tracking-tight">Sign in to your dashboard</h1>
          <p className="mt-1 text-sm text-[var(--text-muted)]">Enter your dashboard password to continue.</p>
        </div>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (password) login.mutate();
          }}
        >
          <Field label="Dashboard password">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              autoFocus
            />
          </Field>
          {error && <p className="text-xs text-[color:var(--color-danger)]">{error}</p>}
          <Button type="submit" className="w-full" disabled={login.isPending || !password}>
            {login.isPending ? "Signing in…" : "Sign in"}
          </Button>
        </form>
        <p className="mt-4 text-center text-xs text-[var(--text-muted)]">
          First run? The default password is{" "}
          <code className="font-mono">keirouter</code>.
        </p>
      </Card>
    </div>
  );
}

function OnboardingScreen() {
  const qc = useQueryClient();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: async () => {
      await api.changePassword(password);
      await api.completeOnboarding();
    },
    onSuccess: () => {
      setError("");
      qc.invalidateQueries({ queryKey: ["auth-status"] });
    },
    onError: (e: Error) => setError(e.message),
  });

  const skip = useMutation({
    mutationFn: () => api.completeOnboarding(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["auth-status"] }),
  });

  const valid = password.length >= 6 && password === confirm;

  return (
    <div className="flex h-full items-center justify-center px-4">
      <Card className="w-full max-w-md p-8 shadow-[var(--shadow-pop)]">
        <img src="/keirouter-logo.png" alt="KeiRouter" className="mb-4 h-11 object-contain" />
        <h1 className="text-lg font-semibold tracking-tight">Welcome to KeiRouter</h1>
        <p className="mt-2 text-sm text-[var(--text-muted)]">
          You're signed in with the default password. Set a new one to secure your dashboard.
        </p>
        <form
          className="mt-5 space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (valid) save.mutate();
          }}
        >
          <Field label="New password">
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoFocus />
          </Field>
          <Field label="Confirm password">
            <Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
          </Field>
          {password && password.length < 6 && (
            <p className="text-xs text-[var(--text-muted)]">Use at least 6 characters.</p>
          )}
          {confirm && password !== confirm && (
            <p className="text-xs text-[color:var(--color-danger)]">Passwords don't match.</p>
          )}
          {error && <p className="text-xs text-[color:var(--color-danger)]">{error}</p>}
          <div className="flex items-center justify-between">
            <Button variant="ghost" type="button" onClick={() => skip.mutate()} disabled={skip.isPending}>
              Keep default for now
            </Button>
            <Button type="submit" disabled={save.isPending || !valid}>
              {save.isPending ? "Saving…" : "Set password"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}