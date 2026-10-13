import { useCallback, useEffect, useState } from 'react';
import { IconActivity, IconBraces, IconCheck, IconClock, IconKey, IconRefresh, IconX } from '@tabler/icons-react';
import { createCredential, fetchAuditLog, fetchCredentials, fetchEventTypes, revokeCredential, rotateCredential, updateTenantStatus, type AuditEntry, type Credential, type EventType, type Tenant } from '../../../shared/api/client';

type TenantDetailDialogProps = {
  tenant: Tenant | null;
  canManagePlatform?: boolean;
  onClose: () => void;
  onUpdated?: (tenant: Tenant) => void;
};

export function TenantDetailDialog({ tenant, canManagePlatform = true, onClose, onUpdated }: TenantDetailDialogProps) {
  const [eventTypes, setEventTypes] = useState<EventType[]>([]);
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [credentialLabel, setCredentialLabel] = useState('dashboard-integration');
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [statusLoading, setStatusLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!tenant) return;
    setLoading(true);
    setError(null);
    try {
      const [contracts, audit, credentialList] = await Promise.all([
        fetchEventTypes({ tenant_id: tenant.tenant_id, environment_id: tenant.environment_id }),
        fetchAuditLog({ tenant_id: tenant.tenant_id, limit: 6 }),
        canManagePlatform ? fetchCredentials({ tenant_id: tenant.tenant_id, environment_id: tenant.environment_id }) : Promise.resolve([]),
      ]);
      setEventTypes(contracts);
      setAuditEntries(audit);
      setCredentials(credentialList);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Event types could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [canManagePlatform, tenant]);

  const changeStatus = async () => {
    if (!tenant) return;
    setStatusLoading(true);
    setError(null);
    try {
      const status = tenant.status === 'suspended' ? 'active' : 'suspended';
      await updateTenantStatus(tenant.tenant_id, status);
      onUpdated?.({ ...tenant, status });
    } catch (statusError) {
      setError(statusError instanceof Error ? statusError.message : 'Tenant status could not be changed.');
    } finally {
      setStatusLoading(false);
    }
  };

  const createNewCredential = async () => {
    if (!tenant || !credentialLabel.trim()) return;
    setStatusLoading(true);
    setError(null);
    try {
      const created = await createCredential({ tenant_id: tenant.tenant_id, environment_id: tenant.environment_id, label: credentialLabel.trim() });
      setCredentials((current) => [created, ...current]);
      setRevealedKey(created.integration_api_key ?? null);
      setCredentialLabel('dashboard-integration');
    } catch (credentialError) {
      setError(credentialError instanceof Error ? credentialError.message : 'Credential could not be created.');
    } finally {
      setStatusLoading(false);
    }
  };

  const rotateActiveCredential = async () => {
    const active = credentials.find((item) => item.active);
    if (!active) return createNewCredential();
    setStatusLoading(true);
    setError(null);
    try {
      const rotated = await rotateCredential(active.id);
      setCredentials((current) => current.map((item) => item.id === rotated.id ? rotated : item));
      setRevealedKey(rotated.integration_api_key ?? null);
    } catch (credentialError) {
      setError(credentialError instanceof Error ? credentialError.message : 'Credential could not be rotated.');
    } finally {
      setStatusLoading(false);
    }
  };

  const revokeActiveCredential = async () => {
    const active = credentials.find((item) => item.active);
    if (!active) return;
    if (!window.confirm(`Revoke ${active.label}? Integrations using this key will stop working.`)) return;
    setStatusLoading(true);
    setError(null);
    try {
      const revoked = await revokeCredential(active.id);
      setCredentials((current) => current.map((item) => item.id === revoked.id ? revoked : item));
      setRevealedKey(null);
    } catch (credentialError) {
      setError(credentialError instanceof Error ? credentialError.message : 'Credential could not be revoked.');
    } finally {
      setStatusLoading(false);
    }
  };

  useEffect(() => { if (tenant) void refresh(); }, [refresh, tenant]);

  if (!tenant) return null;

  return (
    <div className="dialog-layer" role="presentation">
      <button className="dialog-backdrop" type="button" onClick={onClose} aria-label="Close tenant details" />
      <section className="side-dialog tenant-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="tenant-detail-title">
        <div className="dialog-heading"><div><span className="section-label">Tenant workspace</span><h2 id="tenant-detail-title">{tenant.name}</h2></div><button className="icon-button" type="button" onClick={onClose} aria-label="Close dialog"><IconX size={17} /></button></div>
        <div className="tenant-detail-id"><span>{tenant.tenant_id}</span><span className={`status-dot ${tenant.status === 'suspended' ? 'suspended' : ''}`} />{tenant.status}</div>
        <div className="tenant-detail-facts"><div><span>Environment</span><strong>{tenant.environment_name}</strong><small>{tenant.environment_id}</small></div><div><span>Credential</span><strong><IconCheck size={14} />{credentials.filter((item) => item.active).length} active</strong><small>{credentials[0]?.label ?? 'No credential yet'}</small></div></div>
        {canManagePlatform ? <><div className="credential-controls"><div className="credential-control-row">{credentials.some((item) => item.active) ? <><button className="text-button accent" type="button" onClick={() => void rotateActiveCredential()} disabled={statusLoading}>{statusLoading ? 'Rotating…' : 'Rotate active key'}</button><button className="text-button danger" type="button" onClick={() => void revokeActiveCredential()} disabled={statusLoading}>{statusLoading ? 'Updating…' : 'Revoke active key'}</button></> : <><input aria-label="Credential label" value={credentialLabel} onChange={(event) => setCredentialLabel(event.target.value)} maxLength={80} /><button className="text-button accent" type="button" onClick={() => void createNewCredential()} disabled={statusLoading || !credentialLabel.trim()}>{statusLoading ? 'Creating…' : 'Create key'}</button></>}</div>{revealedKey && <div className="revealed-key"><span>Save this key now. It will not be shown again.</span><code>{revealedKey}</code></div>}</div><div className="tenant-status-actions"><span>{tenant.status === 'suspended' ? 'Integration routes are blocked.' : 'Integration routes are accepting traffic.'}</span><button className={`text-button ${tenant.status === 'suspended' ? 'accent' : ''}`} type="button" onClick={() => void changeStatus()} disabled={statusLoading}>{statusLoading ? 'Updating…' : tenant.status === 'suspended' ? 'Activate tenant' : 'Suspend tenant'}</button></div></> : <div className="tenant-detail-note"><IconKey size={15} /><span>Platform credentials and tenant status are managed by a Super Admin.</span></div>}
        <div className="detail-section-heading"><div><span className="section-label">Active contracts</span><h3>Event types</h3></div><button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh tenant event types"><IconRefresh size={15} className={loading ? 'spin' : ''} /></button></div>
        {error ? <div className="dialog-error" role="alert">{error}</div> : loading ? <div className="detail-skeleton" /> : eventTypes.length === 0 ? <div className="detail-empty"><IconBraces size={18} /><strong>No contracts yet</strong><span>Register an event type to start validating this environment.</span></div> : <ul className="detail-event-list">{eventTypes.map((eventType) => <li key={eventType.name}><span className="event-type-icon"><IconBraces size={15} /></span><span><strong>{eventType.name}</strong><small>{Array.isArray(eventType.schema.required) ? eventType.schema.required.length : 0} required fields</small></span><IconClock size={13} /></li>)}</ul>}
        <div className="detail-section-heading audit-heading"><div><span className="section-label">Operator trail</span><h3>Recent activity</h3></div><IconActivity size={16} /></div>
        {auditEntries.length === 0 ? <div className="detail-empty compact"><IconClock size={16} /><span>No audit activity yet.</span></div> : <ul className="audit-list">{auditEntries.map((entry) => <li key={entry.id}><span><strong>{entry.action}</strong><small>{entry.resource_type} · {entry.resource_id ?? '—'}</small></span><time>{new Date(entry.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></li>)}</ul>}
        <div className="tenant-detail-note"><IconKey size={15} /><span>Credentials are hashed at rest. Raw keys are shown only after create or rotate.</span></div>
      </section>
    </div>
  );
}
