import { useEffect, useState, type FormEvent } from 'react';
import { createCurrency, type Currency, type Tenant } from '../../../shared/api/client';
import { IconAlertTriangle } from '@tabler/icons-react';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { Sheet } from '../../../shared/ui/Sheet';

type CurrencyCreateDialogProps = {
  open: boolean;
  activeScope?: Tenant;
  initialCode?: string;
  onClose: () => void;
  onCreated: (currency: Currency) => void;
};

const initialForm = { name: '', code: '', unit_scale: '0' };

export function CurrencyCreateDialog({ open, activeScope, initialCode = '', onClose, onCreated }: CurrencyCreateDialogProps) {
  const [form, setForm] = useState({ ...initialForm, code: initialCode });
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm({ ...initialForm, code: initialCode });
    setError(null);
  }, [open, initialCode]);

  if (!open) return null;

  function update(key: keyof typeof initialForm, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
    setError(null);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeScope) {
      setError('Your workspace membership is still loading.');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const currency = await createCurrency({
        tenant_id: activeScope.tenant_id,
        environment_id: activeScope.environment_id,
        code: form.code.trim().toLowerCase(),
        name: form.name.trim(),
        unit_scale: Number(form.unit_scale),
        policy: {},
      });
      onCreated(currency);
      onClose();
    } catch (creationError) {
      setError(creationError instanceof Error ? creationError.message : 'Currency could not be created.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Sheet labelledBy="create-currency-title" title="Create currency" description="Add the balance this automation should award. It is selected in your draft as soon as it is created." onClose={onClose} footer={<>
      {activeScope && <span className="form-footer-note">Creating in <strong>{activeScope.name}</strong> · {activeScope.environment_name}</span>}
      <button className="secondary-button" type="button" onClick={onClose}>Cancel</button>
      <button className="primary-button" type="submit" form="create-currency-form" disabled={saving || !activeScope}>{saving ? 'Creating…' : 'Create and add currency'}</button>
    </>}>
      <form id="create-currency-form" className="form-stack" onSubmit={(event) => void submit(event)}>
        <label className="field"><span className="field-label">Currency name</span><input required maxLength={120} value={form.name} onChange={(event) => update('name', event.target.value)} placeholder="Coins" /><small className="field-help">A friendly name your team and customers will recognize.</small></label>
        <label className="field"><span className="field-label">Currency code</span><input className="mono" required pattern="[a-z0-9][a-z0-9_-]{0,31}" maxLength={32} value={form.code} onChange={(event) => update('code', event.target.value.toLowerCase())} placeholder="coins" /><small className="field-help">Use lowercase letters, numbers, underscores, or hyphens.</small></label>
        <div className="field"><span className="field-label">Precision</span><CustomSelect value={form.unit_scale} onChange={(value) => update('unit_scale', value)} ariaLabel="Precision" options={[{ value: '0', label: 'Whole units (1, 2, 3)' }, { value: '1', label: 'One decimal place (1.0)' }, { value: '2', label: 'Two decimal places (1.00)' }]} /><small className="field-help">Most loyalty currencies use whole units.</small></div>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
      </form>
    </Sheet>
  );
}
