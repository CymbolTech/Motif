import { adminHeaders as buildAdminHeaders } from '../auth/session';

export type Tenant = {
  tenant_id: string;
  name: string;
  environment_id: string;
  environment_name: string;
  status: 'active' | 'suspended' | string;
};

export type TenantCreation = Tenant & { integration_api_key: string };

export type Environment = {
  tenant_id: string;
  environment_id: string;
  name: string;
  created_at: string;
  integration_api_key?: string;
};

export type AuditEntry = {
  id: number;
  tenant_id?: string;
  environment_id?: string;
  actor_type: string;
  actor_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  metadata: Record<string, unknown>;
  created_at: string;
};

export type Credential = {
  id: number;
  tenant_id: string;
  environment_id: string;
  label: string;
  created_at: string;
  last_used_at?: string;
  revoked_at?: string;
  active: boolean;
  integration_api_key?: string;
};

export type EventType = {
  tenant_id: string;
  environment_id: string;
  name: string;
  schema: Record<string, unknown>;
  version: number;
  active?: boolean;
  created_at?: string;
};

export type EventRecord = {
  event_id: number;
  tenant_id: string;
  environment_id: string;
  external_event_id: string;
  event_type: string;
  status: 'accepted' | 'processing' | 'processed' | 'failed' | string;
  received_at: string;
  processed_at?: string;
};

export type RuleAction = {
  type: string;
  currency?: string;
  amount?: number;
  message?: string;
};

export type Rule = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  description: string;
  event_type: string;
  conditions: Record<string, unknown>;
  action: RuleAction | RuleAction[];
  status: 'draft' | 'published' | 'archived' | string;
  version: number;
  created_at: string;
  updated_at: string;
  published_at?: string;
  starts_at?: string;
  ends_at?: string;
};

export type RuleVersion = {
  id: string;
  rule_id: string;
  version: number;
  event_type: string;
  conditions: Record<string, unknown>;
  action: RuleAction | RuleAction[];
  status: 'draft' | 'published' | 'archived' | string;
  created_at: string;
  published_at?: string;
  starts_at?: string;
  ends_at?: string;
};

export type RuleExecution = {
  id: number;
  rule_id: string;
  rule_name: string;
  event_id: number;
  event_type: string;
  action_type: string;
  status: string;
  attempts?: number;
  last_error?: string;
  rule_version_id?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
};

export type Diagnostics = {
  event_counts: Record<string, number>;
  pending_outbox: number;
  failed_rule_executions: number;
  retryable_rule_executions: number;
  active_rules: number;
  failed_redemptions: number;
  generated_at: string;
};

export type EventReceipt = {
  event_id: number;
  external_event_id: string;
  status: string;
  schema_version?: number;
};

export type AdminPrincipal = {
  id: string;
  email: string;
  role: 'super_admin' | 'tenant_admin' | string;
  tenant_ids?: string[];
};

export type TenantSignupInput = {
  email: string;
  password: string;
  tenant_id: string;
  name: string;
  environment_id?: string;
  environment_name?: string;
};

export type TenantSignupResponse = {
  token: string;
  expires_in: number;
  admin: AdminPrincipal;
  tenant: Tenant;
  integration_key: string;
};

export type AdminUser = {
  id: string;
  email: string;
  role: 'super_admin' | 'tenant_admin' | string;
  active: boolean;
  created_at: string;
  tenant_ids?: string[];
};

export type AdminPlan = {
  id: string;
  name: string;
  description: string;
  billing_interval: 'manual' | 'monthly' | 'yearly' | string;
  price_cents: number;
  entitlements: Record<string, number>;
  active: boolean;
  updated_at?: string;
};

export type TenantUsage = {
  tenant_id: string;
  usage: Record<string, number>;
  limits: Record<string, number>;
  generated_at: string;
};

export type TenantPlan = {
  tenant_id: string;
  plan_id: string;
  plan_name: string;
  entitlements: Record<string, number>;
  overrides: Record<string, number>;
  effective: Record<string, number>;
  assigned_at: string;
  updated_at: string;
};

export type RewardProviderConfig = {
  tenant_id: string;
  environment_id: string;
  provider: 'mock' | 'http' | string;
  delivery_url: string;
  status_url?: string;
  token_configured: boolean;
  timeout_seconds: number;
  active: boolean;
};

export type EventTypeVersion = {
  tenant_id: string;
  environment_id: string;
  name: string;
  version: number;
  schema: Record<string, unknown>;
  created_at: string;
};

export type RedemptionAttempt = {
  id: number;
  redemption_id: string;
  kind: string;
  status: string;
  provider_state?: string;
  provider_reference?: string;
  error_message?: string;
  created_at: string;
  completed_at?: string;
};

export type Goal = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  description: string;
  event_type: string;
  target_count: number;
  active: boolean;
  created_at: string;
};

export type Campaign = {
  id: string;
  tenant_id: string;
  environment_id: string;
  goal_id?: string;
  name: string;
  description: string;
  event_type: string;
  starts_at?: string;
  ends_at?: string;
  active: boolean;
  created_at: string;
};

export type CampaignAnalytics = {
  campaign_id: string;
  name: string;
  event_type: string;
  starts_at?: string;
  ends_at?: string;
  target_count: number;
  event_count: number;
  unique_users: number;
};

export type MetricDefinition = {
  tenant_id: string;
  environment_id: string;
  id: string;
  name: string;
  description: string;
  event_type: string;
  aggregation: 'count' | 'unique_users' | string;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type ReportWidgetConfig = {
  id: string;
  measure: string;
  title: string;
  resource_id?: string;
  event_type?: string;
  interval?: 'day' | 'week';
  size?: 'standard' | 'wide';
};

export type ReportDashboard = {
  tenant_id: string;
  environment_id: string;
  widgets: ReportWidgetConfig[];
  updated_at: string;
};

export type ReportQueryResponse = {
  tenant_id: string;
  environment_id: string;
  measure: string;
  resource_id?: string;
  event_type?: string;
  from: string;
  to: string;
  interval: 'day' | 'week';
  total: number;
  points: Array<{ at: string; value: number }>;
  generated_at: string;
};

export type FunnelReport = {
  tenant_id: string;
  environment_id: string;
  from?: string;
  to?: string;
  steps: Array<{ name: string; event_type: string; users: number; conversion_rate: number }>;
  generated_at: string;
};

export type RetentionReport = {
  tenant_id: string;
  environment_id: string;
  cohort_event_type: string;
  return_event_type: string;
  from?: string;
  to?: string;
  cohort_users: number;
  intervals: Array<{ days: number; returning_users: number; rate: number }>;
  generated_at: string;
};

export type SegmentReport = {
  tenant_id: string;
  environment_id: string;
  event_type: string;
  min_events: number;
  users: number;
  events: number;
  from?: string;
  to?: string;
  generated_at: string;
};

export type ExperimentReport = {
  tenant_id: string;
  environment_id: string;
  id: string;
  conversion_event_type: string;
  variants: Array<{ id: string; weight: number; assigned_users: number; converted_users: number; conversion_rate: number }>;
  from?: string;
  to?: string;
  generated_at: string;
};

export type HealthState = 'healthy' | 'degraded' | 'checking';

export type PointBalance = {
  currency_id: string;
  currency: string;
  name: string;
  unit_scale: number;
  balance: number;
  account_id?: number;
};

export type PointTransaction = {
  id: number;
  account_id: number;
  currency_id: string;
  currency_code: string;
  external_user_id: string;
  entry_type: string;
  amount: number;
  balance_after: number;
  idempotency_key: string;
  provenance: Record<string, unknown>;
  created_at: string;
};

export type UserProgression = {
  user_id: string;
  total_xp: number;
  level: number;
  level_name: string;
  next_level_xp?: number | null;
};

export type AdminRedemption = {
  id: string;
  reward_id: string;
  reward_name: string;
  tenant_id: string;
  environment_id: string;
  external_user_id: string;
  status: string;
  idempotency_key: string;
  provider_reference?: string;
  failure_reason?: string;
  attempt_count?: number;
  reconciliation_attempts?: number;
  next_reconciliation_at?: string;
  last_provider_check_error?: string;
  created_at: string;
  updated_at: string;
};

export type Reward = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  reward_type: 'digital_unlock' | string;
  description: string;
  cost_currency?: string;
  cost_amount?: number;
  payload: Record<string, unknown>;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type Currency = {
  id: string;
  tenant_id: string;
  environment_id: string;
  code: string;
  name: string;
  unit_scale: number;
  policy_version: number;
  policy: Record<string, unknown>;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type Mission = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  event_type: string;
  target_count: number;
  active: boolean;
  created_at: string;
};

export type Achievement = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  description: string;
  event_type: string;
  required_count: number;
  active: boolean;
  created_at: string;
};

export type Level = {
  id: string;
  tenant_id: string;
  environment_id: string;
  level: number;
  name: string;
  xp_threshold: number;
  active: boolean;
  created_at: string;
};

export type Redemption = {
  id: string;
  reward_id: string;
  external_user_id: string;
  status: 'requested' | 'reserved' | 'delivering' | 'fulfilled' | 'failed' | 'cancelled' | string;
  idempotency_key: string;
  provider_reference?: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
};

export type UserMission = {
  id: string;
  name: string;
  event_type: string;
  target_count: number;
  progress: number;
  completed: boolean;
  completed_at?: string | null;
};

export type UserAchievement = {
  id: string;
  name: string;
  description: string;
  event_type: string;
  required_count: number;
  progress: number;
  unlocked: boolean;
  unlocked_at?: string | null;
};

export type UserSnapshot = {
  user_id: string;
  balances: PointBalance[];
  transactions: PointTransaction[];
  next_cursor?: string;
  missions: UserMission[];
  achievements: UserAchievement[];
  progression: UserProgression;
};

type ApiErrorPayload = { error?: { code?: string; message?: string } };

const integrationKey = import.meta.env.VITE_MOTIF_API_KEY ?? (import.meta.env.DEV ? 'dev_only_change_me' : '');
let adminSessionToken = typeof window === 'undefined' ? '' : window.localStorage.getItem('motif.admin.session') ?? '';

function adminHeaders(extra: Record<string, string> = {}): Record<string, string> {
  return buildAdminHeaders(adminSessionToken, extra);
}

async function readResponse<T>(response: Response): Promise<T> {
  const payload = (await response.json().catch(() => ({}))) as T & ApiErrorPayload;
  if (!response.ok) {
    throw new Error(payload.error?.message ?? `Request failed with ${response.status}`);
  }
  return payload as T;
}

export async function loginAdmin(email: string, password: string): Promise<AdminPrincipal> {
  const response = await fetch('/api/v1/admin/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, password }) });
  const payload = await readResponse<{ token: string; admin: AdminPrincipal }>(response);
  adminSessionToken = payload.token;
  window.localStorage.setItem('motif.admin.session', adminSessionToken);
  return payload.admin;
}

export async function signupTenant(input: TenantSignupInput): Promise<TenantSignupResponse> {
  const response = await fetch('/api/v1/admin/auth/signup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      ...input,
      environment_id: input.environment_id ?? 'production',
      environment_name: input.environment_name ?? 'Production',
    }),
  });
  const payload = await readResponse<TenantSignupResponse>(response);
  adminSessionToken = payload.token;
  window.localStorage.setItem('motif.admin.session', adminSessionToken);
  return payload;
}

export function clearAdminSession(): void {
  adminSessionToken = '';
  if (typeof window !== 'undefined') window.localStorage.removeItem('motif.admin.session');
}

export async function fetchAdminPrincipal(): Promise<AdminPrincipal> {
  const response = await fetch('/api/v1/admin/auth/me', { headers: adminHeaders() });
  return readResponse<AdminPrincipal>(response);
}

export async function logoutAdmin(): Promise<void> {
  await fetch('/api/v1/admin/auth/logout', { method: 'POST', headers: adminHeaders() });
  clearAdminSession();
}

export async function fetchTenants(): Promise<Tenant[]> {
  const response = await fetch('/api/v1/admin/tenants', {
    headers: adminHeaders(),
  });
  return readResponse<Tenant[]>(response);
}

export async function fetchEnvironments(tenantId: string): Promise<Environment[]> {
  const response = await fetch(`/api/v1/admin/environments?tenant_id=${encodeURIComponent(tenantId)}`, { headers: adminHeaders() });
  return readResponse<Environment[]>(response);
}

export async function createEnvironment(input: { tenant_id: string; environment_id: string; name: string }): Promise<Environment> {
  const response = await fetch('/api/v1/admin/environments', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<Environment>(response);
}

export async function fetchPlans(): Promise<AdminPlan[]> {
  const response = await fetch('/api/v1/admin/plans', { headers: adminHeaders() });
  return readResponse<AdminPlan[]>(response);
}

export async function fetchTenantUsage(tenantId: string): Promise<TenantUsage> {
  const response = await fetch(`/api/v1/admin/usage?tenant_id=${encodeURIComponent(tenantId)}`, { headers: adminHeaders() });
  return readResponse<TenantUsage>(response);
}

export async function fetchTenantPlans(tenantId?: string): Promise<TenantPlan[]> {
  const query = tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : '';
  const response = await fetch(`/api/v1/admin/tenant-plans${query}`, { headers: adminHeaders() });
  return readResponse<TenantPlan[]>(response);
}

export async function assignTenantPlan(input: { tenant_id: string; plan_id: string; overrides?: Record<string, number> }): Promise<{ tenant_id: string; plan_id: string; overrides: Record<string, number> }> {
  const response = await fetch('/api/v1/admin/tenant-plans', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<{ tenant_id: string; plan_id: string; overrides: Record<string, number> }>(response);
}

export async function archivePlan(planId: string): Promise<{ id: string; active: boolean }> {
  const response = await fetch(`/api/v1/admin/plans/${encodeURIComponent(planId)}/archive`, { method: 'POST', headers: adminHeaders() });
  return readResponse<{ id: string; active: boolean }>(response);
}

export async function createPlan(input: { id: string; name: string; description: string; billing_interval: string; price_cents: number; entitlements: Record<string, number> }): Promise<AdminPlan> {
  const response = await fetch('/api/v1/admin/plans', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<AdminPlan>(response);
}

export async function updatePlan(planId: string, input: { name: string; description: string; billing_interval: string; price_cents: number; entitlements: Record<string, number> }): Promise<AdminPlan> {
  const response = await fetch(`/api/v1/admin/plans/${encodeURIComponent(planId)}`, { method: 'PATCH', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify({ ...input, id: planId }) });
  return readResponse<AdminPlan>(response);
}

export async function fetchAdminUsers(tenantId?: string): Promise<AdminUser[]> {
  const query = tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : '';
  const response = await fetch(`/api/v1/admin/admin-users${query}`, { headers: adminHeaders() });
  return readResponse<AdminUser[]>(response);
}

export async function createAdminUser(input: { email: string; password: string; role: string; tenant_ids?: string[] }): Promise<AdminUser> {
  const response = await fetch('/api/v1/admin/admin-users', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<AdminUser>(response);
}

export async function fetchRewardProviders(filters?: { tenant_id?: string; environment_id?: string }): Promise<RewardProviderConfig[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/reward-providers${query ? `?${query}` : ''}`, { headers: adminHeaders() });
  return readResponse<RewardProviderConfig[]>(response);
}

export async function fetchGoals(filters?: { tenant_id?: string; environment_id?: string }): Promise<Goal[]> { return fetchAdminCollection<Goal>('goals', filters); }
export async function fetchCampaigns(filters?: { tenant_id?: string; environment_id?: string }): Promise<Campaign[]> { return fetchAdminCollection<Campaign>('campaigns', filters); }
export async function createGoal(input: { tenant_id: string; environment_id: string; id: string; name: string; description?: string; event_type: string; target_count: number }): Promise<Goal> { return createAdminResource<Goal>('goals', input); }
export async function createCampaign(input: { tenant_id: string; environment_id: string; id: string; goal_id?: string; name: string; description?: string; event_type: string; starts_at?: string; ends_at?: string }): Promise<Campaign> { return createAdminResource<Campaign>('campaigns', input); }
export async function fetchCampaignAnalytics(scope: { tenant_id: string; environment_id: string }): Promise<{ tenant_id: string; environment_id: string; campaigns: CampaignAnalytics[]; generated_at: string }> {
  const params = new URLSearchParams(scope);
  const response = await fetch(`/api/v1/admin/analytics?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<{ tenant_id: string; environment_id: string; campaigns: CampaignAnalytics[]; generated_at: string }>(response);
}

export async function fetchMetrics(scope: { tenant_id: string; environment_id: string }): Promise<MetricDefinition[]> {
  const params = new URLSearchParams(scope);
  const response = await fetch(`/api/v1/admin/metrics?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<MetricDefinition[]>(response);
}

export async function fetchReportDashboard(scope: { tenant_id: string; environment_id: string }): Promise<ReportDashboard> {
  const params = new URLSearchParams(scope);
  const response = await fetch(`/api/v1/admin/reports/dashboard?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<ReportDashboard>(response);
}

export async function saveReportDashboard(input: { tenant_id: string; environment_id: string; widgets: ReportWidgetConfig[] }): Promise<ReportDashboard> {
  const response = await fetch('/api/v1/admin/reports/dashboard', { method: 'PUT', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<ReportDashboard>(response);
}

export async function fetchReportQuery(input: { tenant_id: string; environment_id: string; measure: string; resource_id?: string; event_type?: string; from?: string; to?: string; interval?: 'day' | 'week' }): Promise<ReportQueryResponse> {
  const params = new URLSearchParams({ tenant_id: input.tenant_id, environment_id: input.environment_id, measure: input.measure });
  if (input.resource_id) params.set('resource_id', input.resource_id);
  if (input.event_type) params.set('event_type', input.event_type);
  if (input.from) params.set('from', input.from);
  if (input.to) params.set('to', input.to);
  if (input.interval) params.set('interval', input.interval);
  const response = await fetch(`/api/v1/admin/analytics/query?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<ReportQueryResponse>(response);
}

export async function createMetric(input: { tenant_id: string; environment_id: string; id: string; name: string; description?: string; event_type: string; aggregation: 'count' | 'unique_users' }): Promise<MetricDefinition> {
  const response = await fetch('/api/v1/admin/metrics', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<MetricDefinition>(response);
}

export async function archiveMetric(scope: { tenant_id: string; environment_id: string }, metricId: string): Promise<{ tenant_id: string; environment_id: string; id: string; active: boolean }> {
  const params = new URLSearchParams({ tenant_id: scope.tenant_id, environment_id: scope.environment_id });
  const response = await fetch(`/api/v1/admin/metrics/${encodeURIComponent(metricId)}/archive?${params.toString()}`, { method: 'POST', headers: adminHeaders() });
  return readResponse<{ tenant_id: string; environment_id: string; id: string; active: boolean }>(response);
}

export async function fetchFunnelAnalytics(input: { tenant_id: string; environment_id: string; steps: Array<{ name: string; event_type: string }>; from?: string; to?: string }): Promise<FunnelReport> {
  const response = await fetch('/api/v1/admin/analytics/funnel', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<FunnelReport>(response);
}

export async function fetchRetentionAnalytics(input: { tenant_id: string; environment_id: string; cohort_event_type: string; return_event_type: string; intervals?: number[]; from?: string; to?: string }): Promise<RetentionReport> {
  const response = await fetch('/api/v1/admin/analytics/retention', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<RetentionReport>(response);
}

export async function fetchSegmentAnalytics(input: { tenant_id: string; environment_id: string; event_type: string; min_events: number; from?: string; to?: string }): Promise<SegmentReport> {
  const response = await fetch('/api/v1/admin/analytics/segment', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<SegmentReport>(response);
}

export async function fetchExperimentAnalytics(input: { tenant_id: string; environment_id: string; id: string; conversion_event_type: string; variants: Array<{ id: string; weight: number }>; from?: string; to?: string }): Promise<ExperimentReport> {
  const response = await fetch('/api/v1/admin/analytics/experiment', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<ExperimentReport>(response);
}

export async function saveRewardProvider(input: { tenant_id: string; environment_id: string; provider: string; delivery_url?: string; status_url?: string; token?: string; timeout_seconds: number; active: boolean }): Promise<RewardProviderConfig> {
  const response = await fetch('/api/v1/admin/reward-providers', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<RewardProviderConfig>(response);
}

export async function createTenant(input: {
  tenant_id: string;
  name: string;
  environment_id: string;
  environment_name: string;
}): Promise<TenantCreation> {
  const response = await fetch('/api/v1/admin/tenants', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<TenantCreation>(response);
}

export async function updateTenantStatus(tenantId: string, status: 'active' | 'suspended'): Promise<{ tenant_id: string; status: string }> {
  const response = await fetch(`/api/v1/admin/tenants/${encodeURIComponent(tenantId)}/status`, {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ status }),
  });
  return readResponse<{ tenant_id: string; status: string }>(response);
}

export async function fetchAuditLog(filters?: { tenant_id?: string; environment_id?: string; limit?: number }): Promise<AuditEntry[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  if (filters?.limit) params.set('limit', String(filters.limit));
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/audit-log${query ? `?${query}` : ''}`, { headers: adminHeaders() });
  return readResponse<AuditEntry[]>(response);
}

export type AccessTokenScope = 'read' | 'write' | 'publish';

export type AccessToken = {
  id: string;
  tenant_id: string;
  name: string;
  hint: string;
  scopes: AccessTokenScope[];
  environment_ids: string[];
  created_by: string;
  expires_at?: string;
  last_used_at?: string;
  revoked_at?: string;
  created_at: string;
};

export async function fetchAccessTokens(tenantId: string): Promise<AccessToken[]> {
  const response = await fetch(`/api/v1/admin/access-tokens?tenant_id=${encodeURIComponent(tenantId)}`, { headers: adminHeaders() });
  return readResponse<AccessToken[]>(response);
}

export async function createAccessToken(input: { tenant_id: string; name: string; scopes: AccessTokenScope[]; environment_ids: string[]; expires_in_days: number }): Promise<{ token: string; access_token: AccessToken }> {
  const response = await fetch('/api/v1/admin/access-tokens', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<{ token: string; access_token: AccessToken }>(response);
}

export async function revokeAccessToken(tenantId: string, id: string): Promise<AccessToken> {
  const response = await fetch(`/api/v1/admin/access-tokens/${encodeURIComponent(id)}/revoke?tenant_id=${encodeURIComponent(tenantId)}`, { method: 'POST', headers: adminHeaders() });
  return readResponse<AccessToken>(response);
}

export async function fetchCredentials(filters?: { tenant_id?: string; environment_id?: string }): Promise<Credential[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/credentials${query ? `?${query}` : ''}`, { headers: adminHeaders() });
  return readResponse<Credential[]>(response);
}

export async function createCredential(input: { tenant_id: string; environment_id: string; label: string }): Promise<Credential> {
  const response = await fetch('/api/v1/admin/credentials', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<Credential>(response);
}

export async function rotateCredential(id: number): Promise<Credential> {
  const response = await fetch(`/api/v1/admin/credentials/${id}/rotate`, { method: 'POST', headers: adminHeaders() });
  return readResponse<Credential>(response);
}

export async function revokeCredential(id: number): Promise<Credential> {
  const response = await fetch(`/api/v1/admin/credentials/${id}/revoke`, { method: 'POST', headers: adminHeaders() });
  return readResponse<Credential>(response);
}

export async function fetchEventTypes(filters?: { tenant_id?: string; environment_id?: string }): Promise<EventType[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/event-types${query ? `?${query}` : ''}`, {
    headers: adminHeaders(),
  });
  return readResponse<EventType[]>(response);
}

export async function fetchEventTypeVersions(name: string, scope: { tenant_id: string; environment_id: string }): Promise<EventTypeVersion[]> {
  const params = new URLSearchParams({ tenant_id: scope.tenant_id, environment_id: scope.environment_id });
  const response = await fetch(`/api/v1/admin/event-types/${encodeURIComponent(name)}/versions?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<EventTypeVersion[]>(response);
}

export async function fetchEvents(limit = 8, filters?: { tenant_id?: string; environment_id?: string; status?: string; offset?: number }): Promise<EventRecord[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  if (filters?.status) params.set('status', filters.status);
  if (filters?.offset) params.set('offset', String(filters.offset));
  const response = await fetch(`/api/v1/admin/events?${params.toString()}`, {
    headers: adminHeaders(),
  });
  return readResponse<EventRecord[]>(response);
}

export async function fetchDiagnostics(filters?: { tenant_id?: string; environment_id?: string }): Promise<Diagnostics> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/diagnostics${query ? `?${query}` : ''}`, {
    headers: adminHeaders(),
  });
  return readResponse<Diagnostics>(response);
}

export async function createEventType(input: {
  tenant_id: string;
  environment_id: string;
  name: string;
  schema: Record<string, unknown>;
}): Promise<EventType> {
  const response = await fetch('/api/v1/admin/event-types', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<EventType>(response);
}

export async function fetchRules(filters?: { tenant_id?: string; environment_id?: string }): Promise<Rule[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/rules${query ? `?${query}` : ''}`, {
    headers: adminHeaders(),
  });
  return readResponse<Rule[]>(response);
}

export async function fetchRuleVersions(ruleId: string, scope?: { tenant_id: string; environment_id: string }): Promise<RuleVersion[]> {
  const params = scope ? `?${new URLSearchParams(scope).toString()}` : '';
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions${params}`, {
    headers: adminHeaders(),
  });
  return readResponse<RuleVersion[]>(response);
}

export async function createRule(input: {
  tenant_id: string;
  environment_id: string;
  name: string;
  description: string;
  event_type: string;
  conditions: Record<string, unknown>;
  action?: Record<string, unknown>;
  actions?: Array<Record<string, unknown>>;
  starts_at?: string;
  ends_at?: string;
}): Promise<Rule> {
  const response = await fetch('/api/v1/admin/rules', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<Rule>(response);
}

export async function createRuleVersion(ruleId: string, input: {
  tenant_id: string;
  environment_id: string;
  name?: string;
  description?: string;
  event_type: string;
  conditions: Record<string, unknown>;
  action?: Record<string, unknown>;
  actions?: Array<Record<string, unknown>>;
  starts_at?: string;
  ends_at?: string;
}): Promise<Rule> {
  const params = new URLSearchParams({ tenant_id: input.tenant_id, environment_id: input.environment_id });
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions?${params.toString()}`, {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<Rule>(response);
}

export async function publishRule(id: string, scope?: { tenant_id: string; environment_id: string }): Promise<Rule> {
  const params = scope ? `?${new URLSearchParams(scope).toString()}` : '';
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(id)}/publish${params}`, {
    method: 'POST',
    headers: adminHeaders(),
  });
  return readResponse<Rule>(response);
}

export async function publishRuleVersion(ruleId: string, version: number, scope?: { tenant_id: string; environment_id: string }): Promise<Rule> {
  const params = scope ? `?${new URLSearchParams(scope).toString()}` : '';
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions/${version}/publish${params}`, {
    method: 'POST',
    headers: adminHeaders(),
  });
  return readResponse<Rule>(response);
}

export async function fetchRuleExecutions(limit = 8, filters?: { tenant_id?: string; environment_id?: string; status?: string; offset?: number }): Promise<RuleExecution[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  if (filters?.status) params.set('status', filters.status);
  if (filters?.offset) params.set('offset', String(filters.offset));
  const response = await fetch(`/api/v1/admin/rule-executions?${params.toString()}`, {
    headers: adminHeaders(),
  });
  return readResponse<RuleExecution[]>(response);
}

export async function fetchHealth(target: 'api' | 'worker'): Promise<void> {
  const response = await fetch(target === 'api' ? '/api/health/ready' : '/worker/health/ready');
  if (!response.ok) throw new Error(`${target} is not ready`);
}

export async function submitEvent(input: {
  external_event_id: string;
  event_type: string;
  user_id: string;
  payload: Record<string, unknown>;
}): Promise<EventReceipt> {
  const response = await fetch('/api/v1/events', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Motif-API-Key': integrationKey,
    },
    body: JSON.stringify({
      event_id: input.external_event_id,
      event_type: input.event_type,
      user_id: input.user_id,
      payload: input.payload,
    }),
  });
  return readResponse<EventReceipt>(response);
}

export async function submitAdminTestEvent(input: {
  tenant_id: string;
  environment_id: string;
  external_event_id: string;
  event_type: string;
  user_id: string;
  payload: Record<string, unknown>;
}): Promise<EventReceipt> {
  const response = await fetch('/api/v1/admin/events/test', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({
      tenant_id: input.tenant_id,
      environment_id: input.environment_id,
      event_id: input.external_event_id,
      event_type: input.event_type,
      user_id: input.user_id,
      payload: input.payload,
    }),
  });
  return readResponse<EventReceipt>(response);
}

export async function fetchScopedEventStatus(externalId: string, filters: { tenant_id: string; environment_id: string }): Promise<EventReceipt> {
  const events = await fetchEvents(20, filters);
  const event = events.find((item) => item.external_event_id === externalId);
  if (!event) throw new Error('Event status is not available yet.');
  return { event_id: event.event_id, external_event_id: event.external_event_id, status: event.status };
}

export async function fetchEventStatus(externalId: string): Promise<EventReceipt> {
  const response = await fetch(`/api/v1/events/${encodeURIComponent(externalId)}`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<EventReceipt>(response);
}

export function hasConfiguredIntegrationKey() {
  return Boolean(import.meta.env.VITE_MOTIF_API_KEY);
}

export async function fetchUserPoints(userId: string): Promise<{ user_id: string; balances: PointBalance[] }> {
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/points`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<{ user_id: string; balances: PointBalance[] }>(response);
}

export async function fetchUserTransactions(userId: string, limit = 12, cursor?: string): Promise<{ user_id: string; transactions: PointTransaction[]; next_cursor?: string }> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set('cursor', cursor);
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/points/transactions?${params.toString()}`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<{ user_id: string; transactions: PointTransaction[]; next_cursor?: string }>(response);
}

export async function fetchUserProgression(userId: string): Promise<UserProgression> {
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/progression`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<UserProgression>(response);
}

export async function fetchUserMissions(userId: string): Promise<{ user_id: string; missions: UserMission[] }> {
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/missions`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<{ user_id: string; missions: UserMission[] }>(response);
}

export async function fetchUserAchievements(userId: string): Promise<{ user_id: string; achievements: UserAchievement[] }> {
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/achievements`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<{ user_id: string; achievements: UserAchievement[] }>(response);
}

export type UserStreak = { user_id: string; event_type?: string; current_streak: number; longest_streak: number; last_active_date?: string | null };

export async function fetchUserStreaks(userId: string, eventType?: string): Promise<UserStreak> {
  const query = eventType ? `?event_type=${encodeURIComponent(eventType)}` : '';
  const response = await fetch(`/api/v1/users/${encodeURIComponent(userId)}/streaks${query}`, { headers: { 'X-Motif-API-Key': integrationKey } });
  return readResponse<UserStreak>(response);
}

export async function fetchAdminUserProfile(scope: { tenant_id: string; environment_id: string }, userId: string): Promise<UserSnapshot> {
  const params = new URLSearchParams({ tenant_id: scope.tenant_id, environment_id: scope.environment_id, user_id: userId });
  const response = await fetch(`/api/v1/admin/user-profile?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<UserSnapshot>(response);
}

export async function fetchAdminUserTransactions(scope: { tenant_id: string; environment_id: string }, userId: string, limit = 12, cursor?: string): Promise<{ user_id: string; transactions: PointTransaction[]; next_cursor?: string }> {
  const params = new URLSearchParams({ tenant_id: scope.tenant_id, environment_id: scope.environment_id, user_id: userId, limit: String(limit) });
  if (cursor) params.set('cursor', cursor);
  const response = await fetch(`/api/v1/admin/user-transactions?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<{ user_id: string; transactions: PointTransaction[]; next_cursor?: string }>(response);
}

export async function createAdminPointTransaction(input: { tenant_id: string; environment_id: string; external_user_id: string; currency: string; amount: number; type: 'award' | 'spend'; idempotency_key: string; provenance?: Record<string, unknown> }): Promise<PointTransaction> {
  const response = await fetch('/api/v1/admin/points/transactions', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<PointTransaction>(response);
}

export async function createAdminPointConversion(input: { tenant_id: string; environment_id: string; external_user_id: string; source_currency: string; target_currency: string; amount: number; rate_numerator: number; rate_denominator: number; idempotency_key: string }): Promise<Record<string, unknown>> {
  const response = await fetch('/api/v1/admin/points/conversions', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<Record<string, unknown>>(response);
}

export async function fetchRewards(filters?: { tenant_id?: string; environment_id?: string }): Promise<Reward[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/rewards${query ? `?${query}` : ''}`, {
    headers: adminHeaders(),
  });
  return readResponse<Reward[]>(response);
}

async function fetchAdminCollection<T>(path: string, filters?: { tenant_id?: string; environment_id?: string }): Promise<T[]> {
  const params = new URLSearchParams();
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  const query = params.toString();
  const response = await fetch(`/api/v1/admin/${path}${query ? `?${query}` : ''}`, { headers: adminHeaders() });
  return readResponse<T[]>(response);
}

export const fetchCurrencies = (filters?: { tenant_id?: string; environment_id?: string }) => fetchAdminCollection<Currency>('currencies', filters);
export const fetchMissions = (filters?: { tenant_id?: string; environment_id?: string }) => fetchAdminCollection<Mission>('missions', filters);
export const fetchAchievements = (filters?: { tenant_id?: string; environment_id?: string }) => fetchAdminCollection<Achievement>('achievements', filters);
export const fetchLevels = (filters?: { tenant_id?: string; environment_id?: string }) => fetchAdminCollection<Level>('levels', filters);

async function createAdminResource<T>(path: string, input: Record<string, unknown>): Promise<T> {
  const response = await fetch(`/api/v1/admin/${path}`, {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<T>(response);
}

export const createCurrency = (input: { tenant_id: string; environment_id: string; code: string; name: string; unit_scale: number; policy: Record<string, unknown> }) => createAdminResource<Currency>('currencies', input);
export const createMission = (input: { tenant_id: string; environment_id: string; name: string; event_type: string; target_count: number }) => createAdminResource<Mission>('missions', input);
export const createAchievement = (input: { tenant_id: string; environment_id: string; name: string; description: string; event_type: string; required_count: number }) => createAdminResource<Achievement>('achievements', input);
export const createLevel = (input: { tenant_id: string; environment_id: string; level: number; name: string; xp_threshold: number; active?: boolean }) => createAdminResource<Level>('levels', input);
export const createReward = (input: { tenant_id: string; environment_id: string; name: string; reward_type: string; description: string; cost_currency?: string; cost_amount?: number; payload: Record<string, unknown> }) => createAdminResource<Reward>('rewards', input);

async function updateAdminResource<T>(path: string, id: string, input: Record<string, unknown>): Promise<T> {
  const response = await fetch(`/api/v1/admin/${path}/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<T>(response);
}

export const updateCurrency = (id: string, input: { tenant_id: string; environment_id: string; code: string; name: string; unit_scale: number; policy: Record<string, unknown> }) => updateAdminResource<Currency>('currencies', id, input);
export const updateMission = (id: string, input: { tenant_id: string; environment_id: string; name: string; event_type: string; target_count: number }) => updateAdminResource<Mission>('missions', id, input);
export const updateAchievement = (id: string, input: { tenant_id: string; environment_id: string; name: string; description: string; event_type: string; required_count: number }) => updateAdminResource<Achievement>('achievements', id, input);
export const updateLevel = (id: string, input: { tenant_id: string; environment_id: string; level: number; name: string; xp_threshold: number }) => updateAdminResource<Level>('levels', id, input);
export const updateReward = (id: string, input: { tenant_id: string; environment_id: string; name: string; reward_type: string; description: string; cost_currency?: string; cost_amount?: number; payload: Record<string, unknown> }) => updateAdminResource<Reward>('rewards', id, input);
export const updateGoal = (id: string, input: { tenant_id: string; environment_id: string; name: string; description?: string; event_type: string; target_count: number }) => updateAdminResource<Goal>('goals', id, input);
export const updateCampaign = (id: string, input: { tenant_id: string; environment_id: string; goal_id?: string; name: string; description?: string; event_type: string; starts_at?: string; ends_at?: string }) => updateAdminResource<Campaign>('campaigns', id, input);

export async function updateCatalogStatus(input: { tenant_id: string; environment_id: string; resource: 'currency' | 'reward' | 'mission' | 'achievement' | 'level'; id: string; active: boolean }): Promise<{ resource: string; id: string; active: boolean }> {
  const response = await fetch('/api/v1/admin/catalog-status', { method: 'POST', headers: adminHeaders({ 'Content-Type': 'application/json' }), body: JSON.stringify(input) });
  return readResponse<{ resource: string; id: string; active: boolean }>(response);
}

export async function redeemReward(rewardId: string, input: { external_user_id: string; idempotency_key: string }): Promise<Redemption> {
  const response = await fetch(`/api/v1/rewards/${encodeURIComponent(rewardId)}/redemptions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Motif-API-Key': integrationKey,
    },
    body: JSON.stringify(input),
  });
  return readResponse<Redemption>(response);
}

export async function redeemAdminReward(scope: { tenant_id: string; environment_id: string }, rewardId: string, input: { external_user_id: string; idempotency_key: string }): Promise<Redemption> {
  const response = await fetch('/api/v1/admin/redemptions/test', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({
      tenant_id: scope.tenant_id,
      environment_id: scope.environment_id,
      reward_id: rewardId,
      external_user_id: input.external_user_id,
      idempotency_key: input.idempotency_key,
    }),
  });
  return readResponse<Redemption>(response);
}

export async function fetchRedemption(redemptionId: string): Promise<Redemption> {
  const response = await fetch(`/api/v1/redemptions/${encodeURIComponent(redemptionId)}`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<Redemption>(response);
}

export async function retryRedemption(redemptionId: string): Promise<Redemption> {
  const response = await fetch(`/api/v1/redemptions/${encodeURIComponent(redemptionId)}/retry`, {
    method: 'POST',
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<Redemption>(response);
}

export async function retryAdminRedemption(scope: { tenant_id: string; environment_id: string }, redemptionId: string): Promise<Redemption> {
  const response = await fetch('/api/v1/admin/redemptions/retry', {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ tenant_id: scope.tenant_id, environment_id: scope.environment_id, redemption_id: redemptionId }),
  });
  return readResponse<Redemption>(response);
}

export async function fetchAdminRedemptions(limit = 8, filters?: { tenant_id?: string; environment_id?: string; status?: string }): Promise<AdminRedemption[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id);
  if (filters?.environment_id) params.set('environment_id', filters.environment_id);
  if (filters?.status) params.set('status', filters.status);
  const response = await fetch(`/api/v1/admin/redemptions?${params.toString()}`, {
    headers: adminHeaders(),
  });
  return readResponse<AdminRedemption[]>(response);
}

export async function fetchRedemptionAttempts(redemptionId: string, scope: { tenant_id: string; environment_id: string }): Promise<RedemptionAttempt[]> {
  const params = new URLSearchParams({ tenant_id: scope.tenant_id, environment_id: scope.environment_id });
  const response = await fetch(`/api/v1/admin/redemptions/${encodeURIComponent(redemptionId)}/attempts?${params.toString()}`, { headers: adminHeaders() });
  return readResponse<RedemptionAttempt[]>(response);
}
