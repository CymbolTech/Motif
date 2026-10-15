import { useCallback, useEffect, useState } from 'react';
import { IconActivity, IconChevronRight, IconRefresh } from '@tabler/icons-react';
import { fetchEvents, type EventRecord } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { formatDateTime, formatTime } from '../../../shared/lib/format';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { Sheet } from '../../../shared/ui/Sheet';

function statusTone(status: EventRecord['status']) {
  return status === 'processed' ? 'success' : status === 'failed' ? 'danger' : 'warning';
}

function statusLabel(status: EventRecord['status']) {
  return status === 'processed' ? 'processed' : status === 'accepted' ? 'accepted' : status === 'processing' ? 'processing' : 'failed';
}

export function RecentEvents() {
  const { scope } = useScope();
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [status, setStatus] = useState('all');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<EventRecord | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    if (!scope) {
      setEvents([]);
      setOffset(0);
      setHasMore(false);
      setLoading(false);
      return;
    }
    try {
      const next = await fetchEvents(8, { tenant_id: scope.tenant_id, environment_id: scope.environment_id, status: status === 'all' ? undefined : status });
      setEvents(next);
      setOffset(next.length);
      setHasMore(next.length === 8);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Recent events could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [scope, status]);

  useEffect(() => { void refresh(); }, [refresh]);

  async function loadMore() {
    if (!scope) return;
    setLoadingMore(true);
    try {
      const next = await fetchEvents(8, { tenant_id: scope.tenant_id, environment_id: scope.environment_id, status: status === 'all' ? undefined : status, offset });
      setEvents((current) => [...current, ...next]);
      setOffset((current) => current + next.length);
      setHasMore(next.length === 8);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'More events could not be loaded.');
    } finally {
      setLoadingMore(false);
    }
  }

  return (
    <section id="recent-events" className="panel recent-events" aria-labelledby="recent-events-title">
      <div className="panel-header">
        <div><h2 id="recent-events-title">Recent events</h2><p>The latest signals received by this environment.</p></div>
        <div className="panel-actions">
          <CustomSelect className="compact" value={status} onChange={setStatus} ariaLabel="Filter events by status" options={[{ value: 'all', label: 'All statuses' }, { value: 'accepted', label: 'Accepted' }, { value: 'processing', label: 'Processing' }, { value: 'processed', label: 'Processed' }, { value: 'failed', label: 'Failed' }]} />
          <button className="icon-button" type="button" onClick={() => void refresh()} disabled={loading} aria-label="Refresh recent events"><IconRefresh size={15} className={loading ? 'spin' : ''} stroke={1.75} /></button>
        </div>
      </div>
      {error ? <div className="panel-body"><p className="alert alert-error" role="alert"><span><strong>Event feed unavailable.</strong> {error}</span></p></div> : loading ? <div className="skeleton-rows" aria-label="Loading events"><span /><span /><span /><span /></div> : events.length === 0 ? <div className="empty-state"><IconActivity size={20} stroke={1.5} aria-hidden="true" /><strong>{status === 'all' ? 'No events received' : `No ${status} events`}</strong><span>{status === 'all' ? 'Send a test event to see the receipt path here.' : 'Try another status filter.'}</span></div> : <>
        <div className="event-table" role="list">
          <div className="event-table-head" aria-hidden="true"><span>Status</span><span>Event</span><span>External ID</span><span>Received</span><span /></div>
          {events.map((event) => <div role="listitem" key={event.event_id}><button className="event-row" type="button" onClick={() => setSelected(event)} aria-label={`${event.event_type}, ${statusLabel(event.status)}, received ${formatTime(event.received_at)}. View details`}>
            <span className={`status-text capitalize tone-${statusTone(event.status)}`}><span className="dot" />{statusLabel(event.status)}</span>
            <span className="event-row-type">{event.event_type}</span>
            <span className="event-row-id">{event.external_event_id}</span>
            <span className="event-row-time">{formatTime(event.received_at)}</span>
            <IconChevronRight className="event-row-chevron" size={15} stroke={1.75} aria-hidden="true" />
          </button></div>)}
        </div>
        {hasMore && <div className="panel-footer"><button className="secondary-button compact" type="button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? 'Loading…' : 'Load more events'}</button></div>}
      </>}
      {selected && <Sheet labelledBy="event-detail-title" title={selected.event_type} description="Receipt record for this event." onClose={() => setSelected(null)}>
        <dl className="kv">
          <div><dt>Status</dt><dd><span className={`badge capitalize badge-${statusTone(selected.status)}`}><span className="dot" />{statusLabel(selected.status)}</span></dd></div>
          <div><dt>External event ID</dt><dd className="mono">{selected.external_event_id}</dd></div>
          <div><dt>Event ID</dt><dd className="mono">#{selected.event_id}</dd></div>
          <div><dt>Environment</dt><dd>{scope?.environment_name ?? selected.environment_id}</dd></div>
          <div><dt>Received</dt><dd>{formatDateTime(selected.received_at, { seconds: true })}</dd></div>
          {selected.processed_at && <div><dt>Processed</dt><dd>{formatDateTime(selected.processed_at, { seconds: true })}</dd></div>}
        </dl>
      </Sheet>}
    </section>
  );
}
