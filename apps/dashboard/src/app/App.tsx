import { useEffect, useMemo, useState, type ComponentType } from 'react';
import { IconAlertTriangle, IconArrowRight, IconBolt, IconBook2, IconCalendarEvent, IconChartBar, IconCheck, IconCode, IconCoin, IconCopy, IconExternalLink, IconFlag, IconGift, IconInfoCircle, IconKey, IconLayoutDashboard, IconLogout, IconMenu2, IconSparkles, IconTarget, IconTrophy, IconUserCircle, IconX } from '@tabler/icons-react';
import { OverviewPage } from '../pages/overview/ui/OverviewPage';
import { LoyaltyPage } from '../pages/loyalty/ui/LoyaltyPage';
import { ConfigurePage } from '../pages/configure/ui/ConfigurePage';
import { CatalogPage } from '../pages/catalog/ui/CatalogPage';
import { EngagementPage } from '../pages/engagement/ui/EngagementPage';
import { AnalyzePage } from '../pages/analyze/ui/AnalyzePage';
import { AccessTokensPage } from '../pages/access-tokens/ui/AccessTokensPage';
import { ScopeProvider, useScope } from '../shared/scope/ScopeContext';
import { TopbarSlotProvider } from '../shared/ui/PageHeader';
import { clearAdminSession, fetchAdminPrincipal, loginAdmin, logoutAdmin, signupTenant, type AdminPrincipal, type TenantSignupResponse } from '../shared/api/client';

type View = 'overview' | 'loyalty' | 'events' | 'automations' | 'currency' | 'level' | 'mission' | 'achievement' | 'reward' | 'goal' | 'campaign' | 'analyze' | 'access';
type NavItem = { label: string; icon: ComponentType<{ size?: number; stroke?: number }>; view: View };
type NavGroup = { label: string; items: NavItem[] };

const viewPaths: Record<View, string> = {
  overview: '/',
  loyalty: '/loyalty',
  events: '/events',
  automations: '/automations',
  currency: '/currencies',
  level: '/levels',
  mission: '/missions',
  achievement: '/achievements',
  reward: '/rewards',
  goal: '/goals',
  campaign: '/campaigns',
  analyze: '/reports',
  access: '/access-tokens',
};

function viewFromPath(pathname: string): View {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/';
  const entries = Object.entries(viewPaths) as [View, string][];
  const match = entries.find(([, path]) => path === normalizedPath) ?? entries.find(([, path]) => path !== '/' && normalizedPath.startsWith(`${path}/`));
  return match?.[0] ?? 'overview';
}

function isKnownPath(pathname: string) {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/';
  return Object.values(viewPaths).includes(normalizedPath) || normalizedPath === '/events/new' || normalizedPath === '/automations/new';
}

const navGroups: NavGroup[] = [
  { label: 'Operate', items: [{ label: 'Overview', icon: IconLayoutDashboard, view: 'overview' }, { label: 'Points & rewards', icon: IconUserCircle, view: 'loyalty' }] },
  { label: 'Build', items: [{ label: 'Events', icon: IconCode, view: 'events' }, { label: 'Automations', icon: IconBolt, view: 'automations' }] },
  { label: 'Loyalty', items: [{ label: 'Currencies', icon: IconCoin, view: 'currency' }, { label: 'Levels', icon: IconTrophy, view: 'level' }, { label: 'Missions', icon: IconFlag, view: 'mission' }, { label: 'Achievements', icon: IconSparkles, view: 'achievement' }, { label: 'Rewards', icon: IconGift, view: 'reward' }] },
  { label: 'Engagement', items: [{ label: 'Goals', icon: IconTarget, view: 'goal' }, { label: 'Campaigns', icon: IconCalendarEvent, view: 'campaign' }] },
  { label: 'Analyze', items: [{ label: 'Reports', icon: IconChartBar, view: 'analyze' }] },
  { label: 'Workspace', items: [{ label: 'Access tokens', icon: IconKey, view: 'access' }] },
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
  if (principal.role === 'super_admin') return <OpsOnlyScreen email={principal.email} onSignedOut={() => { clearAdminSession(); setPrincipal(null); setAuthState('signed_out'); }} />;
  if (principal.role !== 'tenant_admin') return <AccessDeniedScreen onSignedOut={() => { clearAdminSession(); setPrincipal(null); setAuthState('signed_out'); }} />;
  return <ScopeProvider><ConsoleShell principal={principal} onSignedOut={() => { setPrincipal(null); setAuthState('signed_out'); }} /></ScopeProvider>;
}

function AuthLoading() {
  return <main className="auth-screen"><section className="auth-card auth-loading" aria-live="polite"><BrandMark /><span>Checking workspace access…</span></section></main>;
}

function AuthBrand() {
  return <div className="auth-brand"><BrandMark /><span>MotifEngine</span></div>;
}

function OpsOnlyScreen({ email, onSignedOut }: { email: string; onSignedOut: () => void }) {
  return <main className="auth-screen"><section className="auth-card" aria-labelledby="ops-only-title"><AuthBrand /><div className="auth-copy"><h1 id="ops-only-title">This account belongs in Ops.</h1><p>{email} is a platform account. The console is for workspace owners; platform operations live in a separate Ops service.</p></div><div className="auth-note"><IconInfoCircle size={16} aria-hidden="true" /><p><strong>Nothing is wrong with your account.</strong> Sign out and use a workspace account to configure events, loyalty, and automations.</p></div><button className="primary-button auth-submit" type="button" onClick={onSignedOut}>Sign out</button></section></main>;
}

function AccessDeniedScreen({ onSignedOut }: { onSignedOut: () => void }) {
  return <main className="auth-screen"><section className="auth-card" aria-labelledby="access-denied-title"><AuthBrand /><div className="auth-copy"><h1 id="access-denied-title">Your account needs a workspace role.</h1><p>This account does not have a workspace membership yet. Ask your workspace owner to invite you, or create a new workspace account.</p></div><button className="primary-button auth-submit" type="button" onClick={onSignedOut}>Back to sign in</button></section></main>;
}

function LoginScreen({ onAuthenticated }: { onAuthenticated: (principal: AdminPrincipal) => void }) {
  const [mode, setMode] = useState<'signin' | 'signup'>('signin');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [workspaceName, setWorkspaceName] = useState('');
  const [workspaceId, setWorkspaceId] = useState('');
  const [createdWorkspace, setCreatedWorkspace] = useState<TenantSignupResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function workspaceSlug(value: string) {
    return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 64);
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      if (mode === 'signup') {
        const name = workspaceName.trim();
        const tenantId = workspaceSlug(workspaceId || name);
        if (!name) throw new Error('Enter a workspace name.');
        if (!tenantId) throw new Error('Choose a workspace name using letters or numbers.');
        const result = await signupTenant({ email, password, tenant_id: tenantId, name });
        setCreatedWorkspace(result);
      } else {
        onAuthenticated(await loginAdmin(email, password));
      }
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Unable to continue');
    } finally {
      setSubmitting(false);
    }
  }

  if (createdWorkspace) return <WorkspaceCreatedScreen signup={createdWorkspace} onContinue={() => onAuthenticated(createdWorkspace.admin)} />;

  return <main className="auth-screen"><section className="auth-card" aria-labelledby="auth-title">
    <AuthBrand />
    <div className="auth-copy"><h1 id="auth-title">{mode === 'signin' ? 'Sign in to your workspace' : 'Create your workspace'}</h1><p>{mode === 'signin' ? 'Manage events, loyalty, and automations for the workspace you belong to.' : 'Start with a workspace account. You can configure everything from the console.'}</p></div>
    <form className="auth-form" onSubmit={(event) => void submit(event)}>
      {mode === 'signup' && <><label className="field"><span className="field-label">Workspace name</span><input autoFocus required type="text" value={workspaceName} onChange={(event) => setWorkspaceName(event.target.value)} placeholder="Acme Rewards" autoComplete="organization" /></label><label className="field"><span className="field-label">Workspace ID <em>Optional</em></span><input type="text" value={workspaceId} onChange={(event) => setWorkspaceId(event.target.value)} placeholder="acme-rewards" autoComplete="off" /><small className="field-help">Used in API requests. Leave blank to derive it from the workspace name.</small></label></>}
      <label className="field"><span className="field-label">Email</span><input autoFocus={mode === 'signin'} required type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" placeholder="you@company.com" /></label>
      <label className="field"><span className="field-label">Password {mode === 'signup' && <em>At least 12 characters</em>}</span><input required minLength={mode === 'signup' ? 12 : undefined} type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete={mode === 'signin' ? 'current-password' : 'new-password'} placeholder="Your password" /></label>
      {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
      <button className="primary-button auth-submit" type="submit" disabled={submitting}>{submitting ? mode === 'signin' ? 'Signing in…' : 'Creating workspace…' : mode === 'signin' ? 'Continue' : 'Create workspace'}</button>
    </form>
    <p className="auth-switch">{mode === 'signin' ? 'New to MotifEngine?' : 'Already have an account?'} <button className="link-button" type="button" onClick={() => { setMode(mode === 'signin' ? 'signup' : 'signin'); setError(null); }}>{mode === 'signin' ? 'Create a workspace account' : 'Sign in'}</button></p>
  </section></main>;
}

function WorkspaceCreatedScreen({ signup, onContinue }: { signup: TenantSignupResponse; onContinue: () => void }) {
  const [copied, setCopied] = useState(false);

  async function copyKey() {
    await navigator.clipboard?.writeText(signup.integration_key);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return <main className="auth-screen"><section className="auth-card" aria-labelledby="workspace-ready-title">
    <AuthBrand />
    <div className="auth-copy"><h1 id="workspace-ready-title">{signup.tenant.name} is ready</h1><p>Your workspace account is active. Save the integration key now; it is shown only once and belongs in your backend secret store.</p></div>
    <div className="secret-field"><span className="field-label">REST integration key</span><div className="secret-value"><code>{signup.integration_key}</code><button className="icon-button" type="button" onClick={() => void copyKey()} aria-label={copied ? 'Integration key copied' : 'Copy integration key'}>{copied ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div><small className="field-help">Use this key for server-side event requests. Never ship it to a browser or mobile app.</small></div>
    <button className="primary-button auth-submit" type="button" onClick={onContinue}>Open workspace <IconArrowRight size={15} /></button>
  </section></main>;
}

function BrandMark() {
  return <span className="brand-mark" aria-hidden="true"><svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="8" width="3" height="6" rx="1" /><rect x="6.5" y="5" width="3" height="9" rx="1" /><rect x="11" y="2" width="3" height="12" rx="1" /></svg></span>;
}

function ConsoleShell({ principal, onSignedOut }: { principal: AdminPrincipal; onSignedOut: () => void }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [locationPath, setLocationPath] = useState(() => window.location.pathname);
  const view = viewFromPath(locationPath);
  const { scope, loading: scopeLoading, error: scopeError } = useScope();
  const docsURL = import.meta.env.VITE_DOCS_URL || `${window.location.protocol}//${window.location.hostname}:4174/docs`;

  useEffect(() => {
    if (!isKnownPath(window.location.pathname)) {
      window.history.replaceState({}, '', viewPaths.overview);
      setLocationPath(viewPaths.overview);
    }
    const handlePopState = () => setLocationPath(window.location.pathname);
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  function navigate(nextView: View) {
    const nextPath = viewPaths[nextView];
    if (window.location.pathname !== nextPath) window.history.pushState({ view: nextView }, '', nextPath);
    setLocationPath(nextPath);
    setMobileOpen(false);
    window.scrollTo({ top: 0 });
  }

  function navigateConfigure(mode: 'list' | 'create') {
    const nextPath = `${viewPaths[view]}/${mode === 'create' ? 'new' : ''}`.replace(/\/$/, '') || '/';
    if (window.location.pathname !== nextPath) window.history.pushState({ view, mode }, '', nextPath);
    setLocationPath(nextPath);
    window.scrollTo({ top: 0 });
  }

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
  const page = view === 'overview' ? <OverviewPage principal={principal} onNavigate={navigate} /> : view === 'loyalty' ? <LoyaltyPage onNavigate={navigate} /> : view === 'events' || view === 'automations' ? <ConfigurePage area={view === 'events' ? 'events' : 'rules'} initialMode={/\/new\/?$/.test(locationPath) ? 'create' : 'list'} onModeChange={navigateConfigure} /> : view === 'currency' || view === 'level' || view === 'mission' || view === 'achievement' || view === 'reward' ? <CatalogPage key={view} kind={view} /> : view === 'goal' || view === 'campaign' ? <EngagementPage key={view} kind={view} /> : view === 'analyze' ? <AnalyzePage /> : view === 'access' ? <AccessTokensPage /> : <OverviewPage principal={principal} onNavigate={navigate} />;

  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    <aside className={`sidebar ${mobileOpen ? 'open' : ''}`} aria-label="Console navigation">
      <div className="sidebar-brand"><BrandMark /><strong>MotifEngine</strong><button className="icon-button ghost sidebar-close" type="button" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><IconX size={16} /></button></div>
      <div className="sidebar-workspace" aria-label="Active workspace"><span className="workspace-avatar" aria-hidden="true">{workspaceInitial}</span><span className="sidebar-workspace-copy"><strong>{scope?.name ?? (scopeLoading ? 'Loading workspace…' : 'Workspace unavailable')}</strong><span><span className={`dot ${scope ? scope.status === 'active' ? 'dot-success' : 'dot-danger' : 'dot-muted'}`} />{scope ? scope.environment_name : scopeError ? 'Membership unavailable' : 'Loading environment…'}</span></span></div>
      <nav className="sidebar-nav" aria-label="Main navigation">{navGroups.map((group) => <div className="nav-group" key={group.label}><span className="nav-label">{group.label}</span><ul>{group.items.map(({ label, icon: Icon, view: itemView }) => { const active = itemView === view; return <li key={itemView}><button className={`nav-item ${active ? 'active' : ''}`} type="button" aria-current={active ? 'page' : undefined} onClick={() => navigate(itemView)}><Icon size={16} stroke={1.75} /><span>{label}</span></button></li>; })}</ul></div>)}</nav>
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
      <div className="app-content" id="main-content"><TopbarSlotProvider value={topbarSlots}>{page}</TopbarSlotProvider></div>
    </div>
  </div>;
}
