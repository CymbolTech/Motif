import { useEffect, useState, type FormEvent } from 'react';
import { IconChartBar, IconPlus } from '@tabler/icons-react';
import {
  archiveMetric,
  createMetric,
  fetchExperimentAnalytics,
  fetchFunnelAnalytics,
  fetchMetrics,
  fetchRetentionAnalytics,
  fetchSegmentAnalytics,
  type ExperimentReport,
  type FunnelReport,
  type MetricDefinition,
  type RetentionReport,
  type SegmentReport,
  type Tenant,
} from '../../../shared/api/client';

type Props = { scope?: Tenant };

function parseSteps(value: string): Array<{ name: string; event_type: string }> {
  return value.split(',').map((part) => part.trim()).filter(Boolean).map((part) => {
    const [name, eventType] = part.split('=').map((item) => item.trim());
    return { name: name || eventType || '', event_type: eventType || name || '' };
  });
}

export function MeasurementPanel({ scope }: Props) {
  const [metrics, setMetrics] = useState<MetricDefinition[]>([]);
  const [funnel, setFunnel] = useState<FunnelReport | null>(null);
  const [retention, setRetention] = useState<RetentionReport | null>(null);
  const [segment, setSegment] = useState<SegmentReport | null>(null);
  const [experiment, setExperiment] = useState<ExperimentReport | null>(null);
  const [metricForm, setMetricForm] = useState({ id: '', name: '', description: '', event_type: '', aggregation: 'count' as 'count' | 'unique_users' });
  const [funnelForm, setFunnelForm] = useState({ steps: 'Viewed=product.viewed, Purchased=order.completed' });
  const [retentionForm, setRetentionForm] = useState({ cohort_event_type: 'signup.completed', return_event_type: 'session.started', intervals: '1,7,30' });
  const [segmentForm, setSegmentForm] = useState({ event_type: 'order.completed', min_events: '2' });
  const [experimentForm, setExperimentForm] = useState({ id: 'checkout_copy', conversion_event_type: 'order.completed', variants: 'control=50,variant=50' });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    setFunnel(null);
    setRetention(null);
    setSegment(null);
    setExperiment(null);
    if (!scope) {
      setMetrics([]);
      return;
    }
    setLoading(true);
    setError(null);
    void fetchMetrics(scope).then(setMetrics).catch((loadError) => setError(loadError instanceof Error ? loadError.message : 'Metrics could not be loaded.')).finally(() => setLoading(false));
  }, [scope?.tenant_id, scope?.environment_id]);

  async function submitMetric(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const created = await createMetric({ ...scope, ...metricForm });
      setMetrics((current) => [created, ...current]);
      setMetricForm({ id: '', name: '', description: '', event_type: '', aggregation: 'count' });
      setMessage(`Metric ${created.name} created.`);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Metric could not be created.');
    }
  }

  async function handleArchive(metric: MetricDefinition) {
    if (!scope) return;
    setError(null);
    try {
      await archiveMetric(scope, metric.id);
      setMetrics((current) => current.map((item) => item.id === metric.id ? { ...item, active: false } : item));
      setMessage(`Metric ${metric.name} archived.`);
    } catch (archiveError) {
      setError(archiveError instanceof Error ? archiveError.message : 'Metric could not be archived.');
    }
  }

  async function runFunnel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const steps = parseSteps(funnelForm.steps);
      if (steps.length < 2) throw new Error('Add at least two funnel steps using Name=event.type.');
      setFunnel(await fetchFunnelAnalytics({ ...scope, steps }));
    } catch (runError) {
      setError(runError instanceof Error ? runError.message : 'Funnel report could not be generated.');
    }
  }

  async function runRetention(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const intervals = retentionForm.intervals.split(',').map((value) => Number(value.trim())).filter((value) => Number.isFinite(value));
      setRetention(await fetchRetentionAnalytics({ ...scope, cohort_event_type: retentionForm.cohort_event_type, return_event_type: retentionForm.return_event_type, intervals }));
    } catch (runError) {
      setError(runError instanceof Error ? runError.message : 'Retention report could not be generated.');
    }
  }

  async function runSegment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      setSegment(await fetchSegmentAnalytics({ ...scope, event_type: segmentForm.event_type, min_events: Number(segmentForm.min_events) }));
    } catch (runError) {
      setError(runError instanceof Error ? runError.message : 'Segment report could not be generated.');
    }
  }

  async function runExperiment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setError(null);
    try {
      const variants = experimentForm.variants.split(',').map((value) => value.trim()).filter(Boolean).map((value) => {
        const [id, weight] = value.split('=').map((item) => item.trim());
        return { id, weight: Number(weight) };
      });
      setExperiment(await fetchExperimentAnalytics({ ...scope, id: experimentForm.id, conversion_event_type: experimentForm.conversion_event_type, variants }));
    } catch (runError) {
      setError(runError instanceof Error ? runError.message : 'Experiment report could not be generated.');
    }
  }

  if (!scope) return <div className="management-note">Select a tenant environment to use measurement tools.</div>;

  return <div className="measurement-stack">
    {error && <div className="management-message error" role="alert">{error}</div>}
    {message && <div className="management-message success" role="status">{message}</div>}
    <div className="management-note"><IconChartBar size={14} /> Metrics are calculated from accepted event history for <strong>{scope.tenant_id} / {scope.environment_id}</strong>. Rates use the returned user counts as their denominator.</div>
    <section className="measurement-section" aria-labelledby="metric-definitions-title">
      <div className="measurement-heading"><strong id="metric-definitions-title">Metric definitions</strong><span>{loading ? 'Loading…' : `${metrics.length} configured`}</span></div>
      <div className="management-list">{metrics.map((metric) => <div className="management-row" key={metric.id}><span><strong>{metric.name}</strong><small>{metric.id} · {metric.event_type} · {metric.aggregation}</small></span><span className={`management-status ${metric.active ? 'active' : 'inactive'}`}>{metric.active ? 'active' : 'archived'}</span>{metric.active && <button className="text-button" type="button" onClick={() => void handleArchive(metric)}>Archive</button>}</div>)}{!loading && metrics.length === 0 && <p className="management-empty">No business metrics defined for this scope.</p>}</div>
      <form className="management-form" onSubmit={(event) => void submitMetric(event)}><span className="management-form-title"><IconPlus size={14} />Create metric</span><div className="form-grid"><label><span>Metric ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={metricForm.id} onChange={(event) => setMetricForm({ ...metricForm, id: event.target.value })} /></label><label><span>Name</span><input required value={metricForm.name} onChange={(event) => setMetricForm({ ...metricForm, name: event.target.value })} /></label></div><label><span>Description</span><input value={metricForm.description} onChange={(event) => setMetricForm({ ...metricForm, description: event.target.value })} /></label><div className="form-grid"><label><span>Event type</span><input required value={metricForm.event_type} onChange={(event) => setMetricForm({ ...metricForm, event_type: event.target.value })} placeholder="order.completed" /></label><label><span>Aggregation</span><select value={metricForm.aggregation} onChange={(event) => setMetricForm({ ...metricForm, aggregation: event.target.value as 'count' | 'unique_users' })}><option value="count">Event count</option><option value="unique_users">Unique users</option></select></label></div><button className="primary-button" type="submit">Create metric</button></form>
    </section>
    <section className="measurement-section" aria-labelledby="funnel-title"><div className="measurement-heading"><strong id="funnel-title">Funnel report</strong><span>{funnel ? `${funnel.steps.length} steps` : 'On demand'}</span></div><form className="management-form" onSubmit={(event) => void runFunnel(event)}><label><span>Ordered steps, comma separated</span><input value={funnelForm.steps} onChange={(event) => setFunnelForm({ steps: event.target.value })} /></label><button className="primary-button" type="submit">Run funnel</button></form>{funnel && <div className="measurement-report-grid">{funnel.steps.map((step) => <div className="measurement-card" key={step.event_type}><strong>{step.name}</strong><span>{step.users.toLocaleString()} users</span><small>{Math.round(step.conversion_rate * 100)}% of first step</small></div>)}</div>}</section>
    <section className="measurement-section" aria-labelledby="retention-title"><div className="measurement-heading"><strong id="retention-title">Retention report</strong><span>{retention ? `${retention.cohort_users.toLocaleString()} cohort users` : 'On demand'}</span></div><form className="management-form" onSubmit={(event) => void runRetention(event)}><div className="form-grid"><label><span>Cohort event</span><input required value={retentionForm.cohort_event_type} onChange={(event) => setRetentionForm({ ...retentionForm, cohort_event_type: event.target.value })} /></label><label><span>Return event</span><input required value={retentionForm.return_event_type} onChange={(event) => setRetentionForm({ ...retentionForm, return_event_type: event.target.value })} /></label></div><label><span>Intervals in days</span><input required value={retentionForm.intervals} onChange={(event) => setRetentionForm({ ...retentionForm, intervals: event.target.value })} /></label><button className="primary-button" type="submit">Run retention</button></form>{retention && <div className="measurement-report-grid">{retention.intervals.map((interval) => <div className="measurement-card" key={interval.days}><strong>Day {interval.days}</strong><span>{interval.returning_users.toLocaleString()} returning</span><small>{Math.round(interval.rate * 100)}% retention</small></div>)}</div>}</section>
    <section className="measurement-section" aria-labelledby="segment-title"><div className="measurement-heading"><strong id="segment-title">Segment report</strong><span>{segment ? `${segment.users.toLocaleString()} users` : 'On demand'}</span></div><form className="management-form" onSubmit={(event) => void runSegment(event)}><div className="form-grid"><label><span>Event type</span><input required value={segmentForm.event_type} onChange={(event) => setSegmentForm({ ...segmentForm, event_type: event.target.value })} /></label><label><span>Minimum events</span><input required type="number" min="1" value={segmentForm.min_events} onChange={(event) => setSegmentForm({ ...segmentForm, min_events: event.target.value })} /></label></div><button className="primary-button" type="submit">Run segment</button></form>{segment && <div className="measurement-report-grid"><div className="measurement-card"><strong>Matching users</strong><span>{segment.users.toLocaleString()}</span><small>{segment.events.toLocaleString()} matching events</small></div></div>}</section>
    <section className="measurement-section" aria-labelledby="experiment-title"><div className="measurement-heading"><strong id="experiment-title">Experiment / control group</strong><span>{experiment ? `${experiment.variants.length} variants` : 'On demand'}</span></div><form className="management-form" onSubmit={(event) => void runExperiment(event)}><div className="form-grid"><label><span>Experiment ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={experimentForm.id} onChange={(event) => setExperimentForm({ ...experimentForm, id: event.target.value })} /></label><label><span>Conversion event</span><input required value={experimentForm.conversion_event_type} onChange={(event) => setExperimentForm({ ...experimentForm, conversion_event_type: event.target.value })} /></label></div><label><span>Variants, id=weight (sum 100)</span><input required value={experimentForm.variants} onChange={(event) => setExperimentForm({ ...experimentForm, variants: event.target.value })} /></label><button className="primary-button" type="submit">Run experiment</button></form>{experiment && <div className="measurement-report-grid">{experiment.variants.map((variant) => <div className="measurement-card" key={variant.id}><strong>{variant.id}</strong><span>{variant.assigned_users.toLocaleString()} assigned</span><small>{variant.converted_users.toLocaleString()} converted · {Math.round(variant.conversion_rate * 100)}%</small></div>)}</div>}</section>
  </div>;
}
