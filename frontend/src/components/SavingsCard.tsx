import { useRef, useState, useCallback } from "react";
import { toPng } from "html-to-image";
import { Download, Check } from "lucide-react";
import type { UsageInsights } from "../lib/api";

// ─── Helpers ─────────────────────────────────────────────────────────────────

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

const periodLabels: Record<string, string> = {
  today: "Today",
  "24h": "Last 24 Hours",
  week: "Last 7 Days",
  month: "Last 30 Days",
};

// App light palette
const C = {
  bg: "#faf9f7",
  border: "rgba(0,0,0,0.06)",
  borderLight: "rgba(0,0,0,0.04)",
  text: "#1c1b18",
  textSecondary: "#43413a",
  textMuted: "#a8a59a",
  textCaption: "#656259",
  accent: "#C45F3A",
  accentLight: "#d98a6a",
  statValue: "#2d2b27",
};

// ─── Hidden Card (rendered off-screen for capture) ───────────────────────────

interface SavingsCardData {
  costSaved: number;
  tokensSaved: number;
  savingsPct: number;
  totalRequests: number;
  period: string;
  rtkActive: boolean;
  cavemanActive: boolean;
  terseActive: boolean;
  actualCost: number;
}

function SavingsCardContent({ data }: { data: SavingsCardData }) {
  const {
    costSaved,
    tokensSaved,
    savingsPct,
    totalRequests,
    period,
    rtkActive,
    cavemanActive,
    terseActive,
    actualCost,
  } = data;

  const optimizers = [
    rtkActive && { name: "RTK", desc: "Token Slimming" },
    cavemanActive && { name: "Caveman", desc: "Output Compression" },
    terseActive && { name: "Terse", desc: "Output Compression" },
  ].filter(Boolean) as { name: string; desc: string }[];

  return (
    <div
      style={{
        width: 1200,
        height: 630,
        position: "relative",
        overflow: "hidden",
        fontFamily:
          "'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
        background: C.bg,
      }}
    >
      {/* Subtle texture */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage:
            "linear-gradient(rgba(0,0,0,0.018) 1px, transparent 1px), linear-gradient(90deg, rgba(0,0,0,0.018) 1px, transparent 1px)",
          backgroundSize: "60px 60px",
        }}
      />

      {/* Content */}
      <div
        style={{
          position: "relative",
          zIndex: 1,
          display: "flex",
          flexDirection: "column",
          height: "100%",
          padding: "56px 64px",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <img
              src="/keirouter-logo.png"
              alt="KeiRouter"
              style={{ width: 32, height: 32, objectFit: "contain" }}
              crossOrigin="anonymous"
            />
            <span
              style={{
                fontSize: 18,
                fontWeight: 600,
                color: C.text,
                letterSpacing: "-0.01em",
              }}
            >
              KeiRouter
            </span>
            <span
              style={{
                fontSize: 11,
                fontWeight: 600,
                color: C.textCaption,
                letterSpacing: "0.06em",
                textTransform: "uppercase",
                marginLeft: 8,
              }}
            >
              Savings Report
            </span>
          </div>
          <span
            style={{
              fontSize: 13,
              fontWeight: 500,
              color: C.textMuted,
            }}
          >
            {periodLabels[period] || period}
          </span>
        </div>

        {/* Divider */}
        <div
          style={{
            height: 1,
            background: C.border,
            margin: "28px 0",
          }}
        />

        {/* Main content */}
        <div
          style={{
            flex: 1,
            display: "flex",
            gap: 80,
            alignItems: "center",
          }}
        >
          {/* Left: hero */}
          <div style={{ flex: "0 0 auto" }}>
            <span
              style={{
                display: "block",
                fontSize: 13,
                fontWeight: 500,
                color: C.textMuted,
                marginBottom: 12,
                letterSpacing: "0.02em",
              }}
            >
              Total cost saved
            </span>
            <span
              style={{
                display: "block",
                fontSize: 64,
                fontWeight: 700,
                color: C.accent,
                lineHeight: 1,
                letterSpacing: "-0.03em",
              }}
            >
              ${costSaved.toFixed(2)}
            </span>
            <span
              style={{
                display: "block",
                fontSize: 15,
                fontWeight: 500,
                color: C.accentLight,
                marginTop: 12,
              }}
            >
              {savingsPct.toFixed(1)}% reduction
            </span>
          </div>

          {/* Right: stats */}
          <div
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              gap: 20,
            }}
          >
            {/* Stat rows */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 16,
              }}
            >
              <StatRow label="Tokens saved" value={fmtNum(tokensSaved)} />
              <StatRow label="Total requests" value={fmtNum(totalRequests)} />
              <StatRow
                label="Cost with KeiRouter"
                value={`$${actualCost.toFixed(2)}`}
              />
              <StatRow label="Period" value={periodLabels[period] || period} />
            </div>

            {/* Optimizers */}
            {optimizers.length > 0 && (
              <div>
                <span
                  style={{
                    display: "block",
                    fontSize: 11,
                    fontWeight: 600,
                    color: C.textMuted,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    marginBottom: 10,
                  }}
                >
                  Active Optimizers
                </span>
                <div style={{ display: "flex", gap: 10 }}>
                  {optimizers.map((opt) => (
                    <div
                      key={opt.name}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 8,
                        background: "rgba(0,0,0,0.025)",
                        borderRadius: 8,
                        padding: "8px 14px",
                        border: `1px solid ${C.border}`,
                      }}
                    >
                      <span
                        style={{
                          fontSize: 13,
                          fontWeight: 600,
                          color: C.textSecondary,
                        }}
                      >
                        {opt.name}
                      </span>
                      <span
                        style={{
                          fontSize: 11,
                          color: C.textCaption,
                        }}
                      >
                        {opt.desc}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            borderTop: `1px solid ${C.border}`,
            paddingTop: 20,
          }}
        >
          <span style={{ fontSize: 12, color: C.textMuted }}>
            keirouter.dev
          </span>
          <span style={{ fontSize: 12, color: C.textMuted }}>
            AI routing, optimized.
          </span>
        </div>
      </div>
    </div>
  );
}

function StatRow({ label, value }: { label: string; value: string }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "baseline",
        justifyContent: "space-between",
        padding: "10px 0",
        borderBottom: `1px solid ${C.borderLight}`,
      }}
    >
      <span
        style={{
          fontSize: 13,
          color: C.textCaption,
        }}
      >
        {label}
      </span>
      <span
        style={{
          fontSize: 18,
          fontWeight: 600,
          color: C.statValue,
          letterSpacing: "-0.01em",
        }}
      >
        {value}
      </span>
    </div>
  );
}

// ─── Share Button ────────────────────────────────────────────────────────────

export function SavingsCardShareButton({
  insights,
  period,
}: {
  insights: UsageInsights;
  period: string;
}) {
  const cardRef = useRef<HTMLDivElement>(null);
  const [generating, setGenerating] = useState(false);
  const [done, setDone] = useState(false);

  const { summary, savings } = insights;

  const avgRate = 3;
  const tokensSaved = savings?.slim_tokens_saved ?? 0;
  const costSaved = (tokensSaved / 1_000_000) * avgRate;
  const actualCost = summary.cost_usd;
  const originalCost = actualCost + costSaved;
  const savingsPct = originalCost > 0 ? (costSaved / originalCost) * 100 : 0;

  const cardData: SavingsCardData = {
    costSaved,
    tokensSaved,
    savingsPct,
    totalRequests: summary.total_requests,
    period,
    rtkActive: (savings?.slim_tokens_saved ?? 0) > 0,
    cavemanActive: (savings?.caveman_requests ?? 0) > 0,
    terseActive: (savings?.terse_requests ?? 0) > 0,
    actualCost,
  };

  const handleShare = useCallback(async () => {
    if (!cardRef.current || generating) return;
    setGenerating(true);
    setDone(false);

    try {
      const dataUrl = await toPng(cardRef.current, {
        width: 1200,
        height: 630,
        pixelRatio: 2,
        cacheBust: true,
        crossOriginLoading: "anonymous",
      });

      const link = document.createElement("a");
      link.download = `keirouter-savings-${period}.png`;
      link.href = dataUrl;
      link.click();
      setDone(true);
      setTimeout(() => setDone(false), 2000);
    } catch (err) {
      console.error("Failed to generate savings card:", err);
    } finally {
      setGenerating(false);
    }
  }, [generating, period]);

  return (
    <>
      {/* Hidden card for image capture */}
      <div
        style={{
          position: "fixed",
          left: "-9999px",
          top: 0,
          zIndex: -1,
          pointerEvents: "none",
        }}
      >
        <div ref={cardRef}>
          <SavingsCardContent data={cardData} />
        </div>
      </div>

      {/* Download button */}
      <button
        onClick={handleShare}
        disabled={generating || summary.total_requests === 0}
        className="inline-flex h-8 items-center gap-1.5 whitespace-nowrap rounded-lg bg-accent-600 px-3 text-xs font-medium text-white shadow-sm transition-all hover:bg-accent-700 active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-50 dark:bg-accent-500 dark:hover:bg-accent-400"
      >
        {done ? (
          <>
            <Check className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">Downloaded!</span>
          </>
        ) : generating ? (
          <>
            <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
            <span className="hidden sm:inline">Generating…</span>
          </>
        ) : (
          <>
            <img
              src="/keirouter-logo.png"
              alt=""
              className="h-3.5 w-3.5 object-contain"
            />
            <span className="hidden sm:inline">Savings Card</span>
            <Download className="h-3.5 w-3.5 opacity-70" />
          </>
        )}
      </button>
    </>
  );
}