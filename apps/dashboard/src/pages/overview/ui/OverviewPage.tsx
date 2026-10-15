import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconAlertTriangle, IconArrowRight, IconCircleCheck, IconCircleDashed, IconCircleX, IconClockHour4, IconRefresh, IconSend } from '@tabler/icons-react';
import { EventCheck } from '../../../features/event-check/ui/EventCheck';
import { RecentEvents } from '../../../features/event-feed/ui/RecentEvents';
import { fetchCurrencies, fetchDiagnostics, fetchEventTypes, fetchRewards, fetchRules, fetchTenantUsage, type AdminPrincipal, type Currency, type Diagnostics, type EventType, type Reward, type Rule, type TenantUsage } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { PageHeader } from '../../../shared/ui/PageHeader';

type OverviewDestination = 'events' | 'automations' | 'currency' | 'reward';

type Inventory = {
  eventTypes: EventType[] | null;
  rules: Rule[] | null;
  currencies: Currency[] | null;
  rewards: Reward[] | null;
};

type SetupItem = {
  key: string;
  label: string;
  description: string;
  count: number | null;
  actionLabel: string;
  destination: OverviewDestination;
};

const emptyInventory: Inventory = { eventTypes: null, rules: null, currencies: null, rewards: null };

function formatCount(value: number | null | undefined) {
  return value === null || value === undefined ? '-' : value.toLocaleString();
}

export function OverviewPage({ principal: _principal, onNavigate }: { principal?: AdminPrincipal | null; onNavigate?: (view: OverviewDestination) => void }) {
  const { scope } = useScope();
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);
  const [diagnosticsError, setDiagnosticsError] = useState<string | null>(null);
  const [usage, setUsage] = useState<TenantUsage | null>(null);
  const [inventory, setInventory] = useState<Inventory>(emptyInventory);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const refreshDashboard = useCallback(async () => {
    setRefreshing(true);
    if (!scope) {
      setDiagnostics(null);
      setUsage(null);
      setInventory(emptyInventory);
      setLastUpdated(null);
      setRefreshing(false);
      return;
    }

    const filters = { tenant_id: scope.tenant_id, environment_id: scope.environment_id };
    const results = await Promise.allSettled([
      fetchDiagnostics(filters),
      fetchTenantUsage(scope.tenant_id),
      fetchEventTypes(filters),
      fetchRules(filters),
      fetchCurrencies(filters),
      fetchRewards(filters),
    ]);
    const [diagnosticsResult, usageResult, eventTypesResult, rulesResult, currenciesResult, rewardsResult] = results;

    if (diagnosticsResult.status === 'fulfilled') {
      setDiagnostics(diagnosticsResult.value);
      setDiagnosticsError(null);
    } else {
      setDiagnosticsError(diagnosticsResult.reason instanceof Error ? diagnosticsResult.reason.message : 'Workspace activity could not be loaded.');
    }
    setUsage(usageResult.status === 'fulfilled' ? usageResult.value : null);
    setInventory({
      eventTypes: eventTypesResult.status === 'fulfilled' ? eventTypesResult.value : null,
      rules: rulesResult.status === 'fulfilled' ? rulesResult.value : null,
      currencies: currenciesResult.status === 'fulfilled' ? currenciesResult.value : null,
      rewards: rewardsResult.status === 'fulfilled' ? rewardsResult.value : null,
    });
    setLastUpdated(new Date());
    setRefreshing(false);
  }, [scope]);

  useEffect(() => { void refreshDashboard(); }, [refreshDashboard]);
  useEffect(() => {
    const interval = window.setInterval(() => { void refreshDashboard(); }, 30000);
    return () => window.clearInterval(interval);
  }, [refreshDashboard]);

  const setupItems = useMemo<SetupItem[]>(() => [
    { key: 'events', label: 'Event types', description: 'The signals your product sends to MotifEngine.', count: inventory.eventTypes?.filter((eventType) => eventType.active !== false).length ?? null, actionLabel: 'Add event', destination: 'events' },
    { key: 'automations', label: 'Automations', description: 'Rules that turn events into loyalty outcomes.', count: inventory.rules?.filter((rule) => rule.status !== 'archived').length ?? null, actionLabel: 'Create automation', destination: 'automations' },
    { key: 'currencies', label: 'Currencies', description: 'Points, coins, or XP your members can earn.', count: inventory.currencies?.filter((currency) => currency.active).length ?? null, actionLabel: 'Add currency', destination: 'currency' },
    { key: 'rewards', label: 'Rewards', description: 'Benefits members can redeem with their balance.', count: inventory.rewards?.filter((reward) => reward.active).length ?? null, actionLabel: 'Add reward', destination: 'reward' },
  ], [inventory]);

  const configuredCount = setupItems.filter((item) => item.count !== null && item.count > 0).length;
  const nextSetupItem = setupItems.find((item) => item.count === 0);
  const failedWork = diagnostics ? diagnostics.failed_rule_executions + diagnostics.failed_redemptions : null;
  const processedEvents = diagnostics ? diagnostics.event_counts.processed ?? 0 : null;
  const acceptedEvents = diagnostics ? diagnostics.event_counts.accepted ?? 0 : null;
  const pendingOutbox = diagnostics?.pending_outbox ?? null;
  const updatedLabel = lastUpdated ? 'Updated ' + lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'Loading workspace data';
  const setupComplete = configuredCount === setupItems.length;
  const checksLoading = !diagnosticsError && pendingOutbox === null && failedWork === null;
  const allClear = !diagnosticsError && pendingOutbox === 0 && failedWork === 0 && !nextSetupItem && setupItems.every((item) => item.count !== null);

  function goTo(destination: OverviewDestination) {
    onNavigate?.(destination);
  }

  function focusTestEvent() {
    const target = document.getElementById('event-check');
    if (!target) return;
    target.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });
    window.setTimeout(() => target.querySelector<HTMLInputElement>('input')?.focus(), 250);
  }

  function focusRecentEvents() {
    document.getElementById('recent-events')?.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });
  }

  return <main className="page page-overview">
    <PageHeader breadcrumb={['Workspace', 'Overview']} title="Workspace overview" description={scope ? `See what is working, what needs attention, and what to build next. Every number is scoped to ${scope.environment_name}.` : 'See what is working, what needs attention, and what to build next.'} topbarActions={<><span className="updated-at"><IconClockHour4 size={14} stroke={1.75} aria-hidden="true" />{updatedLabel}</span><button className="icon-button ghost compact" type="button" onClick={() => void refreshDashboard()} disabled={refreshing} aria-label="Refresh dashboard"><IconRefresh className={refreshing ? 'spin' : ''} size={15} stroke={1.75} /></button></>} actions={<button className="secondary-button" type="button" onClick={focusTestEvent} disabled={!scope}><IconSend size={15} stroke={1.75} />Send test event</button>} />

    <section className="metrics" aria-label="Workspace activity summary">
      <div className="metric"><span className="metric-label">Processed events</span><strong className="metric-value">{formatCount(processedEvents)}</strong><span className="metric-note">{acceptedEvents === null ? 'Waiting for activity data' : formatCount(acceptedEvents) + ' accepted by the API'}</span></div>
      <div className="metric"><span className="metric-label">Active automations</span><strong className="metric-value">{formatCount(diagnostics?.active_rules)}</strong><span className="metric-note">{diagnostics?.active_rules === 1 ? 'One rule is evaluating events' : 'Published rules evaluating events'}</span></div>
      <div className={'metric ' + (failedWork !== null && failedWork > 0 ? 'metric-danger' : '')}><span className="metric-label">Needs attention</span><strong className="metric-value">{formatCount(failedWork)}</strong><span className="metric-note">{failedWork === null ? 'Waiting for activity data' : failedWork > 0 ? 'Failed work to review' : allClear ? 'Nothing is blocking you' : 'No failed work reported'}</span></div>
    </section>

    <div className="overview-grid">
      <div className="overview-col overview-col-main">
        <RecentEvents />
        <EventCheck />
      </div>

      <div className="overview-col overview-col-side">
        {!allClear && <section className="panel attention-panel" aria-labelledby="attention-title">
          <div className="panel-header"><div><h2 id="attention-title">Needs attention</h2><p>Small checks that keep the workspace moving.</p></div></div>
          <div className="attention-list">
            {diagnosticsError && <div className="attention-row tone-warning"><IconAlertTriangle size={16} stroke={1.75} aria-hidden="true" /><span><strong>Activity data is unavailable</strong><small>{diagnosticsError}</small></span><button className="text-button" type="button" onClick={() => void refreshDashboard()}>Retry</button></div>}
            {pendingOutbox !== null && pendingOutbox > 0 && <div className="attention-row tone-warning"><IconClockHour4 size={16} stroke={1.75} aria-hidden="true" /><span><strong>{pendingOutbox} event{pendingOutbox === 1 ? '' : 's'} waiting to publish</strong><small>Check the event pipeline before adding more automations.</small></span><button className="text-button" type="button" onClick={focusTestEvent}>Test event</button></div>}
            {failedWork !== null && failedWork > 0 && <div className="attention-row tone-danger"><IconCircleX size={16} stroke={1.75} aria-hidden="true" /><span><strong>{failedWork} failed rule or reward action{failedWork === 1 ? '' : 's'}</strong><small>Inspect the latest signals in Recent events.</small></span><button className="text-button" type="button" onClick={focusRecentEvents}>Review</button></div>}
            {nextSetupItem && <div className="attention-row"><IconCircleDashed size={16} stroke={1.75} aria-hidden="true" /><span><strong>Set up {nextSetupItem.label.toLowerCase()}</strong><small>{nextSetupItem.description}</small></span><button className="text-button accent" type="button" onClick={() => goTo(nextSetupItem.destination)}>{nextSetupItem.actionLabel}</button></div>}
            {checksLoading && <div className="skeleton-rows" aria-label="Loading workspace checks"><span /><span /></div>}
          </div>
        </section>}

        <section className="panel" aria-labelledby="setup-title">
          <div className="panel-header"><div><h2 id="setup-title">Build status</h2><p>{setupComplete ? 'Core building blocks are configured.' : 'Get your first loyalty loop running: connect one event to one automation, then verify the result here.'}</p></div><span className="panel-meta">{configuredCount}/{setupItems.length} set up</span></div>
          <ul className="setup-list">{setupItems.map((item) => { const ready = item.count !== null && item.count > 0; return <li key={item.key}><span className={'setup-icon ' + (ready ? 'is-ready' : item.count === null ? '' : 'is-missing')} aria-hidden="true">{ready ? <IconCircleCheck size={16} stroke={1.75} /> : <IconCircleDashed size={16} stroke={1.75} />}</span><span className="setup-copy"><strong>{item.label}</strong><small>{item.count === null ? 'Loading…' : item.count === 0 ? 'Not set up yet' : `${item.count} active`}</small></span><button className={item.count === 0 ? 'secondary-button compact' : 'text-button'} type="button" onClick={() => goTo(item.destination)} aria-label={item.count === 0 ? item.actionLabel : `Open ${item.label.toLowerCase()}`}>{item.count === 0 ? item.actionLabel : <>Open <IconArrowRight size={14} stroke={1.75} /></>}</button></li>; })}</ul>
        </section>

        <section className="panel" aria-labelledby="capacity-title">
          <div className="panel-header"><div><h2 id="capacity-title">Capacity</h2><p>Current usage against workspace limits.</p></div></div>
          {!usage ? <div className="skeleton-rows" aria-label="Loading capacity"><span /><span /><span /></div> : <div className="usage-list">{[['rules', 'Rules'], ['currencies', 'Currencies'], ['rewards', 'Rewards']].map(([key, label]) => { const current = usage.usage[key] ?? 0; const limit = usage.limits['max_' + key] ?? 0; const ratio = limit > 0 ? Math.min(100, Math.round((current / limit) * 100)) : 0; return <div className={'usage-row ' + (ratio >= 90 ? 'is-high' : '')} key={key}><div className="usage-row-head"><span>{label}</span><span className="usage-figures">{current.toLocaleString()} <span>/ {limit ? limit.toLocaleString() : 'Unlimited'}</span></span></div><div className="meter" role="meter" aria-label={`${label} usage`} aria-valuemin={0} aria-valuemax={limit || 100} aria-valuenow={current}><i style={{ width: ratio + '%' }} /></div></div>; })}</div>}
        </section>
      </div>
    </div>
  </main>;
}
