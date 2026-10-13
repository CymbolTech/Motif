import { useEffect, useState, type FormEvent } from 'react';
import { IconBraces, IconCheck, IconX } from '@tabler/icons-react';
import { createRule, type Rule, type Tenant } from '../../../shared/api/client';

type RuleCreateDialogProps = {
  open: boolean;
  tenants: Tenant[];
  onClose: () => void;
  onCreated: (rule: Rule) => void;
};

const initialForm = {
  tenant_id: '',
  environment_id: '',
  name: '',
  description: '',
  event_type: '',
  conditions: '{\n  "all": []\n}',
  action: '{\n  "type": "log",\n  "message": "Rule matched"\n}',
  starts_at: '',
  ends_at: '',
};

export function RuleCreateDialog({ open, tenants, onClose, onCreated }: RuleCreateDialogProps) {
  const [form, setForm] = useState(initialForm);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  useEffect(() => {
    if (open) {
      const firstTenant = tenants[0];
      setForm({ ...initialForm, tenant_id: firstTenant?.tenant_id ?? '', environment_id: firstTenant?.environment_id ?? '' });
      setError(null);
      setCreated(false);
    }
  }, [open, tenants]);

  if (!open) return null;
  const selectedTenant = tenants.find((tenant) => tenant.tenant_id === form.tenant_id && tenant.environment_id === form.environment_id);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const conditions = JSON.parse(form.conditions) as Record<string, unknown>;
      const action = JSON.parse(form.action) as Record<string, unknown>;
      const startsAt = form.starts_at ? new Date(form.starts_at).toISOString() : undefined;
      const endsAt = form.ends_at ? new Date(form.ends_at).toISOString() : undefined;
      if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) {
        throw new Error('End time must be later than start time.');
      }
      const rule = await createRule({ tenant_id: form.tenant_id, environment_id: form.environment_id, name: form.name, description: form.description, event_type: form.event_type, conditions, action, starts_at: startsAt, ends_at: endsAt });
      setCreated(true);
      onCreated(rule);
    } catch (creationError) {
      setError(creationError instanceof SyntaxError ? 'Conditions and action must be valid JSON.' : creationError instanceof Error ? creationError.message : 'Rule could not be created.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="dialog-layer" role="presentation">
      <button className="dialog-backdrop" type="button" onClick={onClose} aria-label="Close create rule dialog" />
      <section className="side-dialog rule-dialog" role="dialog" aria-modal="true" aria-labelledby="create-rule-title">
        <div className="dialog-heading"><div><span className="section-label">Rule draft</span><h2 id="create-rule-title">Create a rule</h2></div><button className="icon-button" type="button" onClick={onClose} aria-label="Close dialog"><IconX size={17} /></button></div>
        {created ? <div className="key-success"><div className="key-success-icon"><IconCheck size={18} stroke={2} /></div><h3>Draft saved.</h3><p>Review the rule in the registry, then publish it when the event contract is ready.</p><button className="primary-button dialog-done" type="button" onClick={onClose}>Done</button></div> : (
          <form className="dialog-form" onSubmit={handleSubmit}>
            <p className="dialog-intro">A rule starts as a draft. The first executable action is recorded as an audit signal when a published rule matches an event.</p>
            <label><span>Tenant environment</span><select required value={`${form.tenant_id}::${form.environment_id}`} onChange={(event) => { const [tenantId, environmentId] = event.target.value.split('::'); setForm({ ...form, tenant_id: tenantId, environment_id: environmentId }); }}><option value="" disabled>Select an environment</option>{tenants.map((tenant) => <option key={`${tenant.tenant_id}-${tenant.environment_id}`} value={`${tenant.tenant_id}::${tenant.environment_id}`}>{tenant.name} · {tenant.environment_name}</option>)}</select></label>
            <label><span>Rule name</span><input required maxLength={120} placeholder="Welcome on signup" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></label>
            <label><span>Description</span><input maxLength={500} placeholder="Record a welcome signal" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label>
            <label><span>When this event arrives</span><input required pattern="[a-z0-9][a-z0-9._-]{0,127}" placeholder="user.signed_up" value={form.event_type} onChange={(event) => setForm({ ...form, event_type: event.target.value })} /></label>
            <div className="form-grid schedule-fields">
              <label><span>Starts at <em>optional</em></span><input type="datetime-local" value={form.starts_at} onChange={(event) => setForm({ ...form, starts_at: event.target.value })} /></label>
              <label><span>Ends at <em>optional</em></span><input type="datetime-local" value={form.ends_at} onChange={(event) => setForm({ ...form, ends_at: event.target.value })} /></label>
            </div>
            <div className="form-grid"><label><span>Conditions</span><div className="code-field"><IconBraces size={15} stroke={1.5} /><textarea required value={form.conditions} onChange={(event) => setForm({ ...form, conditions: event.target.value })} spellCheck={false} /></div></label><label><span>Action</span><div className="code-field"><IconBraces size={15} stroke={1.5} /><textarea required value={form.action} onChange={(event) => setForm({ ...form, action: event.target.value })} spellCheck={false} /></div></label></div>
            {selectedTenant && <div className="dialog-context">Draft for <strong>{selectedTenant.name}</strong> / {selectedTenant.environment_name}</div>}
            {error && <div className="dialog-error" role="alert">{error}</div>}
            <button className="primary-button dialog-submit" type="submit" disabled={saving || tenants.length === 0}>{saving ? 'Saving…' : 'Save draft'}</button>
          </form>
        )}
      </section>
    </div>
  );
}
