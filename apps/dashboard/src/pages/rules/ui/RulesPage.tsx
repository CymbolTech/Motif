import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconActivity, IconAlertTriangle, IconCheck, IconCircleDashed, IconClock, IconCode, IconPlus, IconRefresh, IconSparkles } from '@tabler/icons-react';
import { RuleCreateDialog } from '../../../features/rule-create/ui/RuleCreateDialog';
import { createRuleVersion, fetchRuleExecutions, fetchRuleVersions, fetchRules, fetchTenants, publishRule, publishRuleVersion, type Rule, type RuleExecution, type RuleVersion, type Tenant } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

function scheduleLabel(rule: Pick<Rule, 'starts_at' | 'ends_at'>): string {
  if (!rule.starts_at && !rule.ends_at) return 'Always active';
  const formatter = new Intl.DateTimeFormat([], { dateStyle: 'medium', timeStyle: 'short' });
  const starts = rule.starts_at ? formatter.format(new Date(rule.starts_at)) : 'Immediately';
  const ends = rule.ends_at ? formatter.format(new Date(rule.ends_at)) : 'No end';
  return `${starts} → ${ends}`;
}

const emptyVersionForm = { event_type: '', conditions: '{\n  "all": []\n}', action: '{\n  "type": "log",\n  "message": "Rule matched"\n}', starts_at: '', ends_at: '' };

export function RulesPage() {
  const { scope } = useScope();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [filter, setFilter] = useState('all');
  const [publishing, setPublishing] = useState<string | null>(null);
  const [executions, setExecutions] = useState<RuleExecution[]>([]);
  const [executionStatus, setExecutionStatus] = useState('all');
  const [executionOffset, setExecutionOffset] = useState(0);
  const [executionHasMore, setExecutionHasMore] = useState(false);
  const [executionLoadingMore, setExecutionLoadingMore] = useState(false);
  const [versionsByRule, setVersionsByRule] = useState<Record<string, RuleVersion[]>>({});
  const [openVersionRule, setOpenVersionRule] = useState<string | null>(null);
  const [versionLoading, setVersionLoading] = useState<string | null>(null);
  const [versionSaving, setVersionSaving] = useState(false);
  const [versionPublishing, setVersionPublishing] = useState<number | null>(null);
  const [versionForm, setVersionForm] = useState(emptyVersionForm);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextTenants, nextRules, nextExecutions] = await Promise.all([
        fetchTenants(),
        fetchRules(scope),
        fetchRuleExecutions(8, { tenant_id: scope?.tenant_id, environment_id: scope?.environment_id, status: executionStatus === 'all' ? undefined : executionStatus }),
      ]);
      setTenants(nextTenants);
      setRules(nextRules);
      setExecutions(nextExecutions);
      setExecutionOffset(nextExecutions.length);
      setExecutionHasMore(nextExecutions.length === 8);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Rules could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [executionStatus, scope]);

  useEffect(() => { void refresh(); }, [refresh]);

  useEffect(() => {
    const interval = window.setInterval(async () => {
      try {
        const next = await fetchRuleExecutions(8, { tenant_id: scope?.tenant_id, environment_id: scope?.environment_id, status: executionStatus === 'all' ? undefined : executionStatus });
        setExecutions(next);
        setExecutionOffset(next.length);
        setExecutionHasMore(next.length === 8);
      } catch {
        // Keep the last visible audit snapshot when a background refresh is unavailable.
      }
    }, 20000);
    return () => window.clearInterval(interval);
  }, [executionStatus, scope]);

  const visibleRules = useMemo(() => filter === 'all' ? rules : rules.filter((rule) => `${rule.tenant_id}-${rule.environment_id}` === filter), [filter, rules]);

  async function handlePublish(rule: Rule) {
    setPublishing(rule.id);
    try {
      const published = await publishRule(rule.id);
      setRules((current) => current.map((item) => item.id === published.id ? published : item));
      await loadVersions(rule.id);
    } catch (publishError) {
      setError(publishError instanceof Error ? publishError.message : 'Rule could not be published.');
    } finally {
      setPublishing(null);
    }
  }

  async function loadVersions(ruleId: string) {
    setVersionLoading(ruleId);
    try {
      const versions = await fetchRuleVersions(ruleId);
      setVersionsByRule((current) => ({ ...current, [ruleId]: versions }));
    } catch (versionError) {
      setError(versionError instanceof Error ? versionError.message : 'Rule versions could not be loaded.');
    } finally {
      setVersionLoading(null);
    }
  }

  async function toggleVersions(rule: Rule) {
    if (openVersionRule === rule.id) {
      setOpenVersionRule(null);
      return;
    }
    setOpenVersionRule(rule.id);
    setVersionForm({ ...emptyVersionForm, event_type: rule.event_type, conditions: JSON.stringify(rule.conditions, null, 2), action: JSON.stringify(rule.action, null, 2), starts_at: rule.starts_at ? rule.starts_at.slice(0, 16) : '', ends_at: rule.ends_at ? rule.ends_at.slice(0, 16) : '' });
    await loadVersions(rule.id);
  }

  async function handleCreateVersion(rule: Rule) {
    setVersionSaving(true);
    setError(null);
    try {
      const conditions = JSON.parse(versionForm.conditions) as Record<string, unknown>;
      const action = JSON.parse(versionForm.action) as Record<string, unknown>;
      const startsAt = versionForm.starts_at ? new Date(versionForm.starts_at).toISOString() : undefined;
      const endsAt = versionForm.ends_at ? new Date(versionForm.ends_at).toISOString() : undefined;
      if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) throw new Error('End time must be later than start time.');
      const nextRule = await createRuleVersion(rule.id, { event_type: versionForm.event_type, conditions, action, starts_at: startsAt, ends_at: endsAt });
      setRules((current) => current.map((item) => item.id === nextRule.id ? nextRule : item));
      await loadVersions(rule.id);
    } catch (versionError) {
      setError(versionError instanceof SyntaxError ? 'Conditions and action must be valid JSON.' : versionError instanceof Error ? versionError.message : 'Rule version could not be created.');
    } finally {
      setVersionSaving(false);
    }
  }

  async function handlePublishVersion(rule: Rule, version: RuleVersion) {
    setVersionPublishing(version.version);
    try {
      const published = await publishRuleVersion(rule.id, version.version);
      setRules((current) => current.map((item) => item.id === published.id ? published : item));
      await loadVersions(rule.id);
    } catch (publishError) {
      setError(publishError instanceof Error ? publishError.message : 'Rule version could not be published.');
    } finally {
      setVersionPublishing(null);
    }
  }

  async function loadMoreExecutions() {
    setExecutionLoadingMore(true);
    try {
      const next = await fetchRuleExecutions(8, { tenant_id: scope?.tenant_id, environment_id: scope?.environment_id, status: executionStatus === 'all' ? undefined : executionStatus, offset: executionOffset });
      setExecutions((current) => [...current, ...next]);
      setExecutionOffset((current) => current + next.length);
      setExecutionHasMore(next.length === 8);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'More executions could not be loaded.');
    } finally {
      setExecutionLoadingMore(false);
    }
  }

  return (
    <main className="overview-main rules-page">
      <header className="overview-header"><div><div className="breadcrumb"><span>Platform</span><span>↗</span><span>Rules</span></div><h1>Turn signals into motion.</h1><p>Draft the behavior that should follow a trusted event.</p></div><div className="header-actions"><button className="icon-button refresh-button" type="button" onClick={() => void refresh()} aria-label="Refresh rules"><IconRefresh size={17} stroke={1.6} /></button><button className="primary-button" type="button" onClick={() => setDialogOpen(true)}><IconPlus size={16} stroke={1.8} />New rule</button></div></header>
      <section className="event-type-summary rule-summary"><div><span className="section-label">Behavior registry</span><div className="summary-number">{rules.length}<span>rules</span></div></div><div className="summary-note"><IconSparkles size={18} /><span>Rules start as drafts and become executable only after publishing.</span></div><div className="scope-chip"><span className="status-dot" />{scope ? `${scope.tenant_id} / ${scope.environment_id}` : 'No active scope'}</div></section>
      <section className="panel registry-panel" aria-labelledby="rules-title"><div className="panel-heading"><div><span className="section-label">Current behavior</span><h2 id="rules-title">Rule registry</h2></div><select className="filter-select" value={filter} onChange={(event) => setFilter(event.target.value)} aria-label="Filter rules by tenant"><option value="all">All loaded scopes</option>{tenants.map((tenant) => <option key={`${tenant.tenant_id}-${tenant.environment_id}`} value={`${tenant.tenant_id}-${tenant.environment_id}`}>{tenant.name} · {tenant.environment_name}</option>)}</select></div>{error ? <div className="inline-state error-state" role="alert"><strong>Rule registry unavailable</strong><span>{error}</span></div> : loading ? <div className="registry-loading"><div /><div /><div /></div> : visibleRules.length === 0 ? <div className="inline-state empty-state"><IconCircleDashed size={20} stroke={1.5} /><strong>No rules drafted</strong><span>Start with a rule that listens for one of your registered event types.</span><button className="primary-button" type="button" onClick={() => setDialogOpen(true)}><IconPlus size={15} />Create first rule</button></div> : <ul className="event-type-list rule-list">{visibleRules.map((rule) => <li className="rule-stack" key={rule.id}><div className="event-type-row rule-row"><div className="event-type-icon"><IconSparkles size={17} stroke={1.5} /></div><div className="event-type-copy"><strong>{rule.name}</strong><span>{rule.event_type} · {rule.tenant_id} / {rule.environment_id}</span><small className="rule-schedule">v{rule.version} · {scheduleLabel(rule)}</small></div><div className={`rule-status ${rule.status}`}><span />{rule.status}</div>{rule.status === 'draft' && <button className="text-button accent" type="button" onClick={() => void handlePublish(rule)} disabled={publishing === rule.id}>{publishing === rule.id ? 'Publishing…' : <><IconCheck size={14} />Publish</>}</button>}<button className="text-button" type="button" onClick={() => void toggleVersions(rule)}><IconClock size={14} />{openVersionRule === rule.id ? 'Hide history' : 'History'}</button></div>{openVersionRule === rule.id && <div className="version-panel"><div className="version-panel-heading"><div><span className="section-label">Immutable history</span><strong>Version registry</strong></div><span className="version-count">{versionLoading === rule.id ? 'Loading…' : `${(versionsByRule[rule.id] ?? []).length} versions`}</span></div>{(versionsByRule[rule.id] ?? []).map((version) => <div className="version-row" key={version.id}><span className={`version-badge ${version.status}`}>v{version.version}</span><span className="version-copy"><strong>{version.event_type}</strong><small>{scheduleLabel(version)} · {new Date(version.created_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}</small></span><span className={`version-status ${version.status}`}>{version.status}</span>{version.status === 'draft' && <button className="text-button accent" type="button" onClick={() => void handlePublishVersion(rule, version)} disabled={versionPublishing === version.version}>{versionPublishing === version.version ? 'Publishing…' : 'Publish'}</button>}</div>)}<div className="version-editor"><div className="version-editor-heading"><IconCode size={15} /><strong>Draft next version</strong><span>Creating a version keeps previous behavior intact.</span></div><label><span>Event type</span><input value={versionForm.event_type} onChange={(event) => setVersionForm({ ...versionForm, event_type: event.target.value })} /></label><div className="form-grid schedule-fields"><label><span>Starts at <em>optional</em></span><input type="datetime-local" value={versionForm.starts_at} onChange={(event) => setVersionForm({ ...versionForm, starts_at: event.target.value })} /></label><label><span>Ends at <em>optional</em></span><input type="datetime-local" value={versionForm.ends_at} onChange={(event) => setVersionForm({ ...versionForm, ends_at: event.target.value })} /></label></div><div className="form-grid"><label><span>Conditions</span><textarea value={versionForm.conditions} onChange={(event) => setVersionForm({ ...versionForm, conditions: event.target.value })} spellCheck={false} /></label><label><span>Action</span><textarea value={versionForm.action} onChange={(event) => setVersionForm({ ...versionForm, action: event.target.value })} spellCheck={false} /></label></div><button className="primary-button" type="button" onClick={() => void handleCreateVersion(rule)} disabled={versionSaving}>{versionSaving ? 'Saving…' : 'Create draft version'}</button></div></div>}</li>)}</ul>}</section>
      <section className="panel execution-panel" aria-labelledby="execution-title"><div className="panel-heading"><div><span className="section-label">Worker audit</span><h2 id="execution-title">Recent matches</h2></div><div className="panel-heading-actions"><select className="filter-select" value={executionStatus} onChange={(event) => setExecutionStatus(event.target.value)} aria-label="Filter executions by status"><option value="all">All statuses</option><option value="recorded">Recorded</option><option value="pending">Pending</option><option value="failed">Failed</option></select><span className="execution-count"><IconActivity size={14} />{executions.length} loaded</span></div></div>{loading ? <div className="registry-loading"><div /><div /></div> : executions.length === 0 ? <div className="inline-state empty-state"><IconActivity size={19} stroke={1.5} /><strong>No rule matches yet</strong><span>Publish a rule, then send a matching event to record its action.</span></div> : <><ul className="execution-list">{executions.map((execution) => <li key={execution.id}><span className={`execution-mark ${execution.status}`}><IconCheck size={14} /></span><span><strong>{execution.rule_name}</strong><small>{execution.event_type} · event #{execution.event_id} · v{execution.rule_version_id?.split(':v')[1] ?? '?'}</small></span><span className="execution-detail"><em>{execution.action_type}</em><small>{execution.attempts ?? 0} attempts{execution.last_error ? ` · ${execution.last_error}` : ''}</small></span>{execution.status === 'failed' && <IconAlertTriangle className="execution-warning" size={15} />}</li>)}</ul>{executionHasMore && <button className="load-more-button" type="button" onClick={() => void loadMoreExecutions()} disabled={executionLoadingMore}>{executionLoadingMore ? 'Loading…' : 'Load more matches'}</button>}</>}</section>
      <RuleCreateDialog open={dialogOpen} tenants={tenants} activeScope={scope} onClose={() => setDialogOpen(false)} onCreated={(rule) => setRules((current) => [rule, ...current.filter((item) => item.id !== rule.id)])} />
    </main>
  );
}

