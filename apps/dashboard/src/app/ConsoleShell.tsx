import { useEffect, useMemo, useState, type ComponentType } from 'react';
import { Link, Outlet, useLocation, useRouteContext } from '@tanstack/react-router';
import { IconBolt, IconBook2, IconCalendarEvent, IconChartBar, IconCode, IconCoin, IconExternalLink, IconFlag, IconGift, IconKey, IconLayoutDashboard, IconListNumbers, IconLogout, IconMenu2, IconSparkles, IconTarget, IconTrophy, IconUserCircle, IconX } from '@tabler/icons-react';
import { clearAdminSession, logoutAdmin } from '../shared/api/client';
import { useScope } from '../shared/scope/ScopeContext';
import { BrandMark } from '../shared/ui/BrandMark';
import { TopbarSlotProvider } from '../shared/ui/PageHeader';
import { viewFromPath, viewPaths, type View } from './views';

type NavItem = { label: string; icon: ComponentType<{ size?: number; stroke?: number }>; view: View };
type NavGroup = { label: string; items: NavItem[] };

const navGroups: NavGroup[] = [
  { label: 'Operate', items: [{ label: 'Overview', icon: IconLayoutDashboard, view: 'overview' }, { label: 'Points & rewards', icon: IconUserCircle, view: 'loyalty' }] },
  { label: 'Build', items: [{ label: 'Events', icon: IconCode, view: 'events' }, { label: 'Automations', icon: IconBolt, view: 'automations' }] },
  { label: 'Loyalty', items: [{ label: 'Currencies', icon: IconCoin, view: 'currency' }, { label: 'Levels', icon: IconTrophy, view: 'level' }, { label: 'Missions', icon: IconFlag, view: 'mission' }, { label: 'Achievements', icon: IconSparkles, view: 'achievement' }, { label: 'Rewards', icon: IconGift, view: 'reward' }] },
  { label: 'Engagement', items: [{ label: 'Goals', icon: IconTarget, view: 'goal' }, { label: 'Campaigns', icon: IconCalendarEvent, view: 'campaign' }, { label: 'Leaderboards', icon: IconListNumbers, view: 'leaderboard' }] },
  { label: 'Analyze', items: [{ label: 'Reports', icon: IconChartBar, view: 'analyze' }] },
  { label: 'Workspace', items: [{ label: 'Access keys', icon: IconKey, view: 'access' }] },
];

export function ConsoleShell() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const { signOut: onSignedOut } = useRouteContext({ from: '__root__' });
  const pathname = useLocation({ select: (location) => location.pathname });
  const view = viewFromPath(pathname);
  const { scope, loading: scopeLoading, error: scopeError } = useScope();
  const docsURL = import.meta.env.VITE_DOCS_URL || `${window.location.protocol}//${window.location.hostname}:4174/docs`;

  // Following a link closes the mobile navigation.
  useEffect(() => { setMobileOpen(false); }, [pathname]);

  const signOut = async () => {
    await logoutAdmin().catch(() => undefined);
    // The browser must leave the tenant console even if the API is unavailable;
    // never keep a stale session token after the user signs out.
    clearAdminSession();
    onSignedOut();
  };

  const [crumbSlot, setCrumbSlot] = useState<HTMLElement | null>(null);
  const [actionSlot, setActionSlot] = useState<HTMLElement | null>(null);
  const topbarSlots = useMemo(() => ({ crumbs: crumbSlot, actions: actionSlot }), [crumbSlot, actionSlot]);
  const workspaceInitial = scope?.name.trim().charAt(0).toUpperCase() || 'W';

  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    <aside className={`sidebar ${mobileOpen ? 'open' : ''}`} aria-label="Console navigation">
      <div className="sidebar-brand"><BrandMark /><strong>MotifEngine</strong><button className="icon-button ghost sidebar-close" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><IconX size={16} /></button></div>
      <div className="sidebar-workspace" aria-label="Active workspace"><span className="workspace-avatar" aria-hidden="true">{workspaceInitial}</span><span className="sidebar-workspace-copy"><strong>{scope?.name ?? (scopeLoading ? 'Loading workspace…' : 'Workspace unavailable')}</strong><span><span className={`dot ${scope ? scope.status === 'active' ? 'dot-success' : 'dot-danger' : 'dot-muted'}`} />{scope ? scope.environment_name : scopeError ? 'Membership unavailable' : 'Loading environment…'}</span></span></div>
      <nav className="sidebar-nav" aria-label="Main navigation">{navGroups.map((group) => <div className="nav-group" key={group.label}><span className="nav-label">{group.label}</span><ul>{group.items.map(({ label, icon: Icon, view: itemView }) => { const active = itemView === view; return <li key={itemView}><Link className={`nav-item ${active ? 'active' : ''}`} to={viewPaths[itemView]} aria-current={active ? 'page' : undefined}><Icon size={16} stroke={1.75} /><span>{label}</span></Link></li>; })}</ul></div>)}</nav>
      <div className="sidebar-footer"><a className="nav-item" href={docsURL} target="_blank" rel="noopener noreferrer"><IconBook2 size={16} stroke={1.75} /><span>Docs</span><IconExternalLink className="nav-item-trailing" size={14} stroke={1.75} aria-label="Opens in a new tab" /></a><button className="nav-item" type="button" onClick={() => void signOut()}><IconLogout size={16} stroke={1.75} /><span>Sign out</span></button></div>
    </aside>
    {mobileOpen && <button className="mobile-scrim" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation" />}
    <div className="app-main">
      <div className="topbar">
        <button className="icon-button ghost topbar-menu" type="button" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><IconMenu2 size={18} /></button>
        <nav className="topbar-crumbs" aria-label="Breadcrumb" ref={setCrumbSlot} />
        <div className="topbar-actions" ref={setActionSlot} />
        <span className="scope-chip" title={scope ? `${scope.name} · ${scope.environment_name}` : undefined}><span className={`dot ${scope ? scope.status === 'active' ? 'dot-success' : 'dot-danger' : 'dot-muted'}`} />{scope ? scope.environment_name : scopeLoading ? 'Connecting…' : 'No environment'}</span>
      </div>
      <div className="app-content" id="main-content"><TopbarSlotProvider value={topbarSlots}><Outlet /></TopbarSlotProvider></div>
    </div>
  </div>;
}
