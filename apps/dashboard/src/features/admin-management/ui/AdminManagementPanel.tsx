import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { IconActivity, IconChartBar, IconCloud, IconCreditCard, IconKey, IconPlus, IconUsers } from '@tabler/icons-react';
import {
  archivePlan,
  assignTenantPlan,
  createCampaign,
  createAdminUser,
  createEnvironment,
  createGoal,
  createPlan,
  fetchAdminUsers,
  fetchAuditLog,
  fetchCampaignAnalytics,
  fetchCampaigns,
  fetchEnvironments,
  fetchGoals,
  fetchPlans,
  fetchRewardProviders,
  fetchTenantPlans,
  saveRewardProvider,
  type AdminPlan,
  type AdminPrincipal,
  type AdminUser,
  type AuditEntry,
  type Campaign,
  type CampaignAnalytics,
  type Environment,
  type Goal,
  type RewardProviderConfig,
  type Tenant,
  type TenantPlan,
} from '../../../shared/api/client';
import { MeasurementPanel } from '../../measurement/ui/MeasurementPanel';

type Props = { principal: AdminPrincipal | null; scope?: Tenant };
type Section = 'plans' | 'users' | 'providers' | 'audit' | 'engagement' | 'measurement' | 'environments';

const defaultEntitlements = '{\n  "max_event_payload_bytes": 262144,\n  "max_rules": 100,\n  "max_currencies": 20,\n  "max_rewards": 100\n}';

export function AdminManagementPanel({ principal, scope }: Props) {
  const [section, setSection] = useState<Section>('plans');
  const [plans, setPlans] = useState<AdminPlan[]>([]);
  const [tenantPlans, setTenantPlans] = useState<TenantPlan[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [providers, setProviders] = useState<RewardProviderConfig[]>([]);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [goals, setGoals] = useState<Goal[]>([]);
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [analytics, setAnalytics] = useState<CampaignAnalytics[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [planForm, setPlanForm] = useState({ id: '', name: '', description: '', billing_interval: 'manual', price_cents: '0', entitlements: defaultEntitlements });
  const [userForm, setUserForm] = useState({ email: '', password: '', role: 'tenant_admin', tenant_ids: '' });
  const [providerForm, setProviderForm] = useState({ provider: 'mock', delivery_url: '', status_url: '', token: '', timeout_seconds: '8', active: true });
  const [environmentForm, setEnvironmentForm] = useState({ environment_id: '', name: '' });
  const [revealedEnvironmentKey, setRevealedEnvironmentKey] = useState<string | null>(null);
  const [goalForm, setGoalForm] = useState({ id: '', name: '', description: '', event_type: '', target_count: '1' });
  const [campaignForm, setCampaignForm] = useState({ id: '', name: '', description: '', event_type: '', goal_id: '', starts_at: '', ends_at: '' });

  const isSuperAdmin = principal?.role === 'super_admin';
  const activeProvider = providers.find((provider) => provider.tenant_id === scope?.tenant_id && provider.environment_id === scope?.environment_id);
  const currentTenantPlan = useMemo(() => tenantPlans.find((item) => item.tenant_id === scope?.tenant_id), [scope?.tenant_id, tenantPlans]);

  useEffect(() => {
    if (activeProvider) {
      setProviderForm((current) => ({ ...current, provider: activeProvider.provider, delivery_url: activeProvider.delivery_url, status_url: activeProvider.status_url ?? '', timeout_seconds: String(activeProvider.timeout_seconds), active: activeProvider.active }));
    }
  }, [activeProvider]);

  useEffect(() => {
    if (!principal) return;
    setError(null);
    setMessage(null);
    setLoading(true);
    const load = async () => {
      if (section === 'plans') {
        const [nextPlans, nextTenantPlans] = await Promise.all([fetchPlans(), fetchTenantPlans(scope?.tenant_id)]);
        setPlans(nextPlans);
        setTenantPlans(nextTenantPlans);
      } else if (section === 'users') {
        setUsers(await fetchAdminUsers(scope?.tenant_id));
      } else if (section === 'providers') {
        setProviders(await fetchRewardProviders(scope ? { tenant_id: scope.tenant_id, environment_id: scope.environment_id } : undefined));
      } else if (section === 'audit') {
        setAudit(await fetchAuditLog(scope ? { tenant_id: scope.tenant_id, environment_id: scope.environment_id, limit: 50 } : { limit: 50 }));
      } else if (section === 'engagement' && scope) {
        const [nextGoals, nextCampaigns, nextAnalytics] = await Promise.all([fetchGoals(scope), fetchCampaigns(scope), fetchCampaignAnalytics(scope)]);
        setGoals(nextGoals);
        setCampaigns(nextCampaigns);
        setAnalytics(nextAnalytics.campaigns);
      } else if (section === 'environments' && scope) {
        setEnvironments(await fetchEnvironments(scope.tenant_id));
      }
    };
    void load().catch((loadError) => setError(loadError instanceof Error ? loadError.message : 'Management data could not be loaded.')).finally(() => setLoading(false));
  }, [principal, scope, section]);

  async function submitPlan(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    try {
      const entitlements = JSON.parse(planForm.entitlements) as Record<string, number>;
      const created = await createPlan({ id: planForm.id, name: planForm.name, description: planForm.description, billing_interval: planForm.billing_interval, price_cents: Number(planForm.price_cents), entitlements });
      setPlans((current) => [created, ...current]);
      setMessage(`Plan ${created.name} created.`);
      setPlanForm({ id: '', name: '', description: '', billing_interval: 'manual', price_cents: '0', entitlements: defaultEntitlements });
    } catch (submitError) {
      setError(submitError instanceof SyntaxError ? 'Entitlements must be valid JSON.' : submitError instanceof Error ? submitError.message : 'Plan could not be created.');
    }
  }

  async function handleAssignPlan(planId: string) {
    if (!scope) return;
    setError(null);
    try {
      await assignTenantPlan({ tenant_id: scope.tenant_id, plan_id: planId });
      setTenantPlans(await fetchTenantPlans(scope.tenant_id));
      setMessage(`Assigned ${planId} to ${scope.tenant_id}.`);
    } catch (assignError) {
      setError(assignError instanceof Error ? assignError.message : 'Plan assignment failed.');
    }
  }

  async function handleArchivePlan(planId: string) {
    setError(null);
    try {
      await archivePlan(planId);
      setPlans((current) => current.map((plan) => plan.id === planId ? { ...plan, active: false } : plan));
      setMessage(`Plan ${planId} archived.`);
    } catch (archiveError) {
      setError(archiveError instanceof Error ? archiveError.message : 'Plan could not be archived.');
    }
  }

  async function submitUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    try {
      const created = await createAdminUser({ email: userForm.email, password: userForm.password, role: userForm.role, tenant_ids: userForm.role === 'tenant_admin' ? userForm.tenant_ids.split(',').map((id) => id.trim()).filter(Boolean) : [] });
      setUsers((current) => [created, ...current]);
      setUserForm({ email: '', password: '', role: 'tenant_admin', tenant_ids: '' });
      setMessage(`Administrator ${created.email} created.`);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Administrator could not be created.');
    }
  }

  async function submitProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      await saveRewardProvider({ tenant_id: scope.tenant_id, environment_id: scope.environment_id, provider: providerForm.provider, delivery_url: providerForm.delivery_url, status_url: providerForm.status_url || undefined, token: providerForm.token || undefined, timeout_seconds: Number(providerForm.timeout_seconds), active: providerForm.active });
      setProviders(await fetchRewardProviders(scope));
      setProviderForm((current) => ({ ...current, token: '' }));
      setMessage('Reward provider configuration saved.');
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Provider configuration could not be saved.');
    }
  }

  async function submitEnvironment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const created = await createEnvironment({ tenant_id: scope.tenant_id, environment_id: environmentForm.environment_id, name: environmentForm.name });
      setEnvironments((current) => [...current, created].sort((a, b) => a.environment_id.localeCompare(b.environment_id)));
      setRevealedEnvironmentKey(created.integration_api_key ?? null);
      setEnvironmentForm({ environment_id: '', name: '' });
      setMessage(`Environment ${created.name} created.`);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Environment could not be created.');
    }
  }

  async function submitGoal(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const created = await createGoal({ ...scope, id: goalForm.id, name: goalForm.name, description: goalForm.description, event_type: goalForm.event_type, target_count: Number(goalForm.target_count) });
      setGoals((current) => [created, ...current]);
      setGoalForm({ id: '', name: '', description: '', event_type: '', target_count: '1' });
      setMessage(`Goal ${created.name} created.`);
    } catch (submitError) { setError(submitError instanceof Error ? submitError.message : 'Goal could not be created.'); }
  }

  async function submitCampaign(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const created = await createCampaign({ ...scope, id: campaignForm.id, goal_id: campaignForm.goal_id || undefined, name: campaignForm.name, description: campaignForm.description, event_type: campaignForm.event_type, starts_at: campaignForm.starts_at ? new Date(campaignForm.starts_at).toISOString() : undefined, ends_at: campaignForm.ends_at ? new Date(campaignForm.ends_at).toISOString() : undefined });
      setCampaigns((current) => [created, ...current]);
      setCampaignForm({ id: '', name: '', description: '', event_type: '', goal_id: '', starts_at: '', ends_at: '' });
      setMessage(`Campaign ${created.name} created.`);
    } catch (submitError) { setError(submitError instanceof Error ? submitError.message : 'Campaign could not be created.'); }
  }

  function renderContent() {
    if (section === 'plans') return <div className="management-content">
      <div className="management-list">{plans.map((plan) => <div className="management-row" key={plan.id}><span><strong>{plan.name}</strong><small>{plan.id} · {plan.billing_interval} · {(plan.price_cents / 100).toFixed(2)} USD</small></span><span className={`management-status ${plan.active ? 'active' : 'inactive'}`}>{plan.active ? 'active' : 'archived'}</span>{isSuperAdmin && plan.active && <button className="text-button" type="button" onClick={() => void handleArchivePlan(plan.id)}>Archive</button>}{isSuperAdmin && scope && plan.active && <button className="text-button accent" type="button" onClick={() => void handleAssignPlan(plan.id)}>{currentTenantPlan?.plan_id === plan.id ? 'Assigned' : 'Assign'}</button>}</div>)}{plans.length === 0 && <p className="management-empty">No plans have been defined.</p>}</div>
      {isSuperAdmin && <form className="management-form" onSubmit={(event) => void submitPlan(event)}><span className="management-form-title"><IconPlus size={14} />Create plan</span><div className="form-grid"><label><span>Plan ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={planForm.id} onChange={(event) => setPlanForm({ ...planForm, id: event.target.value })} /></label><label><span>Name</span><input required value={planForm.name} onChange={(event) => setPlanForm({ ...planForm, name: event.target.value })} /></label></div><label><span>Description</span><input value={planForm.description} onChange={(event) => setPlanForm({ ...planForm, description: event.target.value })} /></label><div className="form-grid"><label><span>Billing</span><select value={planForm.billing_interval} onChange={(event) => setPlanForm({ ...planForm, billing_interval: event.target.value })}><option value="manual">Manual</option><option value="monthly">Monthly</option><option value="yearly">Yearly</option></select></label><label><span>Price cents</span><input type="number" min="0" value={planForm.price_cents} onChange={(event) => setPlanForm({ ...planForm, price_cents: event.target.value })} /></label></div><label><span>Entitlements JSON</span><textarea required value={planForm.entitlements} onChange={(event) => setPlanForm({ ...planForm, entitlements: event.target.value })} spellCheck={false} /></label><button className="primary-button" type="submit">Create plan</button></form>}
    </div>;
    if (section === 'users') return <div className="management-content"><div className="management-list">{users.map((user) => <div className="management-row" key={user.id}><span><strong>{user.email}</strong><small>{user.role} · {user.tenant_ids?.join(', ') || 'all tenants'}</small></span><span className={`management-status ${user.active ? 'active' : 'inactive'}`}>{user.active ? 'active' : 'inactive'}</span></div>)}{users.length === 0 && <p className="management-empty">No administrators found.</p>}</div>{isSuperAdmin && <form className="management-form" onSubmit={(event) => void submitUser(event)}><span className="management-form-title"><IconPlus size={14} />Create administrator</span><label><span>Email</span><input required type="email" value={userForm.email} onChange={(event) => setUserForm({ ...userForm, email: event.target.value })} /></label><label><span>Temporary password</span><input required minLength={12} type="password" value={userForm.password} onChange={(event) => setUserForm({ ...userForm, password: event.target.value })} /></label><label><span>Role</span><select value={userForm.role} onChange={(event) => setUserForm({ ...userForm, role: event.target.value })}><option value="tenant_admin">Tenant admin</option><option value="super_admin">Super admin</option></select></label>{userForm.role === 'tenant_admin' && <label><span>Tenant IDs, comma separated</span><input required value={userForm.tenant_ids} onChange={(event) => setUserForm({ ...userForm, tenant_ids: event.target.value })} placeholder="acme" /></label>}<button className="primary-button" type="submit">Create administrator</button></form>}</div>;
    if (section === 'providers') return <div className="management-content"><form className="management-form" onSubmit={(event) => void submitProvider(event)}><span className="management-form-title"><IconKey size={14} />Provider for {scope ? `${scope.tenant_id} / ${scope.environment_id}` : 'active scope'}</span><label><span>Provider</span><select value={providerForm.provider} onChange={(event) => setProviderForm({ ...providerForm, provider: event.target.value })}><option value="mock">Mock</option><option value="http">HTTP</option></select></label>{providerForm.provider === 'http' && <><label><span>Delivery URL</span><input required value={providerForm.delivery_url} onChange={(event) => setProviderForm({ ...providerForm, delivery_url: event.target.value })} placeholder="https://rewards.example/deliver" /></label><label><span>Status URL <em>optional</em></span><input value={providerForm.status_url} onChange={(event) => setProviderForm({ ...providerForm, status_url: event.target.value })} /></label><label><span>Token {activeProvider?.token_configured ? '(leave blank to keep current)' : ''}</span><input type="password" value={providerForm.token} onChange={(event) => setProviderForm({ ...providerForm, token: event.target.value })} /></label></>}<label><span>Timeout seconds</span><input required type="number" min="1" max="60" value={providerForm.timeout_seconds} onChange={(event) => setProviderForm({ ...providerForm, timeout_seconds: event.target.value })} /></label><label className="checkbox-label"><input type="checkbox" checked={providerForm.active} onChange={(event) => setProviderForm({ ...providerForm, active: event.target.checked })} /><span>Active</span></label><button className="primary-button" type="submit" disabled={!scope}>Save provider</button></form><div className="management-note">{activeProvider ? `Configured ${activeProvider.provider} provider · token ${activeProvider.token_configured ? 'stored' : 'not set'}.` : 'No scope-specific provider is configured; the runtime fallback will be used.'}</div></div>;
    if (section === 'audit') return <div className="management-list audit-management-list">{audit.map((entry) => <div className="management-row" key={entry.id}><span><strong>{entry.action}</strong><small>{entry.actor_type} · {entry.resource_type}{entry.resource_id ? `/${entry.resource_id}` : ''}</small></span><time>{new Date(entry.created_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}</time></div>)}{audit.length === 0 && <p className="management-empty">No audit entries for this scope.</p>}</div>;
    if (section === 'engagement') return <div className="management-content"><div className="management-list">{campaigns.map((campaign) => { const metric = analytics.find((item) => item.campaign_id === campaign.id); return <div className="management-row" key={campaign.id}><span><strong>{campaign.name}</strong><small>{campaign.event_type} · {metric?.event_count ?? 0} events · {metric?.unique_users ?? 0} users</small></span><span className="management-status active">{campaign.active ? 'active' : 'archived'}</span></div>; })}{campaigns.length === 0 && <p className="management-empty">No campaigns defined for this scope.</p>}</div><div className="management-note">Analytics counts matching events inside each campaign window and reports unique external users.</div><form className="management-form" onSubmit={(event) => void submitGoal(event)}><span className="management-form-title"><IconPlus size={14} />Create goal</span><div className="form-grid"><label><span>Goal ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={goalForm.id} onChange={(event) => setGoalForm({ ...goalForm, id: event.target.value })} /></label><label><span>Name</span><input required value={goalForm.name} onChange={(event) => setGoalForm({ ...goalForm, name: event.target.value })} /></label></div><label><span>Description</span><input value={goalForm.description} onChange={(event) => setGoalForm({ ...goalForm, description: event.target.value })} /></label><div className="form-grid"><label><span>Event type</span><input required value={goalForm.event_type} onChange={(event) => setGoalForm({ ...goalForm, event_type: event.target.value })} placeholder="order.completed" /></label><label><span>Target events</span><input required type="number" min="1" value={goalForm.target_count} onChange={(event) => setGoalForm({ ...goalForm, target_count: event.target.value })} /></label></div><button className="primary-button" type="submit" disabled={!scope}>Create goal</button></form><form className="management-form" onSubmit={(event) => void submitCampaign(event)}><span className="management-form-title"><IconPlus size={14} />Create campaign</span><div className="form-grid"><label><span>Campaign ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={campaignForm.id} onChange={(event) => setCampaignForm({ ...campaignForm, id: event.target.value })} /></label><label><span>Name</span><input required value={campaignForm.name} onChange={(event) => setCampaignForm({ ...campaignForm, name: event.target.value })} /></label></div><label><span>Description</span><input value={campaignForm.description} onChange={(event) => setCampaignForm({ ...campaignForm, description: event.target.value })} /></label><div className="form-grid"><label><span>Event type</span><input required value={campaignForm.event_type} onChange={(event) => setCampaignForm({ ...campaignForm, event_type: event.target.value })} /><span>Goal ID optional</span><select value={campaignForm.goal_id} onChange={(event) => setCampaignForm({ ...campaignForm, goal_id: event.target.value })}><option value="">No linked goal</option>{goals.map((goal) => <option key={goal.id} value={goal.id}>{goal.name}</option>)}</select></label><label><span>Starts at optional</span><input type="datetime-local" value={campaignForm.starts_at} onChange={(event) => setCampaignForm({ ...campaignForm, starts_at: event.target.value })} /><span>Ends at optional</span><input type="datetime-local" value={campaignForm.ends_at} onChange={(event) => setCampaignForm({ ...campaignForm, ends_at: event.target.value })} /></label></div><button className="primary-button" type="submit" disabled={!scope}>Create campaign</button></form></div>;
    if (section === 'measurement') return <MeasurementPanel scope={scope} />;
    return <div className="management-content"><div className="management-list">{environments.map((environment) => <div className="management-row" key={environment.environment_id}><span><strong>{environment.name}</strong><small>{environment.tenant_id} / {environment.environment_id}</small></span><span className="management-status active">available</span></div>)}{environments.length === 0 && <p className="management-empty">No environments found.</p>}</div>{revealedEnvironmentKey && <div className="revealed-key"><strong>Integration key — copy it now</strong><code>{revealedEnvironmentKey}</code></div>}{isSuperAdmin && <form className="management-form" onSubmit={(event) => void submitEnvironment(event)}><span className="management-form-title"><IconPlus size={14} />Add environment to {scope?.tenant_id}</span><label><span>Environment ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={environmentForm.environment_id} onChange={(event) => setEnvironmentForm({ ...environmentForm, environment_id: event.target.value })} /></label><label><span>Display name</span><input required value={environmentForm.name} onChange={(event) => setEnvironmentForm({ ...environmentForm, name: event.target.value })} /></label><button className="primary-button" type="submit" disabled={!scope}>Create environment</button></form>}</div>;
  }

  if (!principal) return null;

  return (
    <section className="settings-section management-section" aria-labelledby="management-title">
      <div className="settings-section-heading"><IconActivity size={16} /><strong id="management-title">Platform management</strong></div>
      <div className="management-tabs" role="tablist" aria-label="Platform management sections">
        <button type="button" className={section === 'plans' ? 'active' : ''} onClick={() => setSection('plans')}><IconCreditCard size={13} />Plans</button>
        <button type="button" className={section === 'users' ? 'active' : ''} onClick={() => setSection('users')}><IconUsers size={13} />Admins</button>
        <button type="button" className={section === 'providers' ? 'active' : ''} onClick={() => setSection('providers')}><IconKey size={13} />Providers</button>
        <button type="button" className={section === 'audit' ? 'active' : ''} onClick={() => setSection('audit')}><IconActivity size={13} />Audit</button>
        <button type="button" className={section === 'engagement' ? 'active' : ''} onClick={() => setSection('engagement')}><IconActivity size={13} />Engagement</button>
        <button type="button" className={section === 'measurement' ? 'active' : ''} onClick={() => setSection('measurement')}><IconChartBar size={13} />Measurement</button>
        {isSuperAdmin && <button type="button" className={section === 'environments' ? 'active' : ''} onClick={() => setSection('environments')}><IconCloud size={13} />Environments</button>}
      </div>
      {error && <div className="management-message error" role="alert">{error}</div>}
      {message && <div className="management-message success" role="status">{message}</div>}
      {loading ? <div className="registry-loading management-loading"><div /><div /></div> : renderContent()}
    </section>
  );
}
