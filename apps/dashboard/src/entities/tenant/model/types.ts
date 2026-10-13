export type Tenant = {
  tenant_id: string;
  name: string;
  environment_id: string;
  environment_name: string;
  status: 'active' | 'suspended' | string;
};
