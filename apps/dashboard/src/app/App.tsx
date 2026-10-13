import { useEffect, useState } from 'react';
import { IconAdjustmentsHorizontal, IconArrowRight, IconBook2, IconChevronDown, IconCode, IconCoin, IconCommand, IconLayoutDashboard, IconLock, IconLogout, IconMenu2, IconSparkles, IconX } from '@tabler/icons-react';
import { EventTypesPage } from '../pages/event-types/ui/EventTypesPage';
import { OverviewPage } from '../pages/overview/ui/OverviewPage';
import { RulesPage } from '../pages/rules/ui/RulesPage';
import { LoyaltyPage } from '../pages/loyalty/ui/LoyaltyPage';
import { SettingsPanel } from '../features/settings/ui/SettingsPanel';
import { ScopeProvider, useScope } from '../shared/scope/ScopeContext';
import { clearAdminSession, fetchAdminPrincipal, loginAdmin, logoutAdmin, type AdminPrincipal } from '../shared/api/client';

type View = 'overview' | 'event-types' | 'rules' | 'loyalty';

const navItems = [
  { label: 'Overview', icon: IconLayoutDashboard, view: 'overview' as View },
  { label: 'Event types', icon: IconCode, view: 'event-types' as View },
  { label: 'Rules', icon: IconSparkles, view: 'rules' as View },
  { label: 'Points & rewards', icon: IconCoin, view: 'loyalty' as View },
];

export function App() {
  const [principal, setPrincipal] = useState<AdminPrincipal | null>(null);
  const [authState, setAuthState] = useState<'checking' | 'signed_out' | 'signed_in'>('checking');

  useEffect(() => {
    let cancelled = false;
    void fetchAdminPrincipal().then((nextPrincipal) => {
      if (cancelled) return;
      setPrincipal(nextPrincipal);
      setAuthState('signed_in');
    }).catch(() => {
      if (cancelled) return;
      clearAdminSession();
      setPrincipal(null);
      setAuthState('signed_out');
    });
    return () => { cancelled = true; };
  }, []);

  if (authState === 'checking') return <AuthLoading />;
  if (!principal) return <LoginScreen onAuthenticated={(nextPrincipal) => { setPrincipal(nextPrincipal); setAuthState('signed_in'); }} />;
  return <ScopeProvider><ConsoleShell principal={principal} onAuthenticated={setPrincipal} onSignedOut={() => { setPrincipal(null); setAuthState('signed_out'); }} /></ScopeProvider>;
}

function AuthLoading() {
  return <main className="auth-screen"><section className="auth-card auth-loading" aria-live="polite"><div className="auth-mark"><IconLock size={20} /></div><span>Checking console access…</span></section></main>;
}

function LoginScreen({ onAuthenticated }: { onAuthenticated: (principal: AdminPrincipal) => void }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function signIn(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      onAuthenticated(await loginAdmin(email, password));
    } catch (loginError) {
      setError(loginError instanceof Error ? loginError.message : 'Unable to sign in');
    } finally {
      setSubmitting(false);
    }
  }

  return <main className="auth-screen"><section className="auth-card" aria-labelledby="auth-title">
    <div className="auth-brand"><div className="auth-mark"><IconLock size={20} /></div><span>motif / engine console</span></div>
    <div className="auth-copy"><span className="section-label">Private workspace</span><h1 id="auth-title">Sign in to your console.</h1><p>Use an administrator account to access tenants, rules, events, and measurement.</p></div>
    <form className="auth-form" onSubmit={(event) => void signIn(event)}>
      <label><span>Email</span><input autoFocus required type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" placeholder="admin@example.com" /></label>
      <label><span>Password</span><input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" placeholder="Your password" /></label>
      {error && <p className="auth-error" role="alert">{error}</p>}
      <button className="auth-submit" type="submit" disabled={submitting}>{submitting ? 'Signing in…' : <>Continue <IconArrowRight size={16} /></>}</button>
    </form>
  </section></main>;
}

function ConsoleShell({ principal, onAuthenticated, onSignedOut }: { principal: AdminPrincipal; onAuthenticated: (principal: AdminPrincipal) => void; onSignedOut: () => void }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [view, setView] = useState<View>('overview');
  const { tenants, scope, loading: scopeLoading, error: scopeError, selectScope } = useScope();
  const docsURL = import.meta.env.VITE_DOCS_URL || `${window.location.protocol}//${window.location.hostname}:4174/docs`;

  const signOut = async () => {
    await logoutAdmin().catch(() => undefined);
    onSignedOut();
  };

  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileOpen ? 'open' : ''}`}>
        <div className="brand-lockup">
          <div className="brand-mark" aria-hidden="true"><span /><span /><span /></div>
          <div><strong>motif</strong><span>engine console</span></div>
          <button className="mobile-close icon-button" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><IconX size={18} /></button>
        </div>
        <div className="workspace-switcher">
          <div className="workspace-avatar">M</div>
          <div className="workspace-switcher-copy"><strong>{scope?.name ?? 'Motif platform'}</strong><span>{scope ? scope.environment_name : scopeLoading ? 'Loading scope…' : 'No scope available'}</span></div>
          <select className="workspace-scope-select" value={scope ? `${scope.tenant_id}::${scope.environment_id}` : ''} onChange={(event) => selectScope(event.target.value)} aria-label="Active tenant environment" disabled={scopeLoading || tenants.length === 0}><option value="" disabled>{scopeError ? 'Scope unavailable' : 'Select scope'}</option>{tenants.map((tenant) => <option key={`${tenant.tenant_id}-${tenant.environment_id}`} value={`${tenant.tenant_id}::${tenant.environment_id}`}>{tenant.name} · {tenant.environment_name}</option>)}</select>
          <IconChevronDown size={15} stroke={1.6} />
        </div>
        <nav aria-label="Main navigation">
          <span className="nav-label">Operate</span>
          {navItems.map(({ label, icon: Icon, view: itemView }) => { const active = itemView === view; return <button className={`nav-item ${active ? 'active' : ''} ${itemView ? '' : 'disabled'}`} type="button" key={label} disabled={!itemView} onClick={() => { if (itemView) { setView(itemView); setMobileOpen(false); } }}><Icon size={17} stroke={active ? 2 : 1.6} /><span>{label}</span>{active && <span className="nav-pip" />}{!itemView && <span className="nav-soon">soon</span>}</button>; })}
          <span className="nav-label secondary">Manage</span>
          <button className={`nav-item ${settingsOpen ? 'active' : ''}`} type="button" onClick={() => setSettingsOpen(true)}><IconAdjustmentsHorizontal size={17} stroke={1.6} /><span>Settings</span>{settingsOpen && <span className="nav-pip" />}</button>
          <a className="nav-item" href={docsURL} target="_blank" rel="noopener noreferrer"><IconBook2 size={17} stroke={1.6} /><span>Docs</span><IconArrowExternal /></a>
        </nav>
        <div className="sidebar-footer">
          <div className="status-capsule"><span className="status-dot" />All systems normal</div>
          <div className="profile-button"><span className="profile-avatar">{principal?.role === 'tenant_admin' ? 'TA' : 'SA'}</span><span><strong>{principal?.role === 'tenant_admin' ? 'Tenant Admin' : 'Super Admin'}</strong><small>{principal?.email ?? 'Checking access…'}</small></span></div>
          <button className="logout-button" type="button" onClick={() => void signOut()}><IconLogout size={16} /><span>Sign out</span></button>
        </div>
      </aside>
      {mobileOpen && <button className="mobile-scrim" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation" />}
      <div className="app-content">
        <div className="mobile-topbar"><button className="icon-button" type="button" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><IconMenu2 size={20} /></button><span>{view === 'overview' ? 'Overview' : view === 'event-types' ? 'Event types' : view === 'rules' ? 'Rules' : 'Points & rewards'}</span><span className="mobile-topbar-mark"><IconCommand size={15} /></span></div>
        {view === 'overview' ? <OverviewPage principal={principal} /> : view === 'event-types' ? <EventTypesPage /> : view === 'rules' ? <RulesPage /> : <LoyaltyPage />}
      </div>
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} principal={principal} onAuthenticated={onAuthenticated} />
    </div>
  );
}

function IconArrowExternal() {
  return <span className="external-arrow" aria-hidden="true">↗</span>;
}
