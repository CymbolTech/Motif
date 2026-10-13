import { IconArrowUpRight, IconBuildingSkyscraper } from '@tabler/icons-react';
import type { Tenant } from '../model/types';

type TenantRowProps = {
  tenant: Tenant;
  onOpen: (tenant: Tenant) => void;
};

export function TenantRow({ tenant, onOpen }: TenantRowProps) {
  return (
    <li className="tenant-row">
      <div className="tenant-glyph" aria-hidden="true">
        <IconBuildingSkyscraper size={18} stroke={1.5} />
      </div>
      <div className="tenant-copy">
        <strong>{tenant.name}</strong>
        <span>{tenant.tenant_id}</span>
      </div>
      <div className="tenant-environment">
        <span className={`status-dot ${tenant.status === 'suspended' ? 'suspended' : ''}`} aria-hidden="true" />
        <span>{tenant.environment_name}</span>
      </div>
      <span className={`tenant-status ${tenant.status === 'suspended' ? 'suspended' : ''}`}>{tenant.status}</span>
      <button className="icon-button subtle" type="button" aria-label={`Open ${tenant.name}`} onClick={() => onOpen(tenant)}>
        <IconArrowUpRight size={17} stroke={1.6} />
      </button>
    </li>
  );
}
