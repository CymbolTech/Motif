import { useEffect, useState, type FormEvent } from 'react';
import { IconBraces, IconCheck, IconX } from '@tabler/icons-react';
import { createEventType, type EventType, type Tenant } from '../../../shared/api/client';

type EventTypeCreateDialogProps = {
  open: boolean;
  tenants: Tenant[];
  activeScope?: Tenant;
  onClose: () => void;
  onCreated: (eventType: EventType) => void;
};

const initialForm = {
  tenant_id: '',
  environment_id: '',
  name: '',
  schema: '{\n  "required": []\n}',
};

export function EventTypeCreateDialog({ open, tenants, activeScope, onClose, onCreated }: EventTypeCreateDialogProps) {
  const [form, setForm] = useState(initialForm);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  useEffect(() => {
    if (open) {
      const selectedTenant = tenants.find((tenant) => tenant.tenant_id === activeScope?.tenant_id && tenant.environment_id === activeScope?.environment_id) ?? tenants[0];
      setForm({ ...initialForm, tenant_id: selectedTenant?.tenant_id ?? '', environment_id: selectedTenant?.environment_id ?? '' });
      setError(null);
      setCreated(false);
    }
  }, [open, tenants, activeScope]);

  if (!open) return null;

  const selectedTenant = tenants.find((tenant) => tenant.tenant_id === form.tenant_id && tenant.environment_id === form.environment_id);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const schema = JSON.parse(form.schema) as Record<string, unknown>;
      const eventType = await createEventType({ tenant_id: form.tenant_id, environment_id: form.environment_id, name: form.name, schema });
      setCreated(true);
      onCreated(eventType);
    } catch (creationError) {
      setError(creationError instanceof SyntaxError ? 'Schema must be valid JSON.' : creationError instanceof Error ? creationError.message : 'Event type could not be created.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="dialog-layer" role="presentation">
      <button className="dialog-backdrop" type="button" onClick={onClose} aria-label="Close create event type dialog" />
      <section className="side-dialog" role="dialog" aria-modal="true" aria-labelledby="create-event-type-title">
        <div className="dialog-heading"><div><span className="section-label">Event contract</span><h2 id="create-event-type-title">Register an event type</h2></div><button className="icon-button" type="button" onClick={onClose} aria-label="Close dialog"><IconX size={17} /></button></div>
        {created ? <div className="key-success"><div className="key-success-icon"><IconCheck size={18} stroke={2} /></div><h3>Event type registered.</h3><p>The contract is active for the selected tenant environment and can now validate incoming events.</p><button className="primary-button dialog-done" type="button" onClick={onClose}>Done</button></div> : (
          <form className="dialog-form" onSubmit={handleSubmit}>
            <p className="dialog-intro">Keep the event name stable and describe the fields your payload must include. Schema validation currently supports the required-field list.</p>
            <label><span>Tenant environment</span><select required value={`${form.tenant_id}::${form.environment_id}`} onChange={(event) => { const [tenantId, environmentId] = event.target.value.split('::'); setForm({ ...form, tenant_id: tenantId, environment_id: environmentId }); }}><option value="" disabled>Select an environment</option>{tenants.map((tenant) => <option key={`${tenant.tenant_id}-${tenant.environment_id}`} value={`${tenant.tenant_id}::${tenant.environment_id}`}>{tenant.name} · {tenant.environment_name}</option>)}</select></label>
            <label><span>Event type name</span><input required pattern="[a-z0-9][a-z0-9._-]{0,127}" placeholder="order.completed" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /><small>Use lowercase namespaces such as <code>order.completed</code>.</small></label>
            <label><span>Schema</span><div className="code-field"><IconBraces size={15} stroke={1.5} /><textarea required value={form.schema} onChange={(event) => setForm({ ...form, schema: event.target.value })} spellCheck={false} /></div></label>
            {selectedTenant && <div className="dialog-context">Registering in <strong>{selectedTenant.name}</strong> / {selectedTenant.environment_name}</div>}
            {error && <div className="dialog-error" role="alert">{error}</div>}
            <button className="primary-button dialog-submit" type="submit" disabled={saving || tenants.length === 0}>{saving ? 'Registering…' : 'Register event type'}</button>
          </form>
        )}
      </section>
    </div>
  );
}
