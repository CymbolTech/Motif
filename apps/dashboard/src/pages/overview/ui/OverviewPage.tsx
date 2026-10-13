import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconActivity, IconArrowUpRight, IconCircleCheck, IconCloudCheck, IconDatabase, IconGauge, IconRefresh, IconServer } from '@tabler/icons-react';
import { EventCheck } from '../../../features/event-check/ui/EventCheck';
import { TenantCreateDialog } from '../../../features/tenant-create/ui/TenantCreateDialog';
import { RecentEvents } from '../../../features/event-feed/ui/RecentEvents';
import { TenantDetailDialog } from '../../../features/tenant-detail/ui/TenantDetailDialog';
import { TenantList } from '../../../features/tenant-list/ui/TenantList';
import { fetchDiagnostics, fetchHealth, fetchTenantUsage, fetchTenants, type AdminPrincipal, type Diagnostics, type HealthState, type Tenant, type TenantUsage } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

type HealthTileProps = {
  label: string;
  state: HealthState;
  detail: string;
  icon: React.ReactNode;
};

function HealthTile({ label, state, detail, icon }: HealthTileProps) {
  return (
    <div className="health-tile">
      <div className="health-icon">{icon}</div>
      <div className="health-content">
        <div className="health-label"><span className={`health-dot ${state}`} />{label}</div>
        <strong>{state === 'healthy' ? 'Operational' : state === 'checking' ? 'Checking' : 'Needs attention'}</strong>
        <span>{detail}</span>
      </div>
      <IconArrowUpRight className="health-arrow" size={16} stroke={1.4} />
    </div>
  );
}

export function OverviewPage({ principal }: { principal?: AdminPrincipal | null }) {
  const { scope } = useScope();
  const isSuperAdmin = principal?.role === 'super_admin';
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [tenantLoading, setTenantLoading] = useState(true);
  const [tenantError, setTenantError] = useState<string | null>(null);
  const [apiState, setApiState] = useState<HealthState>('checking');
  const [workerState, setWorkerState] = useState<HealthState>('checking');
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [tenantCreateOpen, setTenantCreateOpen] = useState(false);
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null);
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);
  const [diagnosticsError, setDiagnosticsError] = useState<string | null>(null);
  const [usage, setUsage] = useState<TenantUsage | null>(null);

  const refreshTenants = useCallback(async () => {
    setTenantLoading(true);
    setTenantError(null);
    try {
      setTenants(await fetchTenants());
    } catch (error) {
      setTenantError(error instanceof Error ? error.message : 'Unable to load tenants');
    } finally {
      setTenantLoading(false);
    }
  }, []);

  const refreshHealth = useCallback(async () => {
    setApiState('checking');
    setWorkerState('checking');
    const [api, worker] = await Promise.allSettled([fetchHealth('api'), fetchHealth('worker')]);
    setApiState(api.status === 'fulfilled' ? 'healthy' : 'degraded');
    setWorkerState(worker.status === 'fulfilled' ? 'healthy' : 'degraded');
    setLastChecked(new Date());
  }, []);

  const refreshDiagnostics = useCallback(async () => {
    setDiagnosticsError(null);
    try {
      setDiagnostics(await fetchDiagnostics(scope));
    } catch (error) {
      setDiagnosticsError(error instanceof Error ? error.message : 'Diagnostics unavailable');
    }
  }, [scope]);

  const refreshUsage = useCallback(async () => {
    if (!scope) {
      setUsage(null);
      return;
    }
    try {
      setUsage(await fetchTenantUsage(scope.tenant_id));
    } catch {
      setUsage(null);
    }
  }, [scope]);

  useEffect(() => {
    void refreshTenants();
    void refreshHealth();
    void refreshDiagnostics();
    void refreshUsage();
  }, [refreshDiagnostics, refreshHealth, refreshTenants, refreshUsage]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refreshHealth();
      void refreshDiagnostics();
      void refreshUsage();
    }, 15000);
    return () => window.clearInterval(interval);
  }, [refreshDiagnostics, refreshHealth]);

  const healthyCount = useMemo(() => [apiState, workerState].filter((state) => state === 'healthy').length, [apiState, workerState]);
  const checkedLabel = lastChecked ? `Checked ${lastChecked.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}` : 'Checking dependencies';

  return (
    <main className="overview-main">
      <header className="overview-header">
        <div>
          <div className="breadcrumb"><span>{isSuperAdmin ? 'Platform' : 'Workspace'}</span><IconArrowUpRight size={13} /><span>Overview</span></div>
          <h1>{isSuperAdmin ? 'Keep the signal moving.' : 'Keep your workspace moving.'}</h1>
          <p>{isSuperAdmin ? 'One view for the systems that make MotifEngine dependable.' : 'Operate the events, rules, and engagement systems for your workspace.'}</p>
        </div>
        <div className="header-actions">
          <span className="last-checked"><span className="pulse-dot" />{checkedLabel}</span>
          <button className="icon-button refresh-button" type="button" onClick={() => { void refreshHealth(); void refreshTenants(); void refreshDiagnostics(); }} aria-label="Refresh dashboard">
            <IconRefresh size={17} stroke={1.6} />
          </button>
        </div>
      </header>

      <section className="hero-strip" aria-label="System summary">
        <div className="hero-summary">
          <span className="section-label">Control plane / today</span>
          <div className="hero-number">{healthyCount}<span>/2</span></div>
          <p>critical services responding</p>
        </div>
        <div className="hero-sparkline" aria-hidden="true">
          <div className="sparkline-label"><IconActivity size={15} /> event path</div>
          <svg viewBox="0 0 460 110" preserveAspectRatio="none">
            <path d="M0 86 C28 82, 38 62, 61 70 S97 84, 116 61 S148 32, 173 53 S204 91, 228 61 S260 20, 284 46 S320 70, 344 40 S375 25, 397 48 S430 43, 460 12" />
            <path className="sparkline-fill" d="M0 86 C28 82, 38 62, 61 70 S97 84, 116 61 S148 32, 173 53 S204 91, 228 61 S260 20, 284 46 S320 70, 344 40 S375 25, 397 48 S430 43, 460 12 L460 110 L0 110 Z" />
          </svg>
          <div className="sparkline-axis"><span>00:00</span><span>now</span></div>
        </div>
      </section>

      <section className="health-grid" aria-label="Service health">
        <HealthTile label="API" state={apiState} detail="Receipt and admin routes" icon={<IconServer size={19} stroke={1.5} />} />
        <HealthTile label="Worker" state={workerState} detail="JetStream processing loop" icon={<IconGauge size={19} stroke={1.5} />} />
        <HealthTile label="Data plane" state={apiState === 'healthy' ? 'healthy' : apiState} detail="PostgreSQL + NATS ready" icon={<IconDatabase size={19} stroke={1.5} />} />
      </section>

      <section className="panel diagnostics-panel" aria-labelledby="diagnostics-title">
        <div className="panel-heading compact">
          <div>
            <span className="section-label">Operational pulse</span>
            <h2 id="diagnostics-title">What is moving right now</h2>
          </div>
          {diagnostics && <span className="diagnostics-updated">{scope ? `${scope.tenant_id} / ${scope.environment_id} · ` : ''}Updated {new Date(diagnostics.generated_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>}
        </div>
        {diagnosticsError ? <div className="inline-state error-state diagnostics-state" role="alert"><strong>Diagnostics unavailable</strong><span>{diagnosticsError}</span></div> : !diagnostics ? <div className="registry-loading diagnostics-loading"><div /><div /><div /><div /></div> : <div className="diagnostics-grid">
          <div className="diagnostics-stat"><span>Processed events</span><strong>{(diagnostics.event_counts.processed ?? 0).toLocaleString()}</strong><small>{(diagnostics.event_counts.accepted ?? 0).toLocaleString()} awaiting receipt</small></div>
          <div className={`diagnostics-stat ${diagnostics.pending_outbox > 0 ? 'attention' : ''}`}><span>Pending outbox</span><strong>{diagnostics.pending_outbox.toLocaleString()}</strong><small>durable event publications</small></div>
          <div className="diagnostics-stat"><span>Published rules</span><strong>{diagnostics.active_rules.toLocaleString()}</strong><small>eligible configurations</small></div>
          <div className={`diagnostics-stat ${diagnostics.failed_rule_executions + diagnostics.failed_redemptions > 0 ? 'attention' : ''}`}><span>Failed work</span><strong>{(diagnostics.failed_rule_executions + diagnostics.failed_redemptions).toLocaleString()}</strong><small>{diagnostics.retryable_rule_executions.toLocaleString()} rule retries available</small></div>
        </div>}
      </section>

      <div className="content-grid">
        <TenantList tenants={tenants} loading={tenantLoading} error={tenantError} canCreate={isSuperAdmin} onRefresh={() => void refreshTenants()} onCreate={() => setTenantCreateOpen(true)} onOpen={setSelectedTenant} />
        <aside className="side-column">
          <section className="panel readiness-panel" aria-labelledby="readiness-title">
            <div className="panel-heading compact">
              <div>
                <span className="section-label">Readiness</span>
                <h2 id="readiness-title">System posture</h2>
              </div>
              <IconCloudCheck size={19} stroke={1.5} />
            </div>
            <div className="readiness-row"><span>Receipt path</span><strong><IconCircleCheck size={15} /> enabled</strong></div>
            <div className="readiness-row"><span>Tenant isolation</span><strong><IconCircleCheck size={15} /> enforced</strong></div>
            <div className="readiness-row"><span>Async processing</span><strong className={workerState === 'healthy' ? '' : 'warning'}>{workerState === 'healthy' ? <><IconCircleCheck size={15} /> live</> : 'waiting'}</strong></div>
          </section>
          <section className="panel usage-panel" aria-labelledby="usage-title">
            <div className="panel-heading compact"><div><span className="section-label">Plan guardrails</span><h2 id="usage-title">Capacity in view</h2></div><IconGauge size={18} stroke={1.5} /></div>
            {!usage ? <div className="usage-empty">Select a tenant to load limits.</div> : <div className="usage-list">{[['rules', 'Rules'], ['currencies', 'Currencies'], ['rewards', 'Rewards']].map(([key, label]) => { const current = usage.usage[key] ?? 0; const limit = usage.limits[`max_${key}`] ?? 0; const ratio = limit > 0 ? Math.min(100, Math.round((current / limit) * 100)) : 0; return <div className="usage-row" key={key}><div><span>{label}</span><strong>{current.toLocaleString()} <small>/ {limit ? limit.toLocaleString() : '∞'}</small></strong></div><div className="usage-track"><i style={{ width: `${ratio}%` }} /></div></div>; })}</div>}
          </section>
          <div className="signal-note">
            <span className="signal-mark"><IconActivity size={16} stroke={1.6} /></span>
            <div><strong>Start with one signal</strong><span>Use the test event below to verify the complete receipt loop.</span></div>
          </div>
        </aside>
      </div>

      <EventCheck />
      <RecentEvents />
      {isSuperAdmin && <TenantCreateDialog open={tenantCreateOpen} onClose={() => setTenantCreateOpen(false)} onCreated={(tenant) => setTenants((current) => [...current.filter((item) => item.tenant_id !== tenant.tenant_id), tenant])} />}
      <TenantDetailDialog tenant={selectedTenant} canManagePlatform={isSuperAdmin} onClose={() => setSelectedTenant(null)} onUpdated={(updated) => { setSelectedTenant(updated); setTenants((current) => current.map((item) => item.tenant_id === updated.tenant_id ? updated : item)); }} />
    </main>
  );
}
