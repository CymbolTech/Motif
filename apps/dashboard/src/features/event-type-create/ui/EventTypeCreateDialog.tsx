import { useEffect, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconCircleCheck } from '@tabler/icons-react';
import { createEventType, type EventType, type Tenant } from '../../../shared/api/client';
import { Sheet } from '../../../shared/ui/Sheet';
import { PayloadShapeBuilder, createPayloadField, payloadFieldNamePattern, schemaFromPayloadFields, type PayloadField } from '../../../shared/ui/PayloadShapeBuilder';

type EventTypeCreateDialogProps = {
  open: boolean;
  activeScope?: Tenant;
  initialName?: string;
  closeOnCreated?: boolean;
  onClose: () => void;
  onCreated: (eventType: EventType) => void;
};

const initialForm = { name: '' };

export function EventTypeCreateDialog({ open, activeScope, initialName = '', closeOnCreated = false, onClose, onCreated }: EventTypeCreateDialogProps) {
  const [form, setForm] = useState(initialForm);
  const [fields, setFields] = useState<PayloadField[]>([createPayloadField(true)]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  useEffect(() => {
    if (open) {
      setForm({ ...initialForm, name: initialName });
      setFields([createPayloadField(true)]);
      setError(null);
      setCreated(false);
    }
  }, [open, activeScope, initialName]);

  if (!open) return null;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSaving(true);
    try {
      if (!activeScope) throw new Error('Your workspace membership is not ready yet.');
      const names = fields.filter((field) => field.name.trim()).map((field) => field.name.trim());
      if (names.length === 0) throw new Error('Add at least one field so Motif knows what to expect.');
      if (names.some((fieldName) => !payloadFieldNamePattern.test(fieldName))) throw new Error('Field names must start with a letter or underscore.');
      if (new Set(names).size !== names.length) throw new Error('Each field name must be unique.');
      const schema = schemaFromPayloadFields(fields);
      const eventType = await createEventType({ tenant_id: activeScope.tenant_id, environment_id: activeScope.environment_id, name: form.name, schema });
      onCreated(eventType);
      if (closeOnCreated) onClose();
      else setCreated(true);
    } catch (creationError) {
      setError(creationError instanceof Error ? creationError.message : 'Event type could not be created.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Sheet size="wide" labelledBy="create-event-type-title" title="Register an event type" description="Give the event a stable name, then choose the information your app sends with it. No JSON needed." onClose={onClose} footer={created ? undefined : <>
      {activeScope && <span className="form-footer-note">Registering in <strong>{activeScope.name}</strong> · {activeScope.environment_name}</span>}
      <button className="secondary-button" type="button" onClick={onClose}>Cancel</button>
      <button className="primary-button" type="submit" form="create-event-type-form" disabled={saving || !activeScope}>{saving ? 'Registering…' : 'Register event type'}</button>
    </>}>
      {created ? <div className="empty-state"><IconCircleCheck size={24} stroke={1.5} className="tone-success" aria-hidden="true" /><strong>Event type registered</strong><span>The contract is active in your workspace and can now validate incoming events.</span><button className="primary-button" type="button" onClick={onClose}>Done</button></div> : (
        <form id="create-event-type-form" className="form-stack" onSubmit={handleSubmit}>
          <label className="field"><span className="field-label">Event type name</span><input className="mono" required pattern="[a-z0-9][a-z0-9._-]{0,127}" placeholder="order.completed" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} spellCheck={false} /><small className="field-help">Use lowercase namespaces such as <code>order.completed</code>.</small></label>
          <div className="field"><span className="field-label">Event information</span><PayloadShapeBuilder fields={fields} onChange={setFields} /></div>
          {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        </form>
      )}
    </Sheet>
  );
}
