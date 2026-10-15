import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconBraces, IconCheck, IconChevronDown, IconCircleCheck, IconCircleDashed, IconCode, IconCoin, IconFileText, IconPlayerPlay, IconPlus, IconRefresh, IconSparkles, IconTrash } from '@tabler/icons-react';
import { CurrencyCreateDialog } from '../../../features/currency-create/ui/CurrencyCreateDialog';
import { EventTypeCreateDialog } from '../../../features/event-type-create/ui/EventTypeCreateDialog';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { GuidePanel } from '../../../shared/ui/GuidePanel';
import { PayloadShapeBuilder, createPayloadField, payloadFieldsFromSchema, payloadFieldNamePattern, payloadSampleRows, schemaFromPayloadFields, type PayloadField } from '../../../shared/ui/PayloadShapeBuilder';
import { createEventType, createRule, createRuleVersion, fetchCurrencies, fetchEventTypes, fetchRules, fetchScopedEventStatus, publishRule, submitAdminTestEvent, type Currency, type EventReceipt, type EventType, type Rule, type RuleAction, type Tenant } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { PageHeader } from '../../../shared/ui/PageHeader';

type ConfigureTab = 'events' | 'rules';
type ConditionOperator = 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'exists';
type ConditionRow = { id: string; path: string; operator: ConditionOperator; value: string; valueType: 'string' | 'number' | 'boolean' };
type ActionType = 'award_xp' | 'award_points' | 'log';
type ActionRow = { id: string; type: ActionType; amount: string; currency: string; message: string };
type CreateDrawer = { kind: 'event'; initialName: string } | { kind: 'currency'; actionId: string; initialCode: string } | null;

const createEventOption = '__create_event__';
const createCurrencyOption = '__create_currency__';

const configureGuide = {
  events: { purpose: 'Define the stable signals your product sends so every automation and report understands the same data.', steps: ['Register an event name such as order.completed.', 'Choose the information that arrives with the event.', 'Send the event through the REST API and use it in a workflow.'], example: 'Send order.completed with order_id, user_id, and total.', result: 'The event becomes available to automations, goals, and analytics.' },
  rules: { purpose: 'Turn a trusted event into an outcome such as XP, points, or an audit record.', steps: ['Choose the event that starts the automation.', 'Optionally add conditions to narrow the audience.', 'Add one or more outcomes, then publish the draft.'], example: 'When order.completed arrives, give 10 XP and 5 coins.', result: 'One matching event updates progression and the user balance automatically.' },
} as const;

const eventNamePattern = /^[a-z0-9][a-z0-9._-]{0,127}$/;

function makeId(prefix: string) {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}-${Date.now()}`;
}

function scopeKey(scope?: Pick<Tenant, 'tenant_id' | 'environment_id'>) {
  return scope ? `${scope.tenant_id}::${scope.environment_id}` : '';
}

function samplePayloadFromSchema(schema?: Record<string, unknown>): Record<string, unknown> {
  let normalized = schema ?? {};
  if (typeof normalized === 'string') {
    try { normalized = JSON.parse(normalized) as Record<string, unknown>; } catch { normalized = {}; }
  }
  if (normalized.schema && typeof normalized.schema === 'object' && !Array.isArray(normalized.schema)) normalized = normalized.schema as Record<string, unknown>;
  const properties = normalized.properties && typeof normalized.properties === 'object' && !Array.isArray(normalized.properties) ? normalized.properties as Record<string, unknown> : {};
  const required = Array.isArray(normalized.required) ? normalized.required.filter((name): name is string => typeof name === 'string') : [];
  const definitions = Object.keys(properties).length > 0 ? Object.entries(properties) : required.map((name) => [name, {}] as [string, unknown]);
  return definitions.reduce<Record<string, unknown>>((payload, [name, definition]) => {
    const field = definition && typeof definition === 'object' && !Array.isArray(definition) ? definition as Record<string, unknown> : {};
    const type = field.type;
    if (type === 'boolean') payload[name] = true;
    else if (type === 'number' || type === 'integer') payload[name] = 1;
    else if (type === 'array') payload[name] = [];
    else if (type === 'object') payload[name] = {};
    else if (field.format === 'date-time') payload[name] = new Date().toISOString();
    else payload[name] = typeof field.example === 'string' || typeof field.example === 'number' || typeof field.example === 'boolean' ? field.example : 'example';
    return payload;
  }, {});
}

function valueFromCondition(row: ConditionRow): unknown {
  if (row.valueType === 'number') return Number(row.value);
  if (row.valueType === 'boolean') return row.value === 'true';
  return row.value;
}

function conditionsFromRows(group: 'all' | 'any', rows: ConditionRow[]) {
  if (rows.length === 0) return {};
  return { [group]: rows.map((row) => row.operator === 'exists' ? { exists: row.path.trim() } : { [row.operator]: [row.path.trim(), valueFromCondition(row)] }) };
}

function newAction(type: ActionType): ActionRow {
  return { id: makeId('action'), type, amount: '10', currency: 'coins', message: 'Rule matched' };
}

function actionFromRow(row: ActionRow): RuleAction {
  if (row.type === 'award_points') return { type: row.type, currency: row.currency.trim(), amount: Number(row.amount) };
  if (row.type === 'award_xp') return { type: row.type, amount: Number(row.amount) };
  return { type: row.type, message: row.message.trim() || 'Rule matched' };
}

function actionSummary(row: ActionRow) {
  if (row.type === 'award_points') return `+${row.amount || '0'} ${row.currency.trim() || 'points'}`;
  if (row.type === 'award_xp') return `+${row.amount || '0'} XP`;
  return 'Record an audit log';
}

function conditionHasIssue(row: ConditionRow) {
  return !row.path.trim().startsWith('$.') || (row.operator !== 'exists' && !row.value.trim());
}

function conditionRowsFromDefinition(definition: Record<string, unknown> | undefined): { group: 'all' | 'any'; rows: ConditionRow[]; advanced: boolean } {
  if (!definition || typeof definition !== 'object' || Array.isArray(definition)) return { group: 'all', rows: [], advanced: false };
  const group = Array.isArray(definition.any) ? 'any' : 'all';
  const entries = Array.isArray(definition[group]) ? definition[group] : [];
  const rows: ConditionRow[] = [];
  for (const entry of entries) {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return { group, rows: [], advanced: true };
    const object = entry as Record<string, unknown>;
    const operator = Object.keys(object)[0] as ConditionOperator | undefined;
    if (!operator || !['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'exists'].includes(operator)) return { group, rows: [], advanced: true };
    if (operator === 'exists') {
      if (typeof object[operator] !== 'string') return { group, rows: [], advanced: true };
      rows.push({ id: makeId('condition'), path: object[operator] as string, operator, value: '', valueType: 'string' });
      continue;
    }
    const pair = Array.isArray(object[operator]) ? object[operator] : [];
    if (pair.length !== 2 || typeof pair[0] !== 'string') return { group, rows: [], advanced: true };
    const rawValue = pair[1];
    const valueType: ConditionRow['valueType'] = typeof rawValue === 'number' ? 'number' : typeof rawValue === 'boolean' ? 'boolean' : 'string';
    rows.push({ id: makeId('condition'), path: pair[0], operator, value: String(rawValue ?? ''), valueType });
  }
  return { group, rows, advanced: false };
}

function actionRowsFromDefinition(action: RuleAction | RuleAction[] | undefined): ActionRow[] {
  const entries = Array.isArray(action) ? action : action ? [action] : [];
  return entries.map((entry) => {
    const type = entry.type === 'award_xp' || entry.type === 'award_points' || entry.type === 'log' ? entry.type : 'log';
    return { id: makeId('action'), type, amount: entry.amount === undefined ? '10' : String(entry.amount), currency: entry.currency ?? 'coins', message: entry.message ?? 'Rule matched' };
  });
}

function localDateTime(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 16);
}

function EventBuilder({ activeScope, initialName, editingEvent, onCreated, onCancelEdit }: { activeScope?: Tenant; initialName?: string; editingEvent?: EventType | null; onCreated: (eventType: EventType) => void; onCancelEdit: () => void }) {
  const [name, setName] = useState('');
  const [fields, setFields] = useState<PayloadField[]>([createPayloadField(true)]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (editingEvent) {
      setName(editingEvent.name);
      setFields(payloadFieldsFromSchema(editingEvent.schema));
      setSaved(false);
      setError(null);
      return;
    }
  }, [editingEvent]);
  useEffect(() => { if (initialName) setName(initialName); }, [initialName]);
  const selectedScope = activeScope;
  const previewRows = useMemo(() => payloadSampleRows(fields), [fields]);

  function cancelEdit() { onCancelEdit(); setName(''); setFields([createPayloadField(true)]); setError(null); setSaved(false); }

  async function submit() {
    setError(null);
    setSaved(false);
    if (!selectedScope) { setError('Your workspace is not connected yet.'); return; }
    if (!eventNamePattern.test(name.trim())) { setError('Use lowercase event names such as order.completed.'); return; }
    const names = fields.filter((field) => field.name.trim()).map((field) => field.name.trim());
    if (names.length === 0) { setError('Add at least one field so Motif knows what to expect.'); return; }
    if (names.some((fieldName) => !payloadFieldNamePattern.test(fieldName))) { setError('Field names must start with a letter or underscore.'); return; }
    if (new Set(names).size !== names.length) { setError('Each field name must be unique.'); return; }
    const schema = schemaFromPayloadFields(fields);
    setSaving(true);
    try {
      const created = await createEventType({ tenant_id: selectedScope.tenant_id, environment_id: selectedScope.environment_id, name: name.trim(), schema });
      onCreated(created);
      setSaved(true);
      setName('');
      setFields([createPayloadField(true)]);
    } catch (creationError) {
      setError(creationError instanceof Error ? creationError.message : 'Event type could not be created.');
    } finally { setSaving(false); }
  }

  const invalidName = Boolean(name && !eventNamePattern.test(name.trim()));
  return <div className="builder">
    <div className="builder-main">
      <section className="panel" aria-labelledby="event-name-title">
        <div className="panel-header"><div><h2 id="event-name-title">Name the event</h2><p>Use a stable lowercase namespace.</p></div>{editingEvent && <span className="badge badge-neutral">Current · v{editingEvent.version}</span>}</div>
        <div className="panel-body"><label className="field"><span className="field-label">Event type name</span><input className="mono" value={name} onChange={(event) => setName(event.target.value)} placeholder="order.completed" autoComplete="off" spellCheck={false} disabled={Boolean(editingEvent)} aria-invalid={invalidName} />{invalidName ? <small className="field-error">Use lowercase letters, numbers, dots, underscores, or hyphens.</small> : <small className="field-help">{editingEvent ? 'Event names are immutable. Create a new contract to rename this signal.' : 'Examples: user.signed_up, lesson.completed'}</small>}</label></div>
      </section>
      <section className="panel" aria-labelledby="event-fields-title">
        <div className="panel-header"><div><h2 id="event-fields-title">Choose the information to send</h2><p>Use plain fields. No JSON needed.</p></div></div>
        <div className="panel-body"><PayloadShapeBuilder fields={fields} onChange={setFields} /></div>
      </section>
    </div>
    <aside className="builder-aside" aria-label="Event contract review">
      <section className="panel" aria-labelledby="event-preview-title">
        <div className="panel-header"><div><h2 id="event-preview-title">Live preview</h2><p>When <code>{name || 'this event'}</code> arrives, Motif looks for these fields.</p></div><IconFileText size={16} stroke={1.75} className="panel-icon" aria-hidden="true" /></div>
        <ul className="preview-list">{previewRows.map((row) => <li key={row.id}><span className="preview-key">{row.name}</span><span className="preview-value">{row.value}</span><span className={`badge ${row.required ? 'badge-neutral' : 'badge-outline'}`}>{row.required ? 'Required' : 'Optional'}</span></li>)}</ul>
      </section>
      <section className="panel builder-submit" aria-label="Save event contract">
        <p>{editingEvent ? `Editing ${editingEvent.name} creates version ${editingEvent.version + 1}; previous versions remain available for audit.` : 'Motif validates every matching event against this contract.'}</p>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        {saved && <p className="alert alert-success" role="status"><IconCheck size={16} aria-hidden="true" /><span>{editingEvent ? 'New event contract version saved and active.' : 'Event type saved as an active contract.'}</span></p>}
        <button className="primary-button full" type="button" onClick={() => void submit()} disabled={saving || !selectedScope}>{saving ? 'Saving contract…' : editingEvent ? 'Save new event version' : 'Save event type'}</button>
        <span className="form-footer-note">{editingEvent ? `Creates version ${editingEvent.version + 1} in ${selectedScope?.environment_name ?? 'your workspace'}.` : `Creates version 1 in ${selectedScope?.environment_name ?? 'your workspace'}.`}</span>
        {editingEvent && <button className="text-button full-center" type="button" onClick={cancelEdit}>Cancel edit</button>}
      </section>
    </aside>
  </div>;
}

function RuleBuilder({ activeScope, eventTypes, currencies, editingRule, onCreated, onEventCreated, onCurrencyCreated, onCancelEdit }: { activeScope?: Tenant; eventTypes: EventType[]; currencies: Currency[]; editingRule?: Rule | null; onCreated: (rule: Rule) => void; onEventCreated: (eventType: EventType) => void; onCurrencyCreated: (currency: Currency) => void; onCancelEdit: () => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [eventType, setEventType] = useState('');
  const [group, setGroup] = useState<'all' | 'any'>('all');
  const [conditions, setConditions] = useState<ConditionRow[]>([]);
  const [advancedConditions, setAdvancedConditions] = useState(false);
  const [conditionsJson, setConditionsJson] = useState('{}');
  const [actions, setActions] = useState<ActionRow[]>([]);
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [testOpen, setTestOpen] = useState(false);
  const [testUserId, setTestUserId] = useState('demo-user');
  const [testPayload, setTestPayload] = useState('{}');
  const [testReceipt, setTestReceipt] = useState<EventReceipt | null>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const [testSending, setTestSending] = useState(false);
  const [createDrawer, setCreateDrawer] = useState<CreateDrawer>(null);

  useEffect(() => {
    if (editingRule) return;
    setEventType('');
  }, [activeScope, editingRule]);
  useEffect(() => {
    if (!editingRule) return;
    const parsedConditions = conditionRowsFromDefinition(editingRule.conditions);
    setName(editingRule.name);
    setDescription(editingRule.description ?? '');
    setEventType(editingRule.event_type);
    setGroup(parsedConditions.group);
    setConditions(parsedConditions.rows);
    setAdvancedConditions(parsedConditions.advanced);
    setConditionsJson(JSON.stringify(editingRule.conditions ?? {}, null, 2));
    setActions(actionRowsFromDefinition(editingRule.action));
    setStartsAt(localDateTime(editingRule.starts_at));
    setEndsAt(localDateTime(editingRule.ends_at));
    setSaved(false);
    setError(null);
    setTestOpen(false);
  }, [editingRule]);
  useEffect(() => { if (!eventType && activeScope) { const firstEvent = eventTypes.find((item) => scopeKey(item) === scopeKey(activeScope)); if (firstEvent) setEventType(firstEvent.name); } }, [eventType, eventTypes, activeScope]);
  const selectedScope = activeScope;
  const scopedEvents = eventTypes.filter((item) => item.tenant_id === selectedScope?.tenant_id && item.environment_id === selectedScope?.environment_id);
  const scopedCurrencies = currencies.filter((item) => item.tenant_id === selectedScope?.tenant_id && item.environment_id === selectedScope?.environment_id && item.active);
  const eventIsRegistered = scopedEvents.some((item) => item.name === eventType);
  const registeredEvent = scopedEvents.find((item) => item.name === eventType);
  const pointsActions = actions.filter((row) => row.type === 'award_points');
  const pointsCurrencyReady = pointsActions.length === 0 || pointsActions.every((row) => scopedCurrencies.some((currency) => currency.code === row.currency.trim()));
  const conditionPreview = useMemo(() => {
    if (!advancedConditions) return conditionsFromRows(group, conditions);
    try { return JSON.parse(conditionsJson || '{}') as Record<string, unknown>; } catch { return { invalid: true }; }
  }, [advancedConditions, conditions, conditionsJson, group]);
  const actionPreview = useMemo(() => actions.map(actionFromRow), [actions]);

  const preflightItems = [
    { label: selectedScope ? `Workspace connected · ${selectedScope.name}` : 'Connect your workspace', ready: Boolean(selectedScope), tone: 'required' },
    { label: eventIsRegistered ? `Event contract ready · ${eventType}` : eventType ? 'Event contract is missing in this workspace' : 'Choose the event that starts this automation', ready: eventIsRegistered, tone: eventIsRegistered ? 'ready' : 'warning' },
    { label: actions.length > 0 ? `${actions.length} outcome${actions.length === 1 ? '' : 's'} configured` : 'Add at least one outcome', ready: actions.length > 0, tone: 'required' },
    ...(pointsActions.length > 0 ? [{ label: pointsCurrencyReady ? 'Every points outcome uses an active currency' : 'Create or choose an active points currency', ready: pointsCurrencyReady, tone: pointsCurrencyReady ? 'ready' : 'warning' }] : []),
  ];

  function addCondition() { setConditions((current) => [...current, { id: makeId('condition'), path: '$.', operator: 'eq', value: '', valueType: 'string' }]); }
  function updateCondition(id: string, patch: Partial<ConditionRow>) { setConditions((current) => current.map((row) => row.id === id ? { ...row, ...patch } : row)); }
  function removeCondition(id: string) { setConditions((current) => current.filter((row) => row.id !== id)); }
  function addAction(type: ActionType) { setActions((current) => [...current, { ...newAction(type), ...(type === 'award_points' && scopedCurrencies[0] ? { currency: scopedCurrencies[0].code } : {}) }]); }
  function updateAction(id: string, patch: Partial<ActionRow>) { setActions((current) => current.map((row) => row.id === id ? { ...row, ...patch } : row)); }
  function removeAction(id: string) { setActions((current) => current.filter((row) => row.id !== id)); }
  function applyTemplate(template: 'purchase' | 'signup' | 'custom') {
    if (template === 'purchase') {
      setName('Completed purchase reward');
      setDescription('Reward the customer after a completed purchase.');
      setEventType('order.completed');
      setConditions([]);
      setActions([{ ...newAction('award_xp'), amount: '10' }, { ...newAction('award_points'), amount: '5', currency: scopedCurrencies.find((item) => item.code === 'coins')?.code ?? scopedCurrencies[0]?.code ?? 'coins' }]);
      return;
    }
    if (template === 'signup') {
      setName('Welcome reward');
      setDescription('Give new customers a small welcome bonus.');
      setEventType('user.signed_up');
      setConditions([]);
      setActions([{ ...newAction('award_xp'), amount: '25' }, { ...newAction('award_points'), amount: '10', currency: scopedCurrencies.find((item) => item.code === 'coins')?.code ?? scopedCurrencies[0]?.code ?? 'coins' }]);
      return;
    }
    setName('');
    setDescription('');
    setEventType('');
    setConditions([]);
    setActions([]);
  }
  function resetForm() { setName(''); setDescription(''); setConditions([]); setConditionsJson('{}'); setActions([]); setStartsAt(''); setEndsAt(''); }

  function openTestPanel() {
    setTestError(null);
    setTestReceipt(null);
    setTestPayload(JSON.stringify(samplePayloadFromSchema(registeredEvent?.schema), null, 2));
    setTestOpen(true);
  }

  async function submitTestEvent() {
    setTestError(null);
    setTestReceipt(null);
    if (!selectedScope) { setTestError('Your workspace is not connected yet.'); return; }
    if (!eventIsRegistered) { setTestError('Create the event contract first. A test event needs its schema.'); return; }
    if (!testUserId.trim()) { setTestError('Enter the external user ID that should receive the outcome.'); return; }
    let payload: Record<string, unknown>;
    try {
      payload = JSON.parse(testPayload) as Record<string, unknown>;
      if (!payload || Array.isArray(payload) || typeof payload !== 'object') throw new Error();
    } catch {
      setTestError('Sample payload must be a valid JSON object.');
      return;
    }
    setTestSending(true);
    try {
      const externalEventId = `console-automation-${Date.now()}`;
      const receipt = await submitAdminTestEvent({ tenant_id: selectedScope.tenant_id, environment_id: selectedScope.environment_id, external_event_id: externalEventId, event_type: eventType, user_id: testUserId.trim(), payload });
      setTestReceipt(receipt);
      void (async () => {
        for (let attempt = 0; attempt < 6; attempt += 1) {
          await new Promise((resolve) => window.setTimeout(resolve, 350));
          try {
            const status = await fetchScopedEventStatus(receipt.external_event_id, { tenant_id: selectedScope.tenant_id, environment_id: selectedScope.environment_id });
            setTestReceipt(status);
            if (status.status === 'processed' || status.status === 'failed') break;
          } catch {
            // Keep the accepted receipt visible while the worker catches up.
          }
        }
      })();
    } catch (submissionError) {
      setTestError(submissionError instanceof Error ? submissionError.message : 'The sample event could not be sent.');
    } finally { setTestSending(false); }
  }

  async function submit() {
    setError(null); setSaved(false);
    if (!selectedScope) { setError('Your workspace is not connected yet.'); return; }
    if (!name.trim()) { setError('Give the rule a name before saving.'); return; }
    if (!eventType.trim() || !eventNamePattern.test(eventType.trim())) { setError('Choose an event type or use a valid lowercase event name.'); return; }
    if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) { setError('End time must be later than start time.'); return; }
    if (actions.length === 0) { setError('Add at least one outcome for this automation.'); return; }
    let conditionObject: Record<string, unknown>;
    if (advancedConditions) {
      try { conditionObject = JSON.parse(conditionsJson) as Record<string, unknown>; if (!conditionObject || Array.isArray(conditionObject)) throw new Error(); } catch { setError('Advanced conditions must be a valid JSON object.'); return; }
    } else {
      if (conditions.some((row) => !row.path.trim() || !row.path.trim().startsWith('$.') || (row.operator !== 'exists' && !row.value.trim()))) { setError('Complete every condition with a payload path and value.'); return; }
      conditionObject = conditionsFromRows(group, conditions);
    }
    if (actions.some((row) => row.type === 'award_points' && (!row.currency.trim() || Number(row.amount) <= 0))) { setError('Every points outcome needs a currency and a positive amount.'); return; }
    if (actions.some((row) => row.type === 'award_xp' && Number(row.amount) <= 0)) { setError('Every XP outcome needs a positive amount.'); return; }
    if (!pointsCurrencyReady) { setError('Choose an active points currency before saving this automation.'); return; }
    setSaving(true);
    try {
      const input = { tenant_id: selectedScope.tenant_id, environment_id: selectedScope.environment_id, name: name.trim(), description: description.trim(), event_type: eventType.trim(), conditions: conditionObject, actions: actionPreview, starts_at: startsAt ? new Date(startsAt).toISOString() : undefined, ends_at: endsAt ? new Date(endsAt).toISOString() : undefined };
      const savedRule = editingRule
        ? await createRuleVersion(editingRule.id, input)
        : await createRule({ ...input, name: name.trim(), description: description.trim() });
      onCreated(savedRule); setSaved(true); resetForm();
    } catch (creationError) { setError(creationError instanceof Error ? creationError.message : 'Rule could not be saved.'); } finally { setSaving(false); }
  }

  const recipeReady = Boolean(selectedScope && name.trim() && eventIsRegistered && actions.length > 0 && pointsCurrencyReady);
  const summaryEvent = eventType || 'Choose an event';
  const summaryOutcomes = actions.length > 0 ? actions.map(actionSummary).join(' · ') : 'No outcomes yet';
  const readyCount = preflightItems.filter((item) => item.ready).length;
  const outcomeIcon = (type: ActionType) => type === 'award_xp' ? <IconSparkles size={15} stroke={1.75} /> : type === 'award_points' ? <IconCoin size={15} stroke={1.75} /> : <IconFileText size={15} stroke={1.75} />;

  return <>
    <div className="builder">
      <div className="builder-main">
        <section className="panel" aria-labelledby="recipe-basics-title">
          <div className="panel-header"><div><h2 id="recipe-basics-title">Basics</h2><p>Give this automation a clear purpose before you save it.</p></div>{editingRule && <span className="badge badge-neutral">Editing v{editingRule.version}</span>}</div>
          <div className="panel-body form-stack">
            <div className="starter-row" role="group" aria-labelledby="recipe-starter-label"><span id="recipe-starter-label">Start with a recipe</span><button className="chip" type="button" onClick={() => applyTemplate('purchase')}>Completed purchase</button><button className="chip" type="button" onClick={() => applyTemplate('signup')}>Welcome reward</button><button className="chip chip-quiet" type="button" onClick={() => applyTemplate('custom')}>Start blank</button></div>
            <div className="form-row">
              <label className="field"><span className="field-label">Automation name</span><input value={name} onChange={(event) => setName(event.target.value)} placeholder="Reward a completed purchase" aria-invalid={Boolean(name && name.trim().length > 120)} />{name.trim().length > 120 && <small className="field-error">Keep the name under 120 characters.</small>}</label>
              <label className="field"><span className="field-label">Description <em>Optional</em></span><input value={description} onChange={(event) => setDescription(event.target.value)} placeholder="What should happen after the event?" aria-invalid={Boolean(description.length > 500)} />{description.length > 500 && <small className="field-error">Keep the description under 500 characters.</small>}</label>
            </div>
          </div>
        </section>

        <ol className="recipe-steps">
          <li className="recipe-step">
            <section className="panel" aria-labelledby="recipe-when-title">
              <div className="panel-header"><div><h2 id="recipe-when-title"><span className="recipe-key">When</span></h2><p>Choose the event that starts this automation.</p></div><span className={`badge ${eventIsRegistered ? 'badge-success' : 'badge-neutral'}`}>{eventIsRegistered ? 'Contract ready' : 'Event contract'}</span></div>
              <div className="panel-body form-stack">
                <div className="field"><span className="field-label" id="recipe-event-label">Event that starts the automation</span><CustomSelect ariaLabel="Event that starts the automation" value={eventType} placeholder="Choose an event type" onChange={(value) => { if (value === createEventOption) { setCreateDrawer({ kind: 'event', initialName: eventType.trim() }); return; } setEventType(value); }} options={[...(eventType && !eventIsRegistered ? [{ value: eventType, label: `${eventType} · not registered`, tone: 'accent' as const }] : []), ...scopedEvents.map((item) => ({ value: item.name, label: item.name })), { value: '__event_divider__', label: '', disabled: true, divider: true }, { value: createEventOption, label: 'Create new event…', tone: 'accent' }]} /><div className="field-footer"><small className="field-help">{eventIsRegistered ? 'This event is ready to trigger the automation.' : 'Choose an existing event or create one here.'}</small><button className="text-button accent compact" type="button" onClick={() => setCreateDrawer({ kind: 'event', initialName: eventType.trim() })}><IconPlus size={14} stroke={1.75} />Create new event</button></div></div>
                {!eventIsRegistered && eventType && <p className="alert alert-warning"><IconAlertTriangle size={16} aria-hidden="true" /><span><strong>{eventType}</strong> is not registered in this workspace yet. Create its contract here to keep building.</span></p>}
              </div>
            </section>
          </li>

          <li className="recipe-step">
            <section className="panel" aria-labelledby="recipe-if-title">
              <div className="panel-header"><div><h2 id="recipe-if-title"><span className="recipe-key">If</span> <em>Optional</em></h2><p>Add conditions to narrow when this automation should run.</p></div><span className="panel-meta">{conditions.length ? `${conditions.length} condition${conditions.length === 1 ? '' : 's'}` : 'Matches every payload'}</span></div>
              <div className="panel-body form-stack">
                {!advancedConditions ? <>
                  <div className="condition-toolbar"><CustomSelect className="condition-group-select" value={group} onChange={(value) => setGroup(value as 'all' | 'any')} ariaLabel="Condition group" options={[{ value: 'all', label: 'All conditions (AND)' }, { value: 'any', label: 'Any condition (OR)' }]} /><button className="secondary-button" type="button" onClick={addCondition}><IconPlus size={14} stroke={1.75} />Add condition</button></div>
                  {conditions.length > 0 ? <div className="condition-list">{conditions.map((row, index) => <div className={`condition-row ${row.operator === 'exists' ? 'is-exists' : ''}`} key={row.id}><input className="mono" aria-label={`Condition ${index + 1} path`} value={row.path} onChange={(event) => updateCondition(row.id, { path: event.target.value })} placeholder="$.order.total" spellCheck={false} aria-invalid={conditionHasIssue(row) && row.path !== '$.'} /><CustomSelect ariaLabel={`Condition ${index + 1} operator`} value={row.operator} onChange={(value) => updateCondition(row.id, { operator: value as ConditionOperator })} options={[{ value: 'eq', label: 'equals' }, { value: 'neq', label: 'does not equal' }, { value: 'gt', label: 'greater than' }, { value: 'gte', label: 'at least' }, { value: 'lt', label: 'less than' }, { value: 'lte', label: 'at most' }, { value: 'exists', label: 'exists' }]} />{row.operator !== 'exists' && <><CustomSelect ariaLabel={`Condition ${index + 1} value type`} value={row.valueType} onChange={(value) => updateCondition(row.id, { valueType: value as ConditionRow['valueType'] })} options={[{ value: 'string', label: 'text' }, { value: 'number', label: 'number' }, { value: 'boolean', label: 'true/false' }]} /><input aria-label={`Condition ${index + 1} value`} value={row.value} onChange={(event) => updateCondition(row.id, { value: event.target.value })} placeholder="paid" /></>}<button className="icon-button ghost danger" type="button" onClick={() => removeCondition(row.id)} aria-label={`Remove condition ${index + 1}`}><IconTrash size={15} stroke={1.75} /></button></div>)}</div> : <p className="inline-note"><IconCircleDashed size={15} stroke={1.75} aria-hidden="true" />No conditions yet. This automation will match every valid payload for the selected event.</p>}
                </> : <label className="field"><span className="field-label">Advanced conditions JSON</span><textarea className="advanced-json" value={conditionsJson} onChange={(event) => setConditionsJson(event.target.value)} spellCheck={false} /><small className="field-help">Use the restricted operators: all, any, eq, neq, gt, gte, lt, lte, exists.</small></label>}
                <div><button className="text-button" type="button" onClick={() => { setAdvancedConditions(!advancedConditions); if (!advancedConditions) setConditionsJson(JSON.stringify(conditionsFromRows(group, conditions), null, 2)); }}><IconCode size={14} stroke={1.75} />{advancedConditions ? 'Use condition builder' : 'Open advanced JSON'}</button></div>
              </div>
            </section>
          </li>

          <li className="recipe-step">
            <section className="panel" aria-labelledby="recipe-then-title">
              <div className="panel-header"><div><h2 id="recipe-then-title"><span className="recipe-key">Then</span></h2><p>Define what should happen when the event and conditions are met.</p></div><span className="panel-meta">{actions.length ? `${actions.length} outcome${actions.length === 1 ? '' : 's'}` : 'Add an outcome'}</span></div>
              <div className="panel-body form-stack">
                <div className="outcome-palette" role="group" aria-label="Add an outcome"><button className="secondary-button" type="button" onClick={() => addAction('award_xp')}><IconSparkles size={15} stroke={1.75} />Give XP</button><button className="secondary-button" type="button" onClick={() => addAction('award_points')}><IconCoin size={15} stroke={1.75} />Give points</button><button className="secondary-button" type="button" onClick={() => addAction('log')}><IconFileText size={15} stroke={1.75} />Record a log</button></div>
                {actions.length > 0 ? <ul className="outcome-list">{actions.map((row, index) => { const currencyExists = scopedCurrencies.some((currency) => currency.code === row.currency.trim()); return <li className={`outcome-row outcome-${row.type}`} key={row.id}>
                  <span className="outcome-icon" aria-hidden="true">{outcomeIcon(row.type)}</span>
                  <div className="outcome-fields">
                    <div className="field"><span className="field-label">Outcome</span><CustomSelect ariaLabel={`Action ${index + 1} type`} value={row.type} onChange={(value) => updateAction(row.id, { type: value as ActionType })} options={[{ value: 'award_xp', label: 'Give XP' }, { value: 'award_points', label: 'Give points' }, { value: 'log', label: 'Record a log' }]} /></div>
                    {row.type === 'award_points' && <div className="field"><span className="field-label">Points currency</span><CustomSelect ariaLabel={`Action ${index + 1} points name`} value={row.currency} placeholder="Choose a currency" onChange={(value) => { if (value === createCurrencyOption) { setCreateDrawer({ kind: 'currency', actionId: row.id, initialCode: row.currency.trim() || 'coins' }); return; } updateAction(row.id, { currency: value }); }} options={[...(row.currency && !currencyExists ? [{ value: row.currency, label: `${row.currency} · not found`, tone: 'accent' as const }] : []), ...scopedCurrencies.map((currency) => ({ value: currency.code, label: `${currency.name} · ${currency.code}` })), { value: '__currency_divider__', label: '', disabled: true, divider: true }, { value: createCurrencyOption, label: 'Create new currency…', tone: 'accent' }]} />{currencyExists ? null : <div className="field-footer"><small className="field-error">Currency missing</small><button className="text-button accent compact" type="button" onClick={() => setCreateDrawer({ kind: 'currency', actionId: row.id, initialCode: row.currency.trim() || 'coins' })}><IconPlus size={14} stroke={1.75} />Create currency</button></div>}</div>}
                    {row.type === 'award_xp' && <label className="field"><span className="field-label">XP amount</span><input aria-label={`Action ${index + 1} amount`} type="number" min="1" step="1" value={row.amount} onChange={(event) => updateAction(row.id, { amount: event.target.value })} /></label>}
                    {row.type === 'award_points' && <label className="field"><span className="field-label">Amount</span><input aria-label={`Action ${index + 1} amount`} type="number" min="1" step="1" value={row.amount} onChange={(event) => updateAction(row.id, { amount: event.target.value })} /></label>}
                    {row.type === 'log' && <label className="field outcome-wide"><span className="field-label">Log message</span><input aria-label={`Action ${index + 1} message`} value={row.message} onChange={(event) => updateAction(row.id, { message: event.target.value })} placeholder="Rule matched" /></label>}
                  </div>
                  <button className="icon-button ghost danger" type="button" onClick={() => removeAction(row.id)} aria-label={`Remove action ${index + 1}`}><IconTrash size={15} stroke={1.75} /></button>
                </li>; })}</ul> : <p className="inline-note"><IconCircleDashed size={15} stroke={1.75} aria-hidden="true" />Add an outcome above. The same user receives every outcome when the event matches.</p>}
              </div>
            </section>
          </li>
        </ol>

        <details className="panel disclosure"><summary><span><strong>Schedule</strong><small>Optional. Set when this automation can run.</small></span><IconChevronDown size={16} stroke={1.75} aria-hidden="true" /></summary><div className="panel-body form-row"><label className="field"><span className="field-label">Starts at <em>Optional</em></span><input type="datetime-local" value={startsAt} onChange={(event) => setStartsAt(event.target.value)} /></label><label className="field"><span className="field-label">Ends at <em>Optional</em></span><input type="datetime-local" value={endsAt} onChange={(event) => setEndsAt(event.target.value)} /></label></div></details>

        {testOpen && <section className="panel" aria-labelledby="recipe-test-title">
          <div className="panel-header"><div><h2 id="recipe-test-title">Send a sample event</h2><p>Testing does not publish or change the draft.</p></div><button className="icon-button ghost" type="button" onClick={() => setTestOpen(false)} aria-label="Close sample test"><IconChevronDown size={16} stroke={1.75} /></button></div>
          <div className="panel-body form-stack">
            <div className="form-row"><label className="field"><span className="field-label">External user ID</span><input className="mono" value={testUserId} onChange={(event) => setTestUserId(event.target.value)} placeholder="user_123" spellCheck={false} /></label><label className="field"><span className="field-label">Event type</span><input className="mono" value={eventType} readOnly /></label></div>
            <label className="field"><span className="field-label">Sample payload</span><textarea className="code-input" value={testPayload} onChange={(event) => setTestPayload(event.target.value)} spellCheck={false} /><small className="field-help">Generated from the registered event contract.</small></label>
            {testError && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{testError}</span></p>}
            {testReceipt && <div className="receipt" role="status"><span className={`badge capitalize badge-${testReceipt.status === 'processed' ? 'success' : testReceipt.status === 'failed' ? 'danger' : 'warning'}`}><span className="dot" />{testReceipt.status}</span><span className="receipt-copy"><strong>Event {testReceipt.status}</strong><code>{testReceipt.external_event_id} · receipt #{testReceipt.event_id}</code></span><span className="receipt-note">{testReceipt.status === 'processed' ? 'Worker finished' : 'Pipeline receipt'}</span></div>}
            <div className="form-footer"><button className="primary-button" type="button" onClick={() => void submitTestEvent()} disabled={testSending || !eventIsRegistered}><IconPlayerPlay size={15} stroke={1.75} />{testSending ? 'Sending sample…' : 'Send sample event'}</button></div>
          </div>
        </section>}
      </div>

      <aside className="builder-aside" aria-label="Automation review">
        <section className="panel" aria-labelledby="recipe-summary-title">
          <div className="panel-header"><div><h2 id="recipe-summary-title">{name || 'Untitled automation'}</h2><p>{description || 'No description yet.'}</p></div><span className={`badge ${recipeReady ? 'badge-success' : 'badge-neutral'}`}>{recipeReady ? 'Ready to save' : 'Draft'}</span></div>
          <dl className="kv kv-compact"><div><dt>When</dt><dd className={eventType ? 'mono' : 'muted'}>{summaryEvent}</dd></div><div><dt>If</dt><dd>{conditions.length ? `${conditions.length} condition${conditions.length === 1 ? '' : 's'}` : 'Every valid payload'}</dd></div><div><dt>Then</dt><dd className={actions.length ? '' : 'muted'}>{summaryOutcomes}</dd></div></dl>
        </section>
        <section className="panel" aria-labelledby="recipe-readiness-title">
          <div className="panel-header"><div><h2 id="recipe-readiness-title">Readiness</h2><p>{recipeReady ? 'This draft is complete enough to review and publish later.' : 'Complete the items below before saving.'}</p></div><span className={`panel-meta ${recipeReady ? 'tone-success' : ''}`}>{readyCount}/{preflightItems.length}</span></div>
          <ul className="check-list">{preflightItems.map((item) => <li className={item.ready ? 'is-ready' : `is-${item.tone}`} key={item.label}>{item.ready ? <IconCircleCheck size={16} stroke={1.75} aria-hidden="true" /> : <IconAlertTriangle size={16} stroke={1.75} aria-hidden="true" />}<span><strong>{item.ready ? item.label.replace('Workspace connected · ', '').replace('Event contract ready · ', '') : item.label}</strong><small>{item.ready ? 'Ready' : 'Needs attention'}</small></span></li>)}</ul>
          <div className="builder-submit">
            {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
            {saved && <p className="alert alert-success" role="status"><IconCheck size={16} aria-hidden="true" /><span>{editingRule ? `New version ${editingRule.version + 1} saved as a draft. Previous behavior remains available in history.` : 'Automation draft saved. Publish it from the library when it is ready.'}</span></p>}
            <button className="primary-button full" type="button" onClick={() => void submit()} disabled={saving || !selectedScope}>{saving ? 'Saving draft…' : editingRule ? 'Save new version' : 'Save draft'}</button>
            <button className="secondary-button full" type="button" onClick={testOpen ? () => setTestOpen(false) : openTestPanel} disabled={!eventIsRegistered}><IconPlayerPlay size={14} stroke={1.75} />{testOpen ? 'Hide sample test' : 'Test sample event'}</button>
            {editingRule && <button className="text-button full-center" type="button" onClick={onCancelEdit}>Cancel edit</button>}
          </div>
        </section>
        <details className="panel disclosure"><summary><span><strong>Generated definition</strong><small>The JSON this draft will save.</small></span><IconChevronDown size={16} stroke={1.75} aria-hidden="true" /></summary><pre className="code-block">{JSON.stringify({ event_type: eventType || 'event.name', conditions: conditionPreview, actions: actionPreview }, null, 2)}</pre></details>
      </aside>
    </div>
    {createDrawer?.kind === 'event' && <EventTypeCreateDialog open activeScope={selectedScope} initialName={createDrawer.initialName} closeOnCreated onClose={() => setCreateDrawer(null)} onCreated={(created) => { onEventCreated(created); setEventType(created.name); }} />}
    {createDrawer?.kind === 'currency' && <CurrencyCreateDialog open activeScope={selectedScope} initialCode={createDrawer.initialCode} onClose={() => setCreateDrawer(null)} onCreated={(created) => { onCurrencyCreated(created); setActions((current) => current.map((row) => row.id === createDrawer.actionId ? { ...row, currency: created.code } : row)); }} />}
  </>;

}

function RegistryRail({ tab, eventTypes, rules, loading, canCreate, onCreate, onRefresh, onPublish, onEditEvent, onEditRule }: { tab: ConfigureTab; eventTypes: EventType[]; rules: Rule[]; loading: boolean; canCreate: boolean; onCreate: () => void; onRefresh: () => void; onPublish: (rule: Rule) => void; onEditEvent: (eventType: EventType) => void; onEditRule: (rule: Rule) => void }) {
  const isEvents = tab === 'events';
  const count = isEvents ? eventTypes.length : rules.length;
  const publishedCount = rules.filter((rule) => rule.status === 'published').length;
  return <section className="panel" aria-label={isEvents ? 'Event contracts' : 'Automations'}>
    <div className="toolbar">
      <p className="tally">{isEvents ? <><strong>{count}</strong> event contract{count === 1 ? '' : 's'}</> : <><strong>{count}</strong> total <span aria-hidden="true">·</span> <strong>{publishedCount}</strong> published</>}</p>
      <div className="toolbar-controls"><button className="icon-button" type="button" onClick={onRefresh} aria-label={isEvents ? 'Refresh event contracts' : 'Refresh automations'}><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button></div>
    </div>
    {loading ? <div className="skeleton-rows" aria-label="Loading"><span /><span /><span /></div> : count === 0 ? <div className="empty-state"><IconCircleDashed size={22} stroke={1.5} aria-hidden="true" /><strong>{isEvents ? 'No event contracts yet' : 'No automations yet'}</strong><span>{isEvents ? 'Your first event definition will appear here.' : 'Saved automations will wait here for review.'}</span><button className="primary-button" type="button" disabled={!canCreate} onClick={onCreate}><IconPlus size={15} stroke={1.75} />{isEvents ? 'Create your first event' : 'Create your first automation'}</button></div> : isEvents
      ? <ul className="resource-list">{eventTypes.map((eventType) => { const required = Array.isArray(eventType.schema.required) ? eventType.schema.required.length : 0; return <li className="resource-row" key={`${eventType.tenant_id}:${eventType.environment_id}:${eventType.name}`}><span className="resource-row-icon" aria-hidden="true"><IconBraces size={15} stroke={1.75} /></span><div className="resource-row-main"><strong className="mono">{eventType.name}</strong><span>Version {eventType.version} · {required} required field{required === 1 ? '' : 's'}</span></div><span className="badge badge-success"><span className="dot" />Active</span><div className="resource-row-actions"><button className="text-button" type="button" aria-label={`Edit event ${eventType.name}`} onClick={() => onEditEvent(eventType)}>Edit</button></div></li>; })}</ul>
      : <ul className="resource-list">{rules.map((rule) => <li className="resource-row" key={rule.id}><span className="resource-row-icon" aria-hidden="true"><IconSparkles size={15} stroke={1.75} /></span><div className="resource-row-main"><strong>{rule.name}</strong><span><span className="mono">{rule.event_type}</span> · Version {rule.version}</span></div><span className={`badge capitalize ${rule.status === 'published' ? 'badge-success' : rule.status === 'draft' ? 'badge-warning' : 'badge-neutral'}`}><span className="dot" />{rule.status}</span><div className="resource-row-actions">{rule.status === 'draft' && <button className="text-button accent" type="button" onClick={() => onPublish(rule)} aria-label={`Publish ${rule.name}`}>Publish</button>}<button className="text-button" type="button" aria-label={`Edit automation ${rule.name}`} onClick={() => onEditRule(rule)}>Edit</button></div></li>)}</ul>}
  </section>;
}

type ConfigurePageProps = {
  area: ConfigureTab;
  initialMode?: 'list' | 'create';
  onModeChange?: (mode: 'list' | 'create') => void;
};

export function ConfigurePage({ area, initialMode = 'list', onModeChange }: ConfigurePageProps) {
  const { scope } = useScope();
  const tab = area;
  const [mode, setMode] = useState<'list' | 'create'>(initialMode);
  const [editingEvent, setEditingEvent] = useState<EventType | null>(null);
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [eventTypes, setEventTypes] = useState<EventType[]>([]);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  function goToMode(nextMode: 'list' | 'create') {
    setMode(nextMode);
    onModeChange?.(nextMode);
  }

  useEffect(() => { setMode(initialMode); }, [initialMode]);
  useEffect(() => {
    setMode(initialMode);
    setEditingEvent(null);
    setEditingRule(null);
  }, [area, scope?.tenant_id, scope?.environment_id]);

  const refresh = useCallback(async () => {
    if (!scope) { setEventTypes([]); setCurrencies([]); setRules([]); setLoading(false); return; }
    setLoading(true); setError(null);
    try {
      // The active scope comes from the authenticated membership context. The builder
      // never asks the workspace owner to choose a tenant or environment.
      const scopedFilters = { tenant_id: scope.tenant_id, environment_id: scope.environment_id };
      const [nextEventTypes, nextCurrencies, nextRules] = await Promise.all([fetchEventTypes(scopedFilters), fetchCurrencies(scopedFilters), fetchRules(scopedFilters)]);
      setEventTypes(nextEventTypes); setCurrencies(nextCurrencies); setRules(nextRules);
    } catch (refreshError) { setError(refreshError instanceof Error ? refreshError.message : 'Configure registry could not be loaded.'); } finally { setLoading(false); }
  }, [scope]);
  useEffect(() => { void refresh(); }, [refresh]);

  async function handlePublish(rule: Rule) {
    try {
      if (!scope) throw new Error('Your workspace is not connected yet.');
      const published = await publishRule(rule.id, scope);
      setRules((current) => current.map((item) => item.id === published.id ? published : item));
    } catch (publishError) { setError(publishError instanceof Error ? publishError.message : 'Rule could not be published.'); }
  }

  const startCreate = () => { setEditingEvent(null); setEditingRule(null); goToMode('create'); };
  const listEmpty = tab === 'events' ? eventTypes.length === 0 : rules.length === 0;
  return <main className={`page ${mode === 'create' ? 'page-builder' : 'page-resource'}`}>
    <PageHeader breadcrumb={['Workspace', tab === 'rules' ? 'Automations' : 'Events', ...(mode === 'create' ? [editingEvent || editingRule ? 'Edit' : 'Create'] : [])]} title={mode === 'list' ? tab === 'rules' ? 'Automations' : 'Events' : tab === 'rules' ? editingRule ? `Edit ${editingRule.name}` : 'Create automation' : editingEvent ? `Edit ${editingEvent.name}` : 'Create event'} description={tab === 'rules' ? 'Decide what happens after a trusted event. Drafts only run after publishing.' : 'Define the event contracts your product sends to this workspace.'} actions={mode === 'list' ? <button className="primary-button" type="button" disabled={!scope} onClick={startCreate}><IconPlus size={15} stroke={1.75} />{tab === 'rules' ? 'Add automation' : 'Add event'}</button> : <button className="secondary-button" type="button" onClick={() => { goToMode('list'); setEditingEvent(null); setEditingRule(null); }}><IconArrowLeft size={15} stroke={1.75} />Back to list</button>} />
    {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Try again</button></p>}
    {mode === 'list' && <GuidePanel id={`guide-${tab}`} title={tab === 'rules' ? 'Automations' : 'Events'} {...configureGuide[tab]} defaultOpen={!loading && !error && listEmpty} />}
    {mode === 'list' ? <RegistryRail tab={tab} eventTypes={eventTypes} rules={rules} loading={loading} canCreate={Boolean(scope)} onCreate={startCreate} onRefresh={() => void refresh()} onPublish={(rule) => void handlePublish(rule)} onEditEvent={(eventType) => { setEditingEvent(eventType); setEditingRule(null); goToMode('create'); }} onEditRule={(rule) => { setEditingRule(rule); setEditingEvent(null); goToMode('create'); }} /> : tab === 'events' ? <EventBuilder activeScope={scope} editingEvent={editingEvent} onCancelEdit={() => { setEditingEvent(null); goToMode('list'); }} onCreated={(eventType) => { setEventTypes((current) => [eventType, ...current.filter((item) => item.name !== eventType.name || item.tenant_id !== eventType.tenant_id || item.environment_id !== eventType.environment_id)]); setEditingEvent(null); goToMode('list'); }} /> : <RuleBuilder activeScope={scope} eventTypes={eventTypes} currencies={currencies} editingRule={editingRule} onCancelEdit={() => { setEditingRule(null); goToMode('list'); }} onCreated={(rule) => { setRules((current) => [rule, ...current.filter((item) => item.id !== rule.id)]); setEditingRule(null); goToMode('list'); }} onEventCreated={(eventType) => setEventTypes((current) => [eventType, ...current.filter((item) => item.name !== eventType.name || item.tenant_id !== eventType.tenant_id || item.environment_id !== eventType.environment_id)])} onCurrencyCreated={(currency) => setCurrencies((current) => [currency, ...current.filter((item) => item.code !== currency.code || item.tenant_id !== currency.tenant_id || item.environment_id !== currency.environment_id)])} />}
  </main>;
}
