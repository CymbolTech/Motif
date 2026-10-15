import { createContext, useCallback, useContext, useEffect, useState, type PropsWithChildren } from 'react';
import { fetchTenants, type Tenant } from '../api/client';

export type ActiveScope = Tenant;

type ScopeContextValue = {
  scope?: ActiveScope;
  loading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
};

const ScopeContext = createContext<ScopeContextValue | null>(null);

export function ScopeProvider({ children }: PropsWithChildren) {
  const [scope, setScope] = useState<ActiveScope>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await fetchTenants();
      // The tenant console is membership-scoped. A workspace account gets its
      // first authorized scope automatically; there is no cross-account
      // selector or persisted scope that can leak a previous workspace.
      setScope(next[0]);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Workspaces could not be loaded. Refresh to try again.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  return <ScopeContext.Provider value={{ scope, loading, error, refresh }}>{children}</ScopeContext.Provider>;
}

export function useScope() {
  const context = useContext(ScopeContext);
  if (!context) throw new Error('useScope must be used inside ScopeProvider');
  return context;
}
