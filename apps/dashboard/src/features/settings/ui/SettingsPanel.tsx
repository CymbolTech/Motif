import { useEffect, useState } from 'react';
import { IconCheck, IconCloud, IconKey, IconRefresh, IconServer, IconX } from '@tabler/icons-react';
import { fetchHealth, hasConfiguredKeys, loginAdmin, type AdminPrincipal } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

type SettingsPanelProps = { open: boolean; onClose: () => void; principal: AdminPrincipal | null; onAuthenticated: (principal: AdminPrincipal) => void };
type ServiceState = 'checking' | 'ready' | 'degraded';

export function SettingsPanel({ open, onClose, principal, onAuthenticated }: SettingsPanelProps) {
  const { scope, refresh: refreshScope } = useScope();
  const [apiState, setApiState] = useState<ServiceState>('checking');
  const [workerState, setWorkerState] = useState<ServiceState>('checking');
  const [refreshing, setRefreshing] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loginError, setLoginError] = useState<string | null>(null);
  const [loggingIn, setLoggingIn] = useState(false);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void Promise.allSettled([fetchHealth('api'), fetchHealth('worker')]).then(([api, worker]) => {
      if (cancelled) return;
      setApiState(api.status === 'fulfilled' ? 'ready' : 'degraded');
      setWorkerState(worker.status === 'fulfilled' ? 'ready' : 'degraded');
    });
    return () => { cancelled = true; };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose, open]);

  async function refresh() {
    setRefreshing(true);
    await refreshScope();
    const [api, worker] = await Promise.allSettled([fetchHealth('api'), fetchHealth('worker')]);
    setApiState(api.status === 'fulfilled' ? 'ready' : 'degraded');
    setWorkerState(worker.status === 'fulfilled' ? 'ready' : 'degraded');
    setRefreshing(false);
  }

  async function signIn(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoggingIn(true);
    setLoginError(null);
    try {
      const nextPrincipal = await loginAdmin(email, password);
      onAuthenticated(nextPrincipal);
      await refreshScope();
      setPassword('');
    } catch (error) {
      setLoginError(error instanceof Error ? error.message : 'Unable to sign in');
    } finally {
      setLoggingIn(false);
    }
  }

  if (!open) return null;

  return (
    <>
      <button className="settings-scrim" type="button" onClick={onClose} aria-label="Close settings" />
      <aside className="settings-drawer" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <div className="settings-heading">
          <div><span className="section-label">Manage</span><h2 id="settings-title">Workspace settings</h2></div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close settings"><IconX size={16} /></button>
        </div>
        <p className="settings-intro">Review the active workspace boundary and local runtime connections used by the console.</p>

        <section className="settings-section" aria-labelledby="settings-scope-title">
          <div className="settings-section-heading"><IconCloud size={16} /><strong id="settings-scope-title">Active scope</strong></div>
          <div className="settings-scope-card">{scope ? <><strong>{scope.name}</strong><span>{scope.tenant_id} / {scope.environment_id}</span><small>{scope.environment_name}</small></> : <span>No tenant environment is available.</span>}</div>
        </section>

        <section className="settings-section" aria-labelledby="settings-runtime-title">
          <div className="settings-section-heading"><IconServer size={16} /><strong id="settings-runtime-title">Runtime connections</strong></div>
          <div className="settings-status-list">
            <div><span>API</span><strong className={apiState}><i />{apiState === 'checking' ? 'Checking' : apiState === 'ready' ? 'Ready' : 'Unavailable'}</strong></div>
            <div><span>Worker</span><strong className={workerState}><i />{workerState === 'checking' ? 'Checking' : workerState === 'ready' ? 'Ready' : 'Unavailable'}</strong></div>
          </div>
        </section>

        <section className="settings-section" aria-labelledby="settings-access-title">
          <div className="settings-section-heading"><IconKey size={16} /><strong id="settings-access-title">Console access</strong></div>
          <div className="settings-access-row"><span>Admin and integration keys</span><strong className={hasConfiguredKeys() ? 'ready' : 'degraded'}>{hasConfiguredKeys() ? <IconCheck size={14} /> : null}{hasConfiguredKeys() ? 'configured' : 'development defaults'}</strong></div>
          <small className="settings-note">Keys stay in the local runtime configuration and are never shown in the dashboard. Set the Vite key variables before deploying.</small>
          <form className="settings-login" onSubmit={(event) => void signIn(event)}>
            <span className="settings-login-label">Sign in with an admin session</span>
            <input aria-label="Admin email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="admin@example.com" autoComplete="username" />
            <input aria-label="Admin password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="Password" autoComplete="current-password" />
            {loginError && <small className="settings-login-error" role="alert">{loginError}</small>}
            <button className="primary-button" type="submit" disabled={loggingIn || !email || !password}>{loggingIn ? 'Signing in…' : principal?.id === 'bootstrap-key' ? 'Switch to session' : 'Sign in'}</button>
          </form>
        </section>

        <div className="settings-actions"><button className="text-button" type="button" onClick={onClose}>Close</button><button className="primary-button" type="button" onClick={() => void refresh()} disabled={refreshing}><IconRefresh size={15} className={refreshing ? 'spin' : ''} />{refreshing ? 'Refreshing…' : 'Refresh connections'}</button></div>
      </aside>
    </>
  );
}
