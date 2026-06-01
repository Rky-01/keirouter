// Typed client for the KeiRouter admin API. All calls go through the dev-server
// proxy (or the embedded static server in production) to /api.

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
}

export interface EndpointSettings {
  rtk_enabled: boolean;
  caveman_enabled: boolean;
  caveman_level: string;
  terse_enabled: boolean;
  terse_level: string;
}

export interface OAuthProvider {
  provider: string;
  display_name: string;
  flow: string; // authorization_code_pkce | authorization_code | device_code
  icon: string;
  color: string;
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
}

export interface CreatedKey {
  id: string;
  name: string;
  key: string;
  display: string;
}

export interface Account {
  id: string;
  provider: string;
  label: string;
  auth_kind: string;
  priority: number;
  disabled: boolean;
  created_at: string;
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
  steps: ChainStep[];
}

export interface Budget {
  id: string;
  scope_kind: string;
  scope_id: string;
  limit_micros: number;
  period: string;
  alert_pct: number;
  hard_cutoff: boolean;
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
    since: string;
  };
  providers: ProviderUsage[];
  recent: RecentActivity[];
  series: SeriesPoint[];
  busiest: string;
}

export interface QuotaAccount {
  id: string;
  provider: string;
  provider_name: string;
  label: string;
  auth_kind: string;
  priority: number;
  status: string; // active | paused | needs_attention
  total_requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  cost_usd: number;
  updated_at: string;
}

export interface ConsoleEntry {
  id: string;
  level: string; // info | warn | error
  provider: string;
  model: string;
  tokens: number;
  cost_usd: number;
  cache_hit: boolean;
  latency_ms: number;
  created_at: string;
}

export interface ProxyPool {
  id: string;
  name: string;
  proxies: string[];
  enabled: boolean;
  created_at: string;
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
  endpoint_url: string;
}

export interface CLITool {
  id: string;
  name: string;
  dialect: string;
  instructions: string;
  snippet: string;
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

  listKeys: () => request<{ keys: APIKey[] }>("GET", "/keys"),
  createKey: (name: string) => request<CreatedKey>("POST", "/keys", { name }),
  deleteKey: (id: string) => request<void>("DELETE", `/keys/${id}`),

  listAccounts: () => request<{ accounts: Account[] }>("GET", "/accounts"),
  createAccount: (input: { provider: string; label: string; api_key: string; base_url?: string }) =>
    request<{ id: string }>("POST", "/accounts", input),
  deleteAccount: (id: string) => request<void>("DELETE", `/accounts/${id}`),

  listChains: () => request<{ chains: Chain[] }>("GET", "/chains"),
  createChain: (input: { name: string; steps: { provider: string; model: string }[] }) =>
    request<{ id: string }>("POST", "/chains", input),
  deleteChain: (id: string) => request<void>("DELETE", `/chains/${id}`),

  listBudgets: () => request<{ budgets: Budget[] }>("GET", "/budgets"),
  createBudget: (input: { scope_kind?: string; limit_usd: number; period?: string }) =>
    request<{ id: string }>("POST", "/budgets", input),
  deleteBudget: (id: string) => request<void>("DELETE", `/budgets/${id}`),

  usage: (period: string) => request<UsageSummary>("GET", `/usage?period=${period}`),
  usageInsights: (period: string) =>
    request<UsageInsights>("GET", `/usage/insights?period=${period}`),

  quota: (period: string) =>
    request<{ accounts: QuotaAccount[]; since: string }>("GET", `/quota?period=${period}`),

  consoleLog: () => request<{ entries: ConsoleEntry[] }>("GET", "/console"),

  cliTools: (model?: string) =>
    request<CLIToolsResponse>("GET", model ? `/cli-tools?model=${encodeURIComponent(model)}` : "/cli-tools"),

  listProxyPools: () => request<{ pools: ProxyPool[] }>("GET", "/proxy-pools"),
  createProxyPool: (input: { name: string; proxies: string[]; enabled?: boolean }) =>
    request<ProxyPool>("POST", "/proxy-pools", input),
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

  // OAuth provider connections.
  oauthProviders: () => request<{ providers: OAuthProvider[] }>("GET", "/oauth/providers"),
  oauthAuthorize: (provider: string, redirectUri: string) =>
    request<{ authorize_url: string; state: string }>("POST", `/oauth/${provider}/authorize`, {
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

export { APIError };