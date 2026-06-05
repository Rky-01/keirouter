// Typed client for the KeiRouter admin API. All calls go through the dev-server
// proxy (or the embedded static server in production) to /api.

export interface RegionOption {
  id: string;
  label: string;
  base_url: string;
}

export interface Provider {
  id: string;
  display_name: string;
  alias: string;
  dialect: string;
  auth_kind: string;
  auth_modes: string[];
  service_kinds: string[];
  color: string;
  website: string;
  api_key_url: string;
  icon: string;
  deprecated: boolean;
  hidden: boolean;
  notice: string;
  drivable: boolean;
  input_per_m: number;
  output_per_m: number;
  regions?: RegionOption[];
  default_region?: string;
}

export interface EndpointSettings {
  rtk_enabled: boolean;
  caveman_enabled: boolean;
  caveman_level: string;
  terse_enabled: boolean;
  terse_level: string;
  routing_strategy: string;
  sticky_limit: number;
  combo_strategy: string;
  combo_sticky_limit: number;
  outbound_proxy_enabled: boolean;
  outbound_proxy_url: string;
  outbound_no_proxy: string;
  observability_enabled?: boolean;
}

export interface ProviderRoutingSettings {
  routing_strategy: "inherit" | "fill-first" | "round-robin" | "smart-round-robin" | string;
  sticky_limit: number;
  affinity_ttl_minutes: number;
}

export interface OAuthProvider {
  provider: string;
  display_name: string;
  flow: string; // authorization_code_pkce | authorization_code | device_code
  icon: string;
  color: string;
  callback_path?: string;
  fixed_port?: number;
  loopback_host?: string;
}

export interface DeviceCode {
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete: string;
  expires_in: number;
  interval: number;
}

export interface OAuthPollResult {
  status: string; // pending | complete
  slow_down?: boolean;
  id?: string;
  provider?: string;
}

export interface APIKey {
  id: string;
  name: string;
  display: string;
  disabled: boolean;
  created_at: string;
  allowed_models?: string[];
}

export interface CreatedKey {
  id: string;
  name: string;
  key: string;
  display: string;
  budget?: {
    id: string;
    scope_kind: string;
    limit_micros: number;
    limit_tokens: number;
    period: string;
    alert_pct: number;
    hard_cutoff: boolean;
  };
  allowed_models?: string[];
}

export interface Account {
  id: string;
  provider: string;
  label: string;
  auth_kind: string;
  priority: number;
  disabled: boolean;
  proxy_pool_id?: string;
  created_at: string;
}

export interface AccountInput {
  provider: string;
  label: string;
  api_key?: string;
  base_url?: string;
  region?: string;
  account_id?: string;
  azure_endpoint?: string;
  azure_deployment?: string;
  azure_api_version?: string;
  azure_organization?: string;
  proxy_pool_id?: string;
  priority?: number;
}

export interface ChainStep {
  provider: string;
  model: string;
  position: number;
}

export interface Chain {
  id: string;
  name: string;
  strategy: string;
  fallback_provider?: string;
  fallback_model?: string;
  steps: ChainStep[];
}

export interface Budget {
  id: string;
  scope_kind: string;
  scope_id: string;
  limit_micros: number;
  limit_tokens: number;
  period: string;
  alert_pct: number;
  hard_cutoff: boolean;
}

export interface BudgetStatus {
  id: string;
  scope_kind: string;
  scope_id: string;
  scope_name: string;
  limit_micros: number;
  limit_tokens: number;
  period: string;
  alert_pct: number;
  hard_cutoff: boolean;
  spent_micros: number;
  spent_tokens: number;
  pct_used: number;
  tokens_pct_used: number;
  period_start: string;
}

export interface UsageSummary {
  total_requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  cost_usd: number;
  cache_hits: number;
  since: string;
}

export interface ProviderUsage {
  provider: string;
  display_name: string;
  color: string;
  icon: string;
  total_requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost_usd: number;
  share_pct: number;
}

export interface RecentActivity {
  id: string;
  provider: string;
  model: string;
  tokens: number;
  cost_usd: number;
  cache_hit: boolean;
  latency_ms: number;
  created_at: string;
  ttft_ms?: number;
  slim_bytes_saved?: number;
  slim_tokens_saved?: number;
  slim_rules?: string;
  caveman_active?: boolean;
  terse_active?: boolean;
}

export interface RuleSaving {
  rule: string;
  count: number;
  bytes_saved: number;
  tokens_saved: number;
}

export interface TokenSavings {
  slim_bytes_saved: number;
  slim_tokens_saved: number;
  caveman_requests: number;
  terse_requests: number;
  rules: RuleSaving[];
}

export interface ModelUsage {
  provider: string;
  provider_name: string;
  model: string;
  total_requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost_usd: number;
  input_per_m?: number;
  output_per_m?: number;
  cached_input_per_m?: number;
}

export interface SeriesPoint {
  label: string;
  count: number;
}

export interface UsageInsights {
  summary: {
    total_requests: number;
    prompt_tokens: number;
    completion_tokens: number;
    cached_tokens: number;
    cost_usd: number;
    cache_hits: number;
    success_rate: number;
    avg_latency_ms: number;
    avg_ttft_ms: number;
    since: string;
  };
  savings: TokenSavings;
  providers: ProviderUsage[];
  recent: RecentActivity[];
  series: SeriesPoint[];
  busiest: string;
}

export interface UpstreamQuota {
  resource_type: string;
  used: number;
  limit: number;
  remaining: number;
  reset_at?: string;
}

export interface QuotaAccount {
  id: string;
  provider: string;
  provider_name: string;
  label: string;
  auth_kind: string;
  priority: number;
  status: string; // active | paused | needs_attention
  usage_type: string; // token | credit
  total_requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  cost_usd: number;
  input_per_m: number;
  output_per_m: number;
  notice?: string;
  plan_name?: string;
  message?: string;
  upstream_quotas?: UpstreamQuota[];
  updated_at: string;
}

// Console log now uses raw text lines (like 9router).
// The /api/console endpoint returns { logs: string[] }.

export interface ProxyPool {
  id: string;
  name: string;
  type: string; // http | vercel | cloudflare | deno
  proxy_url: string;
  no_proxy: string;
  strict: boolean;
  is_active: boolean;
  test_status: string; // unknown | active | error
  last_tested?: string;
  last_error?: string;
}

export interface Skill {
  id: string;
  name: string;
  description: string;
  prompt: string;
  enabled: boolean;
  created_at: string;
}

export interface AccessSettings {
  local_enabled: boolean;
  tunnel_enabled: boolean;
  tailscale_enabled: boolean;
  tunnel_url?: string;
  tailscale_url?: string;
  endpoint_url: string;
}

export interface TunnelStatus {
  enabled: boolean;
  settingsEnabled: boolean;
  tunnelUrl: string;
  shortId: string;
  publicUrl: string;
  running: boolean;
}

export interface TailscaleStatus {
  enabled: boolean;
  settingsEnabled: boolean;
  tunnelUrl: string;
  running: boolean;
  loggedIn: boolean;
  installed: boolean;
  platform: string;
}

export interface TunnelCombinedStatus {
  tunnel: TunnelStatus;
  tailscale: TailscaleStatus;
  download: { downloading: boolean; progress: number };
}

export interface TunnelEnableResult {
  success: boolean;
  tunnelUrl: string;
  shortId: string;
  publicUrl: string;
  alreadyRunning?: boolean;
}

export interface TailscaleCheckResult {
  installed: boolean;
  loggedIn: boolean;
  platform: string;
  daemonRunning: boolean;
  hasCachedPassword: boolean;
}

export interface TailscaleEnableResult {
  success: boolean;
  tunnelUrl?: string;
  needsLogin?: boolean;
  authUrl?: string;
  funnelNotEnabled?: boolean;
  enableUrl?: string;
  error?: string;
}

export interface CLITool {
  id: string;
  name: string;
  dialect: string;
  instructions: string;
  snippet: string;
  installed: boolean;
  configured: boolean;
  config_path: string;
}

export interface CLIToolsResponse {
  base_url: string;
  model: string;
  tools: CLITool[];
}

export interface AuthStatus {
  authenticated: boolean;
  using_default: boolean;
  onboarding_complete: boolean;
}

class APIError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

/** Returns the browser's IANA timezone (e.g. "Asia/Jakarta"), falling back to UTC. */
function browserTZ(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const data = await res.json();
      message = data?.error?.message ?? message;
    } catch {
      // keep statusText
    }
    throw new APIError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export interface KeyUsageData {
  key_id: string;
  key_name: string;
  budgets: {
    period: string;
    limit_tokens: number;
    tokens_used: number;
    tokens_remaining: number;
    tokens_pct_used: number;
    limit_usd: number;
    spent_usd: number;
    usd_remaining: number;
    usd_pct_used: number;
    alert: boolean;
  }[];
  allowed_models: string[];
  current_period: {
    prompt_tokens: number;
    completion_tokens: number;
    total_requests: number;
    cost_usd: number;
  };
}

/**
 * Fetch usage stats for an API Key, authenticated via the key itself (public portal)
 */
export async function fetchKeyUsage(key: string): Promise<KeyUsageData> {
  const resp = await fetch("/v1/keys/me/usage", {
    headers: { Authorization: `Bearer ${key}` },
  });
  if (!resp.ok) {
    const data = await resp.json().catch(() => ({}));
    throw new Error(data.error || "Invalid key or server error");
  }
  return resp.json();
}

/**
 * Fetch usage stats for an API Key using its database ID (public portal link sharing)
 */
export async function fetchKeyUsageById(id: string): Promise<KeyUsageData> {
  const resp = await fetch(`/v1/portal/keys/${id}/usage`);
  if (!resp.ok) {
    const data = await resp.json().catch(() => ({}));
    throw new Error(data.error || "Invalid key ID or server error");
  }
  return resp.json();
}

export const api = {
  // Auth (no session required for status/login/logout).
  authStatus: () => request<AuthStatus>("GET", "/auth/status"),
  login: (password: string) =>
    request<{ ok: boolean; using_default: boolean; onboarding_complete: boolean }>(
      "POST",
      "/auth/login",
      { password },
    ),
  logout: () => request<{ ok: boolean }>("POST", "/auth/logout"),
  changePassword: (newPassword: string) =>
    request<{ ok: boolean }>("POST", "/auth/password", { new_password: newPassword }),
  completeOnboarding: () => request<{ ok: boolean }>("POST", "/auth/onboarding/complete"),

  providers: () => request<{ providers: Provider[] }>("GET", "/providers"),
  providerModels: (id: string) =>
    request<{ models: { id: string; name: string; kind: string }[] }>("GET", `/providers/${id}/models`),
  providerRouting: (id: string) =>
    request<ProviderRoutingSettings>("GET", `/providers/${id}/routing`),
  updateProviderRouting: (id: string, patch: Partial<ProviderRoutingSettings>) =>
    request<ProviderRoutingSettings>("POST", `/providers/${id}/routing`, patch),

  listKeys: () => request<{ keys: APIKey[] }>("GET", "/keys"),
  createKey: (name: string, opts?: {
    budget_limit_usd?: number;
    budget_limit_tokens?: number;
    budget_period?: string;
    budget_alert_pct?: number;
    budget_hard_cutoff?: boolean;
    allowed_models?: string[];
  }) =>
    request<CreatedKey>("POST", "/keys", { name, ...(opts ? opts : {}) }),
  updateKey: (id: string, patch: { disabled?: boolean; allowed_models?: string[] }) =>
    request<{ id: string; disabled?: boolean; allowed_models?: string[] }>("PATCH", `/keys/${id}`, patch),
  deleteKey: (id: string) => request<void>("DELETE", `/keys/${id}`),
  deleteKeys: (ids: string[]) => Promise.all(ids.map((id) => request<void>("DELETE", `/keys/${id}`))),

  listAccounts: () => request<{ accounts: Account[] }>("GET", "/accounts"),
  createAccount: (input: AccountInput) =>
    request<{ id: string }>("POST", "/accounts", input),
  updateAccount: (id: string, patch: { label?: string; priority?: number; disabled?: boolean; proxy_pool_id?: string }) =>
    request<{ id: string }>("PATCH", `/accounts/${id}`, patch),
  deleteAccount: (id: string) => request<void>("DELETE", `/accounts/${id}`),
  testAccount: (id: string) =>
    request<{ id: string; status: string; message: string }>("POST", `/accounts/${id}/test`),
  validateKey: (input: AccountInput) =>
    request<{ status: string; message?: string }>("POST", "/validate-key", input),
  accountQuota: (id: string) =>
    request<{ provider: string; supported: boolean; plan_name?: string; message?: string; quotas?: UpstreamQuota[] }>(
      "GET", `/accounts/${id}/quota`,
    ),

  listChains: () => request<{ chains: Chain[] }>("GET", "/chains"),
  createChain: (input: { name: string; strategy?: string; fallback_provider?: string; fallback_model?: string; steps: { provider: string; model: string }[] }) =>
    request<{ id: string }>("POST", "/chains", input),
  updateChain: (id: string, patch: { name?: string; strategy?: string; fallback_provider?: string; fallback_model?: string; steps?: { provider: string; model: string }[] }) =>
    request<{ id: string }>("PATCH", `/chains/${id}`, patch),
  deleteChain: (id: string) => request<void>("DELETE", `/chains/${id}`),

  listBudgets: () => request<{ budgets: Budget[] }>("GET", "/budgets"),
  budgetStatus: () => request<{ budgets: BudgetStatus[] }>("GET", "/budgets/status"),
  createBudget: (input: { scope_kind?: string; scope_id?: string; limit_usd?: number; limit_tokens?: number; period?: string; alert_pct?: number; hard_cutoff?: boolean }) =>
    request<{ id: string }>("POST", "/budgets", input),
  updateBudget: (id: string, patch: { limit_usd?: number; limit_tokens?: number; period?: string; alert_pct?: number; hard_cutoff?: boolean }) =>
    request<void>("PATCH", `/budgets/${id}`, patch),
  deleteBudget: (id: string) => request<void>("DELETE", `/budgets/${id}`),

  usage: (period: string) => request<UsageSummary>("GET", `/usage?period=${period}&tz=${browserTZ()}`),
  usageInsights: (period: string) =>
    request<UsageInsights>("GET", `/usage/insights?period=${period}&tz=${browserTZ()}`),
  modelUsage: (period: string) =>
    request<{ models: ModelUsage[] }>("GET", `/usage/models?period=${period}&tz=${browserTZ()}`),

  quota: (period: string) =>
    request<{ accounts: QuotaAccount[]; since: string }>("GET", `/quota?period=${period}&tz=${browserTZ()}`),

  consoleLog: () => request<{ logs: string[] }>("GET", "/console"),

  cliTools: (model?: string) =>
    request<CLIToolsResponse>("GET", model ? `/cli-tools?model=${encodeURIComponent(model)}` : "/cli-tools"),
  cliToolConfigure: (toolId: string, body: { base_url: string; api_key: string; models?: string[] }) =>
    request<{ ok: boolean }>("POST", `/cli-tools/${toolId}/configure`, body),
  cliToolRemove: (toolId: string) =>
    request<{ ok: boolean }>("POST", `/cli-tools/${toolId}/remove`),

  listProxyPools: () => request<{ pools: ProxyPool[] }>("GET", "/proxy-pools"),
  createProxyPool: (input: { name: string; type?: string; proxy_url: string; no_proxy?: string; strict?: boolean; is_active?: boolean }) =>
    request<{ id: string }>("POST", "/proxy-pools", input),
  updateProxyPool: (id: string, patch: { name?: string; proxy_url?: string; no_proxy?: string; strict?: boolean; is_active?: boolean }) =>
    request<void>("PATCH", `/proxy-pools/${id}`, patch),
  deleteProxyPool: (id: string) => request<void>("DELETE", `/proxy-pools/${id}`),

  listSkills: () => request<{ skills: Skill[] }>("GET", "/skills"),
  createSkill: (input: { name: string; description?: string; prompt?: string; enabled?: boolean }) =>
    request<Skill>("POST", "/skills", input),
  updateSkill: (id: string, patch: { enabled?: boolean }) =>
    request<void>("POST", `/skills/${id}`, patch),
  deleteSkill: (id: string) => request<void>("DELETE", `/skills/${id}`),

  endpointSettings: () => request<EndpointSettings>("GET", "/settings/endpoint"),
  updateEndpointSettings: (patch: Partial<EndpointSettings>) =>
    request<EndpointSettings>("POST", "/settings/endpoint", patch),

  accessSettings: () => request<AccessSettings>("GET", "/settings/access"),
  updateAccessSettings: (patch: Partial<Omit<AccessSettings, "endpoint_url">>) =>
    request<AccessSettings>("POST", "/settings/access", patch),

  // Tunnel management.
  tunnelStatus: () => request<TunnelCombinedStatus>("GET", "/tunnel/status"),
  tunnelEnable: () => request<TunnelEnableResult>("POST", "/tunnel/enable"),
  tunnelDisable: () => request<{ success: boolean }>("POST", "/tunnel/disable"),
  tailscaleCheck: () => request<TailscaleCheckResult>("GET", "/tunnel/tailscale-check"),
  tailscaleEnable: (sudoPassword?: string) =>
    request<TailscaleEnableResult>("POST", "/tunnel/tailscale-enable", sudoPassword ? { sudoPassword } : {}),
  tailscaleDisable: () => request<{ success: boolean }>("POST", "/tunnel/tailscale-disable"),

  // Model management.
  listDisabledModels: (providerAlias: string) =>
    request<{ ids: string[] }>("GET", `/models/disabled?provider=${encodeURIComponent(providerAlias)}`),
  disableModels: (providerAlias: string, ids: string[]) =>
    request<{ ids: string[] }>("POST", "/models/disabled", { providerAlias, ids }),
  enableModels: (providerAlias: string, ids: string[]) =>
    request<{ ids: string[] }>("DELETE", "/models/disabled", { providerAlias, ids }),

  // Database export/import.
  exportDatabase: () => request<Record<string, unknown>>("GET", "/settings/database"),
  importDatabase: (payload: Record<string, unknown>) =>
    request<{ imported: number }>("POST", "/settings/database", payload),

  // Proxy test.
  testProxy: (proxyUrl: string) =>
    request<{ ok: boolean; status?: number; elapsedMs?: number; error?: string }>("POST", "/settings/proxy-test", { proxyUrl }),

  // Proxy pool test.
  testProxyPool: (id: string) =>
    request<{ status: string; last_tested?: string }>("POST", `/proxy-pools/${id}/test`),

  // OAuth provider connections.
  oauthProviders: () => request<{ providers: OAuthProvider[] }>("GET", "/oauth/providers"),
  oauthAuthorize: (provider: string, redirectUri: string) =>
    request<{ authorize_url: string; state: string; redirect_uri?: string }>("POST", `/oauth/${provider}/authorize`, {
      redirect_uri: redirectUri,
    }),
  oauthExchange: (provider: string, input: { code: string; state: string; label?: string }) =>
    request<{ id: string; provider: string; email: string }>("POST", `/oauth/${provider}/exchange`, input),
  oauthDeviceCode: (provider: string) =>
    request<DeviceCode>("POST", `/oauth/${provider}/device-code`, {}),
  oauthPoll: (provider: string, deviceCode: string, label?: string) =>
    request<OAuthPollResult>("POST", `/oauth/${provider}/poll`, { device_code: deviceCode, label }),

  // Kiro connect flow (AWS SSO OIDC device flows + import token). Mounted under
  // /kiro (not /oauth/kiro) to avoid the chi /oauth/{provider} route collision.
  kiroDeviceStart: (input: { method: "builder-id" | "idc"; start_url?: string; region?: string }) =>
    request<DeviceCode>("POST", "/kiro/device-start", input),
  kiroDevicePoll: (deviceCode: string, label?: string) =>
    request<OAuthPollResult>("POST", "/kiro/device-poll", { device_code: deviceCode, label }),
  kiroImport: (refreshToken: string, label?: string) =>
    request<{ id: string; provider: string }>("POST", "/kiro/import", {
      refresh_token: refreshToken,
      label,
    }),
};

// ---- SSE usage stream --------------------------------------------------------

export interface UsageEvent {
  provider: string;
  model: string;
  account_id: string;
  tokens: number;
}

/**
 * Creates an EventSource connected to the usage SSE stream. The caller
 * provides a callback that fires on each usage event. Returns a cleanup
 * function that closes the connection.
 */
export function connectUsageStream(onEvent: (ev: UsageEvent) => void): () => void {
  let es: EventSource | null = null;
  let retryCount = 0;
  const maxRetries = 10;
  let closed = false;

  function connect() {
    if (closed) return;
    es = new EventSource("/api/usage/stream");
    
    es.onopen = () => {
      retryCount = 0; // reset on successful connection
    };
    
    es.onmessage = (msg) => {
      try {
        const ev = JSON.parse(msg.data) as UsageEvent;
        onEvent(ev);
      } catch { /* ignore malformed events */ }
    };
    
    es.onerror = () => {
      es?.close();
      if (closed) return;
      
      if (retryCount < maxRetries) {
        const delay = Math.min(1000 * 2 ** retryCount, 30000);
        setTimeout(connect, delay);
        retryCount++;
      }
    };
  }

  connect();

  return () => {
    closed = true;
    es?.close();
  };
}

export { APIError };
