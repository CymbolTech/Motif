import { useEffect, useState } from 'react';
import { IconAdjustmentsHorizontal, IconBook2, IconChevronDown, IconCode, IconCoin, IconCommand, IconLayoutDashboard, IconLogout, IconMenu2, IconSparkles, IconX } from '@tabler/icons-react';
import { EventTypesPage } from '../pages/event-types/ui/EventTypesPage';
import { OverviewPage } from '../pages/overview/ui/OverviewPage';
import { RulesPage } from '../pages/rules/ui/RulesPage';
import { LoyaltyPage } from '../pages/loyalty/ui/LoyaltyPage';
import { SettingsPanel } from '../features/settings/ui/SettingsPanel';
import { ScopeProvider, useScope } from '../shared/scope/ScopeContext';
import { fetchAdminPrincipal, logoutAdmin, type AdminPrincipal } from '../shared/api/client';

type View = 'overview' | 'event-types' | 'rules' | 'loyalty';

const navItems = [
  { label: 'Overview', icon: IconLayoutDashboard, view: 'overview' as View },
  { label: 'Event types', icon: IconCode, view: 'event-types' as View },
  { label: 'Rules', icon: IconSparkles, view: 'rules' as View },
  { label: 'Points & rewards', icon: IconCoin, view: 'loyalty' as View },
];

export function App() {
  return <ScopeProvider><ConsoleShell /></ScopeProvider>;
}

function ConsoleShell() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [view, setView] = useState<View>('overview');
  const { tenants, scope, loading: scopeLoading, error: scopeError, selectScope } = useScope();
  const [principal, setPrincipal] = useState<AdminPrincipal | null>(null);

  useEffect(() => {
    void fetchAdminPrincipal().then(setPrincipal).catch(() => setPrincipal(null));
  }, []);

  const signOut = async () => {
    await logoutAdmin().catch(() => undefined);
    setPrincipal(null);
    window.location.reload();
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
          <button className="nav-item" type="button"><IconBook2 size={17} stroke={1.6} /><span>Docs</span><IconArrowExternal /></button>
        </nav>
        <div className="sidebar-footer">
          <div className="status-capsule"><span className="status-dot" />All systems normal</div>
          <button className="profile-button" type="button"><span className="profile-avatar">{principal?.role === 'tenant_admin' ? 'TA' : 'SA'}</span><span><strong>{principal?.role === 'tenant_admin' ? 'Tenant Admin' : 'Super Admin'}</strong><small>{principal?.email ?? 'Checking access…'}</small></span><IconChevronDown size={15} /></button>
          <button className="logout-button" type="button" onClick={() => void signOut()}><IconLogout size={16} /><span>Sign out</span></button>
        </div>
      </aside>
      {mobileOpen && <button className="mobile-scrim" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation" />}
      <div className="app-content">
        <div className="mobile-topbar"><button className="icon-button" type="button" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><IconMenu2 size={20} /></button><span>{view === 'overview' ? 'Overview' : view === 'event-types' ? 'Event types' : view === 'rules' ? 'Rules' : 'Points & rewards'}</span><span className="mobile-topbar-mark"><IconCommand size={15} /></span></div>
        {view === 'overview' ? <OverviewPage principal={principal} /> : view === 'event-types' ? <EventTypesPage /> : view === 'rules' ? <RulesPage /> : <LoyaltyPage />}
      </div>
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} principal={principal} onAuthenticated={setPrincipal} />
    </div>
  );
}

function IconArrowExternal() {
  return <span className="external-arrow" aria-hidden="true">↗</span>;
}
