import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconBraces, IconCircleCheck, IconCode, IconPlus, IconRefresh } from '@tabler/icons-react';
import { EventTypeCreateDialog } from '../../../features/event-type-create/ui/EventTypeCreateDialog';
import { fetchEventTypes, fetchTenants, type EventType, type Tenant } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

export function EventTypesPage() {
  const { scope } = useScope();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [eventTypes, setEventTypes] = useState<EventType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [filter, setFilter] = useState('all');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextTenants, nextEventTypes] = await Promise.all([fetchTenants(), fetchEventTypes(scope)]);
      setTenants(nextTenants);
      setEventTypes(nextEventTypes);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Event types could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [scope]);

  useEffect(() => { void refresh(); }, [refresh]);

  const visibleEventTypes = useMemo(() => filter === 'all' ? eventTypes : eventTypes.filter((eventType) => `${eventType.tenant_id}-${eventType.environment_id}` === filter), [eventTypes, filter]);

  return (
    <main className="overview-main event-types-page">
      <header className="overview-header">
        <div><div className="breadcrumb"><span>Platform</span><span>↗</span><span>Event types</span></div><h1>Make every signal legible.</h1><p>Define the contracts that keep incoming events trustworthy.</p></div>
        <div className="header-actions"><button className="icon-button refresh-button" type="button" onClick={() => void refresh()} aria-label="Refresh event types"><IconRefresh size={17} stroke={1.6} /></button><button className="primary-button" type="button" onClick={() => setDialogOpen(true)}><IconPlus size={16} stroke={1.8} />New event type</button></div>
      </header>
      <section className="event-type-summary"><div><span className="section-label">Contract registry</span><div className="summary-number">{eventTypes.length}<span>registered</span></div></div><div className="summary-note"><IconCircleCheck size={18} /><span>Required-field validation is active on every registered contract.</span></div><div className="scope-chip"><span className="status-dot" />{scope ? `${scope.tenant_id} / ${scope.environment_id}` : 'No active scope'}</div></section>
      <section className="panel registry-panel" aria-labelledby="registry-title">
        <div className="panel-heading"><div><span className="section-label">Current contracts</span><h2 id="registry-title">Event type registry</h2></div><select className="filter-select" value={filter} onChange={(event) => setFilter(event.target.value)} aria-label="Filter event types by tenant"><option value="all">All loaded scopes</option>{tenants.map((tenant) => <option key={`${tenant.tenant_id}-${tenant.environment_id}`} value={`${tenant.tenant_id}-${tenant.environment_id}`}>{tenant.name} · {tenant.environment_name}</option>)}</select></div>
        {error ? <div className="inline-state error-state" role="alert"><strong>Registry unavailable</strong><span>{error}</span></div> : loading ? <div className="registry-loading"><div /><div /><div /></div> : visibleEventTypes.length === 0 ? <div className="inline-state empty-state"><IconCode size={20} stroke={1.5} /><strong>No event types registered</strong><span>Start with one stable contract for the signals your product emits.</span><button className="primary-button" type="button" onClick={() => setDialogOpen(true)}><IconPlus size={15} />Register first event type</button></div> : <ul className="event-type-list">{visibleEventTypes.map((eventType) => <li className="event-type-row" key={`${eventType.tenant_id}-${eventType.environment_id}-${eventType.name}`}><div className="event-type-icon"><IconBraces size={17} stroke={1.5} /></div><div className="event-type-copy"><strong>{eventType.name}</strong><span>{eventType.tenant_id} / {eventType.environment_id}</span></div><div className="event-type-schema"><span>required</span><strong>{Array.isArray(eventType.schema.required) ? eventType.schema.required.length : 0} fields</strong></div><div className="event-type-active"><span className="status-dot" />active</div></li>)}</ul>}
      </section>
      <EventTypeCreateDialog open={dialogOpen} tenants={tenants} onClose={() => setDialogOpen(false)} onCreated={(eventType) => { setEventTypes((current) => [eventType, ...current.filter((item) => item.name !== eventType.name || item.tenant_id !== eventType.tenant_id || item.environment_id !== eventType.environment_id)]); }} />
    </main>
  );
}
