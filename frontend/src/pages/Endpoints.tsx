import { useEffect, useState, useRef, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Copy,
  Check,
  KeyRound,
  Plus,
  Trash2,
  ToggleLeft,
  ToggleRight,
  Loader2,
  ArrowUpRight,
} from "lucide-react";
import {
  api,
  type CreatedKey,
  type TailscaleEnableResult,
} from "../lib/api";
import { PageHeader } from "../components/Layout";
import { formatTokenLimit, FormattedTokenInput, ModelMultiSelect } from "../components/ModelSelect";
import {
  Card,
  CardHeader,
  Button,
  Input,
  Select,
  Field,
  Badge,
  Spinner,
  EmptyState,
  Toggle,
  Modal,
} from "../components/ui";

// Polling intervals (ms).
const STATUS_POLL_FAST = 5000;
const STATUS_POLL_SLOW = 30000;
const PING_INTERVAL = 2000;
const PING_TIMEOUT = 5000;
const PING_MAX_MS = 300000;
const REACHABLE_MISS_THRESHOLD = 5;

// ---------------------------------------------------------------------------
// Brand SVG logos
// ---------------------------------------------------------------------------

function CloudflareLogo({ className = "" }: { className?: string }) {
  return (
    <svg role="img" viewBox="0 0 24 24" className={className} fill="currentColor" aria-label="Cloudflare">
      <path d="M16.5088 16.8447c.1475-.5068.0908-.9707-.1553-1.3154-.2246-.3164-.6045-.499-1.0615-.5205l-8.6592-.1123a.1559.1559 0 0 1-.1333-.0713c-.0283-.042-.0351-.0986-.021-.1553.0278-.084.1123-.1484.2036-.1562l8.7359-.1123c1.0351-.0489 2.1601-.8868 2.5537-1.9136l.499-1.3013c.0215-.0561.0293-.1128.0147-.168-.5625-2.5463-2.835-4.4453-5.5499-4.4453-2.5039 0-4.6284 1.6177-5.3876 3.8614-.4927-.3658-1.1187-.5625-1.794-.499-1.2026.119-2.1665 1.083-2.2861 2.2856-.0283.31-.0069.6128.0635.894C1.5683 13.171 0 14.7754 0 16.752c0 .1748.0142.3515.0352.5273.0141.083.0844.1475.1689.1475h15.9814c.0909 0 .1758-.0645.2032-.1553l.12-.4268zm2.7568-5.5634c-.0771 0-.1611 0-.2383.0112-.0566 0-.1054.0415-.127.0976l-.3378 1.1744c-.1475.5068-.0918.9707.1543 1.3164.2256.3164.6055.498 1.0625.5195l1.8437.1133c.0557 0 .1055.0263.1329.0703.0283.043.0351.1074.0214.1562-.0283.084-.1132.1485-.204.1553l-1.921.1123c-1.041.0488-2.1582.8867-2.5527 1.914l-.1406.3585c-.0283.0713.0215.1416.0986.1416h6.5977c.0771 0 .1474-.0489.169-.126.1122-.4082.1757-.837.1757-1.2803 0-2.6025-2.125-4.727-4.7344-4.727" />
    </svg>
  );
}

function TailscaleLogo({ className = "" }: { className?: string }) {
  return (
    <svg role="img" viewBox="0 0 24 24" className={className} fill="currentColor" aria-label="Tailscale">
      <path d="M24 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0zm-9 9a3 3 0 1 1-6 0 3 3 0 0 1 6 0zm0-9a3 3 0 1 1-6 0 3 3 0 0 1 6 0zm6-6a3 3 0 1 1 0-6 3 3 0 0 1 0 6zm0-.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM3 24a3 3 0 1 1 0-6 3 3 0 0 1 0 6zm0-.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zm18 .5a3 3 0 1 1 0-6 3 3 0 0 1 0 6zm0-.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM6 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0zm9-9a3 3 0 1 1-6 0 3 3 0 0 1 6 0zm-3 2.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM6 3a3 3 0 1 1-6 0 3 3 0 0 1 6 0zM3 5.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z" />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function EndpointsPage() {
  return (
    <>
      <PageHeader
        title="Endpoints"
        description="How your applications connect to KeiRouter."
      />
      <div className="space-y-5 sm:space-y-6">
        <PrimaryEndpoint />
        <TunnelSection />
        <APIKeys />
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Primary endpoint — hero card
// ---------------------------------------------------------------------------

function PrimaryEndpoint() {
  const access = useQuery({ queryKey: ["access-settings"], queryFn: () => api.accessSettings() });
  const [copied, setCopied] = useState(false);
  const url = access.data?.endpoint_url ?? "";

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      // no-op
    }
  };

  return (
    <Card>
      <div className="px-4 py-4 sm:px-6 sm:py-5">
        <p className="text-[11px] font-medium uppercase tracking-wider text-[var(--text-muted)]">
          Primary endpoint
        </p>
        {/* Mobile: stack vertically. Desktop: side-by-side. */}
        <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-stretch">
          <div className="flex min-w-0 flex-1 items-center rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2.5 sm:px-4 sm:py-3">
            <span className="truncate font-mono text-[13px] text-[var(--text)]">
              {url || "Loading…"}
            </span>
          </div>
          <button
            onClick={copy}
            className="flex shrink-0 items-center justify-center gap-2 rounded-xl bg-secondary-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-secondary-700 dark:bg-secondary-500 dark:hover:bg-secondary-400 sm:py-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-secondary-400/60"
          >
            {copied ? (
              <>
                <Check className="h-4 w-4" />
                <span>Copied</span>
              </>
            ) : (
              <>
                <Copy className="h-4 w-4" />
                <span>Copy</span>
              </>
            )}
          </button>
        </div>
        <p className="mt-2.5 text-xs text-[var(--text-muted)] sm:mt-3">
          Point your applications at this URL. All providers are accessible through this single endpoint.
        </p>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Tunnel section — single card with list
// ---------------------------------------------------------------------------

function TunnelSection() {
  return (
    <Card>
      <CardHeader
        title="Tunnels"
        description="Expose KeiRouter to external networks"
      />
      <div className="divide-y divide-[var(--border)] border-t border-[var(--border)]">
        <CloudflareTunnel />
        {/* Tailscale — temporarily disabled, under active development */}
        <TunnelRow
          name="Tailscale"
          description="Private network with HTTPS"
          logo={<TailscaleLogo className="h-5 w-5 text-[#2255CC]/40 dark:text-[#5990FF]/40" />}
          brandColor="bg-[#2255CC]/5 dark:bg-[#5990FF]/5"
          loading={false}
          isRunning={false}
          displayUrl=""
          reachable={null}
          actionsDisabled
          onEnable={() => {}}
          onDisable={() => {}}
        >
          <div className="flex items-center gap-2 rounded-lg border border-amber-300/40 bg-amber-500/5 px-3 py-2 dark:border-amber-500/20 dark:bg-amber-500/10">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-amber-500" />
            </span>
            <p className="text-xs font-medium text-amber-700 dark:text-amber-400">
              Under development — this feature is being built and will be available soon.
            </p>
          </div>
        </TunnelRow>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Cloudflare tunnel row
// ---------------------------------------------------------------------------

function CloudflareTunnel() {
  const qc = useQueryClient();
  const [loading, setLoading] = useState(false);
  const [reachable, setReachable] = useState<boolean | null>(null);
  const missRef = useRef(0);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const status = useQuery({
    queryKey: ["tunnel-status"],
    queryFn: () => api.tunnelStatus(),
    refetchInterval: STATUS_POLL_SLOW,
  });

  const tunnel = status.data?.tunnel;
  const download = status.data?.download;
  const tunnelUrl = tunnel?.tunnelUrl || "";
  const publicUrl = tunnel?.publicUrl || "";
  const isRunning = tunnel?.running ?? false;
  const displayUrl = publicUrl || tunnelUrl;

  const pingTunnel = useCallback(async () => {
    const url = publicUrl || tunnelUrl;
    if (!url) return;
    try {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), PING_TIMEOUT);
      const res = await fetch(`${url}/healthz`, { mode: "cors", signal: ctrl.signal });
      clearTimeout(timer);
      if (res.ok) {
        setReachable(true);
        missRef.current = 0;
      } else {
        missRef.current++;
        if (missRef.current >= REACHABLE_MISS_THRESHOLD) setReachable(false);
      }
    } catch {
      missRef.current++;
      if (missRef.current >= REACHABLE_MISS_THRESHOLD) setReachable(false);
    }
  }, [publicUrl, tunnelUrl]);

  useEffect(() => {
    if (isRunning && (publicUrl || tunnelUrl)) {
      pingTunnel();
      pingTimerRef.current = setInterval(pingTunnel, PING_INTERVAL);
      const stopAt = Date.now() + PING_MAX_MS;
      const check = setInterval(() => {
        if (Date.now() > stopAt) {
          clearInterval(pingTimerRef.current);
          clearInterval(check);
        }
      }, 10000);
      return () => {
        clearInterval(pingTimerRef.current);
        clearInterval(check);
      };
    } else {
      setReachable(null);
      missRef.current = 0;
    }
  }, [isRunning, publicUrl, tunnelUrl, pingTunnel]);

  useEffect(() => {
    if (loading) {
      pollTimerRef.current = setInterval(
        () => qc.invalidateQueries({ queryKey: ["tunnel-status"] }),
        STATUS_POLL_FAST,
      );
      return () => clearInterval(pollTimerRef.current);
    }
  }, [loading, qc]);

  const enable = useMutation({
    mutationFn: () => api.tunnelEnable(),
    onMutate: () => setLoading(true),
    onSuccess: () => {
      setLoading(false);
      qc.invalidateQueries({ queryKey: ["tunnel-status"] });
      qc.invalidateQueries({ queryKey: ["access-settings"] });
    },
    onError: () => setLoading(false),
  });

  const disable = useMutation({
    mutationFn: () => api.tunnelDisable(),
    onSuccess: () => {
      setReachable(null);
      qc.invalidateQueries({ queryKey: ["tunnel-status"] });
      qc.invalidateQueries({ queryKey: ["access-settings"] });
    },
  });

  return (
    <TunnelRow
      name="Cloudflare Tunnel"
      description="Quick tunnel — no account needed"
      logo={<CloudflareLogo className="h-5 w-5 text-[#F6821F]" />}
      brandColor="bg-[#F6821F]/10 dark:bg-[#F6821F]/15"
      isRunning={isRunning}
      reachable={reachable}
      loading={loading}
      displayUrl={displayUrl}
      subText={
        download?.downloading
          ? `Downloading cloudflared… ${download.progress}%`
          : undefined
      }
      onEnable={() => enable.mutate()}
      onDisable={() => disable.mutate()}
      enablePending={enable.isPending}
      disablePending={disable.isPending}
    />
  );
}

// ---------------------------------------------------------------------------
// Tailscale tunnel row
// ---------------------------------------------------------------------------

function TailscaleTunnel() {
  const qc = useQueryClient();
  const [loading, setLoading] = useState(false);
  const [reachable, setReachable] = useState<boolean | null>(null);
  const [sudoPassword, setSudoPassword] = useState("");
  const [installLog, setInstallLog] = useState<string[]>([]);
  const [installing, setInstalling] = useState(false);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [showInstall, setShowInstall] = useState(false);
  const missRef = useRef(0);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const status = useQuery({
    queryKey: ["tunnel-status"],
    queryFn: () => api.tunnelStatus(),
    refetchInterval: STATUS_POLL_SLOW,
  });

  const tsCheck = useQuery({
    queryKey: ["tailscale-check"],
    queryFn: () => api.tailscaleCheck(),
    refetchInterval: STATUS_POLL_SLOW,
  });

  const ts = status.data?.tailscale;
  const isRunning = ts?.running ?? false;
  const isLoggedIn = ts?.loggedIn ?? false;
  const isInstalled = tsCheck.data?.installed ?? false;
  const tunnelUrl = ts?.tunnelUrl || "";

  const pingTailscale = useCallback(async () => {
    if (!tunnelUrl) return;
    try {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), PING_TIMEOUT);
      const res = await fetch(`${tunnelUrl}/healthz`, { mode: "cors", signal: ctrl.signal });
      clearTimeout(timer);
      if (res.ok) {
        setReachable(true);
        missRef.current = 0;
      } else {
        missRef.current++;
        if (missRef.current >= REACHABLE_MISS_THRESHOLD) setReachable(false);
      }
    } catch {
      missRef.current++;
      if (missRef.current >= REACHABLE_MISS_THRESHOLD) setReachable(false);
    }
  }, [tunnelUrl]);

  useEffect(() => {
    if (isRunning && tunnelUrl) {
      pingTailscale();
      pingTimerRef.current = setInterval(pingTailscale, PING_INTERVAL);
      return () => clearInterval(pingTimerRef.current);
    } else {
      setReachable(null);
    }
  }, [isRunning, tunnelUrl, pingTailscale]);

  const handleInstall = async () => {
    setInstalling(true);
    setInstallLog([]);
    try {
      const res = await fetch("/api/tunnel/tailscale-install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ sudoPassword }),
      });
      if (!res.ok || !res.body) {
        setInstallLog((prev) => [...prev, "Failed to start install"]);
        setInstalling(false);
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";
        for (const line of lines) {
          if (line.startsWith("event: ")) {
            const event = line.slice(7);
            const dataLine = lines[lines.indexOf(line) + 1];
            if (dataLine?.startsWith("data: ")) {
              try {
                const data = JSON.parse(dataLine.slice(6));
                if (event === "progress") {
                  setInstallLog((prev) => [...prev, data.message]);
                } else if (event === "done") {
                  setInstallLog((prev) => [...prev, "Installation complete!"]);
                  setInstalling(false);
                  qc.invalidateQueries({ queryKey: ["tailscale-check"] });
                } else if (event === "error") {
                  setInstallLog((prev) => [...prev, `Error: ${data.error}`]);
                  setInstalling(false);
                }
              } catch { /* ignore parse errors */ }
            }
          }
        }
      }
    } catch (e) {
      setInstallLog((prev) => [...prev, `Error: ${(e as Error).message}`]);
      setInstalling(false);
    }
  };

  const enable = useMutation({
    mutationFn: () => api.tailscaleEnable(sudoPassword || undefined),
    onMutate: () => setLoading(true),
    onSuccess: (data: TailscaleEnableResult) => {
      setLoading(false);
      if (data.needsLogin && data.authUrl) {
        setAuthUrl(data.authUrl);
        window.open(data.authUrl, "_blank", "width=600,height=700");
      } else if (data.funnelNotEnabled && data.enableUrl) {
        window.open(data.enableUrl, "_blank", "width=600,height=700");
      } else if (data.success) {
        setAuthUrl(null);
        qc.invalidateQueries({ queryKey: ["tunnel-status"] });
        qc.invalidateQueries({ queryKey: ["access-settings"] });
      }
    },
    onError: () => setLoading(false),
  });

  const disable = useMutation({
    mutationFn: () => api.tailscaleDisable(),
    onSuccess: () => {
      setReachable(null);
      qc.invalidateQueries({ queryKey: ["tunnel-status"] });
      qc.invalidateQueries({ queryKey: ["access-settings"] });
    },
  });

  return (
    <TunnelRow
      name="Tailscale"
      description="Private network with HTTPS"
      logo={<TailscaleLogo className="h-5 w-5 text-[#2255CC] dark:text-[#5990FF]" />}
      brandColor="bg-[#2255CC]/10 dark:bg-[#5990FF]/15"
      isRunning={isRunning}
      reachable={reachable}
      loading={loading}
      displayUrl={tunnelUrl}
      onEnable={() => {
        if (!isInstalled) setShowInstall(true);
        else enable.mutate();
      }}
      onDisable={() => disable.mutate()}
      enablePending={enable.isPending}
      disablePending={disable.isPending}
    >
      {!isInstalled && showInstall && (
        <div className="space-y-3">
          <Field label="Sudo password (for installation)">
            <Input
              type="password"
              value={sudoPassword}
              onChange={(e) => setSudoPassword(e.target.value)}
              placeholder="Required for system install"
            />
          </Field>
          <div className="flex gap-2">
            <Button onClick={handleInstall} disabled={installing || !sudoPassword.trim()}>
              {installing ? <Loader2 className="h-4 w-4 animate-spin" /> : "Install"}
            </Button>
            <Button variant="ghost" onClick={() => setShowInstall(false)}>
              Cancel
            </Button>
          </div>
          {installLog.length > 0 && (
            <div className="max-h-36 overflow-y-auto rounded-lg bg-ink-950 p-3 font-mono text-[11px] leading-relaxed text-ink-300">
              {installLog.map((line, i) => (
                <div key={i}>{line}</div>
              ))}
            </div>
          )}
        </div>
      )}

      {isInstalled && authUrl && (
        <div className="rounded-lg border border-accent-200 bg-accent-50 px-3 py-2 dark:border-accent-800/50 dark:bg-accent-800/20">
          <p className="text-xs text-accent-700 dark:text-accent-200">
            Login required —{" "}
            <a href={authUrl} target="_blank" rel="noopener noreferrer" className="font-medium underline">
              authenticate here
            </a>
          </p>
        </div>
      )}

      {isInstalled && !isRunning && isLoggedIn && (
        <div className="max-w-sm">
          <Field label="Sudo password (optional, for TUN mode)">
            <Input
              type="password"
              value={sudoPassword}
              onChange={(e) => setSudoPassword(e.target.value)}
              placeholder="Leave empty for userspace"
            />
          </Field>
        </div>
      )}

      {isInstalled && !isLoggedIn && (
        <p className="text-xs text-[var(--text-muted)]">
          Log in to your Tailscale account to enable the funnel.
        </p>
      )}
    </TunnelRow>
  );
}

// ---------------------------------------------------------------------------
// Shared tunnel row component
// ---------------------------------------------------------------------------

function TunnelRow({
  name,
  description,
  logo,
  brandColor,
  isRunning,
  reachable,
  loading,
  displayUrl,
  subText,
  onEnable,
  onDisable,
  enablePending,
  disablePending,
  actionsDisabled,
  children,
}: {
  name: string;
  description: string;
  logo: React.ReactNode;
  brandColor: string;
  isRunning: boolean;
  reachable: boolean | null;
  loading: boolean;
  displayUrl?: string;
  subText?: string;
  onEnable: () => void;
  onDisable: () => void;
  enablePending: boolean;
  disablePending: boolean;
  actionsDisabled?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col px-4 py-4 sm:px-6 sm:py-5">
      <div className="flex items-center gap-4">
        <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${brandColor}`}>
          {logo}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-[var(--text)]">{name}</h3>
            {isRunning && <TunnelBadge reachable={reachable} />}
          </div>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">{description}</p>
          {isRunning && displayUrl && (
            <div className="mt-1.5 flex items-center gap-1.5">
              <TunnelDot running={isRunning} reachable={reachable} />
              <a
                href={displayUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono text-xs text-[var(--text-muted)] transition-colors hover:text-[var(--text)] truncate flex items-center gap-1"
              >
                <span>{displayUrl}</span>
                <ArrowUpRight className="h-3 w-3 shrink-0" />
              </a>
            </div>
          )}
          {subText && (
            <p className="mt-1.5 text-xs text-[var(--text-muted)]">{subText}</p>
          )}
        </div>
        <div className="flex shrink-0 items-center pl-2">
          {isRunning ? (
            <Button variant="danger" onClick={onDisable} disabled={actionsDisabled || disablePending}>
              {disablePending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Disable"}
            </Button>
          ) : (
            <Button onClick={onEnable} disabled={actionsDisabled || loading || enablePending} className={actionsDisabled ? "opacity-50 cursor-not-allowed" : ""}>
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Enable"}
            </Button>
          )}
        </div>
      </div>
      {children && (
        <div className="mt-4 pt-4 border-t border-[var(--border)]">
          {children}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared tunnel UI atoms
// ---------------------------------------------------------------------------

function TunnelDot({
  running,
  reachable,
}: {
  running: boolean;
  reachable: boolean | null;
}) {
  const color = running
    ? reachable === true
      ? "bg-green-500"
      : reachable === false
        ? "bg-red-500"
        : "bg-amber-500 animate-pulse"
    : "bg-ink-300 dark:bg-ink-600";
  return <span className={`block h-2 w-2 rounded-full ${color}`} />;
}

function TunnelBadge({ reachable }: { reachable: boolean | null }) {
  const tone = reachable === true ? "success" : reachable === false ? "danger" : "neutral";
  const label = reachable === true ? "Reachable" : reachable === false ? "Unreachable" : "Checking…";
  return <Badge tone={tone}>{label}</Badge>;
}

// ---------------------------------------------------------------------------
// API Keys
// ---------------------------------------------------------------------------

const budgetPeriods = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
  { value: "total", label: "All time" },
];

function APIKeys() {
  const qc = useQueryClient();
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api.listKeys() });

  const [modalOpen, setModalOpen] = useState(false);
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [name, setName] = useState("");
  const [budgetLimit, setBudgetLimit] = useState("");
  const [budgetLimitTokens, setBudgetLimitTokens] = useState("");
  const [budgetPeriod, setBudgetPeriod] = useState("monthly");
  const [budgetAlertPct, setBudgetAlertPct] = useState(80);
  const [budgetHardCutoff, setBudgetHardCutoff] = useState(true);
  const [allowedModels, setAllowedModels] = useState<string[]>([]);
  const [created, setCreated] = useState<CreatedKey | null>(null);
  const [copied, setCopied] = useState(false);

  const openModal = () => {
    setName("");
    setBudgetLimit("");
    setBudgetLimitTokens("");
    setBudgetPeriod("monthly");
    setBudgetAlertPct(80);
    setBudgetHardCutoff(true);
    setAllowedModels([]);
    setCreated(null);
    setCopied(false);
    setStep(1);
    setModalOpen(true);
  };

  const closeModal = () => {
    setModalOpen(false);
    if (created) { setCreated(null); setCopied(false); }
  };

  const create = useMutation({
    mutationFn: () => {
      const hasLimit = parseFloat(budgetLimit) > 0;
      const hasTokenLimit = parseInt(budgetLimitTokens) > 0;
      const opts = hasLimit || hasTokenLimit || allowedModels.length > 0
        ? {
            ...(hasLimit ? { budget_limit_usd: parseFloat(budgetLimit) } : {}),
            ...(hasTokenLimit ? { budget_limit_tokens: parseInt(budgetLimitTokens) } : {}),
            ...(hasLimit || hasTokenLimit ? { budget_period: budgetPeriod, budget_alert_pct: budgetAlertPct, budget_hard_cutoff: budgetHardCutoff } : {}),
            ...(allowedModels.length > 0 ? { allowed_models: allowedModels } : {}),
          }
        : undefined;
      return api.createKey(name, opts);
    },
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ["keys"] });
      qc.invalidateQueries({ queryKey: ["budgets"] });
      qc.invalidateQueries({ queryKey: ["budget-status"] });
      setCreated(data);
      setStep(3);
    },
    onError: () => {},
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["keys"] }),
  });

  const toggleDisabled = useMutation({
    mutationFn: ({ id, disabled }: { id: string; disabled: boolean }) => api.updateKey(id, { disabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["keys"] }),
  });

  return (
    <Card>
      <CardHeader
        title="API keys"
        description="Authenticate your applications"
        action={
          <Button onClick={openModal}>
            <Plus className="h-4 w-4" />
            New key
          </Button>
        }
      />

      <Modal
        open={modalOpen}
        onClose={closeModal}
        title={step === 3 ? "Key created" : "Create API key"}
        subtitle={
          step === 1 ? "Name your key so you can identify it later."
            : step === 2 ? "Optionally set budget limits and model restrictions."
            : undefined
        }
      >
        {step === 1 && (
          <div className="space-y-4 px-6 py-5">
            <Field label="Key name">
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="laptop"
                autoFocus
                onKeyDown={(e) => { if (e.key === "Enter" && name.trim()) { e.preventDefault(); setStep(2); } }}
              />
            </Field>
            <Button className="w-full" onClick={() => setStep(2)} disabled={!name.trim()}>
              Next
            </Button>
          </div>
        )}
        {step === 2 && (
          <div className="space-y-4 px-6 py-5">
            {(keys.data?.keys?.length ?? 0) === 0 && (
              <div className="rounded-xl border border-accent-200 bg-accent-50 px-4 py-3 dark:border-accent-800 dark:bg-accent-950/30">
                <p className="text-sm font-medium text-accent-800 dark:text-accent-200">Set a budget to control spending</p>
                <p className="mt-0.5 text-xs text-accent-700 dark:text-accent-300">This is your first key. Adding a budget now prevents surprise bills.</p>
              </div>
            )}
            <div className="flex gap-3">
              <div className="flex-1"><Field label="Limit (USD)"><Input type="number" min="0" step="0.01" value={budgetLimit} onChange={(e) => setBudgetLimit(e.target.value)} placeholder="50.00" /></Field></div>
              <div className="flex-1"><Field label="Limit (Tokens)"><FormattedTokenInput value={budgetLimitTokens} onChange={setBudgetLimitTokens} placeholder="100000000" /></Field></div>
              <div className="w-36"><Field label="Period"><Select value={budgetPeriod} onChange={(e) => setBudgetPeriod(e.target.value)}>{budgetPeriods.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}</Select></Field></div>
            </div>
            <Field label="Allowed models">
              <ModelMultiSelect value={allowedModels} onChange={setAllowedModels} />
              <p className="mt-1 text-[10px] text-[var(--text-muted)]">Select models or add custom patterns with * wildcard (e.g. claude-*)</p>
            </Field>
            <div className="flex items-end gap-6">
              <div className="w-40"><Field label="Alert threshold (%)"><Input type="number" min="1" max="100" value={budgetAlertPct} onChange={(e) => setBudgetAlertPct(parseInt(e.target.value) || 80)} /></Field></div>
              <div className="flex items-center gap-2 pb-0.5"><Toggle checked={budgetHardCutoff} onChange={setBudgetHardCutoff} /><span className="text-sm">Hard cutoff</span></div>
            </div>
            <div className="flex gap-2 pt-2 border-t border-[var(--border)]">
              <Button variant="ghost" onClick={() => setStep(1)}>Back</Button>
              <div className="flex-1" />
              <Button variant="ghost" onClick={() => create.mutate()} disabled={create.isPending}>{create.isPending ? "Creating…" : "Skip budget"}</Button>
              <Button onClick={() => create.mutate()} disabled={create.isPending}>{create.isPending ? "Creating…" : "Create key"}</Button>
            </div>
          </div>
        )}
        {step === 3 && created && (
          <div className="space-y-4 px-6 py-5">
            <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-subtle)] p-4">
              <p className="text-xs font-medium text-[var(--text-muted)]">Copy this key now — it won't be shown again.</p>
              <div className="mt-2 flex items-center gap-2">
                <code className="flex-1 overflow-x-auto rounded-lg bg-[var(--bg-elevated)] px-3 py-2.5 font-mono text-sm">{created.key}</code>
                <Button onClick={() => { navigator.clipboard.writeText(created.key); setCopied(true); setTimeout(() => setCopied(false), 1500); }}>
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  {copied ? "Copied" : "Copy"}
                </Button>
              </div>
            </div>
            {created.budget && (
              <div className="rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] px-4 py-3">
                <p className="text-xs font-medium text-[var(--text-muted)]">Budget attached</p>
                <p className="mt-0.5 text-sm">
                  {created.budget.limit_micros > 0 && `$${(created.budget.limit_micros / 1_000_000).toFixed(2)}`}
                  {created.budget.limit_micros > 0 && created.budget.limit_tokens > 0 && " + "}
                  {created.budget.limit_tokens > 0 && `${formatTokenLimit(String(created.budget.limit_tokens))} tokens`}
                  {` / ${created.budget.period}`}
                  {created.budget.hard_cutoff ? " (hard cutoff)" : ""}
                </p>
              </div>
            )}
            {created.allowed_models && created.allowed_models.length > 0 && (
              <div className="rounded-lg border border-[var(--border)] bg-[var(--bg-subtle)] px-4 py-3">
                <p className="text-xs font-medium text-[var(--text-muted)]">Allowed models</p>
                <p className="mt-0.5 text-sm">{created.allowed_models.join(", ")}</p>
              </div>
            )}
            <Button className="w-full" onClick={closeModal}>Done</Button>
          </div>
        )}
      </Modal>

      {keys.isLoading ? (
        <Spinner />
      ) : !keys.data?.keys?.length ? (
        <EmptyState title="No API keys yet" hint="Create a key to authenticate your app." />
      ) : (
        <div className="divide-y divide-[var(--border)]">
          {keys.data.keys.map((k) => (
            <div key={k.id} className="flex items-center gap-3 px-4 py-3 sm:px-5 sm:py-3.5">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-ink-100 text-ink-500 dark:bg-ink-800 dark:text-ink-400">
                <KeyRound className="h-4 w-4" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="truncate text-sm font-medium">{k.name}</p>
                  {k.allowed_models && k.allowed_models.length > 0 && (
                    <Badge tone="accent">{k.allowed_models.length} model{k.allowed_models.length > 1 ? "s" : ""}</Badge>
                  )}
                </div>
                <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">{k.display}</p>
              </div>
              <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
                <Badge tone={k.disabled ? "neutral" : "success"}>
                  {k.disabled ? "Off" : "On"}
                </Badge>
                <button
                  onClick={() => toggleDisabled.mutate({ id: k.id, disabled: !k.disabled })}
                  className="rounded-lg p-1.5 text-[var(--text-muted)] transition-colors hover:bg-ink-100 hover:text-[var(--text)] dark:hover:bg-ink-800"
                  title={k.disabled ? "Enable key" : "Disable key"}
                >
                  {k.disabled ? <ToggleLeft className="h-4 w-4" /> : <ToggleRight className="h-4 w-4" />}
                </button>
                <button
                  onClick={() => remove.mutate(k.id)}
                  className="rounded-lg p-1.5 text-[var(--text-muted)] transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400"
                  title="Delete key"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
