import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { IconAlertTriangle, IconArrowLeft, IconCheck, IconCopy, IconKey, IconPlus, IconRefresh } from '@tabler/icons-react';
import { Link, useMatchRoute, useNavigate } from '@tanstack/react-router';
import { createAccessKey, deleteAccessKey, fetchAccessKeys, revokeAccessKey, updateAccessKey, type AccessKey, type AccessKeyScope } from '../../../shared/api/client';
import { formatDateTime } from '../../../shared/lib/format';
import { useScope } from '../../../shared/scope/ScopeContext';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { PageHeader } from '../../../shared/ui/PageHeader';

const scopeOrder: AccessKeyScope[] = ['read', 'write', 'edit', 'delete', 'mcp'];

const scopeOptions: Array<{ scope: AccessKeyScope; label: string; description: string }> = [
  { scope: 'read', label: 'Read', description: 'Read balances, history, levels, missions, achievements, leaderboards and event status.' },
  { scope: 'write', label: 'Write', description: 'Send events, award and spend points, convert points and redeem rewards. Through MCP: create catalog items, rule drafts and leaderboard rounds, and send test events.' },
  { scope: 'edit', label: 'Edit', description: 'Through MCP: change catalog items, publish rules, and update or end leaderboard rounds.' },
  { scope: 'delete', label: 'Delete', description: 'Through MCP: delete leaderboards.' },
  { scope: 'mcp', label: 'MCP', description: 'Let AI tools such as Claude Code, Cursor and VS Code configure this environment through the MCP server, within the scopes above. Needs Read.' },
];

const expiryOptions = [
  { value: '0', label: 'No expiry' },
  { value: '30', label: '30 days' },
  { value: '90', label: '90 days' },
  { value: '365', label: '1 year' },
];

const clients = ['Claude Code', 'Cursor', 'VS Code'] as const;
type Client = (typeof clients)[number];

type KeyForm = { name: string; scopes: AccessKeyScope[]; anyOrigin: boolean; origins: string; expiry: string };

const emptyForm = (): KeyForm => ({ name: '', scopes: ['read', 'write'], anyOrigin: true, origins: '', expiry: '0' });

const formFor = (key: AccessKey): KeyForm => ({ name: key.name, scopes: key.scopes, anyOrigin: key.allowed_origins.length === 0, origins: key.allowed_origins.join('\n'), expiry: '0' });

function mcpURL() {
  return import.meta.env.VITE_MCP_URL || `${window.location.origin}/mcp`;
}

function apiURL() {
  return `${window.location.origin}/api`;
}

function connectSnippet(client: Client, key: string) {
  const url = mcpURL();
  if (client === 'Claude Code') return `claude mcp add --transport http motif ${url} \\\n  --header "Authorization: Bearer ${key}"`;
  if (client === 'Cursor') return JSON.stringify({ mcpServers: { motif: { url, headers: { Authorization: `Bearer ${key}` } } } }, null, 2);
  return JSON.stringify({ servers: { motif: { type: 'http', url, headers: { Authorization: `Bearer ${key}` } } } }, null, 2);
}

function snippetHint(client: Client) {
  if (client === 'Claude Code') return 'Run in your project directory.';
  if (client === 'Cursor') return 'Add to .cursor/mcp.json in your project.';
  return 'Add to .vscode/mcp.json in your project.';
}

function keyState(key: AccessKey) {
  if (key.revoked_at) return { label: 'Revoked', tone: 'badge-neutral' };
  if (key.expires_at && new Date(key.expires_at) <= new Date()) return { label: 'Expired', tone: 'badge-warning' };
  return { label: 'Active', tone: 'badge-success' };
}

function originSummary(key: AccessKey) {
  if (key.allowed_origins.length === 0) return 'Any website';
  if (key.allowed_origins.length === 1) return key.allowed_origins[0].replace(/^https?:\/\//, '');
  return `${key.allowed_origins.length} websites`;
}

export function AccessKeysPage() {
  const { scope } = useScope();
  const [keys, setKeys] = useState<AccessKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<KeyForm>(emptyForm);
  // The key the form holds values for, so an edit never renders blank fields first.
  const [formKeyId, setFormFor] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<{ key: string; access_key: AccessKey } | null>(null);
  const [confirming, setConfirming] = useState<{ id: string; action: 'revoke' | 'delete' } | null>(null);

  const refresh = useCallback(async () => {
    if (!scope) { setKeys([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    try { setKeys(await fetchAccessKeys(scope)); }
    catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Access keys could not be loaded.'); }
    finally { setLoading(false); }
  }, [scope]);
  useEffect(() => { void refresh(); }, [refresh]);

  const navigate = useNavigate();
  const matchRoute = useMatchRoute();
  const creating = Boolean(matchRoute({ to: '/access-keys/new' }));
  const editMatch = matchRoute({ to: '/access-keys/$id/edit' });
  const editingId = editMatch ? editMatch.id : null;
  const editing = editingId ? keys.find((key) => key.id === editingId) ?? null : null;
  const open = creating || editingId !== null;
  const activeCount = keys.filter((key) => keyState(key).label === 'Active').length;

  // Opening a form starts it from the key or blank; leaving it forgets a revealed key.
  useEffect(() => {
    setError(null);
    if (creating) { setForm(emptyForm()); setCreated(null); }
    else if (!open) setCreated(null);
  }, [creating, open]);
  useEffect(() => {
    if (!editing) return;
    setForm(formFor(editing));
    setFormFor(editing.id);
  }, [editing?.id]);

  const closeEditor = () => void navigate({ to: '/access-keys' });

  function toggleScope(value: AccessKeyScope) {
    setForm((current) => {
      const has = current.scopes.includes(value);
      let next = has ? current.scopes.filter((item) => item !== value) : [...current.scopes, value];
      // MCP reads the workspace first, so it brings Read along.
      if (value === 'mcp' && !has && !next.includes('read')) next = [...next, 'read'];
      return { ...current, scopes: scopeOrder.filter((item) => next.includes(item)) };
    });
  }

  function input() {
    const name = form.name.trim();
    if (!name) throw new Error('Name the key after the service or tool that will hold it.');
    if (form.scopes.length === 0) throw new Error('Choose at least one scope.');
    const origins = form.anyOrigin ? [] : form.origins.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean);
    if (!form.anyOrigin && origins.length === 0) throw new Error('List at least one website, or allow any website.');
    return { name, scopes: form.scopes, allowed_origins: origins };
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    setSaving(true);
    setError(null);
    try {
      if (editing) {
        const updated = await updateAccessKey(scope.tenant_id, editing.id, input());
        setKeys((current) => current.map((item) => item.id === updated.id ? updated : item));
        closeEditor();
      } else {
        const result = await createAccessKey(scope, { ...input(), expires_in_days: Number(form.expiry) });
        setCreated(result);
        setKeys((current) => [result.access_key, ...current]);
      }
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'The access key could not be saved.'); }
    finally { setSaving(false); }
  }

  async function act(key: AccessKey, action: 'revoke' | 'delete') {
    if (!scope) return;
    if (confirming?.id !== key.id || confirming.action !== action) { setConfirming({ id: key.id, action }); return; }
    setConfirming(null);
    try {
      if (action === 'revoke') {
        const revoked = await revokeAccessKey(scope.tenant_id, key.id);
        setKeys((current) => current.map((item) => item.id === revoked.id ? revoked : item));
      } else {
        await deleteAccessKey(scope.tenant_id, key.id);
        setKeys((current) => current.filter((item) => item.id !== key.id));
      }
    } catch (actionError) { setError(actionError instanceof Error ? actionError.message : `The access key could not be ${action === 'revoke' ? 'revoked' : 'deleted'}.`); }
  }

  const isConfirming = (key: AccessKey, action: 'revoke' | 'delete') => confirming?.id === key.id && confirming.action === action;
  const where = scope ? `${scope.name} · ${scope.environment_name}` : '';
  const title = creating ? created ? 'Key created' : 'Create access key' : editingId ? 'Edit access key' : 'Access keys';
  const description = creating ? created ? 'Copy the key now. It is shown only once.' : 'Choose what the key may do and which websites may use it.' : editingId ? 'Change the name, scopes or websites. The secret stays the same.' : `Keys your backend and AI tools use to call MotifEngine in ${scope?.environment_name ?? 'this environment'}. Each key is shown once, when it is created.`;

  return <main className={`page page-resource ${open ? 'page-narrow' : ''}`}>
    <PageHeader breadcrumb={['Workspace', 'Access keys', ...(creating ? [created ? 'Created' : 'Create'] : editingId ? ['Edit'] : [])]} title={title} description={description} actions={open ? <button className="secondary-button" type="button" onClick={closeEditor}><IconArrowLeft size={15} stroke={1.75} />Back to keys</button> : <Link className="primary-button" to="/access-keys/new" disabled={!scope}><IconPlus size={15} stroke={1.75} />Create key</Link>} />

    {!open && <section className="panel" aria-label="Access keys">
      <div className="toolbar">
        <p className="tally"><strong>{keys.length}</strong> total <span aria-hidden="true">·</span> <strong>{activeCount}</strong> active</p>
        <div className="toolbar-controls"><button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh access keys"><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button></div>
      </div>
      {error && <div className="panel-body"><p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span><button className="text-button" type="button" onClick={() => void refresh()}>Retry</button></p></div>}
      {loading ? <div className="skeleton-rows" aria-label="Loading access keys"><span /><span /></div> : keys.length === 0 ? <div className="empty-state"><IconKey size={22} stroke={1.5} aria-hidden="true" /><strong>No access keys yet</strong><span>Create a key for your backend to send events and read user state, or one with the MCP scope to connect Claude Code, Cursor or VS Code.</span><Link className="primary-button" to="/access-keys/new" disabled={!scope}><IconPlus size={15} stroke={1.75} />Create key</Link></div> : <ul className="resource-list">{keys.map((key) => {
        const state = keyState(key);
        return <li key={key.id} className={`resource-row ${state.label === 'Active' ? '' : 'is-archived'}`}>
          <div className="resource-row-main"><strong>{key.name}</strong><span>{key.hint ? <span className="mono">{key.hint}</span> : 'Created before access keys'} · {originSummary(key)} · {key.last_used_at ? `Last used ${formatDateTime(key.last_used_at)}` : 'Never used'}{key.expires_at && !key.revoked_at ? ` · Expires ${formatDateTime(key.expires_at)}` : ''}</span><span className="token-badges">{key.scopes.map((item) => <span className="badge badge-outline" key={item}>{item}</span>)}</span></div>
          <span className={`badge ${state.tone}`}><span className="dot" />{state.label}</span>
          <div className="resource-row-actions" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) setConfirming((current) => current?.id === key.id ? null : current); }}>
            {state.label === 'Active' && <Link className="text-button" to="/access-keys/$id/edit" params={{ id: key.id }}>Edit</Link>}
            {state.label === 'Active' && <button className={`text-button ${isConfirming(key, 'revoke') ? 'danger' : ''}`} type="button" onClick={() => void act(key, 'revoke')} aria-label={isConfirming(key, 'revoke') ? `Confirm revoking ${key.name}` : `Revoke ${key.name}`}>{isConfirming(key, 'revoke') ? 'Confirm revoke' : 'Revoke'}</button>}
            <button className="text-button danger" type="button" onClick={() => void act(key, 'delete')} aria-label={isConfirming(key, 'delete') ? `Confirm deleting ${key.name}` : `Delete ${key.name}`}>{isConfirming(key, 'delete') ? 'Confirm delete' : 'Delete'}</button>
          </div>
        </li>;
      })}</ul>}
    </section>}

    {editingId && !editing && !loading && <section className="panel"><div className="empty-state"><IconKey size={22} stroke={1.5} aria-hidden="true" /><strong>This key does not exist</strong><span>It may have been deleted, or it belongs to another environment.</span><button className="secondary-button" type="button" onClick={closeEditor}>Back to keys</button></div></section>}

    {open && !created && (creating || (editing && formKeyId === editing.id)) && <section className="panel resource-editor" aria-label={editing ? 'Edit access key' : 'Create access key'}>
      <form className="resource-form" onSubmit={(event) => void submit(event)}>
        <div className="resource-form-fields">
          <label className="field field-wide"><span className="field-label">Name</span><input autoFocus required maxLength={80} value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="Checkout service" /><small className="field-help">Name it after the service or tool that will hold it, so the audit log stays readable.</small></label>
          <fieldset className="field field-wide"><legend className="field-label">Scopes</legend><ul className="option-list">{scopeOptions.map((option) => {
            const locked = option.scope === 'read' && form.scopes.includes('mcp');
            return <li key={option.scope}><label className={`option-row ${locked ? 'is-locked' : ''}`}><input type="checkbox" checked={form.scopes.includes(option.scope)} disabled={locked} onChange={() => toggleScope(option.scope)} /><span><strong>{option.label}</strong><small>{option.description}</small></span></label></li>;
          })}</ul><small className="field-help">No key can manage access keys, configure reward providers, or award points through the console API.</small></fieldset>
          <fieldset className="field field-wide"><legend className="field-label">Websites</legend><div className="segmented" role="group" aria-label="Browser access"><button type="button" aria-pressed={form.anyOrigin} onClick={() => setForm({ ...form, anyOrigin: true })}>Any website</button><button type="button" aria-pressed={!form.anyOrigin} onClick={() => setForm({ ...form, anyOrigin: false })}>Only these websites</button></div>
            {!form.anyOrigin && <textarea className="code-input" aria-label="Allowed websites" rows={3} value={form.origins} onChange={(event) => setForm({ ...form, origins: event.target.value })} placeholder={'https://shop.example.com\nhttps://admin.example.com'} />}
            <small className="field-help">{form.anyOrigin ? 'Browsers on any website may call the API with this key.' : 'One origin per line, such as https://shop.example.com. Browsers on other websites are refused.'} Requests from servers are not affected, so never put a key that can write in a web page.</small></fieldset>
          {creating && <div className="field"><span className="field-label">Expires</span><CustomSelect value={form.expiry} onChange={(value) => setForm({ ...form, expiry: value })} ariaLabel="Key expiry" options={expiryOptions} /></div>}
        </div>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        <div className="resource-form-footer"><span className="form-footer-note">{editing ? 'Key in' : 'Creating in'} <strong>{where}</strong></span><button className="secondary-button" type="button" onClick={closeEditor}>Cancel</button><button className="primary-button" type="submit" disabled={saving || !scope}>{saving ? 'Saving…' : editing ? 'Save changes' : 'Create key'}</button></div>
      </form>
    </section>}

    {creating && created && <KeyReveal created={created} onDone={closeEditor} />}
  </main>;
}

function KeyReveal({ created, onDone }: { created: { key: string; access_key: AccessKey }; onDone: () => void }) {
  const [client, setClient] = useState<Client>('Claude Code');
  const [copied, setCopied] = useState<'key' | 'snippet' | null>(null);
  const mcp = created.access_key.scopes.includes('mcp');
  const snippet = mcp ? connectSnippet(client, created.key) : `curl ${apiURL()}/v1/leaderboards \\\n  -H "X-Motif-API-Key: ${created.key}"`;

  async function copy(value: string, what: 'key' | 'snippet') {
    await navigator.clipboard?.writeText(value);
    setCopied(what);
    window.setTimeout(() => setCopied(null), 1600);
  }

  return <section className="panel resource-editor" aria-label="New access key">
    <div className="panel-body token-reveal">
      <div className="secret-field"><span className="field-label">{created.access_key.name}</span><div className="secret-value"><code>{created.key}</code><button className="icon-button" type="button" onClick={() => void copy(created.key, 'key')} aria-label={copied === 'key' ? 'Key copied' : 'Copy key'}>{copied === 'key' ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div><small className="field-help">Store it like a password, in your server's secret store. Anyone holding it can act within its scopes until you revoke or delete it.</small></div>
      <div className="field"><span className="field-label">{mcp ? 'Connect an MCP client' : 'Call the API'}</span><div className="connect-snippet"><div className="connect-snippet-head">{mcp ? <div className="segmented" role="group" aria-label="MCP client">{clients.map((item) => <button key={item} type="button" aria-pressed={client === item} onClick={() => setClient(item)}>{item}</button>)}</div> : <span className="field-help">Send the key in the X-Motif-API-Key header.</span>}<button className="icon-button" type="button" onClick={() => void copy(snippet, 'snippet')} aria-label={copied === 'snippet' ? 'Snippet copied' : 'Copy snippet'}>{copied === 'snippet' ? <IconCheck size={15} /> : <IconCopy size={15} />}</button></div><pre className="code-block">{snippet}</pre></div>{mcp && <small className="field-help">{snippetHint(client)} Other clients connect to <span className="mono">{mcpURL()}</span> with the key as a bearer header.</small>}</div>
    </div>
    <div className="resource-form-footer"><span className="form-footer-note">The key will not be shown again.</span><button className="primary-button" type="button" onClick={onDone}>Done</button></div>
  </section>;
}
