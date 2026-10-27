import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconCircleDashed, IconPlus, IconRefresh, IconSearch } from '@tabler/icons-react';
import { EventTypeCreateDialog } from '../../../features/event-type-create/ui/EventTypeCreateDialog';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { GuidePanel } from '../../../shared/ui/GuidePanel';
import { createCampaign, createGoal, fetchCampaignAnalytics, fetchCampaigns, fetchEventTypes, fetchGoals, updateCampaign, updateGoal, type Campaign, type CampaignAnalytics, type EventType, type Goal } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { Link, useMatchRoute, useNavigate } from '@tanstack/react-router';
import { PageHeader } from '../../../shared/ui/PageHeader';

export type EngagementKind = 'goal' | 'campaign';
const engagementRoutes = {
  goal: { list: '/goals', create: '/goals/new', edit: '/goals/$id/edit' },
  campaign: { list: '/campaigns', create: '/campaigns/new', edit: '/campaigns/$id/edit' },
} as const;
const createEventOption = '__create_event__';
const createGoalOption = '__create_goal__';
const emptyForm = () => ({ id: '', name: '', description: '', eventType: '', targetCount: '1', goalId: '', startsAt: '', endsAt: '' });

const engagementGuide = {
  goal: { purpose: 'Measure whether users reach a concrete target based on trusted events.', steps: ['Choose the event that counts toward success.', 'Set the number of events required.', 'Use the goal in a campaign or inspect progress over time.'], example: 'Create “First 100 orders” and count order.completed 100 times.', result: 'Each matching event moves the user or workspace closer to a visible goal.' },
  campaign: { purpose: 'Group event activity into a time window so you can measure a focused initiative.', steps: ['Choose the event and optional linked goal.', 'Set the campaign start and end dates.', 'Compare event volume and unique users in the results.'], example: 'Run a weekend campaign for purchases from Friday to Sunday.', result: 'You can see how many events and users the campaign generated.' },
} as const;

export function EngagementPage({ kind }: { kind: EngagementKind }) {
  const { scope } = useScope();
  const [goals, setGoals] = useState<Goal[]>([]);
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [analytics, setAnalytics] = useState<CampaignAnalytics[]>([]);
  const [eventTypes, setEventTypes] = useState<EventType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [formKind, setFormKind] = useState<EngagementKind>(kind);
  const [creatingGoalForCampaign, setCreatingGoalForCampaign] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [campaignDraft, setCampaignDraft] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [creatingEvent, setCreatingEvent] = useState(false);
  const navigate = useNavigate();
  const matchRoute = useMatchRoute();
  const routes = engagementRoutes[kind];

  const refresh = useCallback(async () => {
    if (!scope) { setGoals([]); setCampaigns([]); setAnalytics([]); setEventTypes([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    try {
      const [nextGoals, nextCampaigns, report, events] = await Promise.all([fetchGoals(scope), fetchCampaigns(scope), fetchCampaignAnalytics(scope), fetchEventTypes(scope)]);
      setGoals(nextGoals); setCampaigns(nextCampaigns); setAnalytics(report.campaigns); setEventTypes(events);
    } catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Engagement definitions could not be loaded.'); }
    finally { setLoading(false); }
  }, [scope]);
  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => { setQuery(''); }, [kind, scope?.tenant_id, scope?.environment_id]);

  const items = kind === 'goal' ? goals : campaigns;
  const visible = useMemo(() => items.filter((item) => `${item.name} ${item.id} ${item.event_type}`.toLowerCase().includes(query.trim().toLowerCase())).sort((a, b) => a.name.localeCompare(b.name)), [items, query]);
  const title = kind === 'goal' ? 'Goals' : 'Campaigns';
  const singular = kind === 'goal' ? 'goal' : 'campaign';
  const intro = kind === 'goal' ? 'Set a measurable event target for this workspace.' : 'Group event activity into a named time window and follow its results.';

  function beginCreate() { setFormKind(kind); setForm(emptyForm()); setCreatingGoalForCampaign(false); setError(null); }
  function localDateTime(value?: string) { return value ? new Date(value).toISOString().slice(0, 16) : ''; }
  function beginEdit(item: Goal | Campaign) {
    if (kind === 'goal') {
      const goal = item as Goal;
      setFormKind('goal');
      setForm({ ...emptyForm(), id: goal.id, name: goal.name, description: goal.description, eventType: goal.event_type, targetCount: String(goal.target_count) });
    } else {
      const campaign = item as Campaign;
      setFormKind('campaign');
      setForm({ ...emptyForm(), id: campaign.id, name: campaign.name, description: campaign.description, eventType: campaign.event_type, goalId: campaign.goal_id ?? '', startsAt: localDateTime(campaign.starts_at), endsAt: localDateTime(campaign.ends_at) });
    }
    setCreatingGoalForCampaign(false);
    setError(null);
  }
  function update(field: keyof ReturnType<typeof emptyForm>, value: string) { setForm((current) => ({ ...current, [field]: value })); setError(null); }
  function createGoalInline() { setCampaignDraft(form); setForm(emptyForm()); setFormKind('goal'); setCreatingGoalForCampaign(true); setError(null); }
  function backToCampaign() { setForm(campaignDraft); setFormKind('campaign'); setCreatingGoalForCampaign(false); setError(null); }

  // The route says what is open: /new creates, /$id/edit edits, the section root lists.
  const creating = Boolean(matchRoute({ to: routes.create }));
  const editMatch = matchRoute({ to: routes.edit });
  const editId = editMatch ? editMatch.id : null;
  const editing = editId ? (kind === 'goal' ? goals : campaigns as (Goal | Campaign)[]).find((entry) => entry.id === editId) ?? null : null;
  const open = creating || Boolean(editing);
  // Fill the form once per item opened, not on every refresh of the list.
  const formFor = creating ? 'new' : editing?.id ?? null;
  useEffect(() => {
    if (formFor === 'new') beginCreate();
    else if (editing) beginEdit(editing);
  }, [formFor]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setSaving(true);
    setError(null);
    try {
      const id = form.id.trim().toLowerCase(), name = form.name.trim();
      if (!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(id)) throw new Error('Use a lowercase ID with letters, numbers, hyphens, or underscores.');
      if (!name) throw new Error('Enter a name before saving.');
      if (!eventTypes.some((item) => item.name === form.eventType)) throw new Error('Choose an event contract or create one here first.');
      if (formKind === 'goal') {
        const count = Number(form.targetCount);
        if (!Number.isInteger(count) || count < 1) throw new Error('Target events must be a positive whole number.');
        const input = { ...scope, id, name, description: form.description.trim(), event_type: form.eventType, target_count: count };
        const created = editing ? await updateGoal(editing.id, input) : await createGoal(input);
        setGoals((current) => editing ? current.map((item) => item.id === created.id ? created : item) : [created, ...current]);
        if (creatingGoalForCampaign) { setForm({ ...campaignDraft, goalId: created.id }); setFormKind('campaign'); setCreatingGoalForCampaign(false); }
        else { setForm(emptyForm()); void navigate({ to: routes.list }); }
      } else {
        const startsAt = form.startsAt ? new Date(form.startsAt).toISOString() : undefined;
        const endsAt = form.endsAt ? new Date(form.endsAt).toISOString() : undefined;
        if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) throw new Error('End time must be later than start time.');
        const input = { ...scope, id, name, description: form.description.trim(), event_type: form.eventType, goal_id: form.goalId || undefined, starts_at: startsAt, ends_at: endsAt };
        const created = editing ? await updateCampaign(editing.id, input) : await createCampaign(input);
        setCampaigns((current) => editing ? current.map((item) => item.id === created.id ? created : item) : [created, ...current]);
        setForm(emptyForm()); void navigate({ to: routes.list });
      }
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Definition could not be created.'); }
    finally { setSaving(false); }
  }

  const closeEditor = () => void navigate({ to: routes.list });
  const startCreate = () => void navigate({ to: routes.create });
  const activeCount = items.filter((item) => item.active).length;
  return <main className={`page page-resource ${open ? 'page-narrow' : ''}`}>
    <PageHeader breadcrumb={['Workspace', 'Engagement', title, ...(open ? [editing ? 'Edit' : 'Create'] : [])]} title={open ? `${editing ? 'Edit' : 'Add'} ${formKind}` : title} description={open ? formKind === 'goal' ? 'Count the event that represents success.' : 'Measure an event across a named time window.' : intro} actions={open ? <button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to list</button> : <button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add {singular}</button>} />
    {!open && !editId && <GuidePanel id={`guide-${kind}`} title={title} {...engagementGuide[kind]} defaultOpen={!loading && !error && items.length === 0} />}

    {editId && !editing && <section className="panel">{loading || !scope ? <div className="skeleton-rows" aria-label="Loading"><span /><span /><span /></div> : <div className="empty-state"><IconAlertTriangle size={22} stroke={1.5} aria-hidden="true" /><strong>This {singular} was not found</strong><span>It may have been removed, or it belongs to another environment.</span><button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to list</button></div>}</section>}

    {!open && !editId && <section className="panel" aria-label={`${title} list`}>
      <div className="toolbar">
        <p className="tally"><strong>{items.length}</strong> total <span aria-hidden="true">·</span> <strong>{activeCount}</strong> active</p>
        <div className="toolbar-controls"><label className="search-field"><IconSearch size={15} stroke={1.75} aria-hidden="true" /><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={`Search ${title.toLowerCase()}`} aria-label={`Search ${title.toLowerCase()}`} /></label><button className="icon-button" type="button" onClick={() => void refresh()} aria-label={`Refresh ${title.toLowerCase()}`}><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button></div>
      </div>
      {error && <div className="panel-body"><p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Retry</button></p></div>}
      {loading ? <div className="skeleton-rows" aria-label={`Loading ${title.toLowerCase()}`}><span /><span /><span /></div> : items.length === 0 ? <div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>No {title.toLowerCase()} yet</strong><span>{kind === 'goal' ? 'Give your team a measurable outcome to work toward.' : 'Create a campaign to see activity during a specific window.'}</span><button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add {singular}</button></div> : visible.length === 0 ? <div className="empty-state"><IconSearch size={20} stroke={1.5} aria-hidden="true" /><strong>No matching results</strong><span>Try a different search.</span><button className="secondary-button compact" type="button" onClick={() => setQuery('')}>Clear search</button></div> : <ul className="resource-list">{visible.map((item) => { const report = kind === 'campaign' ? analytics.find((entry) => entry.campaign_id === item.id) : undefined; return <li key={item.id} className={`resource-row ${item.active ? '' : 'is-archived'}`}><div className="resource-row-main"><strong>{item.name}</strong><span><span className="mono">{item.event_type}</span> · {kind === 'goal' ? `${(item as Goal).target_count.toLocaleString()} target events` : `${(report?.event_count ?? 0).toLocaleString()} events · ${(report?.unique_users ?? 0).toLocaleString()} users`}</span>{item.description && <small>{item.description}</small>}</div><span className={`badge ${item.active ? 'badge-success' : 'badge-neutral'}`}><span className="dot" />{item.active ? 'Active' : 'Archived'}</span><div className="resource-row-actions"><Link className="text-button" to={routes.edit} params={{ id: item.id }} aria-label={`Edit ${item.name}`}>Edit</Link></div></li>; })}</ul>}
    </section>}

    {open && <section className="panel resource-editor" aria-label={editing ? `Update ${formKind}` : `Set up a ${formKind}`}>
      {creatingGoalForCampaign && <div className="resource-editor-context"><button className="text-button" type="button" onClick={backToCampaign}><IconArrowLeft size={14} stroke={1.75} />Back to campaign</button><span>Creating a goal for your campaign draft</span></div>}
      <form className="resource-form" onSubmit={(event) => void submit(event)}>
        <div className="resource-form-fields">
          <label className="field"><span className="field-label">{formKind === 'goal' ? 'Goal' : 'Campaign'} name</span><input autoFocus required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder={formKind === 'goal' ? 'Complete 100 purchases' : 'Weekend purchase bonus'} /></label>
          <label className="field"><span className="field-label">Stable ID</span><input className="mono" required maxLength={64} value={form.id} onChange={(event) => update('id', event.target.value.toLowerCase())} placeholder={formKind === 'goal' ? 'first-100-orders' : 'weekend-bonus'} disabled={Boolean(editing)} spellCheck={false} /></label>
          <label className="field field-wide"><span className="field-label">Description <em>Optional</em></span><input value={form.description} onChange={(event) => update('description', event.target.value)} placeholder="What this tracks" /></label>
          <div className="field"><span className="field-label">Event to measure</span><CustomSelect value={form.eventType} onChange={(value) => value === createEventOption ? setCreatingEvent(true) : update('eventType', value)} ariaLabel="Event to measure" placeholder="Choose an event" options={[...eventTypes.map((item) => ({ value: item.name, label: item.name })), ...(eventTypes.length ? [{ value: '__event_divider__', label: '', disabled: true, divider: true }] : []), { value: createEventOption, label: 'Create event here…', tone: 'accent' as const }]} /></div>
          {formKind === 'goal' ? <label className="field"><span className="field-label">Target number of events</span><input required type="number" min="1" step="1" value={form.targetCount} onChange={(event) => update('targetCount', event.target.value)} /></label> : <>
            <div className="field"><span className="field-label">Linked goal <em>Optional</em></span><CustomSelect value={form.goalId} onChange={(value) => value === createGoalOption ? createGoalInline() : update('goalId', value)} ariaLabel="Linked goal" placeholder="No linked goal" options={[{ value: '', label: 'No linked goal' }, ...goals.filter((goal) => goal.active).map((goal) => ({ value: goal.id, label: goal.name })), { value: '__goal_divider__', label: '', disabled: true, divider: true }, { value: createGoalOption, label: 'Create goal here…', tone: 'accent' as const }]} /></div>
            <label className="field"><span className="field-label">Starts <em>Optional</em></span><input type="datetime-local" value={form.startsAt} onChange={(event) => update('startsAt', event.target.value)} /></label>
            <label className="field"><span className="field-label">Ends <em>Optional</em></span><input type="datetime-local" value={form.endsAt} onChange={(event) => update('endsAt', event.target.value)} /></label>
          </>}
        </div>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        <div className="resource-form-footer"><span className="form-footer-note">{editing ? 'Editing in' : 'Creating in'} <strong>{scope?.name}</strong> · {scope?.environment_name}</span><button className="secondary-button" type="button" onClick={closeEditor}>Cancel</button><button className="primary-button" type="submit" disabled={saving || !scope}>{saving ? 'Saving…' : editing ? `Save ${formKind}` : `Create ${formKind}`}</button></div>
      </form>
    </section>}
    {creatingEvent && <EventTypeCreateDialog open activeScope={scope} closeOnCreated onClose={() => setCreatingEvent(false)} onCreated={(created) => { setEventTypes((current) => [created, ...current]); update('eventType', created.name); }} />}
  </main>;
}
