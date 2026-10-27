import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconChevronDown, IconCircleDashed, IconPlus, IconRefresh, IconSearch } from '@tabler/icons-react';
import { CurrencyCreateDialog } from '../../../features/currency-create/ui/CurrencyCreateDialog';
import { EventTypeCreateDialog } from '../../../features/event-type-create/ui/EventTypeCreateDialog';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { GuidePanel } from '../../../shared/ui/GuidePanel';
import { createAchievement, createCurrency, createLevel, createMission, createReward, fetchAchievements, fetchCurrencies, fetchEventTypes, fetchLevels, fetchMissions, fetchRewards, updateAchievement, updateCatalogStatus, updateCurrency, updateLevel, updateMission, updateReward, type Achievement, type Currency, type EventType, type Level, type Mission, type Reward, type Tenant } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { Link, useMatchRoute, useNavigate } from '@tanstack/react-router';
import { PageHeader } from '../../../shared/ui/PageHeader';

export type CatalogKind = 'currency' | 'level' | 'mission' | 'achievement' | 'reward';
type CatalogItem = Currency | Level | Mission | Achievement | Reward;
type FormState = { name: string; code: string; description: string; eventType: string; count: string; threshold: string; precision: string; expiryDays: string; costCurrency: string; costAmount: string; payloadJson: string };

const copy: Record<CatalogKind, { title: string; singular: string; intro: string; empty: string; guide: { purpose: string; steps: string[]; example: string; result: string } }> = {
  currency: { title: 'Currencies', singular: 'currency', intro: 'Balances users can earn and spend. Each currency has its own ledger and policy.', empty: 'Create a currency before an automation awards points.', guide: { purpose: 'Store points, coins, miles, or any balance users can earn and spend.', steps: ['Create a name and stable code such as coins.', 'Choose the precision and any expiry policy.', 'Select the currency in an automation outcome.'], example: 'When order.completed arrives, give the user 10 coins.', result: 'The user balance increases by 10 coins in the ledger.' } },
  level: { title: 'Levels', singular: 'level', intro: 'Name the milestones users reach as their XP grows.', empty: 'Add the first XP threshold to start a progression ladder.', guide: { purpose: 'Turn accumulated XP into clear progression milestones users can understand.', steps: ['Create levels with ascending XP thresholds.', 'Name each milestone so it is meaningful to users.', 'Give XP from automations when users complete actions.'], example: 'Set Gold at 500 XP and award 50 XP after a completed order.', result: 'When the user reaches 500 XP, their level becomes Gold.' } },
  mission: { title: 'Missions', singular: 'mission', intro: 'Track repeated actions toward a goal for each user.', empty: 'Create a mission tied to a trusted event.', guide: { purpose: 'Give users a measurable, repeatable objective such as completing five purchases.', steps: ['Choose the event that counts toward progress.', 'Set how many times it must happen.', 'Use an automation to celebrate or reward completion.'], example: 'Count order.completed events until a user reaches 5.', result: 'The mission shows progress and becomes complete at the fifth order.' } },
  achievement: { title: 'Achievements', singular: 'achievement', intro: 'Give users a lasting milestone when they complete an activity.', empty: 'Create an achievement tied to a trusted event.', guide: { purpose: 'Recognize a milestone once and keep it visible as part of the user history.', steps: ['Choose the event and required count.', 'Give the achievement a memorable name.', 'Optionally attach a reward automation for the unlock.'], example: 'Unlock “First 10 orders” after ten order.completed events.', result: 'The user receives a permanent achievement when the count is reached.' } },
  reward: { title: 'Rewards', singular: 'reward', intro: 'Define what users can redeem, for free or with a points currency.', empty: 'Add a reward to make earned points useful.', guide: { purpose: 'Define something valuable users can redeem with their earned balance.', steps: ['Create the reward and describe what the user receives.', 'Choose a currency and redemption cost, or make it free.', 'Connect the redemption to your provider workflow.'], example: 'Create a $10 coffee voucher that costs 500 coins.', result: 'Eligible users can redeem it and the system records the delivery.' } },
};

const catalogRoutes = {
  currency: { list: '/currencies', create: '/currencies/new', edit: '/currencies/$id/edit' },
  level: { list: '/levels', create: '/levels/new', edit: '/levels/$id/edit' },
  mission: { list: '/missions', create: '/missions/new', edit: '/missions/$id/edit' },
  achievement: { list: '/achievements', create: '/achievements/new', edit: '/achievements/$id/edit' },
  reward: { list: '/rewards', create: '/rewards/new', edit: '/rewards/$id/edit' },
} as const;

const emptyForm = (): FormState => ({ name: '', code: '', description: '', eventType: '', count: '1', threshold: '0', precision: '0', expiryDays: '', costCurrency: '', costAmount: '', payloadJson: '{}' });
const createEventOption = '__create_event__';
const createCurrencyOption = '__create_currency__';

async function fetchItems(kind: CatalogKind, scope: Tenant): Promise<CatalogItem[]> {
  if (kind === 'currency') return fetchCurrencies(scope);
  if (kind === 'level') return fetchLevels(scope);
  if (kind === 'mission') return fetchMissions(scope);
  if (kind === 'achievement') return fetchAchievements(scope);
  return fetchRewards(scope);
}

function itemDetail(kind: CatalogKind, item: CatalogItem): string {
  if (kind === 'currency') {
    const currency = item as Currency;
    const expiry = currency.policy?.expiry_days;
    return `${currency.code} · ${currency.unit_scale === 0 ? 'whole units' : `${currency.unit_scale} decimal places`}${typeof expiry === 'number' && expiry > 0 ? ` · expires after ${expiry} days` : ''}`;
  }
  if (kind === 'level') { const level = item as Level; return `Level ${level.level} · ${level.xp_threshold.toLocaleString()} XP`; }
  if (kind === 'mission') { const mission = item as Mission; return `${mission.event_type} · ${mission.target_count} events to complete`; }
  if (kind === 'achievement') { const achievement = item as Achievement; return `${achievement.event_type} · ${achievement.required_count} events to unlock`; }
  const reward = item as Reward;
  return reward.cost_currency && reward.cost_amount ? `${reward.cost_amount} ${reward.cost_currency}` : 'Free redemption';
}

export function CatalogPage({ kind }: { kind: CatalogKind }) {
  const { scope } = useScope();
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [eventTypes, setEventTypes] = useState<EventType[]>([]);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<'all' | 'active' | 'archived'>('all');
  const [form, setForm] = useState<FormState>(emptyForm);
  const [costMode, setCostMode] = useState<'free' | 'points'>('free');
  const [advancedPayload, setAdvancedPayload] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toggling, setToggling] = useState<string | null>(null);
  const [dependency, setDependency] = useState<'event' | 'currency' | null>(null);
  const navigate = useNavigate();
  const matchRoute = useMatchRoute();
  const routes = catalogRoutes[kind];

  const refresh = useCallback(async () => {
    if (!scope) { setItems([]); setEventTypes([]); setCurrencies([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    try {
      const [nextItems, nextEvents, nextCurrencies] = await Promise.all([
        fetchItems(kind, scope),
        kind === 'mission' || kind === 'achievement' ? fetchEventTypes(scope) : Promise.resolve([]),
        kind === 'reward' ? fetchCurrencies(scope) : Promise.resolve([]),
      ]);
      setItems(nextItems);
      setEventTypes(nextEvents);
      setCurrencies(nextCurrencies);
    } catch (loadError) { setError(loadError instanceof Error ? loadError.message : `${copy[kind].title} could not be loaded.`); }
    finally { setLoading(false); }
  }, [kind, scope]);

  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => { setQuery(''); setFilter('all'); setCostMode('free'); setDependency(null); }, [kind, scope?.tenant_id, scope?.environment_id]);
  const visible = useMemo(() => {
    const term = query.trim().toLowerCase();
    return items.filter((item) => (filter === 'all' || item.active === (filter === 'active')) && (!term || `${item.name} ${itemDetail(kind, item)}`.toLowerCase().includes(term)))
      .sort((a, b) => kind === 'level' ? (a as Level).level - (b as Level).level : a.name.localeCompare(b.name));
  }, [filter, items, kind, query]);

  function update(field: keyof FormState, value: string) { setForm((current) => ({ ...current, [field]: value })); setError(null); }
  function beginCreate() { setForm(emptyForm()); setCostMode('free'); setAdvancedPayload(false); setError(null); }
  function beginEdit(item: CatalogItem) {
    if (kind === 'currency') {
      const currency = item as Currency;
      const expiry = currency.policy?.expiry_days;
      setForm({ ...emptyForm(), name: currency.name, code: currency.code, precision: String(currency.unit_scale), expiryDays: typeof expiry === 'number' ? String(expiry) : '' });
    } else if (kind === 'level') {
      const level = item as Level;
      setForm({ ...emptyForm(), name: level.name, count: String(level.level), threshold: String(level.xp_threshold) });
    } else if (kind === 'mission') {
      const mission = item as Mission;
      setForm({ ...emptyForm(), name: mission.name, eventType: mission.event_type, count: String(mission.target_count) });
    } else if (kind === 'achievement') {
      const achievement = item as Achievement;
      setForm({ ...emptyForm(), name: achievement.name, description: achievement.description, eventType: achievement.event_type, count: String(achievement.required_count) });
    } else {
      const reward = item as Reward;
      const hasCost = Boolean(reward.cost_currency && reward.cost_amount);
      setForm({ ...emptyForm(), name: reward.name, description: reward.description, costCurrency: reward.cost_currency ?? '', costAmount: reward.cost_amount ? String(reward.cost_amount) : '', payloadJson: JSON.stringify(reward.payload ?? {}, null, 2) });
      setCostMode(hasCost ? 'points' : 'free');
      setAdvancedPayload(Object.keys(reward.payload ?? {}).length > 0);
    }
    setError(null);
  }

  // The route says what is open: /new creates, /$id/edit edits, the section root lists.
  const creating = Boolean(matchRoute({ to: routes.create }));
  const editMatch = matchRoute({ to: routes.edit });
  const editId = editMatch ? editMatch.id : null;
  const editing = editId ? items.find((entry) => entry.id === editId) ?? null : null;
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
      const name = form.name.trim();
      if (!name) throw new Error('Enter a name before saving.');
      if (kind === 'currency') {
        const code = form.code.trim().toLowerCase();
        if (!/^[a-z0-9][a-z0-9_-]{0,31}$/.test(code)) throw new Error('Use a lowercase currency code, such as coins.');
        const expiry = form.expiryDays.trim();
        if (expiry && (!Number.isInteger(Number(expiry)) || Number(expiry) < 1)) throw new Error('Expiry must be a positive number of days.');
        const input = { ...scope, code, name, unit_scale: Number(form.precision), policy: expiry ? { expiry_days: Number(expiry) } : {} };
        if (editing) await updateCurrency(editing.id, input); else await createCurrency(input);
      } else if (kind === 'level') {
        const level = Number(form.count), xp = Number(form.threshold);
        if (!Number.isInteger(level) || level < 1 || !Number.isInteger(xp) || xp < 0) throw new Error('Level must be positive and XP threshold cannot be negative.');
        const input = { ...scope, level, name, xp_threshold: xp };
        if (editing) await updateLevel(editing.id, input); else await createLevel(input);
      } else if (kind === 'mission' || kind === 'achievement') {
        if (!eventTypes.some((item) => item.name === form.eventType)) throw new Error('Choose an event contract or create one here first.');
        const count = Number(form.count);
        if (!Number.isInteger(count) || count < 1) throw new Error('Target count must be a positive whole number.');
        if (kind === 'mission') {
          const input = { ...scope, name, event_type: form.eventType, target_count: count };
          if (editing) await updateMission(editing.id, input); else await createMission(input);
        } else {
          const input = { ...scope, name, description: form.description.trim(), event_type: form.eventType, required_count: count };
          if (editing) await updateAchievement(editing.id, input); else await createAchievement(input);
        }
      } else {
        if (costMode === 'points' && !currencies.some((item) => item.active && item.code === form.costCurrency)) throw new Error('Choose an active currency or create one here first.');
        const amount = Number(form.costAmount);
        if (costMode === 'points' && (!Number.isInteger(amount) || amount < 1)) throw new Error('Reward cost must be a positive whole number.');
        let payload: Record<string, unknown> = {};
        if (advancedPayload) {
          try { payload = JSON.parse(form.payloadJson) as Record<string, unknown>; }
          catch { throw new Error('Delivery data must be valid JSON.'); }
          if (!payload || typeof payload !== 'object' || Array.isArray(payload)) throw new Error('Delivery data must be a JSON object.');
        }
        const input = { ...scope, name, description: form.description.trim(), reward_type: 'digital_unlock', cost_currency: costMode === 'points' ? form.costCurrency : undefined, cost_amount: costMode === 'points' ? amount : undefined, payload };
        if (editing) await updateReward(editing.id, input); else await createReward(input);
      }
      setForm(emptyForm());
      void navigate({ to: routes.list });
      await refresh();
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : `${copy[kind].singular} could not be created.`); }
    finally { setSaving(false); }
  }

  async function toggle(item: CatalogItem) {
    if (!scope) return;
    setToggling(item.id);
    setError(null);
    try {
      await updateCatalogStatus({ ...scope, resource: kind, id: item.id, active: !item.active });
      setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, active: !item.active } : entry));
    } catch (statusError) { setError(statusError instanceof Error ? statusError.message : 'Status could not be changed.'); }
    finally { setToggling(null); }
  }

  const activeCount = items.filter((item) => item.active).length;
  const closeEditor = () => void navigate({ to: routes.list });
  const startCreate = () => void navigate({ to: routes.create });
  return <main className={`page page-resource ${open ? 'page-narrow' : ''}`}>
    <PageHeader breadcrumb={['Workspace', 'Loyalty', copy[kind].title, ...(open ? [editing ? 'Edit' : 'Create'] : [])]} title={open ? `${editing ? 'Edit' : 'Add'} ${copy[kind].singular}` : copy[kind].title} description={copy[kind].intro} actions={open ? <button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to list</button> : <button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add {copy[kind].singular}</button>} />

    {!open && !editId && <GuidePanel id={`guide-${kind}`} title={copy[kind].title} {...copy[kind].guide} defaultOpen={!loading && !error && items.length === 0} />}

    {editId && !editing && <section className="panel">{loading || !scope ? <div className="skeleton-rows" aria-label="Loading"><span /><span /><span /></div> : <div className="empty-state"><IconAlertTriangle size={22} stroke={1.5} aria-hidden="true" /><strong>This {copy[kind].singular} was not found</strong><span>It may have been removed, or it belongs to another environment.</span><button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to list</button></div>}</section>}

    {!open && !editId && <section className="panel" aria-label={`${copy[kind].title} list`}>
      <div className="toolbar">
        <p className="tally"><strong>{items.length}</strong> total <span aria-hidden="true">·</span> <strong>{activeCount}</strong> active</p>
        <div className="toolbar-controls">
          <label className="search-field"><IconSearch size={15} stroke={1.75} aria-hidden="true" /><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={`Search ${copy[kind].title.toLowerCase()}`} aria-label={`Search ${copy[kind].title.toLowerCase()}`} /></label>
          <CustomSelect className="toolbar-select" value={filter} onChange={(value) => setFilter(value as typeof filter)} ariaLabel="Filter status" options={[{ value: 'all', label: 'All statuses' }, { value: 'active', label: 'Active' }, { value: 'archived', label: 'Archived' }]} />
          <button className="icon-button" type="button" onClick={() => void refresh()} aria-label={`Refresh ${copy[kind].title.toLowerCase()}`}><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button>
        </div>
      </div>
      {error && <div className="panel-body"><p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Retry</button></p></div>}
      {loading ? <div className="skeleton-rows" aria-label={`Loading ${copy[kind].title.toLowerCase()}`}><span /><span /><span /></div> : items.length === 0 ? <div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>No {copy[kind].title.toLowerCase()} yet</strong><span>{copy[kind].empty}</span><button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add {copy[kind].singular}</button></div> : visible.length === 0 ? <div className="empty-state"><IconSearch size={20} stroke={1.5} aria-hidden="true" /><strong>No matching results</strong><span>Try another search or status filter.</span><button className="secondary-button compact" type="button" onClick={() => { setQuery(''); setFilter('all'); }}>Clear filters</button></div> : <ul className="resource-list">{visible.map((item) => <li key={item.id} className={`resource-row ${item.active ? '' : 'is-archived'}`}><div className="resource-row-main"><strong>{item.name}</strong><span>{itemDetail(kind, item)}</span>{kind === 'achievement' && (item as Achievement).description && <small>{(item as Achievement).description}</small>}{kind === 'reward' && (item as Reward).description && <small>{(item as Reward).description}</small>}</div><span className={`badge ${item.active ? 'badge-success' : 'badge-neutral'}`}><span className="dot" />{item.active ? 'Active' : 'Archived'}</span><div className="resource-row-actions"><Link className="text-button" to={routes.edit} params={{ id: item.id }} aria-label={`Edit ${item.name}`}>Edit</Link><button className="text-button" type="button" disabled={toggling === item.id} onClick={() => void toggle(item)} aria-label={`${item.active ? 'Archive' : 'Activate'} ${item.name}`}>{toggling === item.id ? 'Saving…' : item.active ? 'Archive' : 'Activate'}</button></div></li>)}</ul>}
    </section>}

    {open && <section className="panel resource-editor" aria-label={editing ? `Update ${copy[kind].singular}` : `Set up a ${copy[kind].singular}`}><form className="resource-form" onSubmit={(event) => void submit(event)}>
      <div className="resource-form-fields">
      {kind === 'currency' && <><label className="field"><span className="field-label">Display name</span><input autoFocus required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Coins" /><small className="field-help">The name people will see in the product.</small></label><label className="field"><span className="field-label">Currency code</span><input className="mono" required maxLength={32} value={form.code} onChange={(event) => update('code', event.target.value.toLowerCase())} placeholder="coins" disabled={Boolean(editing)} /><small className="field-help">{editing ? 'Currency codes are stable identifiers and cannot be changed.' : 'A stable identifier used by automations and the API.'}</small></label><div className="field"><span className="field-label">Precision</span><CustomSelect value={form.precision} onChange={(value) => update('precision', value)} ariaLabel="Currency precision" options={[{ value: '0', label: 'Whole units (1, 2, 3)' }, { value: '1', label: 'One decimal place (1.0)' }, { value: '2', label: 'Two decimal places (1.00)' }]} /></div><label className="field"><span className="field-label">Expires after days <em>Optional</em></span><input type="number" min="1" step="1" value={form.expiryDays} onChange={(event) => update('expiryDays', event.target.value)} placeholder="Never expires" /><small className="field-help">Leave empty when earned points should not expire.</small></label></>}
      {kind === 'level' && <><label className="field"><span className="field-label">Level number</span><input autoFocus required type="number" min="1" step="1" value={form.count} onChange={(event) => update('count', event.target.value)} disabled={Boolean(editing)} /></label><label className="field"><span className="field-label">Level name</span><input required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Explorer" /></label><label className="field field-wide"><span className="field-label">XP needed to reach this level</span><input required type="number" min="0" step="1" value={form.threshold} onChange={(event) => update('threshold', event.target.value)} /></label></>}
      {(kind === 'mission' || kind === 'achievement') && <><label className="field field-wide"><span className="field-label">{kind === 'mission' ? 'Mission' : 'Achievement'} name</span><input autoFocus required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder={kind === 'mission' ? 'Complete three purchases' : 'First purchase'} /></label>{kind === 'achievement' && <label className="field field-wide"><span className="field-label">Description <em>Optional</em></span><input maxLength={500} value={form.description} onChange={(event) => update('description', event.target.value)} placeholder="Awarded after a customer's first purchase" /></label>}<div className="field"><span className="field-label">Event to track</span><CustomSelect value={form.eventType} onChange={(value) => value === createEventOption ? setDependency('event') : update('eventType', value)} ariaLabel="Event to track" placeholder="Choose an event" options={[...eventTypes.map((item) => ({ value: item.name, label: item.name })), ...(eventTypes.length ? [{ value: '__event_divider__', label: '', disabled: true, divider: true }] : []), { value: createEventOption, label: 'Create event here…', tone: 'accent' as const }]} /><small className="field-help">{eventTypes.length === 0 ? 'No event in this workspace yet. Create one here to continue.' : 'The target increases each time this event is processed.'}</small></div><label className="field"><span className="field-label">{kind === 'mission' ? 'Events needed to complete' : 'Events needed to unlock'}</span><input required type="number" min="1" step="1" value={form.count} onChange={(event) => update('count', event.target.value)} /></label></>}
      {kind === 'reward' && <><label className="field field-wide"><span className="field-label">Reward name</span><input autoFocus required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Free delivery" /></label><label className="field field-wide"><span className="field-label">Description <em>Optional</em></span><input maxLength={500} value={form.description} onChange={(event) => update('description', event.target.value)} placeholder="What the user receives" /></label><div className="field field-wide"><span className="field-label">Redemption cost</span><div className="segmented" role="group" aria-label="Redemption cost"><button type="button" aria-pressed={costMode === 'free'} onClick={() => setCostMode('free')}>Free</button><button type="button" aria-pressed={costMode === 'points'} onClick={() => setCostMode('points')}>Costs points</button></div></div>{costMode === 'points' && <><div className="field"><span className="field-label">Points currency</span><CustomSelect value={form.costCurrency} onChange={(value) => value === createCurrencyOption ? setDependency('currency') : update('costCurrency', value)} ariaLabel="Points currency" placeholder="Choose a currency" options={[...currencies.filter((item) => item.active).map((item) => ({ value: item.code, label: `${item.name} · ${item.code}` })), ...(currencies.some((item) => item.active) ? [{ value: '__currency_divider__', label: '', disabled: true, divider: true }] : []), { value: createCurrencyOption, label: 'Create currency here…', tone: 'accent' as const }]} /></div><label className="field"><span className="field-label">Points required</span><input required type="number" min="1" step="1" value={form.costAmount} onChange={(event) => update('costAmount', event.target.value)} placeholder="100" /></label></>}<div className="field field-wide resource-advanced"><button className="text-button" type="button" aria-expanded={advancedPayload} onClick={() => setAdvancedPayload((current) => !current)}><IconChevronDown className={advancedPayload ? 'is-flipped' : ''} size={15} stroke={1.75} />{advancedPayload ? 'Hide delivery data' : 'Advanced: delivery data'}</button>{advancedPayload && <label className="field"><span className="field-label">Provider payload JSON</span><textarea className="code-input" value={form.payloadJson} onChange={(event) => update('payloadJson', event.target.value)} spellCheck={false} /><small className="field-help">Optional data passed to the reward delivery provider. Most rewards can use an empty object.</small></label>}</div></>}
      </div>
      {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
      <div className="resource-form-footer"><span className="form-footer-note">{editing ? 'Editing in' : 'Creating in'} <strong>{scope?.name}</strong> · {scope?.environment_name}</span><button className="secondary-button" type="button" onClick={closeEditor}>Cancel</button><button className="primary-button" type="submit" disabled={saving || !scope}>{saving ? 'Saving…' : editing ? `Save ${copy[kind].singular}` : `Create ${copy[kind].singular}`}</button></div>
    </form></section>}
    {dependency === 'event' && <EventTypeCreateDialog open activeScope={scope} closeOnCreated onClose={() => setDependency(null)} onCreated={(created) => { setEventTypes((current) => [created, ...current]); update('eventType', created.name); }} />}
    {dependency === 'currency' && <CurrencyCreateDialog open activeScope={scope} onClose={() => setDependency(null)} onCreated={(created) => { setCurrencies((current) => [created, ...current]); update('costCurrency', created.code); }} />}
  </main>;
}
