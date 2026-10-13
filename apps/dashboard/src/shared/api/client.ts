export type Tenant = {
  tenant_id: string;
  name: string;
  environment_id: string;
  environment_name: string;
  status: 'active' | 'suspended' | string;
};

export type TenantCreation = Tenant & { integration_api_key: string };

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

export type Rule = {
  id: string;
  tenant_id: string;
  environment_id: string;
  name: string;
  description: string;
  event_type: string;
  conditions: Record<string, unknown>;
  action: Record<string, unknown>;
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
  action: Record<string, unknown>;
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

type ApiErrorPayload = { error?: { code?: string; message?: string } };

const adminKey = import.meta.env.VITE_MOTIF_ADMIN_KEY ?? 'admin_dev_only_change_me';
const integrationKey = import.meta.env.VITE_MOTIF_API_KEY ?? 'dev_only_change_me';
let adminSessionToken = typeof window === 'undefined' ? '' : window.localStorage.getItem('motif.admin.session') ?? '';

function adminHeaders(extra: Record<string, string> = {}): Record<string, string> {
  return adminSessionToken ? { ...extra, Authorization: `Bearer ${adminSessionToken}` } : { ...extra, 'X-Motif-Admin-Key': adminKey };
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

export async function fetchAdminPrincipal(): Promise<AdminPrincipal> {
  const response = await fetch('/api/v1/admin/auth/me', { headers: adminHeaders() });
  return readResponse<AdminPrincipal>(response);
}

export async function logoutAdmin(): Promise<void> {
  await fetch('/api/v1/admin/auth/logout', { method: 'POST', headers: adminHeaders() });
  adminSessionToken = '';
  window.localStorage.removeItem('motif.admin.session');
}

export async function fetchTenants(): Promise<Tenant[]> {
  const response = await fetch('/api/v1/admin/tenants', {
    headers: adminHeaders(),
  });
  return readResponse<Tenant[]>(response);
}

export async function fetchPlans(): Promise<AdminPlan[]> {
  const response = await fetch('/api/v1/admin/plans', { headers: adminHeaders() });
  return readResponse<AdminPlan[]>(response);
}

export async function fetchTenantUsage(tenantId: string): Promise<TenantUsage> {
  const response = await fetch(`/api/v1/admin/usage?tenant_id=${encodeURIComponent(tenantId)}`, { headers: adminHeaders() });
  return readResponse<TenantUsage>(response);
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

export async function fetchRuleVersions(ruleId: string): Promise<RuleVersion[]> {
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions`, {
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
  action: Record<string, unknown>;
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
  event_type: string;
  conditions: Record<string, unknown>;
  action: Record<string, unknown>;
  starts_at?: string;
  ends_at?: string;
}): Promise<Rule> {
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions`, {
    method: 'POST',
    headers: adminHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(input),
  });
  return readResponse<Rule>(response);
}

export async function publishRule(id: string): Promise<Rule> {
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(id)}/publish`, {
    method: 'POST',
    headers: adminHeaders(),
  });
  return readResponse<Rule>(response);
}

export async function publishRuleVersion(ruleId: string, version: number): Promise<Rule> {
  const response = await fetch(`/api/v1/admin/rules/${encodeURIComponent(ruleId)}/versions/${version}/publish`, {
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

export async function fetchEventStatus(externalId: string): Promise<EventReceipt> {
  const response = await fetch(`/api/v1/events/${encodeURIComponent(externalId)}`, {
    headers: { 'X-Motif-API-Key': integrationKey },
  });
  return readResponse<EventReceipt>(response);
}

export function hasConfiguredKeys() {
  return Boolean(import.meta.env.VITE_MOTIF_ADMIN_KEY && import.meta.env.VITE_MOTIF_API_KEY);
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
