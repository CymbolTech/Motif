import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { IconCheck, IconPlus, IconSettings, IconX } from '@tabler/icons-react';
import { createAchievement, createCurrency, createLevel, createMission, createReward, fetchAchievements, fetchCurrencies, fetchLevels, fetchMissions, fetchRewards, type Achievement, type Currency, type Level, type Mission, type Reward, type Tenant } from '../../../shared/api/client';

type CatalogScope = Pick<Tenant, 'tenant_id' | 'environment_id'> & Partial<Pick<Tenant, 'name' | 'environment_name'>>;
type CatalogAdminPanelProps = { tenant?: CatalogScope; onChanged?: () => void };

type CatalogKind = 'currency' | 'mission' | 'achievement' | 'level' | 'reward';

const initialForm = { name: '', code: '', description: '', event_type: '', count: '1', threshold: '0', unit_scale: '1', cost_currency: '', cost_amount: '0', payload: '{}', policy: '{}' };

export function CatalogAdminPanel({ tenant, onChanged }: CatalogAdminPanelProps) {
  const [kind, setKind] = useState<CatalogKind>('currency');
  const [form, setForm] = useState(initialForm);
  const [counts, setCounts] = useState({ currencies: 0, missions: 0, achievements: 0, levels: 0 });
  const [items, setItems] = useState<{ currencies: Currency[]; missions: Mission[]; achievements: Achievement[]; levels: Level[]; rewards: Reward[] }>({ currencies: [], missions: [], achievements: [], levels: [], rewards: [] });
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshCounts = useCallback(async () => {
    if (!tenant) return;
    try {
      const [currencies, missions, achievements, levels, rewards] = await Promise.all([
        fetchCurrencies(tenant),
        fetchMissions(tenant),
        fetchAchievements(tenant),
        fetchLevels(tenant),
        fetchRewards(tenant),
      ]);
      setCounts({ currencies: currencies.length, missions: missions.length, achievements: achievements.length, levels: levels.length });
      setItems({ currencies, missions, achievements, levels, rewards });
    } catch {
      // The main workspace already surfaces API errors; counts remain last-known.
    }
  }, [tenant]);

  useEffect(() => { void refreshCounts(); }, [refreshCounts]);

  function update(key: keyof typeof initialForm, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
    setSaved(false);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!tenant) return;
    setSaving(true);
    setError(null);
    try {
      if (kind === 'currency') {
        await createCurrency({ ...tenant, code: form.code, name: form.name, unit_scale: Number(form.unit_scale), policy: JSON.parse(form.policy) as Record<string, unknown> });
      } else if (kind === 'mission') {
        await createMission({ ...tenant, name: form.name, event_type: form.event_type, target_count: Number(form.count) });
      } else if (kind === 'achievement') {
        await createAchievement({ ...tenant, name: form.name, description: form.description, event_type: form.event_type, required_count: Number(form.count) });
      } else if (kind === 'level') {
        await createLevel({ ...tenant, level: Number(form.count), name: form.name, xp_threshold: Number(form.threshold) });
      } else {
        await createReward({ ...tenant, name: form.name, reward_type: 'digital_unlock', description: form.description, cost_currency: form.cost_currency || undefined, cost_amount: form.cost_currency ? Number(form.cost_amount) : undefined, payload: JSON.parse(form.payload) as Record<string, unknown> });
      }
      setSaved(true);
      setForm(initialForm);
      await refreshCounts();
      onChanged?.();
    } catch (saveError) {
      setError(saveError instanceof SyntaxError ? 'Policy and payload must be valid JSON.' : saveError instanceof Error ? saveError.message : 'Configuration could not be saved.');
    } finally {
      setSaving(false);
    }
  }

  if (!tenant) return null;

  return (
    <section className="panel catalog-admin-panel" aria-labelledby="catalog-admin-title">
      <div className="panel-heading"><div><span className="section-label">Admin registry</span><h2 id="catalog-admin-title">Shape the catalog</h2></div><button className="icon-button" type="button" onClick={() => setOpen((current) => !current)} aria-expanded={open} aria-label={open ? 'Close catalog form' : 'Open catalog form'}>{open ? <IconX size={16} /> : <IconSettings size={17} />}</button></div>
      <div className="catalog-counts"><span><strong>{counts.currencies}</strong> currencies</span><span><strong>{counts.missions}</strong> missions</span><span><strong>{counts.achievements}</strong> achievements</span><span><strong>{counts.levels}</strong> levels</span></div>
      <div className="catalog-preview" aria-label="Current catalog entries">{[...items.currencies.map((item) => ({ kind: 'currency', name: item.name, detail: item.code })), ...items.missions.map((item) => ({ kind: 'mission', name: item.name, detail: `${item.target_count} target` })), ...items.achievements.map((item) => ({ kind: 'achievement', name: item.name, detail: `${item.required_count} required` })), ...items.levels.map((item) => ({ kind: 'level', name: `Level ${item.level} · ${item.name}`, detail: `${item.xp_threshold} XP` })), ...items.rewards.map((item) => ({ kind: 'reward', name: item.name, detail: item.cost_amount ? `${item.cost_amount} ${item.cost_currency}` : 'free' }))].slice(0, 8).map((item) => <span key={`${item.kind}-${item.name}`}><small>{item.kind}</small><strong>{item.name}</strong><em>{item.detail}</em></span>)}{items.currencies.length + items.missions.length + items.achievements.length + items.levels.length + items.rewards.length === 0 && <span className="catalog-preview-empty">No catalog entries in this scope yet.</span>}</div>
      {!open ? <div className="catalog-collapsed"><span>Configure tenant-scoped currencies, progression, missions, achievements, and rewards from one place.</span><button className="text-button accent" type="button" onClick={() => setOpen(true)}><IconPlus size={14} />Add configuration</button></div> : <form className="catalog-form" onSubmit={submit}><div className="catalog-kind-tabs" role="tablist" aria-label="Configuration type">{(['currency', 'mission', 'achievement', 'level', 'reward'] as CatalogKind[]).map((value) => <button className={kind === value ? 'active' : ''} type="button" role="tab" aria-selected={kind === value} key={value} onClick={() => { setKind(value); setError(null); }}>{value}</button>)}</div><p className="catalog-scope">Writing to <strong>{tenant.tenant_id}</strong> / {tenant.environment_id}</p>{kind === 'currency' && <><label><span>Currency code</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={form.code} onChange={(event) => update('code', event.target.value)} placeholder="loyalty" /></label><label><span>Display name</span><input required value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Loyalty points" /></label><label><span>Unit scale</span><input required type="number" min="1" value={form.unit_scale} onChange={(event) => update('unit_scale', event.target.value)} /></label><label><span>Policy JSON</span><textarea required value={form.policy} onChange={(event) => update('policy', event.target.value)} spellCheck={false} /></label></>}{kind === 'mission' && <><label><span>Mission name</span><input required value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Complete lessons" /></label><label><span>Event type</span><input required value={form.event_type} onChange={(event) => update('event_type', event.target.value)} placeholder="lesson.completed" /></label><label><span>Target count</span><input required type="number" min="1" value={form.count} onChange={(event) => update('count', event.target.value)} /></label></>}{kind === 'achievement' && <><label><span>Achievement name</span><input required value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Lesson regular" /></label><label><span>Description</span><input value={form.description} onChange={(event) => update('description', event.target.value)} placeholder="Complete three lessons" /></label><label><span>Event type</span><input required value={form.event_type} onChange={(event) => update('event_type', event.target.value)} placeholder="lesson.completed" /></label><label><span>Required count</span><input required type="number" min="1" value={form.count} onChange={(event) => update('count', event.target.value)} /></label></>}{kind === 'level' && <><label><span>Level number</span><input required type="number" min="1" value={form.count} onChange={(event) => update('count', event.target.value)} /></label><label><span>Level name</span><input required value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Rising signal" /></label><label><span>XP threshold</span><input required type="number" min="0" value={form.threshold} onChange={(event) => update('threshold', event.target.value)} /></label></>}{kind === 'reward' && <><label><span>Reward name</span><input required value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Starter unlock" /></label><label><span>Description</span><input value={form.description} onChange={(event) => update('description', event.target.value)} /></label><div className="form-grid"><label><span>Cost currency <em>optional</em></span><input value={form.cost_currency} onChange={(event) => update('cost_currency', event.target.value)} placeholder="loyalty" /></label><label><span>Cost amount <em>optional</em></span><input type="number" min="1" value={form.cost_amount} onChange={(event) => update('cost_amount', event.target.value)} /></label></div><label><span>Payload JSON</span><textarea required value={form.payload} onChange={(event) => update('payload', event.target.value)} spellCheck={false} /></label></>}{error && <div className="dialog-error" role="alert">{error}</div>}<div className="catalog-actions"><button className="text-button" type="button" onClick={() => setOpen(false)}>Cancel</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Saving…' : saved ? <><IconCheck size={14} />Saved</> : 'Create configuration'}</button></div></form>}
    </section>
  );
}

