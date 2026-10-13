import { IconPlus, IconRefresh, IconUsers } from '@tabler/icons-react';
import type { Tenant } from '../../../shared/api/client';
import { TenantRow } from '../../../entities/tenant/ui/TenantRow';

type TenantListProps = {
  tenants: Tenant[];
  loading: boolean;
  error: string | null;
  onRefresh: () => void;
  onCreate: () => void;
  onOpen: (tenant: Tenant) => void;
  canCreate: boolean;
};

export function TenantList({ tenants, loading, error, onRefresh, onCreate, onOpen, canCreate }: TenantListProps) {
  return (
    <section className="panel tenants-panel" aria-labelledby="tenant-title">
      <div className="panel-heading">
        <div>
          <span className="section-label">Workspace inventory</span>
          <h2 id="tenant-title">Tenants</h2>
        </div>
        <div className="panel-heading-actions"><button className="text-button" type="button" onClick={onRefresh} disabled={loading}><IconRefresh size={15} className={loading ? 'spin' : ''} stroke={1.7} />Refresh</button>{canCreate && <button className="text-button accent" type="button" onClick={onCreate}><IconPlus size={15} stroke={1.8} />New tenant</button>}</div>
      </div>
      {error ? (
        <div className="inline-state error-state" role="alert">
          <strong>Tenant list unavailable</strong>
          <span>{error}. Check the admin key and API readiness.</span>
        </div>
      ) : loading ? (
        <div className="tenant-skeletons" aria-label="Loading tenants">
          {[1, 2].map((item) => <div className="tenant-skeleton" key={item} />)}
        </div>
      ) : tenants.length === 0 ? (
        <div className="inline-state empty-state">
          <IconUsers size={18} stroke={1.5} />
          <strong>{canCreate ? 'No tenants yet' : 'No workspace memberships yet'}</strong>
          <span>{canCreate ? 'Create your first tenant through the admin API to see it here.' : 'Ask a Super Admin to add you to a tenant workspace.'}</span>
        </div>
      ) : (
        <ul className="tenant-list">
          {tenants.map((tenant) => <TenantRow key={`${tenant.tenant_id}-${tenant.environment_id}`} tenant={tenant} onOpen={onOpen} />)}
        </ul>
      )}
    </section>
  );
}
