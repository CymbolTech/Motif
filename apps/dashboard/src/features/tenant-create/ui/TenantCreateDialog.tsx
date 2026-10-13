import { useEffect, useState, type FormEvent } from 'react';
import { IconCheck, IconCopy, IconKey, IconX } from '@tabler/icons-react';
import { createTenant, type Tenant, type TenantCreation } from '../../../shared/api/client';

type TenantCreateDialogProps = {
  open: boolean;
  onClose: () => void;
  onCreated: (tenant: Tenant) => void;
};

const initialForm = {
  tenant_id: '',
  name: '',
  environment_id: 'sandbox',
  environment_name: 'Sandbox',
};

export function TenantCreateDialog({ open, onClose, onCreated }: TenantCreateDialogProps) {
  const [form, setForm] = useState(initialForm);
  const [created, setCreated] = useState<TenantCreation | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) {
      setForm(initialForm);
      setCreated(null);
      setError(null);
      setCopied(false);
    }
  }, [open]);

  if (!open) return null;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const tenant = await createTenant(form);
      setCreated(tenant);
      onCreated(tenant);
    } catch (creationError) {
      setError(creationError instanceof Error ? creationError.message : 'Tenant could not be created.');
    } finally {
      setSaving(false);
    }
  }

  async function copyKey() {
    if (!created?.integration_api_key) return;
    await navigator.clipboard?.writeText(created.integration_api_key);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  return (
    <div className="dialog-layer" role="presentation">
      <button className="dialog-backdrop" type="button" onClick={onClose} aria-label="Close create tenant dialog" />
      <section className="side-dialog" role="dialog" aria-modal="true" aria-labelledby="create-tenant-title">
        <div className="dialog-heading">
          <div>
            <span className="section-label">Platform setup</span>
            <h2 id="create-tenant-title">Create a tenant</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close dialog"><IconX size={17} /></button>
        </div>
        {created ? (
          <div className="key-success">
            <div className="key-success-icon"><IconCheck size={18} stroke={2} /></div>
            <h3>{created.name} is ready.</h3>
            <p>The integration key is shown once. Copy it now and store it in the tenant's server environment.</p>
            <div className="secret-key"><IconKey size={15} /><code>{created.integration_api_key}</code><button className="icon-button" type="button" onClick={copyKey} aria-label="Copy integration key">{copied ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div>
            <button className="primary-button dialog-done" type="button" onClick={onClose}>Done</button>
          </div>
        ) : (
          <form className="dialog-form" onSubmit={handleSubmit}>
            <p className="dialog-intro">A tenant starts with one environment and one integration credential. You can add event types after it is created.</p>
            <label><span>Tenant ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" placeholder="acme" value={form.tenant_id} onChange={(event) => setForm({ ...form, tenant_id: event.target.value })} /><small>Lowercase letters, numbers, hyphens, and underscores.</small></label>
            <label><span>Display name</span><input required maxLength={120} placeholder="Acme Corp" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></label>
            <div className="form-grid"><label><span>Environment ID</span><input required pattern="[a-z0-9][a-z0-9_-]{0,63}" value={form.environment_id} onChange={(event) => setForm({ ...form, environment_id: event.target.value })} /></label><label><span>Environment name</span><input required value={form.environment_name} onChange={(event) => setForm({ ...form, environment_name: event.target.value })} /></label></div>
            {error && <div className="dialog-error" role="alert">{error}</div>}
            <button className="primary-button dialog-submit" type="submit" disabled={saving}>{saving ? 'Creating…' : 'Create tenant'}</button>
          </form>
        )}
      </section>
    </div>
  );
}
