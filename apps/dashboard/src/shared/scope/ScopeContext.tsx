import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react';
import { fetchTenants, type Tenant } from '../api/client';

export type ActiveScope = Tenant;

type ScopeContextValue = {
  tenants: Tenant[];
  scope?: ActiveScope;
  loading: boolean;
  error: string | null;
  selectScope: (key: string) => void;
  refresh: () => Promise<void>;
};

const ScopeContext = createContext<ScopeContextValue | null>(null);
const storageKey = 'motif.active-scope';

function scopeKey(scope: Pick<Tenant, 'tenant_id' | 'environment_id'>) {
  return `${scope.tenant_id}::${scope.environment_id}`;
}

export function ScopeProvider({ children }: PropsWithChildren) {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedKey, setSelectedKey] = useState(() => window.localStorage.getItem(storageKey) ?? '');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await fetchTenants();
      setTenants(next);
      setSelectedKey((current) => {
        if (current && next.some((tenant) => scopeKey(tenant) === current)) return current;
        return next[0] ? scopeKey(next[0]) : '';
      });
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Tenant scopes could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    if (selectedKey) window.localStorage.setItem(storageKey, selectedKey);
  }, [selectedKey]);

  const scope = useMemo(() => tenants.find((tenant) => scopeKey(tenant) === selectedKey), [selectedKey, tenants]);
  const selectScope = useCallback((key: string) => setSelectedKey(key), []);

  return <ScopeContext.Provider value={{ tenants, scope, loading, error, selectScope, refresh }}>{children}</ScopeContext.Provider>;
}

export function useScope() {
  const context = useContext(ScopeContext);
  if (!context) throw new Error('useScope must be used inside ScopeProvider');
  return context;
}

export function scopeKeyFor(scope: Pick<Tenant, 'tenant_id' | 'environment_id'>) {
  return scopeKey(scope);
}
