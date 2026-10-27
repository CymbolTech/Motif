import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconCheck, IconCopy, IconKey, IconPlus, IconRefresh } from '@tabler/icons-react';
import { createAccessToken, fetchAccessTokens, fetchEnvironments, revokeAccessToken, type AccessToken, type AccessTokenScope, type Environment } from '../../../shared/api/client';
import { formatDateTime } from '../../../shared/lib/format';
import { useScope } from '../../../shared/scope/ScopeContext';
import { useMatchRoute, useNavigate } from '@tanstack/react-router';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { PageHeader } from '../../../shared/ui/PageHeader';

const scopeOptions: Array<{ scope: AccessTokenScope; label: string; description: string }> = [
  { scope: 'read', label: 'Read', description: 'See event contracts, catalog, rules, events, traces and user state. Check rules without saving them.' },
  { scope: 'write', label: 'Write', description: 'Register event contracts, create catalog items and rule drafts, and send test events.' },
  { scope: 'publish', label: 'Publish', description: 'Publish rule versions so they run on incoming events.' },
];

const expiryOptions = [
  { value: '30', label: '30 days' },
  { value: '90', label: '90 days' },
  { value: '365', label: '1 year' },
  { value: '0', label: 'No expiry' },
];

const clients = ['Claude Code', 'Cursor', 'VS Code'] as const;
type Client = (typeof clients)[number];

const emptyForm = () => ({ name: '', scopes: ['read', 'write'] as AccessTokenScope[], allEnvironments: true, environmentIds: [] as string[], expiry: '90' });

function mcpURL() {
  return import.meta.env.VITE_MCP_URL || `${window.location.origin}/mcp`;
}

function connectSnippet(client: Client, token: string) {
  const url = mcpURL();
  if (client === 'Claude Code') return `claude mcp add --transport http motif ${url} \\\n  --header "Authorization: Bearer ${token}"`;
  if (client === 'Cursor') return JSON.stringify({ mcpServers: { motif: { url, headers: { Authorization: `Bearer ${token}` } } } }, null, 2);
  return JSON.stringify({ servers: { motif: { type: 'http', url, headers: { Authorization: `Bearer ${token}` } } } }, null, 2);
}

function snippetHint(client: Client) {
  if (client === 'Claude Code') return 'Run in your project directory.';
  if (client === 'Cursor') return 'Add to .cursor/mcp.json in your project.';
  return 'Add to .vscode/mcp.json in your project.';
}

function tokenState(token: AccessToken) {
  if (token.revoked_at) return { label: 'Revoked', tone: 'badge-neutral' };
  if (token.expires_at && new Date(token.expires_at) <= new Date()) return { label: 'Expired', tone: 'badge-warning' };
  return { label: 'Active', tone: 'badge-success' };
}

export function AccessTokensPage() {
  const { scope } = useScope();
  const [tokens, setTokens] = useState<AccessToken[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<{ token: string; access_token: AccessToken } | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!scope) { setTokens([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    try {
      const [nextTokens, nextEnvironments] = await Promise.all([fetchAccessTokens(scope.tenant_id), fetchEnvironments(scope.tenant_id)]);
      setTokens(nextTokens);
      setEnvironments(nextEnvironments);
    } catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Access tokens could not be loaded.'); }
    finally { setLoading(false); }
  }, [scope]);
  useEffect(() => { void refresh(); }, [refresh]);

  const environmentName = useMemo(() => new Map(environments.map((item) => [item.environment_id, item.name])), [environments]);
  const activeCount = tokens.filter((token) => tokenState(token).label === 'Active').length;

  const navigate = useNavigate();
  const open = Boolean(useMatchRoute()({ to: '/access-tokens/new' }));
  // Opening the form starts it fresh; leaving it forgets a revealed token.
  useEffect(() => {
    if (open) beginCreate();
    else { setCreated(null); setError(null); }
  }, [open]);

  function beginCreate() { setForm({ ...emptyForm(), environmentIds: scope ? [scope.environment_id] : [] }); setCreated(null); setError(null); }
  function closeEditor() { void navigate({ to: '/access-tokens' }); }
  const startCreate = () => void navigate({ to: '/access-tokens/new' });
  function toggleScope(value: AccessTokenScope) {
    if (value === 'read') return;
    setForm((current) => ({ ...current, scopes: current.scopes.includes(value) ? current.scopes.filter((item) => item !== value) : [...current.scopes, value] }));
  }
  function toggleEnvironment(value: string) {
    setForm((current) => ({ ...current, environmentIds: current.environmentIds.includes(value) ? current.environmentIds.filter((item) => item !== value) : [...current.environmentIds, value] }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setSaving(true);
    setError(null);
    try {
      const name = form.name.trim();
      if (!name) throw new Error('Name the token after the tool or person that will use it.');
      if (!form.allEnvironments && form.environmentIds.length === 0) throw new Error('Choose at least one environment, or allow every environment.');
      const result = await createAccessToken({ tenant_id: scope.tenant_id, name, scopes: (['read', 'write', 'publish'] as AccessTokenScope[]).filter((item) => form.scopes.includes(item)), environment_ids: form.allEnvironments ? [] : form.environmentIds, expires_in_days: Number(form.expiry) });
      setCreated(result);
      setTokens((current) => [result.access_token, ...current]);
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Access token could not be created.'); }
    finally { setSaving(false); }
  }

  async function revoke(token: AccessToken) {
    if (!scope) return;
    if (confirming !== token.id) { setConfirming(token.id); return; }
    setConfirming(null);
    try {
      const revoked = await revokeAccessToken(scope.tenant_id, token.id);
      setTokens((current) => current.map((item) => item.id === revoked.id ? revoked : item));
    } catch (revokeError) { setError(revokeError instanceof Error ? revokeError.message : 'Access token could not be revoked.'); }
  }

  return <main className={`page page-resource ${open ? 'page-narrow' : ''}`}>
    <PageHeader breadcrumb={['Workspace', 'Access tokens', ...(open ? [created ? 'Created' : 'Create'] : [])]} title={open ? created ? 'Token created' : 'Create access token' : 'Access tokens'} description={open ? created ? 'Copy the token now. It is shown only once.' : 'Choose what the token may do and where.' : 'Long-lived tokens for MCP clients and scripts that configure and inspect this workspace. Your backend keeps sending events with the REST integration key.'} actions={open ? <button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to tokens</button> : <button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Create token</button>} />

    {!open && <section className="panel" aria-label="Access tokens">
      <div className="toolbar">
        <p className="tally"><strong>{tokens.length}</strong> total <span aria-hidden="true">·</span> <strong>{activeCount}</strong> active</p>
        <div className="toolbar-controls"><button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh access tokens"><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button></div>
      </div>
      {error && <div className="panel-body"><p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Retry</button></p></div>}
      {loading ? <div className="skeleton-rows" aria-label="Loading access tokens"><span /><span /></div> : tokens.length === 0 ? <div className="empty-state"><IconKey size={22} stroke={1.5} aria-hidden="true" /><strong>No access tokens yet</strong><span>Create one to connect Claude Code, Cursor, VS Code or another MCP client to this workspace.</span><button className="primary-button" type="button" onClick={startCreate} disabled={!scope}><IconPlus size={15} stroke={1.75} />Create token</button></div> : <ul className="resource-list">{tokens.map((token) => {
        const state = tokenState(token);
        const where = token.environment_ids.length === 0 ? 'All environments' : token.environment_ids.map((id) => environmentName.get(id) ?? id).join(', ');
        return <li key={token.id} className={`resource-row ${state.label === 'Active' ? '' : 'is-archived'}`}>
          <div className="resource-row-main"><strong>{token.name}</strong><span><span className="mono">{token.hint}</span> · {where} · {token.last_used_at ? `Last used ${formatDateTime(token.last_used_at)}` : 'Never used'}{token.expires_at && !token.revoked_at ? ` · Expires ${formatDateTime(token.expires_at)}` : ''}</span><span className="token-badges">{token.scopes.map((item) => <span className="badge badge-outline" key={item}>{item}</span>)}</span></div>
          <span className={`badge ${state.tone}`}><span className="dot" />{state.label}</span>
          <div className="resource-row-actions">{state.label === 'Active' && <button className={`text-button ${confirming === token.id ? 'danger' : ''}`} type="button" onClick={() => void revoke(token)} onBlur={() => setConfirming((current) => current === token.id ? null : current)} aria-label={confirming === token.id ? `Confirm revoking ${token.name}` : `Revoke ${token.name}`}>{confirming === token.id ? 'Confirm revoke' : 'Revoke'}</button>}</div>
        </li>;
      })}</ul>}
    </section>}

    {open && !created && <section className="panel resource-editor" aria-label="Create access token">
      <form className="resource-form" onSubmit={(event) => void submit(event)}>
        <div className="resource-form-fields">
          <label className="field field-wide"><span className="field-label">Name</span><input autoFocus required maxLength={80} value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="Claude Code · checkout service" /><small className="field-help">Name it after the tool or person that will hold it, so the audit log stays readable.</small></label>
          <fieldset className="field field-wide"><legend className="field-label">Permissions</legend><ul className="option-list">{scopeOptions.map((option) => <li key={option.scope}><label className={`option-row ${option.scope === 'read' ? 'is-locked' : ''}`}><input type="checkbox" checked={form.scopes.includes(option.scope)} disabled={option.scope === 'read'} onChange={() => toggleScope(option.scope)} /><span><strong>{option.label}</strong><small>{option.description}</small></span></label></li>)}</ul><small className="field-help">Tokens never touch integration keys, reward providers, point transactions or redemptions.</small></fieldset>
          <fieldset className="field field-wide"><legend className="field-label">Environments</legend><div className="segmented" role="group" aria-label="Environment access"><button type="button" aria-pressed={form.allEnvironments} onClick={() => setForm({ ...form, allEnvironments: true })}>All environments</button><button type="button" aria-pressed={!form.allEnvironments} onClick={() => setForm({ ...form, allEnvironments: false })}>Only selected</button></div>{!form.allEnvironments && <ul className="option-list">{environments.map((environment) => <li key={environment.environment_id}><label className="option-row"><input type="checkbox" checked={form.environmentIds.includes(environment.environment_id)} onChange={() => toggleEnvironment(environment.environment_id)} /><span><strong>{environment.name}</strong><small className="mono">{environment.environment_id}</small></span></label></li>)}</ul>}</fieldset>
          <div className="field"><span className="field-label">Expires</span><CustomSelect value={form.expiry} onChange={(value) => setForm({ ...form, expiry: value })} ariaLabel="Token expiry" options={expiryOptions} /></div>
        </div>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        <div className="resource-form-footer"><span className="form-footer-note">Creating in <strong>{scope?.name}</strong></span><button className="secondary-button" type="button" onClick={closeEditor}>Cancel</button><button className="primary-button" type="submit" disabled={saving || !scope}>{saving ? 'Creating…' : 'Create token'}</button></div>
      </form>
    </section>}

    {open && created && <TokenReveal created={created} onDone={closeEditor} />}
  </main>;
}

function TokenReveal({ created, onDone }: { created: { token: string; access_token: AccessToken }; onDone: () => void }) {
  const [client, setClient] = useState<Client>('Claude Code');
  const [copied, setCopied] = useState<'token' | 'snippet' | null>(null);
  const snippet = connectSnippet(client, created.token);

  async function copy(value: string, what: 'token' | 'snippet') {
    await navigator.clipboard?.writeText(value);
    setCopied(what);
    window.setTimeout(() => setCopied(null), 1600);
  }

  return <section className="panel resource-editor" aria-label="New access token">
    <div className="panel-body token-reveal">
      <div className="secret-field"><span className="field-label">{created.access_token.name}</span><div className="secret-value"><code>{created.token}</code><button className="icon-button" type="button" onClick={() => void copy(created.token, 'token')} aria-label={copied === 'token' ? 'Token copied' : 'Copy token'}>{copied === 'token' ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div><small className="field-help">Store it like a password. Anyone holding it can act in this workspace within its permissions until you revoke it.</small></div>
      <div className="field"><span className="field-label">Connect an MCP client</span><div className="connect-snippet"><div className="connect-snippet-head"><div className="segmented" role="group" aria-label="MCP client">{clients.map((item) => <button key={item} type="button" aria-pressed={client === item} onClick={() => setClient(item)}>{item}</button>)}</div><button className="icon-button" type="button" onClick={() => void copy(snippet, 'snippet')} aria-label={copied === 'snippet' ? 'Snippet copied' : `Copy ${client} setup`}>{copied === 'snippet' ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div><pre className="code-block">{snippet}</pre></div><small className="field-help">{snippetHint(client)} Other clients connect to <span className="mono">{mcpURL()}</span> with the token as a bearer header.</small></div>
    </div>
    <div className="resource-form-footer"><span className="form-footer-note">The token will not be shown again.</span><button className="primary-button" type="button" onClick={onDone}>Done</button></div>
  </section>;
}
