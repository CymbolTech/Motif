import { useCallback, useEffect, useState } from 'react';
import { IconActivity, IconClock, IconEye, IconRefresh, IconX } from '@tabler/icons-react';
import { fetchEvents, type EventRecord } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

function formatReceived(value: string) {
  const date = new Date(value);
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
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
    try {
      const next = await fetchEvents(8, { tenant_id: scope?.tenant_id, environment_id: scope?.environment_id, status: status === 'all' ? undefined : status });
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
    setLoadingMore(true);
    try {
      const next = await fetchEvents(8, { tenant_id: scope?.tenant_id, environment_id: scope?.environment_id, status: status === 'all' ? undefined : status, offset });
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
    <section className="panel recent-events-panel" aria-labelledby="recent-events-title">
      <div className="panel-heading">
        <div><span className="section-label">Signal trace</span><h2 id="recent-events-title">Recent events</h2><span className="panel-context">{scope ? `${scope.tenant_id} / ${scope.environment_id}` : 'Waiting for an active scope'}</span></div>
        <div className="panel-heading-actions"><select className="filter-select" value={status} onChange={(event) => setStatus(event.target.value)} aria-label="Filter events by status"><option value="all">All statuses</option><option value="accepted">Accepted</option><option value="processing">Processing</option><option value="processed">Processed</option><option value="failed">Failed</option></select><button className="text-button" type="button" onClick={() => void refresh()} disabled={loading}><IconRefresh size={15} className={loading ? 'spin' : ''} stroke={1.7} />Refresh</button></div>
      </div>
      {error ? <div className="inline-state error-state" role="alert"><strong>Event feed unavailable</strong><span>{error}</span></div> : loading ? <div className="registry-loading"><div /><div /></div> : events.length === 0 ? <div className="inline-state empty-state"><IconActivity size={19} stroke={1.5} /><strong>No events received</strong><span>Send a test event to see the receipt path here.</span></div> : <><ul className="recent-events-list">{events.map((event) => <li className="recent-event-row" key={event.event_id} onClick={() => setSelected(event)}><div className={`event-status-mark ${event.status}`}><IconActivity size={15} stroke={1.7} /></div><div className="recent-event-copy"><strong>{event.event_type}</strong><span>{event.tenant_id} / {event.environment_id} · {event.external_event_id}</span></div><div className="recent-event-meta"><span className={`event-status ${event.status}`}>{statusLabel(event.status)}</span><span><IconClock size={12} stroke={1.5} />{formatReceived(event.received_at)}</span></div><button className="icon-button subtle" type="button" onClick={(clickEvent) => { clickEvent.stopPropagation(); setSelected(event); }} aria-label={`View event ${event.external_event_id}`}><IconEye size={15} /></button></li>)}</ul>{hasMore && <button className="load-more-button" type="button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? 'Loading…' : 'Load more events'}</button>}</>}
      {selected && <div className="event-detail-drawer" role="dialog" aria-modal="true" aria-labelledby="event-detail-title"><div className="event-detail-heading"><div><span className="section-label">Receipt record</span><h3 id="event-detail-title">{selected.event_type}</h3></div><button className="icon-button" type="button" onClick={() => setSelected(null)} aria-label="Close event details"><IconX size={16} /></button></div><dl><div><dt>External event ID</dt><dd>{selected.external_event_id}</dd></div><div><dt>Event ID</dt><dd>#{selected.event_id}</dd></div><div><dt>Scope</dt><dd>{selected.tenant_id} / {selected.environment_id}</dd></div><div><dt>Status</dt><dd className={`event-status ${selected.status}`}>{statusLabel(selected.status)}</dd></div><div><dt>Received</dt><dd>{new Date(selected.received_at).toLocaleString()}</dd></div>{selected.processed_at && <div><dt>Processed</dt><dd>{new Date(selected.processed_at).toLocaleString()}</dd></div>}</dl></div>}
    </section>
  );
}

