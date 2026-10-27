import { useEffect, useState } from 'react';
import { IconAlertTriangle, IconArrowRight, IconCheck, IconCopy, IconInfoCircle } from '@tabler/icons-react';
import { RouterProvider } from '@tanstack/react-router';
import { router } from './router';
import { ScopeProvider } from '../shared/scope/ScopeContext';
import { BrandMark } from '../shared/ui/BrandMark';
import { clearAdminSession, fetchAdminPrincipal, loginAdmin, signupTenant, type AdminPrincipal, type TenantSignupResponse } from '../shared/api/client';

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
  return <ScopeProvider><RouterProvider router={router} context={{ principal, signOut: () => { setPrincipal(null); setAuthState('signed_out'); } }} /></ScopeProvider>;
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
