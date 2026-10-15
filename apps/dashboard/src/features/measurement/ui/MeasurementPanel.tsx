import { useCallback, useEffect, useMemo, useRef, useState, type ComponentType, type DragEvent as ReactDragEvent } from 'react';
import { IconActivity, IconArrowDown, IconArrowLeft, IconArrowRight, IconArrowUp, IconBolt, IconChartAreaLine, IconCheck, IconChevronDown, IconCoin, IconEdit, IconGift, IconGripVertical, IconMaximize, IconMinimize, IconPlus, IconRefresh, IconSearch, IconSparkles, IconTrash, IconTrophy, IconUsers, IconX } from '@tabler/icons-react';
import {
  fetchAchievements,
  fetchCampaigns,
  fetchCurrencies,
  fetchEventTypes,
  fetchGoals,
  fetchMissions,
  fetchReportDashboard,
  fetchReportQuery,
  fetchRewards,
  fetchRules,
  saveReportDashboard,
  type Achievement,
  type Campaign,
  type Currency,
  type EventType,
  type Goal,
  type Mission,
  type ReportQueryResponse,
  type ReportWidgetConfig,
  type Reward,
  type Rule,
  type Tenant,
} from '../../../shared/api/client';
import { CustomSelect, type CustomSelectOption } from '../../../shared/ui/CustomSelect';

type Props = { scope?: Tenant };
type ResourceKind = 'achievement' | 'mission' | 'currency' | 'reward' | 'campaign' | 'goal' | 'event' | 'rule';
type Icon = ComponentType<{ size?: number; stroke?: number }>;
type MeasureDefinition = { id: string; label: string; question: string; description: string; category: string; resource?: ResourceKind; advanced?: boolean; defaultSize?: 'standard' | 'wide'; icon: Icon };
type DropSide = 'before' | 'after';
type DropTarget = { widgetId: string; side: DropSide };

const measures: MeasureDefinition[] = [
  { id: 'events.count', label: 'Event volume', question: 'How much activity is coming in?', description: 'Count every event received by your workspace.', category: 'Activity', resource: 'event', defaultSize: 'wide', icon: IconActivity },
  { id: 'users.active', label: 'Active members', question: 'How many members are active?', description: 'Unique members who sent at least one event.', category: 'Activity', resource: 'event', defaultSize: 'standard', icon: IconUsers },
  { id: 'events.processed', label: 'Processed events', question: 'How many events finished successfully?', description: 'Events that completed processing successfully.', category: 'Activity', resource: 'event', defaultSize: 'standard', icon: IconCheck },
  { id: 'events.accepted', label: 'Accepted events', question: 'How many events were accepted?', description: 'Events accepted into the processing pipeline.', category: 'Activity', resource: 'event', advanced: true, defaultSize: 'standard', icon: IconCheck },
  { id: 'events.processing', label: 'Events in progress', question: 'How many events are still processing?', description: 'Events still moving through the processing pipeline.', category: 'Activity', resource: 'event', advanced: true, defaultSize: 'standard', icon: IconActivity },
  { id: 'events.failed', label: 'Failed events', question: 'How many events failed?', description: 'Events that could not be processed.', category: 'Activity', resource: 'event', advanced: true, defaultSize: 'standard', icon: IconX },
  { id: 'automations.executions', label: 'Automation activity', question: 'How often are automations running?', description: 'Published automation runs in the selected period.', category: 'Automations', resource: 'rule', defaultSize: 'standard', icon: IconBolt },
  { id: 'automations.failures', label: 'Automation failures', question: 'Are any automations failing?', description: 'Automation runs that ended in failure.', category: 'Automations', resource: 'rule', advanced: true, defaultSize: 'standard', icon: IconX },
  { id: 'achievements.unlocks', label: 'Achievement unlocks', question: 'How many members unlocked an achievement?', description: 'Members who unlocked an achievement.', category: 'Progression', resource: 'achievement', defaultSize: 'wide', icon: IconTrophy },
  { id: 'achievements.progress', label: 'Achievement progress', question: 'How much progress are members making?', description: 'Progress events recorded for an achievement.', category: 'Progression', resource: 'achievement', advanced: true, defaultSize: 'wide', icon: IconTrophy },
  { id: 'missions.completions', label: 'Mission completions', question: 'How many missions are being completed?', description: 'Members who completed a mission.', category: 'Progression', resource: 'mission', defaultSize: 'standard', icon: IconSparkles },
  { id: 'missions.progress', label: 'Mission progress', question: 'How much progress are members making on missions?', description: 'Progress events recorded for a mission.', category: 'Progression', resource: 'mission', advanced: true, defaultSize: 'standard', icon: IconSparkles },
  { id: 'xp.awarded', label: 'XP awarded', question: 'How much XP are members earning?', description: 'Total XP added to member accounts.', category: 'Loyalty', defaultSize: 'standard', icon: IconSparkles },
  { id: 'points.awarded', label: 'Points awarded', question: 'How many points are members earning?', description: 'Points added to member balances.', category: 'Loyalty', resource: 'currency', defaultSize: 'standard', icon: IconCoin },
  { id: 'points.spent', label: 'Points spent', question: 'How many points are members spending?', description: 'Points spent or reserved by members.', category: 'Loyalty', resource: 'currency', defaultSize: 'standard', icon: IconCoin },
  { id: 'points.expired', label: 'Points expired', question: 'How many points expired?', description: 'Points removed because their earning window ended.', category: 'Loyalty', resource: 'currency', advanced: true, defaultSize: 'standard', icon: IconCoin },
  { id: 'points.conversions', label: 'Point conversions', question: 'How many points are moving between currencies?', description: 'Points converted between currencies.', category: 'Loyalty', resource: 'currency', advanced: true, defaultSize: 'standard', icon: IconCoin },
  { id: 'rewards.redemptions', label: 'Reward redemptions', question: 'How often are rewards being redeemed?', description: 'Redemption requests created by members.', category: 'Rewards', resource: 'reward', defaultSize: 'standard', icon: IconGift },
  { id: 'rewards.fulfilled', label: 'Rewards fulfilled', question: 'How many rewards were delivered?', description: 'Reward deliveries completed successfully.', category: 'Rewards', resource: 'reward', advanced: true, defaultSize: 'standard', icon: IconGift },
  { id: 'rewards.failed', label: 'Failed redemptions', question: 'Are any reward deliveries failing?', description: 'Reward deliveries that ended in failure.', category: 'Rewards', resource: 'reward', advanced: true, defaultSize: 'standard', icon: IconX },
  { id: 'campaigns.events', label: 'Campaign activity', question: 'How much activity are campaigns driving?', description: 'Events matched by a campaign.', category: 'Engagement', resource: 'campaign', advanced: true, defaultSize: 'standard', icon: IconSparkles },
  { id: 'campaigns.unique_users', label: 'Campaign reach', question: 'How many people are campaigns reaching?', description: 'Unique members reached by campaign events.', category: 'Engagement', resource: 'campaign', defaultSize: 'standard', icon: IconUsers },
  { id: 'goals.events', label: 'Goal activity', question: 'How close are members getting to a goal?', description: 'Events contributing to a goal.', category: 'Engagement', resource: 'goal', advanced: true, defaultSize: 'standard', icon: IconSparkles },
];

type ReportTemplate = { id: string; title: string; hint: string; measure: string; icon: Icon };

const reportTemplates: ReportTemplate[] = [
  { id: 'achievement-unlocks', title: 'Achievement progress', hint: 'See how many members unlocked a specific achievement.', measure: 'achievements.unlocks', icon: IconTrophy },
  { id: 'event-volume', title: 'Incoming activity', hint: 'See how much event activity your workspace is receiving.', measure: 'events.count', icon: IconActivity },
  { id: 'active-members', title: 'Active members', hint: 'See how many unique members are participating.', measure: 'users.active', icon: IconUsers },
  { id: 'points-awarded', title: 'Points earned', hint: 'See how many points members earned over time.', measure: 'points.awarded', icon: IconCoin },
  { id: 'reward-redemptions', title: 'Reward usage', hint: 'See which reward activity is happening in your program.', measure: 'rewards.redemptions', icon: IconGift },
  { id: 'automation-activity', title: 'Automation activity', hint: 'See whether your published automations are running.', measure: 'automations.executions', icon: IconBolt },
];

const defaultWidgets: ReportWidgetConfig[] = [
  { id: 'events-over-time', measure: 'events.count', title: 'Event volume', interval: 'day', size: 'wide' },
  { id: 'active-users', measure: 'users.active', title: 'Active members', interval: 'day', size: 'standard' },
  { id: 'automation-activity', measure: 'automations.executions', title: 'Automation activity', interval: 'day', size: 'standard' },
  { id: 'achievement-unlocks', measure: 'achievements.unlocks', title: 'Achievement unlocks', interval: 'day', size: 'wide' },
  { id: 'points-awarded', measure: 'points.awarded', title: 'Points awarded', interval: 'day', size: 'standard' },
  { id: 'reward-redemptions', measure: 'rewards.redemptions', title: 'Reward redemptions', interval: 'day', size: 'standard' },
];

type Catalog = { achievements: Achievement[]; campaigns: Campaign[]; currencies: Currency[]; events: EventType[]; goals: Goal[]; missions: Mission[]; rewards: Reward[]; rules: Rule[] };
const emptyCatalog: Catalog = { achievements: [], campaigns: [], currencies: [], events: [], goals: [], missions: [], rewards: [], rules: [] };

function makeRange(days: number) {
  const to = new Date();
  const from = new Date(to);
  from.setDate(from.getDate() - days);
  return { from: from.toISOString(), to: to.toISOString() };
}

function formatValue(value: number) {
  if (!Number.isFinite(value)) return '—';
  return Intl.NumberFormat(undefined, { notation: value > 9999 ? 'compact' : 'standard', maximumFractionDigits: value % 1 ? 1 : 0 }).format(value);
}

function measureFor(id: string) { return measures.find((item) => item.id === id) ?? measures[0]; }

function resourceItems(kind: ResourceKind | undefined, catalog: Catalog) {
  if (kind === 'achievement') return catalog.achievements.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'mission') return catalog.missions.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'currency') return catalog.currencies.map((item) => ({ id: item.code, name: item.name }));
  if (kind === 'reward') return catalog.rewards.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'campaign') return catalog.campaigns.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'goal') return catalog.goals.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'rule') return catalog.rules.map((item) => ({ id: item.id, name: item.name }));
  if (kind === 'event') return catalog.events.map((item) => ({ id: item.name, name: item.name }));
  return [];
}

function resourceLabel(kind: ResourceKind) {
  const labels: Record<ResourceKind, string> = { achievement: 'achievement', mission: 'mission', currency: 'currency', reward: 'reward', campaign: 'campaign', goal: 'goal', rule: 'automation', event: 'event type' };
  return labels[kind];
}

function resourceCollectionLabel(kind: ResourceKind) {
  const labels: Record<ResourceKind, string> = { achievement: 'achievements', mission: 'missions', currency: 'currencies', reward: 'rewards', campaign: 'campaigns', goal: 'goals', rule: 'automations', event: 'event types' };
  return labels[kind];
}

function rangeLabel(days: number) { return days === 30 ? 'last 30 days' : days === 90 ? 'last 90 days' : 'last 7 days'; }

function Chart({ points, wide }: { points: Array<{ at: string; value: number }>; wide?: boolean }) {
  const [active, setActive] = useState<number | null>(null);
  const values = points.map((point) => point.value);
  const max = Math.max(...values, 1);
  const width = wide ? 640 : 320;
  const height = 140;
  const coordinates = points.map((point, index) => ({
    x: points.length <= 1 ? width / 2 : (index / (points.length - 1)) * width,
    y: height - (point.value / max) * (height - 16) - 4,
  }));
  const line = coordinates.map(({ x, y }) => `${x},${y}`).join(' ');
  const area = points.length > 1 ? `M 0,${height} L ${line.replaceAll(' ', ' L ')} L ${width},${height} Z` : '';
  const dateLabel = (value?: string) => value ? new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) : '–';
  const activePoint = active === null ? undefined : points[active];
  const activeCoord = active === null ? undefined : coordinates[active];

  function track(event: React.PointerEvent<HTMLDivElement>) {
    if (points.length === 0) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
    setActive(Math.round(ratio * (points.length - 1)));
  }

  return <div className="report-chart">
    <div className="report-chart-plot" onPointerMove={track} onPointerLeave={() => setActive(null)}>
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
        {[0.25, 0.5, 0.75].map((ratio) => <line key={ratio} className="report-chart-grid" x1="0" x2={width} y1={height * ratio} y2={height * ratio} vectorEffect="non-scaling-stroke" />)}
        <line className="report-chart-baseline" x1="0" x2={width} y1={height - 0.5} y2={height - 0.5} vectorEffect="non-scaling-stroke" />
        {area && <path className="report-chart-area" d={area} />}
        <polyline className="report-chart-line" points={line} vectorEffect="non-scaling-stroke" />
        {activeCoord && <line className="report-chart-crosshair" x1={activeCoord.x} x2={activeCoord.x} y1="0" y2={height} vectorEffect="non-scaling-stroke" />}
      </svg>
      {activeCoord && activePoint && <>
        <span className="report-chart-dot" style={{ left: `${(activeCoord.x / width) * 100}%`, top: `${(activeCoord.y / height) * 100}%` }} />
        <span className={`report-chart-tip ${activeCoord.x / width > 0.7 ? 'is-left' : ''}`} style={{ left: `${(activeCoord.x / width) * 100}%` }}><strong>{formatValue(activePoint.value)}</strong>{dateLabel(activePoint.at)}</span>
      </>}
    </div>
    <div className="report-chart-axis"><span>{dateLabel(points[0]?.at)}</span><span>{dateLabel(points.at(-1)?.at)}</span></div>
  </div>;
}

function WidgetCard({ widget, result, loading, periodLabel, dragging, dropSide, onEdit, onRemove, onMove, onResize, onDragStart, onDragOver, onDrop, onDragEnd }: { widget: ReportWidgetConfig; result?: ReportQueryResponse; loading: boolean; periodLabel: string; dragging: boolean; dropSide?: DropSide; onEdit: () => void; onRemove: () => void; onMove: (direction: -1 | 1) => void; onResize: () => void; onDragStart: (event: ReactDragEvent<HTMLButtonElement>) => void; onDragOver: (event: ReactDragEvent<HTMLElement>) => void; onDrop: (event: ReactDragEvent<HTMLElement>) => void; onDragEnd: () => void }) {
  const definition = measureFor(widget.measure);
  const DefinitionIcon = definition.icon;
  const points = result?.points ?? [];
  const hasActivity = points.some((point) => point.value !== 0);
  return <article className={`report-widget report-widget-${widget.size ?? 'standard'} ${dragging ? 'is-dragging' : ''} ${dropSide ? `is-drop-${dropSide}` : ''}`} onDragOver={onDragOver} onDrop={onDrop}>
    <header className="report-widget-header">
      <button className="report-widget-handle" type="button" draggable aria-label={`Drag ${widget.title} to reorder. Use the move buttons for keyboard control.`} title="Drag to reorder" onDragStart={onDragStart} onDragEnd={onDragEnd}><IconGripVertical size={15} stroke={1.75} /></button>
      <span className="report-widget-icon" aria-hidden="true"><DefinitionIcon size={15} stroke={1.75} /></span>
      <div className="report-widget-title"><h3>{widget.title || definition.label}</h3><p title={definition.description}>{definition.description}</p></div>
      <div className="report-widget-actions" role="toolbar" aria-label={`${widget.title} view controls`}>
        <button className="icon-button ghost compact" type="button" onClick={onEdit} aria-label={`Edit ${widget.title}`} title="Edit view"><IconEdit size={15} stroke={1.75} /></button>
        <button className="icon-button ghost compact" type="button" onClick={onResize} aria-label={`Make ${widget.title} ${widget.size === 'wide' ? 'half width' : 'full width'}`} title={widget.size === 'wide' ? 'Use half width' : 'Make full width'} aria-pressed={widget.size === 'wide'}>{widget.size === 'wide' ? <IconMinimize size={15} stroke={1.75} /> : <IconMaximize size={15} stroke={1.75} />}</button>
        <button className="icon-button ghost compact" type="button" onClick={() => onMove(-1)} aria-label={`Move ${widget.title} up`} title="Move up"><IconArrowUp size={15} stroke={1.75} /></button>
        <button className="icon-button ghost compact" type="button" onClick={() => onMove(1)} aria-label={`Move ${widget.title} down`} title="Move down"><IconArrowDown size={15} stroke={1.75} /></button>
        <button className="icon-button ghost compact danger" type="button" onClick={onRemove} aria-label={`Remove ${widget.title}`} title="Remove view"><IconTrash size={15} stroke={1.75} /></button>
      </div>
    </header>
    <div className="report-widget-total">{loading ? <span className="report-value-loading" aria-label="Loading" /> : <strong>{formatValue(result?.total ?? 0)}</strong>}<span>Total · {periodLabel}</span></div>
    {loading ? <div className="report-chart-placeholder" aria-label="Loading chart" /> : points.length === 0 || !hasActivity ? <div className="report-widget-empty"><IconChartAreaLine size={18} stroke={1.5} aria-hidden="true" /><span>No activity in this period</span></div> : <Chart points={points} wide={widget.size === 'wide'} />}
  </article>;
}

function TemplateCard({ template, selected, onSelect }: { template: ReportTemplate; selected: boolean; onSelect: () => void }) {
  const TemplateIcon = template.icon;
  return <button className={`report-template ${selected ? 'is-selected' : ''}`} type="button" onClick={onSelect} aria-pressed={selected}><span className="report-template-icon" aria-hidden="true"><TemplateIcon size={16} stroke={1.75} /></span><span className="report-template-copy"><strong>{template.title}</strong><small>{template.hint}</small></span>{selected ? <IconCheck className="report-template-mark" size={15} stroke={2} aria-hidden="true" /> : <IconArrowRight className="report-template-mark" size={15} stroke={1.75} aria-hidden="true" />}</button>;
}

export function MeasurementPanel({ scope }: Props) {
  const [widgets, setWidgets] = useState<ReportWidgetConfig[]>(defaultWidgets);
  const [results, setResults] = useState<Record<string, ReportQueryResponse | undefined>>({});
  const [loadingWidgets, setLoadingWidgets] = useState<Record<string, boolean>>({});
  const [catalog, setCatalog] = useState<Catalog>(emptyCatalog);
  const [rangeDays, setRangeDays] = useState(7);
  const [studioOpen, setStudioOpen] = useState(false);
  const [studioMode, setStudioMode] = useState<'templates' | 'configure'>('templates');
  const [selectedMeasure, setSelectedMeasure] = useState('');
  const [selectedResource, setSelectedResource] = useState('');
  const [selectedEventType, setSelectedEventType] = useState('');
  const [customTitle, setCustomTitle] = useState('');
  const [editingWidgetId, setEditingWidgetId] = useState<string | null>(null);
  const [builderSearch, setBuilderSearch] = useState('');
  const [showAllMeasures, setShowAllMeasures] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [error, setError] = useState<string | null>(null);
  const loadedRef = useRef(false);
  const [draggingWidgetId, setDraggingWidgetId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null);

  const selectedDefinition = measureFor(selectedMeasure);
  const selectedResources = useMemo(() => resourceItems(selectedDefinition.resource, catalog), [selectedDefinition.resource, catalog]);
  const periodLabel = rangeLabel(rangeDays);
  const selectedResourceValue = selectedDefinition.resource === 'event' ? selectedEventType : selectedResource;
  const selectedResourceName = selectedResources.find((item) => item.id === selectedResourceValue)?.name;
  const resourceOptions = useMemo<CustomSelectOption[]>(() => selectedDefinition.resource ? [{ value: '', label: `All ${resourceCollectionLabel(selectedDefinition.resource)}` }, ...selectedResources.map((item) => ({ value: item.id, label: item.name }))] : [], [selectedDefinition.resource, selectedResources]);
  const SelectedIcon = selectedDefinition.icon;
  const visibleTemplates = useMemo(() => {
    const query = builderSearch.trim().toLowerCase();
    if (!query && !showAllMeasures) return reportTemplates;
    const source = query ? measures.filter((measure) => `${measure.label} ${measure.question} ${measure.description}`.toLowerCase().includes(query)) : measures;
    const seen = new Set<string>();
    return source.filter((measure) => { if (seen.has(measure.id)) return false; seen.add(measure.id); return true; }).map((measure) => ({ id: measure.id, title: measure.label, hint: measure.question, measure: measure.id, icon: measure.icon }));
  }, [builderSearch, showAllMeasures]);

  const loadCatalogAndDashboard = useCallback(async () => {
    if (!scope) return;
    setLoading(true);
    setError(null);
    try {
      const [dashboard, events, rules, achievements, missions, currencies, rewards, campaigns, goals] = await Promise.all([
        fetchReportDashboard(scope), fetchEventTypes(scope).catch(() => []), fetchRules(scope).catch(() => []), fetchAchievements(scope).catch(() => []), fetchMissions(scope).catch(() => []), fetchCurrencies(scope).catch(() => []), fetchRewards(scope).catch(() => []), fetchCampaigns(scope).catch(() => []), fetchGoals(scope).catch(() => []),
      ]);
      const saved = dashboard.widgets.length > 0 ? dashboard.widgets : defaultWidgets;
      setWidgets(saved);
      setCatalog({ events, rules, achievements, missions, currencies, rewards, campaigns, goals });
      loadedRef.current = true;
      if (dashboard.widgets.length === 0) void saveReportDashboard({ ...scope, widgets: defaultWidgets }).catch(() => undefined);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Reports could not be loaded.');
      loadedRef.current = true;
    } finally {
      setLoading(false);
    }
  }, [scope]);

  const refreshReports = useCallback(async () => {
    if (!scope || widgets.length === 0) return;
    const range = makeRange(rangeDays);
    setLoadingWidgets(Object.fromEntries(widgets.map((widget) => [widget.id, true])));
    const responses = await Promise.all(widgets.map(async (widget) => {
      try {
        const result = await fetchReportQuery({ ...scope, measure: widget.measure, resource_id: widget.resource_id, event_type: widget.event_type, from: range.from, to: range.to, interval: widget.interval ?? 'day' });
        return [widget.id, result] as const;
      } catch {
        return [widget.id, undefined] as const;
      }
    }));
    setResults(Object.fromEntries(responses));
    setLoadingWidgets({});
    setLastUpdated(new Date());
  }, [rangeDays, scope, widgets]);

  useEffect(() => { void loadCatalogAndDashboard(); }, [loadCatalogAndDashboard]);
  useEffect(() => { if (loadedRef.current) void refreshReports(); }, [refreshReports]);
  useEffect(() => {
    if (!scope || !loadedRef.current) return undefined;
    const timer = window.setInterval(() => void refreshReports(), 30_000);
    return () => window.clearInterval(timer);
  }, [refreshReports, scope]);
  useEffect(() => {
    if (!scope || !loadedRef.current) return;
    setSaving(true);
    const timer = window.setTimeout(() => { void saveReportDashboard({ ...scope, widgets }).catch(() => undefined).finally(() => setSaving(false)); }, 350);
    return () => window.clearTimeout(timer);
  }, [scope, widgets]);
  useEffect(() => {
    if (!studioOpen) return undefined;
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setStudioOpen(false); };
    window.addEventListener('keydown', closeOnEscape);
    const previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      window.removeEventListener('keydown', closeOnEscape);
      document.body.style.overflow = previousBodyOverflow;
    };
  }, [studioOpen]);

  function moveWidget(widgetId: string, direction: -1 | 1) {
    setWidgets((current) => {
      const index = current.findIndex((widget) => widget.id === widgetId);
      const nextIndex = index + direction;
      if (index < 0 || nextIndex < 0 || nextIndex >= current.length) return current;
      const next = [...current];
      [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
      return next;
    });
  }

  function startWidgetDrag(widgetId: string, event: ReactDragEvent<HTMLButtonElement>) {
    setDraggingWidgetId(widgetId);
    setDropTarget(null);
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', widgetId);
  }

  function dragOverWidget(widgetId: string, event: ReactDragEvent<HTMLElement>) {
    if (!draggingWidgetId || draggingWidgetId === widgetId) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    const targetWidget = widgets.find((widget) => widget.id === widgetId);
    if (!targetWidget) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    const side: DropSide = targetWidget.size === 'wide'
      ? (event.clientY < bounds.top + bounds.height / 2 ? 'before' : 'after')
      : (event.clientX < bounds.left + bounds.width / 2 ? 'before' : 'after');
    setDropTarget((current) => current?.widgetId === widgetId && current.side === side ? current : { widgetId, side });
  }

  function dragOverGrid(event: ReactDragEvent<HTMLDivElement>) {
    if (!draggingWidgetId || event.target !== event.currentTarget || widgets.length === 0) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    const lastWidget = widgets[widgets.length - 1];
    setDropTarget({ widgetId: lastWidget.id, side: 'after' });
  }

  function dropWidget(widgetId: string, event: ReactDragEvent<HTMLElement>) {
    event.preventDefault();
    event.stopPropagation();
    const target = dropTarget?.widgetId === widgetId ? dropTarget : null;
    const sourceId = draggingWidgetId;
    if (!sourceId || !target || sourceId === widgetId) {
      endWidgetDrag();
      return;
    }
    setWidgets((current) => {
      const sourceIndex = current.findIndex((widget) => widget.id === sourceId);
      const targetIndex = current.findIndex((widget) => widget.id === widgetId);
      if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return current;
      const moved = current[sourceIndex];
      const next = current.filter((widget) => widget.id !== sourceId);
      const nextTargetIndex = next.findIndex((widget) => widget.id === widgetId);
      next.splice(nextTargetIndex + (target.side === 'after' ? 1 : 0), 0, moved);
      return next;
    });
    endWidgetDrag();
  }

  function endWidgetDrag() {
    setDraggingWidgetId(null);
    setDropTarget(null);
  }

  function resizeWidget(widgetId: string) {
    setWidgets((current) => current.map((widget) => widget.id === widgetId ? { ...widget, size: widget.size === 'wide' ? 'standard' : 'wide' } : widget));
  }

  function openBuilder(widget?: ReportWidgetConfig) {
    if (widget) {
      const definition = measureFor(widget.measure);
      setSelectedMeasure(widget.measure);
      setSelectedResource(widget.resource_id ?? '');
      setSelectedEventType(widget.event_type ?? '');
      setCustomTitle(widget.title === definition.label || widget.title?.startsWith(`${definition.label} · `) ? '' : widget.title ?? '');
      setEditingWidgetId(widget.id);
      setStudioMode('configure');
    } else {
      setSelectedMeasure('');
      setSelectedResource('');
      setSelectedEventType('');
      setCustomTitle('');
      setEditingWidgetId(null);
      setStudioMode('templates');
    }
    setBuilderSearch('');
    setShowAllMeasures(false);
    setStudioOpen(true);
  }

  function closeBuilder() {
    setStudioOpen(false);
    setStudioMode('templates');
    setSelectedMeasure('');
    setSelectedResource('');
    setSelectedEventType('');
    setCustomTitle('');
    setEditingWidgetId(null);
    setBuilderSearch('');
  }

  function selectMeasure(measureId: string) {
    setSelectedMeasure(measureId);
    setSelectedResource('');
    setSelectedEventType('');
    setCustomTitle('');
    setStudioMode('configure');
  }

  function saveWidget() {
    if (!selectedMeasure) return;
    const definition = measureFor(selectedMeasure);
    const title = customTitle.trim() || (selectedResourceName ? `${definition.label} · ${selectedResourceName}` : definition.label);
    const existingWidget = editingWidgetId ? widgets.find((widget) => widget.id === editingWidgetId) : undefined;
    const next: ReportWidgetConfig = { id: editingWidgetId ?? `${selectedMeasure}-${Date.now()}`, measure: selectedMeasure, title, resource_id: selectedResource || undefined, event_type: selectedEventType || undefined, interval: existingWidget?.interval ?? 'day', size: existingWidget?.size ?? definition.defaultSize ?? 'standard' };
    setWidgets((current) => editingWidgetId ? current.map((widget) => widget.id === editingWidgetId ? next : widget) : [...current, next]);
    closeBuilder();
  }

  if (!scope) return <p className="alert">Your workspace membership is still loading. Reports will appear when the workspace is connected.</p>;

  const refreshingWidgets = Object.keys(loadingWidgets).length > 0;
  const previewTitle = customTitle.trim() || (selectedResourceName ? `${selectedDefinition.label} · ${selectedResourceName}` : selectedDefinition.label);

  return <div className="reports">
    <div className="reports-toolbar">
      <p className="reports-status"><span><span className="dot dot-success" />Shared with this workspace</span><span>{saving ? 'Saving layout…' : 'Layout saved automatically'}</span>{lastUpdated && <span>Updated {lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>}</p>
      <div className="reports-toolbar-actions">
        <CustomSelect className="toolbar-select" ariaLabel="Report period" value={String(rangeDays)} options={[{ value: '7', label: 'Last 7 days' }, { value: '30', label: 'Last 30 days' }, { value: '90', label: 'Last 90 days' }]} onChange={(value) => setRangeDays(Number(value))} />
        <button className="icon-button" type="button" onClick={() => void refreshReports()} disabled={refreshingWidgets} aria-label="Refresh reports"><IconRefresh size={15} stroke={1.75} className={refreshingWidgets ? 'spin' : undefined} /></button>
        <button className="primary-button" type="button" onClick={() => openBuilder()}><IconPlus size={15} stroke={1.75} />Create a report</button>
      </div>
    </div>
    {error && <p className="alert alert-error" role="alert"><span>{error}</span></p>}
    {loading ? <div className="report-grid"><div className="report-widget report-widget-wide report-chart-placeholder tall" /><div className="report-widget report-chart-placeholder tall" /><div className="report-widget report-chart-placeholder tall" /></div> : widgets.length === 0 ? <div className="panel empty-state"><IconChartAreaLine size={22} stroke={1.5} aria-hidden="true" /><strong>Your shared dashboard is ready</strong><span>Start with a question such as “How many members unlocked an achievement?” and add the answer here.</span><button className="primary-button" type="button" onClick={() => openBuilder()}><IconPlus size={15} stroke={1.75} />Create a report</button></div> : <div className="report-grid" onDragOver={dragOverGrid} onDrop={(event) => { if (event.target === event.currentTarget && dropTarget) dropWidget(dropTarget.widgetId, event); }}>{widgets.map((widget) => <WidgetCard key={widget.id} widget={widget} result={results[widget.id]} loading={Boolean(loadingWidgets[widget.id])} periodLabel={periodLabel} dragging={draggingWidgetId === widget.id} dropSide={dropTarget?.widgetId === widget.id ? dropTarget.side : undefined} onEdit={() => openBuilder(widget)} onRemove={() => setWidgets((current) => current.filter((item) => item.id !== widget.id))} onMove={(direction) => moveWidget(widget.id, direction)} onResize={() => resizeWidget(widget.id)} onDragStart={(event) => startWidgetDrag(widget.id, event)} onDragOver={(event) => dragOverWidget(widget.id, event)} onDrop={(event) => dropWidget(widget.id, event)} onDragEnd={endWidgetDrag} />)}</div>}

    {studioOpen && <div className="dialog-layer" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeBuilder(); }}>
      <section className="dialog report-studio" role="dialog" aria-modal="true" aria-labelledby="report-studio-title" onMouseDown={(event) => event.stopPropagation()}>
        <header className="dialog-header">
          <div><h2 id="report-studio-title">{editingWidgetId ? 'Make this report yours' : 'Answer a real question'}</h2><p>{editingWidgetId ? 'Change the focus or time period. Your dashboard keeps the same place.' : 'Pick a question in plain language. We choose the measurement and chart for you.'}</p></div>
          <button className="icon-button ghost" type="button" onClick={closeBuilder} aria-label="Close report studio"><IconX size={16} /></button>
        </header>
        <ol className="stepper" aria-label="Report creation steps"><li className={studioMode === 'templates' ? 'is-current' : 'is-done'}><span>{studioMode === 'templates' ? '1' : <IconCheck size={11} stroke={2.5} />}</span>Pick a question</li><li className={studioMode === 'configure' ? 'is-current' : ''}><span>2</span>Make it yours</li><li><span>3</span>Add to dashboard</li></ol>
        <div className="report-studio-body">
          <div className="report-studio-library">
            <label className="search-field"><IconSearch size={15} stroke={1.75} aria-hidden="true" /><input value={builderSearch} onChange={(event) => setBuilderSearch(event.target.value)} placeholder="Search questions" aria-label="Search report questions" /></label>
            <div className="report-template-list">{visibleTemplates.length > 0 ? visibleTemplates.map((template) => <TemplateCard key={template.id} template={template} selected={template.measure === selectedMeasure} onSelect={() => selectMeasure(template.measure)} />) : <p className="column-empty">No matching question. Try “points”, “achievement”, or “members”.</p>}</div>
            {studioMode === 'templates' && !builderSearch && <button className="text-button" type="button" onClick={() => setShowAllMeasures((open) => !open)}><IconChevronDown size={14} stroke={1.75} className={showAllMeasures ? 'is-flipped' : ''} />{showAllMeasures ? 'Show common questions' : 'Explore every measurement'}</button>}
          </div>
          <div className="report-studio-detail">
            {studioMode === 'templates' ? <div className="report-studio-welcome"><IconChartAreaLine size={24} stroke={1.5} aria-hidden="true" /><h3>Start with the question, not a metric</h3><p>Choose one of the questions on the left. You can then focus it on a specific achievement, currency, reward, or event without leaving this screen.</p><div className="report-studio-example"><span>Example</span><strong>How many members unlocked “First purchase”?</strong><small>Achievement unlocks · Last 30 days</small></div></div> : <div className="form-stack">
              <button className="text-button report-studio-back" type="button" onClick={() => setStudioMode('templates')}><IconArrowLeft size={14} stroke={1.75} />Choose another question</button>
              <div className="report-selected-question"><span className="report-template-icon" aria-hidden="true"><SelectedIcon size={16} stroke={1.75} /></span><div><strong>{selectedDefinition.question}</strong><small>{selectedDefinition.description}</small></div></div>
              {selectedDefinition.resource && <div className="field"><span className="field-label" id="report-resource-label">Focus on one {resourceLabel(selectedDefinition.resource)} <em>Optional</em></span><CustomSelect ariaLabel={`Focus on one ${resourceLabel(selectedDefinition.resource)}`} value={selectedResourceValue} options={resourceOptions} onChange={(value) => selectedDefinition.resource === 'event' ? setSelectedEventType(value) : setSelectedResource(value)} /><small className="field-help">Leave this on all to see the whole workspace.</small></div>}
              <div className="field"><span className="field-label">When should we look?</span><div className="segmented" role="group" aria-label="Report period">{[7, 30, 90].map((days) => <button key={days} type="button" aria-pressed={rangeDays === days} onClick={() => setRangeDays(days)}>{days} days</button>)}</div><small className="field-help">This also changes the dashboard view after you save.</small></div>
              <label className="field"><span className="field-label">Name this view <em>Optional</em></span><input value={customTitle} onChange={(event) => setCustomTitle(event.target.value)} placeholder={selectedDefinition.label} /></label>
              <div className="report-preview"><div className="report-preview-head"><strong>{previewTitle}</strong><span className="badge badge-neutral">Sample shape</span></div><p>{selectedDefinition.question}{selectedResourceName ? ` for ${selectedResourceName}` : ''} · {periodLabel}</p><Chart points={[{ at: '2025-01-01', value: 8 }, { at: '2025-01-02', value: 14 }, { at: '2025-01-03', value: 11 }, { at: '2025-01-04', value: 23 }, { at: '2025-01-05', value: 18 }, { at: '2025-01-06', value: 28 }, { at: '2025-01-07', value: 24 }]} wide /></div>
            </div>}
          </div>
        </div>
        {studioMode === 'configure' && <footer className="dialog-footer"><button className="secondary-button" type="button" onClick={closeBuilder}>Cancel</button><button className="primary-button" type="button" onClick={saveWidget}><IconCheck size={15} stroke={2} />{editingWidgetId ? 'Save changes' : 'Add to dashboard'}</button></footer>}
      </section>
    </div>}
  </div>;
}
