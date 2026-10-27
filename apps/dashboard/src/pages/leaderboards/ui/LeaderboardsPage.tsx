import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconChevronRight, IconCircleDashed, IconLock, IconPencil, IconTrash, IconPlayerPlay, IconPlus, IconRefresh, IconSearch } from '@tabler/icons-react';
import { cancelLeaderboardRound, createLeaderboard, deleteLeaderboard, createLeaderboardRound, endLeaderboardRound, fetchCurrencies, fetchLeaderboardRounds, fetchLeaderboards, fetchLeaderboardStandings, updateCatalogStatus, updateLeaderboard, updateLeaderboardRound, type Currency, type Leaderboard, type LeaderboardRound, type LeaderboardSchedule, type LeaderboardStandings } from '../../../shared/api/client';
import { CurrencyCreateDialog } from '../../../features/currency-create/ui/CurrencyCreateDialog';
import { formatDateTime } from '../../../shared/lib/format';
import { useScope, type ActiveScope } from '../../../shared/scope/ScopeContext';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { GuidePanel } from '../../../shared/ui/GuidePanel';
import { PageHeader } from '../../../shared/ui/PageHeader';
import { Sheet } from '../../../shared/ui/Sheet';
import { Link, useMatchRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { TimezoneSelect } from '../../../shared/ui/TimezoneSelect';

type Ids = { tenant_id: string; environment_id: string };

const pageSize = 25;
const createCurrencyOption = '__create_currency__';
const weekdays = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];
const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

/** The viewer's first day of the week (Saturday in Iran, Monday in most of Europe). */
function localeWeekStart() {
  try {
    const locale = new Intl.Locale(navigator.language) as Intl.Locale & { weekInfo?: { firstDay: number }; getWeekInfo?: () => { firstDay: number } };
    const day = locale.getWeekInfo?.().firstDay ?? locale.weekInfo?.firstDay;
    return day ? weekdays[day % 7] : 'monday';
  } catch {
    return 'monday';
  }
}

const guide = {
  purpose: 'Rank users by what they earn during a round, then read the ranking from your backend by the board key.',
  steps: ['Choose what to rank: points in one currency, or XP.', 'Pick manual rounds you start yourself, or a recurring schedule.', 'Read GET /v1/leaderboards/{key} from your backend to show the current round.'],
  example: 'A weekly board you rotate by hand, and a monthly board that resets on the 1st.',
  result: 'Spending, expiry, conversions, and refunds never change a ranking.',
};

function ids(scope: ActiveScope): Ids {
  return { tenant_id: scope.tenant_id, environment_id: scope.environment_id };
}

/** The key a new board gets from its name: a slug, numbered when another board already uses it. */
function keyFromName(name: string, takenKeys: string[]) {
  const base = slug(name).slice(0, 60) || 'leaderboard';
  let key = base;
  for (let suffix = 2; takenKeys.includes(key); suffix++) key = `${base}-${suffix}`;
  return key;
}

function slug(value: string) {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 64);
}

/** datetime-local works in the browser's zone; the API speaks RFC 3339. */
function toLocalInput(value: string | null | undefined) {
  if (!value) return '';
  const date = new Date(value);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}

function fromLocalInput(value: string) {
  return new Date(value).toISOString();
}

function describeMetric(board: Leaderboard, currencies: Currency[]) {
  if (board.metric.type === 'xp') return 'XP earned';
  const currency = currencies.find((item) => item.code === board.metric.currency);
  return `${currency?.name ?? board.metric.currency} earned`;
}

function describeSchedule(schedule: LeaderboardSchedule) {
  if (schedule.type === 'manual') return 'Manual rounds';
  const interval = schedule.interval ?? 1;
  return `Every ${interval > 1 ? `${interval} ` : ''}${schedule.every}${interval > 1 ? 's' : ''} · ${schedule.timezone}`;
}

function describeRound(round: LeaderboardRound | null) {
  if (!round) return 'No round yet';
  if (round.status === 'ended') return `${round.name} ended ${formatDateTime(round.ends_at!)}`;
  if (round.status === 'scheduled') return `${round.name} starts ${formatDateTime(round.starts_at)}`;
  return round.ends_at ? `${round.name} · ends ${formatDateTime(round.ends_at)}` : `${round.name} · open until ended`;
}

/** "in 6 days" or "2 hours ago": how far a round's end is from now. */
function relativeTime(value: string) {
  const seconds = (new Date(value).getTime() - Date.now()) / 1000;
  const units: [Intl.RelativeTimeFormatUnit, number][] = [['day', 86400], ['hour', 3600], ['minute', 60]];
  const [unit, size] = units.find(([, size]) => Math.abs(seconds) >= size) ?? ['minute', 60];
  return new Intl.RelativeTimeFormat([], { numeric: 'auto' }).format(Math.round(seconds / size), unit);
}

/** A compact stamp for dense lists: "Oct 27, 2:46 PM", with the year only when it differs. */
function formatShort(value: string) {
  const date = new Date(value);
  return date.toLocaleString([], { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', ...(date.getFullYear() === new Date().getFullYear() ? {} : { year: 'numeric' }) });
}

function formatRange(start: string, end: string | null) {
  const day = (value: string) => new Date(value).toLocaleDateString([], { month: 'short', day: 'numeric' });
  return end ? `${day(start)} – ${day(end)}` : `Since ${day(start)}`;
}

/** Continue the operator's own naming: "Week 44" is followed by "Week 45". */
function suggestNextName(rounds: LeaderboardRound[]) {
  const latest = [...rounds].sort((a, b) => b.number - a.number)[0];
  const match = latest?.name.match(/^(.*?)(\d+)$/);
  return match ? `${match[1]}${Number(match[2]) + 1}` : `Round ${(latest?.number ?? 0) + 1}`;
}

function weekdayOf(anchor: string | undefined, timezone: string | undefined) {
  if (!anchor) return 'monday';
  return new Intl.DateTimeFormat('en-US', { weekday: 'long', timeZone: timezone || 'UTC' }).format(new Date(anchor)).toLowerCase();
}

function RoundBadge({ round }: { round: LeaderboardRound | null }) {
  if (!round) return <span className="badge badge-neutral"><span className="dot" />No round</span>;
  const tone = round.status === 'active' ? 'badge-success' : round.status === 'scheduled' ? 'badge-accent' : 'badge-neutral';
  return <span className={`badge ${tone}`}><span className="dot" />{round.status === 'active' ? 'Running' : round.status === 'scheduled' ? 'Scheduled' : 'Ended'}</span>;
}

export function LeaderboardsPage() {
  const { scope } = useScope();
  const [boards, setBoards] = useState<Leaderboard[]>([]);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const navigate = useNavigate();
  const matchRoute = useMatchRoute();
  // The route says what is open: /new, /$key, or /$key/edit below /leaderboards.
  const creating = Boolean(matchRoute({ to: '/leaderboards/new' }));
  const editMatch = matchRoute({ to: '/leaderboards/$key/edit' });
  const detailMatch = creating ? false : matchRoute({ to: '/leaderboards/$key' });
  const routeKey = editMatch ? editMatch.key : detailMatch ? detailMatch.key : null;

  const refresh = useCallback(async () => {
    if (!scope) { setBoards([]); setCurrencies([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    try {
      const [nextBoards, nextCurrencies] = await Promise.all([fetchLeaderboards(ids(scope)), fetchCurrencies(ids(scope))]);
      setBoards(nextBoards); setCurrencies(nextCurrencies);
    } catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Leaderboards could not be loaded.'); }
    finally { setLoading(false); }
  }, [scope]);
  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => { setQuery(''); }, [scope?.tenant_id, scope?.environment_id]);

  const visible = useMemo(() => boards.filter((board) => `${board.name} ${board.key}`.toLowerCase().includes(query.trim().toLowerCase())), [boards, query]);
  const replace = (board: Leaderboard) => setBoards((current) => current.some((item) => item.id === board.id) ? current.map((item) => item.id === board.id ? board : item) : [board, ...current]);

  const routeBoard = routeKey ? boards.find((board) => board.key === routeKey) ?? boards.find((board) => board.id === routeKey) : undefined;
  const toList = () => void navigate({ to: '/leaderboards' });
  const toBoard = (board: Leaderboard, options: { replace?: boolean } = {}) => void navigate({ to: '/leaderboards/$key', params: { key: board.key }, replace: options.replace });

  if (scope && creating) {
    return <LeaderboardForm scope={scope} currencies={currencies} onCurrencyCreated={(currency) => setCurrencies((current) => [currency, ...current])} takenKeys={boards.map((board) => board.key)} onCancel={toList} onSaved={(board) => { replace(board); toBoard(board, { replace: true }); }} />;
  }
  if (routeKey) {
    if (loading && !routeBoard) return <main className="page page-resource"><PageHeader breadcrumb={['Workspace', 'Engagement', 'Leaderboards', routeKey]} title={routeKey} /><div className="skeleton-rows" aria-label="Loading leaderboard"><span /><span /><span /></div></main>;
    if (!routeBoard) return <main className="page page-resource">
      <PageHeader breadcrumb={['Workspace', 'Engagement', 'Leaderboards', routeKey]} title="Leaderboard not found" />
      <section className="panel"><div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>No leaderboard with the key <span className="mono">{routeKey}</span></strong><span>{error ?? 'It may have been deleted, or it belongs to another environment.'}</span><Link className="secondary-button" to="/leaderboards"><IconArrowLeft size={15} stroke={1.75} />All leaderboards</Link></div></section>
    </main>;
    if (scope && editMatch) {
      return <LeaderboardForm scope={scope} currencies={currencies} onCurrencyCreated={(currency) => setCurrencies((current) => [currency, ...current])} takenKeys={boards.map((board) => board.key)} editing={routeBoard} onCancel={() => toBoard(routeBoard)} onSaved={(board) => { replace(board); toBoard(board, { replace: true }); }} />;
    }
    if (scope) {
      return <LeaderboardDetail key={routeBoard.id} scope={scope} board={routeBoard} currencies={currencies} onBack={() => { toList(); void refresh(); }} onEdit={() => void navigate({ to: '/leaderboards/$key/edit', params: { key: routeBoard.key } })} onChanged={replace} onDeleted={() => { setBoards((current) => current.filter((board) => board.id !== routeBoard.id)); void navigate({ to: '/leaderboards', replace: true }); }} />;
    }
  }

  const activeCount = boards.filter((board) => board.active).length;
  return <main className="page page-resource">
    <PageHeader breadcrumb={['Workspace', 'Engagement', 'Leaderboards']} title="Leaderboards" description="Rank users by what they earn in each round. Your backend reads the ranking by the board key." actions={<button className="primary-button" type="button" onClick={() => void navigate({ to: '/leaderboards/new' })} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add leaderboard</button>} />
    <GuidePanel id="guide-leaderboard" title="Leaderboards" {...guide} defaultOpen={!loading && !error && boards.length === 0} />
    <section className="panel" aria-label="Leaderboards list">
      <div className="toolbar">
        <p className="tally"><strong>{boards.length}</strong> total <span aria-hidden="true">·</span> <strong>{activeCount}</strong> active</p>
        <div className="toolbar-controls"><label className="search-field"><IconSearch size={15} stroke={1.75} aria-hidden="true" /><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search leaderboards" aria-label="Search leaderboards" /></label><button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh leaderboards"><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button></div>
      </div>
      {error && <div className="panel-body"><p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Retry</button></p></div>}
      {loading ? <div className="skeleton-rows" aria-label="Loading leaderboards"><span /><span /><span /></div> : boards.length === 0 ? <div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>No leaderboards yet</strong><span>Create a weekly or monthly board and read it from your backend.</span><button className="primary-button" type="button" onClick={() => void navigate({ to: '/leaderboards/new' })} disabled={!scope}><IconPlus size={15} stroke={1.75} />Add leaderboard</button></div> : visible.length === 0 ? <div className="empty-state"><IconSearch size={20} stroke={1.5} aria-hidden="true" /><strong>No matching results</strong><span>Try a different search.</span><button className="secondary-button compact" type="button" onClick={() => setQuery('')}>Clear search</button></div> : <ul className="resource-list">{visible.map((board) => <li key={board.id} className={`resource-row ${board.active ? '' : 'is-archived'}`}><div className="resource-row-main"><strong>{board.name}</strong><span><span className="mono">{board.key}</span> · {describeMetric(board, currencies)} · {describeSchedule(board.schedule)}</span><small>{board.active ? describeRound(board.current_round) : 'Turned off · not scoring'}</small></div>{board.active ? <RoundBadge round={board.current_round} /> : <span className="badge badge-neutral"><span className="dot" />Off</span>}<div className="resource-row-actions"><Link className="text-button" to="/leaderboards/$key" params={{ key: board.key }} aria-label={`Open ${board.name}`}>Open<IconChevronRight size={14} stroke={1.75} /></Link></div></li>)}</ul>}
    </section>
  </main>;
}

function LeaderboardForm({ scope, currencies, onCurrencyCreated, takenKeys, editing, onCancel, onSaved }: { scope: ActiveScope; currencies: Currency[]; onCurrencyCreated: (currency: Currency) => void; takenKeys: string[]; editing?: Leaderboard; onCancel: () => void; onSaved: (board: Leaderboard) => void }) {
  const original = editing?.schedule;
  const originalWeekStart = weekdayOf(original?.anchor, original?.timezone);
  const firstCurrency = currencies.find((item) => item.active)?.code;
  const [form, setForm] = useState(() => ({
    name: editing?.name ?? '', description: editing?.description ?? '',
    rank: editing ? editing.metric.type === 'xp' ? 'xp' : `points:${editing.metric.currency}` : firstCurrency ? `points:${firstCurrency}` : 'xp',
    schedule: original?.type ?? 'manual', every: original?.every ?? 'week', interval: String(original?.interval ?? 1),
    timezone: original?.type === 'recurring' && original.timezone ? original.timezone : browserTimezone, weekStart: original?.type === 'recurring' ? originalWeekStart : localeWeekStart(),
  }));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [creatingCurrency, setCreatingCurrency] = useState(false);
  const [rankChosen, setRankChosen] = useState(Boolean(editing));
  const update = (patch: Partial<typeof form>) => { setForm((current) => ({ ...current, ...patch })); setError(null); };
  // Currencies can arrive after the form opens; until someone picks, default to the first one.
  useEffect(() => {
    if (!rankChosen && firstCurrency) setForm((current) => current.rank === 'xp' ? { ...current, rank: `points:${firstCurrency}` } : current);
  }, [rankChosen, firstCurrency]);
  const plural = Number(form.interval) > 1;
  const key = editing?.key ?? keyFromName(form.name, takenKeys);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const name = form.name.trim();
      if (!name) throw new Error('Enter a name before saving.');
      let schedule: LeaderboardSchedule = { type: 'manual', timezone: form.timezone };
      if (form.schedule === 'recurring') {
        const interval = Number(form.interval);
        if (!Number.isInteger(interval) || interval < 1 || interval > 366) throw new Error('Repeat every must be a whole number between 1 and 366.');
        schedule = { type: 'recurring', every: form.every as 'day' | 'week' | 'month', interval, timezone: form.timezone };
        // Keep the stored anchor unless the period boundaries changed.
        const keepAnchor = original?.type === 'recurring' && original.every === form.every && original.timezone === form.timezone && (form.every !== 'week' || form.weekStart === originalWeekStart);
        if (keepAnchor) schedule.anchor = original.anchor;
        else if (form.every === 'week') schedule.week_start = form.weekStart;
      }
      const metric = form.rank === 'xp' ? { type: 'xp' as const } : { type: 'points' as const, currency: form.rank.slice('points:'.length) };
      const input = { ...ids(scope), key, name, description: form.description.trim(), metric, schedule };
      onSaved(editing ? await updateLeaderboard(editing.id, input) : await createLeaderboard(input));
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Leaderboard could not be saved.'); }
    finally { setSaving(false); }
  }

  const rankable = currencies.filter((item) => item.active || `points:${item.code}` === form.rank);
  const rankOptions = [
    ...rankable.map((item) => ({ value: `points:${item.code}`, label: `${item.name} earned` })),
    ...(rankable.length ? [{ value: '__divider__', label: '', disabled: true, divider: true }] : []),
    { value: 'xp', label: 'XP earned' },
    { value: '__create_divider__', label: '', disabled: true, divider: true },
    { value: createCurrencyOption, label: 'Create currency here…', tone: 'accent' as const },
  ];
  const switchingToRecurring = editing?.schedule.type === 'manual' && form.schedule === 'recurring';
  return <main className="page page-resource page-narrow">
    <PageHeader breadcrumb={['Workspace', 'Engagement', 'Leaderboards', editing ? 'Edit' : 'Create']} title={editing ? `Edit ${editing.name}` : 'Add leaderboard'} description="Choose what to rank and who starts each round." actions={<button className="secondary-button" type="button" onClick={onCancel}><IconArrowLeft size={15} stroke={1.75} />Back</button>} />
    <section className="panel resource-editor" aria-label={editing ? 'Update leaderboard' : 'Set up a leaderboard'}>
      <form className="resource-form" onSubmit={(event) => void submit(event)}>
        <div className="resource-form-fields">
          <label className="field"><span className="field-label">Name</span><input autoFocus required maxLength={120} value={form.name} onChange={(event) => update({ name: event.target.value })} placeholder="Weekly top customers" /></label>
          <label className="field"><span className="field-label">Key</span><span className="locked-input"><input className="mono" value={key} disabled readOnly aria-describedby="leaderboard-key-help" /><IconLock size={14} stroke={1.75} aria-hidden="true" /></span><small className="field-help" id="leaderboard-key-help">{editing ? <>Your backend reads <span className="mono">/v1/leaderboards/{key}</span>. Keys never change.</> : <>Made from the name. Your backend reads <span className="mono">/v1/leaderboards/{key}</span>, and it never changes.</>}</small></label>
          <label className="field field-wide"><span className="field-label">Description <em>Optional</em></span><input maxLength={500} value={form.description} onChange={(event) => update({ description: event.target.value })} placeholder="Who wins and what they get" /></label>
          <div className="field"><span className="field-label">What to rank</span><CustomSelect value={form.rank} onChange={(value) => { if (value === createCurrencyOption) { setCreatingCurrency(true); return; } setRankChosen(true); update({ rank: value }); }} ariaLabel="What to rank" disabled={Boolean(editing)} options={rankOptions} /><small className="field-help">{editing ? 'Create a new leaderboard to rank something else.' : rankable.length === 0 ? 'This environment has no active currency yet, so only XP can be ranked. Create one here to rank points.' : 'Only earnings count. Spending, expiry, and refunds never change a rank.'}</small></div>
          <div className="field"><span className="field-label">Rounds</span><CustomSelect value={form.schedule} onChange={(value) => update({ schedule: value as 'manual' | 'recurring' })} ariaLabel="Rounds" options={[{ value: 'manual', label: 'I start each round' }, { value: 'recurring', label: 'Repeat on a schedule' }]} /><small className="field-help">{form.schedule === 'manual' ? 'Start and end rounds yourself, so a round can run 6 days one week and 8 the next.' : 'A new round starts at every boundary on its own.'}</small></div>
          {form.schedule === 'recurring' && <>
            <div className="field"><span className="field-label">Repeat every</span><div className="interval-input"><input type="number" min="1" max="366" step="1" value={form.interval} onChange={(event) => update({ interval: event.target.value })} aria-label="Interval" /><CustomSelect value={form.every} onChange={(value) => update({ every: value as 'day' | 'week' | 'month' })} ariaLabel="Period" options={[{ value: 'day', label: plural ? 'days' : 'day' }, { value: 'week', label: plural ? 'weeks' : 'week' }, { value: 'month', label: plural ? 'months' : 'month' }]} /></div></div>
            <div className="field"><span className="field-label">Time zone</span><TimezoneSelect value={form.timezone} onChange={(zone) => update({ timezone: zone })} /><small className="field-help">Rounds start at midnight in this zone{form.every === 'month' ? ', on the 1st' : ''}.</small></div>
            {form.every === 'week' && <div className="field"><span className="field-label">Week starts on</span><CustomSelect value={form.weekStart} onChange={(value) => update({ weekStart: value })} ariaLabel="Week starts on" options={weekdays.map((day) => ({ value: day, label: day.charAt(0).toUpperCase() + day.slice(1) }))} /></div>}
          </>}
        </div>
        {switchingToRecurring && <p className="alert alert-warning" role="status"><IconAlertTriangle size={16} aria-hidden="true" /><span>The open round will end where the first scheduled round begins. Cancel any scheduled rounds first.</span></p>}
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        <div className="resource-form-footer"><span className="form-footer-note">{editing ? 'Editing in' : 'Creating in'} <strong>{scope.name}</strong> · {scope.environment_name}</span><button className="secondary-button" type="button" onClick={onCancel}>Cancel</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Saving…' : editing ? 'Save leaderboard' : 'Create leaderboard'}</button></div>
      </form>
    </section>
    {creatingCurrency && <CurrencyCreateDialog open activeScope={scope} onClose={() => setCreatingCurrency(false)} onCreated={(currency) => { onCurrencyCreated(currency); setRankChosen(true); update({ rank: `points:${currency.code}` }); }} />}
  </main>;
}

function LeaderboardDetail({ scope, board, currencies, onBack, onEdit, onChanged, onDeleted }: { scope: ActiveScope; board: Leaderboard; currencies: Currency[]; onBack: () => void; onEdit: () => void; onChanged: (board: Leaderboard) => void; onDeleted: () => void }) {
  const [standings, setStandings] = useState<LeaderboardStandings | null>(null);
  const [rounds, setRounds] = useState<LeaderboardRound[]>([]);
  const navigate = useNavigate();
  // The round on screen lives in the URL (?round=43), so a past round survives a refresh.
  const roundSelector = useSearch({ strict: false }).round ?? 'current';
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [starting, setStarting] = useState(false);
  const [confirmEnd, setConfirmEnd] = useState(false);
  const [endDraft, setEndDraft] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const manual = board.schedule.type === 'manual';
  const unit = board.metric.type === 'xp' ? 'XP' : currencies.find((item) => item.code === board.metric.currency)?.name.toLowerCase() ?? board.metric.currency;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextStandings, nextRounds] = await Promise.all([fetchLeaderboardStandings(ids(scope), board.id, { round: roundSelector, limit: pageSize, offset }), fetchLeaderboardRounds(ids(scope), board.id)]);
      setStandings(nextStandings); setRounds(nextRounds);
    } catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Standings could not be loaded.'); }
    finally { setLoading(false); }
  }, [scope, board.id, roundSelector, offset]);
  useEffect(() => { void load(); }, [load]);

  async function act(action: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try { await action(); setConfirmEnd(false); setEndDraft(null); await load(); }
    catch (actionError) { setError(actionError instanceof Error ? actionError.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }

  function showRound(selector: string) {
    void navigate({ to: '/leaderboards/$key', params: { key: board.key }, search: selector === 'current' ? {} : { round: selector } });
  }
  useEffect(() => { setOffset(0); setConfirmEnd(false); setEndDraft(null); }, [roundSelector]);

  const round = standings?.round ?? null;
  const viewingCurrent = roundSelector === 'current';
  const running = viewingCurrent && round?.status === 'active';
  // The round that is live now, whichever round the page is showing.
  const liveRound = rounds.find((item) => item.status === 'active') ?? null;
  const leader = round ? rounds.find((item) => item.id === round.id)?.leader : undefined;
  const nextName = suggestNextName(rounds);

  const endControls = manual && running && round ? endDraft === null ? <div className="metric-actions">
    <button className="text-button" type="button" onClick={() => setEndDraft(toLocalInput(round.ends_at))}>Change end</button>
    {confirmEnd ? <><button className="text-button text-danger" type="button" disabled={busy} onClick={() => void act(() => endLeaderboardRound(ids(scope), board.id, round.id))}>Confirm end</button><button className="text-button" type="button" onClick={() => setConfirmEnd(false)}>Keep</button></> : <button className="text-button" type="button" onClick={() => setConfirmEnd(true)}>End now</button>}
  </div> : <div className="metric-edit">
    <input type="datetime-local" value={endDraft} onChange={(event) => setEndDraft(event.target.value)} aria-label="Round end" />
    <div className="metric-actions"><button className="text-button" type="button" disabled={busy || !endDraft} onClick={() => void act(() => updateLeaderboardRound(ids(scope), board.id, round.id, { ends_at: fromLocalInput(endDraft) }))}>Save</button><button className="text-button" type="button" disabled={busy} onClick={() => void act(() => updateLeaderboardRound(ids(scope), board.id, round.id, { ends_at: null }))}>Keep open</button><button className="text-button" type="button" onClick={() => setEndDraft(null)}>Cancel</button></div>
  </div> : null;

  return <main className="page page-resource">
    <PageHeader breadcrumb={['Workspace', 'Engagement', 'Leaderboards', board.name]} title={board.name} description={<><span className="mono">{board.key}</span> · {describeMetric(board, currencies)} · {describeSchedule(board.schedule)}</>} topbarActions={<button className="icon-button" type="button" onClick={() => void load()} aria-label="Refresh standings"><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button>} actions={<>
      <button className="icon-button" type="button" onClick={onBack} aria-label="Back to leaderboards" title="Back to leaderboards"><IconArrowLeft size={15} stroke={1.75} /></button>
      <button className="secondary-button" type="button" onClick={onEdit}><IconPencil size={15} stroke={1.75} />Edit</button>
      <button className="icon-button" type="button" onClick={() => setDeleting(true)} aria-label="Delete leaderboard" title="Delete leaderboard"><IconTrash size={15} stroke={1.75} /></button>
      <button className="secondary-button" type="button" disabled={busy} onClick={() => void act(async () => { await updateCatalogStatus({ ...ids(scope), resource: 'leaderboard', id: board.id, active: !board.active }); onChanged({ ...board, active: !board.active }); })}>{board.active ? 'Turn off' : 'Turn on'}</button>
      {manual && board.active && <button className="primary-button" type="button" onClick={() => setStarting(true)}><IconPlayerPlay size={15} stroke={1.75} />{liveRound ? 'Next round' : 'Start round'}</button>}
    </>} />
    {!board.active && <p className="alert alert-warning" role="status"><IconAlertTriangle size={16} aria-hidden="true" /><span>This leaderboard is off. Awards do not count and no new rounds start; past rounds stay readable.</span></p>}
    {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void load()}>Retry</button></p>}
    {standings?.next_round && viewingCurrent && <p className="alert alert-success" role="status"><IconPlayerPlay size={16} aria-hidden="true" /><span>{standings.next_round.name} starts {formatDateTime(standings.next_round.starts_at)}.</span><button className="text-button" type="button" disabled={busy} onClick={() => void act(() => cancelLeaderboardRound(ids(scope), board.id, standings.next_round!.id))}>Cancel it</button></p>}

    <section className="metrics" aria-label="Round summary">
      <div className="metric"><span className="metric-label">{viewingCurrent ? 'Current round' : 'Viewing'}</span><span className="metric-value">{round?.name ?? '—'}</span><span className="metric-note"><RoundBadge round={round} /></span></div>
      <div className="metric"><span className="metric-label">{round?.status === 'ended' ? 'Ended' : 'Ends'}</span><span className="metric-value">{round ? round.ends_at ? relativeTime(round.ends_at) : 'When you end it' : '—'}</span><span className="metric-note">{round ? round.ends_at ? formatDateTime(round.ends_at) : `Started ${formatShort(round.starts_at)}` : manual ? 'Start a round to begin scoring' : 'Starts with the first award'}</span>{endControls}</div>
      <div className="metric"><span className="metric-label">Participants</span><span className="metric-value">{(standings?.participants ?? 0).toLocaleString()}</span><span className="metric-note">Users who scored this round</span></div>
      <div className="metric"><span className="metric-label">{round?.status === 'ended' ? 'Winner' : 'Leader'}</span><span className="metric-value metric-value-text">{leader?.user_id ?? '—'}</span><span className="metric-note">{leader ? `${leader.score.toLocaleString()} ${unit}` : 'Nobody has scored yet'}</span></div>
    </section>

    <div className="leaderboard-grid">
      <section className="panel" aria-label="Standings">
        <header className="panel-header"><div><h2>Standings{!viewingCurrent && round && <em>{round.name}</em>}</h2><p>Higher score first; on a tie, whoever got there first.</p></div>{!viewingCurrent && <div className="panel-actions"><button className="text-button" type="button" onClick={() => showRound('current')}>Back to current round</button></div>}</header>
        {loading && !standings ? <div className="skeleton-rows" aria-label="Loading standings"><span /><span /><span /></div> : !round || standings!.entries.length === 0 ? <div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>{round ? `No one has earned ${unit} in ${round.name} yet` : 'No round yet'}</strong><span>{round ? round.status === 'ended' ? 'This round ended without any scores.' : `Players appear here as soon as they earn ${unit}. Points earned before ${round.name} started do not count.` : manual ? 'Start a round. Points earned before it count nowhere.' : 'The first round starts when someone earns or the board is read.'}</span></div> : <ol className="standings-list">
          <li className="standings-head" aria-hidden="true"><span>#</span><span>User</span><span>{board.metric.type === 'xp' ? 'XP' : 'Score'}</span><span>Reached</span></li>
          {standings!.entries.map((entry) => <li key={entry.user_id}><span className="standings-rank">{entry.rank}</span><span className="mono standings-user">{entry.user_id}</span><span className="standings-score">{entry.score.toLocaleString()}</span><small className="muted">{formatShort(entry.reached_at)}</small></li>)}
        </ol>}
        {standings && (offset > 0 || standings.participants > offset + pageSize) && <footer className="panel-footer"><button className="text-button" type="button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - pageSize))}>Previous</button><span className="panel-meta">{offset + 1}–{Math.min(offset + pageSize, standings.participants)} of {standings.participants.toLocaleString()}</span><button className="text-button" type="button" disabled={standings.participants <= offset + pageSize} onClick={() => setOffset(offset + pageSize)}>Next</button></footer>}
      </section>
      <section className="panel" aria-label="Rounds">
        <header className="panel-header"><div><h2>Rounds</h2><p>{rounds.some((item) => item.status === 'ended') ? 'Select a round to see its standings.' : 'Past rounds and their winners appear here once a round ends.'}</p></div></header>
        {rounds.length === 0 ? <div className="empty-state"><span>No rounds yet.</span></div> : <ul className="round-list">{rounds.map((item) => { const selected = round?.id === item.id; return <li key={item.id}><button className={`round-row ${selected ? 'is-selected' : ''}`} type="button" disabled={item.status === 'scheduled'} aria-current={selected ? 'true' : undefined} onClick={() => showRound(item.status === 'active' ? 'current' : String(item.number))}>
          <span className="round-row-main"><strong>{item.name}{item.status !== 'ended' && <RoundBadge round={item} />}</strong><span>{formatRange(item.starts_at, item.ends_at)}{selected && <em className="round-row-showing"> · Showing</em>}</span></span>
          <span className="round-row-meta">{item.leader ? <span className="round-row-leader"><em>{item.status === 'ended' ? 'Winner' : 'Leading'}</em><strong>{item.leader.user_id}</strong></span> : <span className="round-row-leader"><em>No scores</em></span>}<span>{item.participants ?? 0} {item.participants === 1 ? 'player' : 'players'}</span></span>
        </button></li>; })}</ul>}
      </section>
    </div>
    {deleting && <DeleteLeaderboardSheet scope={scope} board={board} rounds={rounds.length} onClose={() => setDeleting(false)} onDeleted={onDeleted} />}
    {starting && <StartRoundSheet scope={scope} board={board} current={liveRound} suggestedName={nextName} onClose={() => setStarting(false)} onStarted={() => { setStarting(false); showRound('current'); void load(); }} />}
  </main>;
}

function StartRoundSheet({ scope, board, current, suggestedName, onClose, onStarted }: { scope: ActiveScope; board: Leaderboard; current: LeaderboardRound | null; suggestedName: string; onClose: () => void; onStarted: () => void }) {
  const [name, setName] = useState('');
  const [when, setWhen] = useState<'now' | 'later'>('now');
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [endCurrent, setEndCurrent] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function endAfter(days: number) {
    const start = when === 'later' && startsAt ? new Date(startsAt) : new Date();
    start.setDate(start.getDate() + days);
    setEndsAt(toLocalInput(start.toISOString()));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError(null);
    try {
      if (when === 'later' && !startsAt) throw new Error('Choose when the round starts.');
      await createLeaderboardRound(ids(scope), board.id, { name: name.trim() || suggestedName, starts_at: when === 'later' ? fromLocalInput(startsAt) : undefined, ends_at: endsAt ? fromLocalInput(endsAt) : undefined, end_current: current ? endCurrent : true });
      onStarted();
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'The round could not be started.'); }
    finally { setSaving(false); }
  }

  return <Sheet labelledBy="start-round-title" title={current ? 'Start the next round' : 'Start a round'} description={`${board.name} · scores start from zero in the new round.`} onClose={onClose}>
    <form className="form-stack" onSubmit={(event) => void submit(event)}>
      <label className="field"><span className="field-label">Name <em>Optional</em></span><input maxLength={120} value={name} onChange={(event) => setName(event.target.value)} placeholder={suggestedName} /></label>
      <div className="field"><span className="field-label">Starts</span><div className="segmented" role="group" aria-label="Starts"><button type="button" aria-pressed={when === 'now'} onClick={() => setWhen('now')}>Now</button><button type="button" aria-pressed={when === 'later'} onClick={() => setWhen('later')}>Later</button></div>{when === 'later' && <input type="datetime-local" value={startsAt} onChange={(event) => setStartsAt(event.target.value)} aria-label="Start time" />}</div>
      <div className="field"><span className="field-label">Ends <em>Optional</em></span><input type="datetime-local" value={endsAt} onChange={(event) => setEndsAt(event.target.value)} aria-label="End time" /><div className="round-presets"><span className="round-presets-label">End after</span>{[6, 7, 8, 30].map((days) => <button key={days} className="chip" type="button" onClick={() => endAfter(days)}>{days} days</button>)}{endsAt && <button className="chip chip-quiet" type="button" onClick={() => setEndsAt('')}>Keep open</button>}</div><small className="field-help">{endsAt ? 'The round closes on its own at this time.' : 'The round runs until you end it or start the next one.'}</small></div>
      {current && <label className="option-row"><input type="checkbox" checked={endCurrent} onChange={(event) => setEndCurrent(event.target.checked)} /><span><strong>End {current.name} when this round starts</strong><small>{endCurrent ? 'No award falls between the two rounds.' : 'Only works if the rounds do not overlap.'}</small></span></label>}
      {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
      <div className="form-footer"><button className="secondary-button" type="button" onClick={onClose}>Cancel</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Starting…' : when === 'later' ? 'Schedule round' : 'Start round'}</button></div>
    </form>
  </Sheet>;
}

function DeleteLeaderboardSheet({ scope, board, rounds, onClose, onDeleted }: { scope: ActiveScope; board: Leaderboard; rounds: number; onClose: () => void; onDeleted: () => void }) {
  const [typed, setTyped] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const confirmed = typed.trim() === board.key;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!confirmed) return;
    setSaving(true);
    setError(null);
    try { await deleteLeaderboard(ids(scope), board.id); onDeleted(); }
    catch (deleteError) { setError(deleteError instanceof Error ? deleteError.message : 'The leaderboard could not be deleted.'); setSaving(false); }
  }

  return <Sheet labelledBy="delete-leaderboard-title" title={`Delete ${board.name}`} description="This cannot be undone." onClose={onClose}>
    <form className="form-stack" onSubmit={(event) => void submit(event)}>
      <ul className="check-list delete-consequences">
        <li className="is-required"><IconAlertTriangle size={16} aria-hidden="true" /><span><strong>{rounds === 1 ? '1 round' : `${rounds.toLocaleString()} rounds`} and every score in them are deleted</strong><small>Past winners can no longer be looked up.</small></span></li>
        <li className="is-required"><IconAlertTriangle size={16} aria-hidden="true" /><span><strong><span className="mono">/v1/leaderboards/{board.key}</span> stops working</strong><small>Your backend gets 404 until a board with this key exists again.</small></span></li>
        <li className="is-ready"><IconCircleDashed size={16} aria-hidden="true" /><span><strong>Balances and XP are not touched</strong><small>Only the ranking is removed.</small></span></li>
      </ul>
      {board.active && <p className="inline-note">To stop scoring but keep the history, turn the leaderboard off instead.</p>}
      <label className="field"><span className="field-label">Type <span className="mono">{board.key}</span> to confirm</span><input className="mono" value={typed} onChange={(event) => setTyped(event.target.value)} autoComplete="off" spellCheck={false} aria-label={`Type ${board.key} to confirm`} /></label>
      {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
      <div className="form-footer"><button className="secondary-button" type="button" onClick={onClose}>Cancel</button><button className="danger-button" type="submit" disabled={!confirmed || saving}>{saving ? 'Deleting…' : 'Delete leaderboard'}</button></div>
    </form>
  </Sheet>;
}
