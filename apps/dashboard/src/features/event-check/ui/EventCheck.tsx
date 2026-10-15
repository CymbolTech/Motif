import { useState } from 'react';
import { IconAlertTriangle, IconCheck, IconCopy, IconSend } from '@tabler/icons-react';
import { fetchScopedEventStatus, submitAdminTestEvent, type EventReceipt } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

type EventFormState = {
  event_type: string;
  user_id: string;
  payload: string;
};

const initialForm: EventFormState = {
  event_type: 'console.healthcheck',
  user_id: 'console-check',
  payload: '{\n  "source": "motif-console",\n  "intent": "verify-pipeline"\n}',
};

export function EventCheck() {
  const { scope } = useScope();
  const [form, setForm] = useState<EventFormState>(initialForm);
  const [receipt, setReceipt] = useState<EventReceipt | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [copied, setCopied] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setReceipt(null);
    setSending(true);
    try {
      const payload = JSON.parse(form.payload) as Record<string, unknown>;
      if (!payload || Array.isArray(payload) || typeof payload !== 'object') throw new Error('Payload must be a JSON object.');
      if (!scope) throw new Error('Your workspace membership is still loading.');
      const externalEventID = `console-${Date.now()}`;
      const nextReceipt = await submitAdminTestEvent({
        tenant_id: scope.tenant_id,
        environment_id: scope.environment_id,
        external_event_id: externalEventID,
        event_type: form.event_type,
        user_id: form.user_id,
        payload,
      });
      setReceipt(nextReceipt);
      void (async () => {
        for (let attempt = 0; attempt < 6; attempt += 1) {
          await new Promise((resolve) => window.setTimeout(resolve, 350));
          try {
            const status = await fetchScopedEventStatus(nextReceipt.external_event_id, { tenant_id: scope.tenant_id, environment_id: scope.environment_id });
            setReceipt(status);
            if (status.status === 'processed' || status.status === 'failed') break;
          } catch {
            // The accepted receipt remains visible while the worker catches up.
          }
        }
      })();
    } catch (submissionError) {
      setError(submissionError instanceof SyntaxError ? 'Payload must be valid JSON.' : (submissionError as Error).message);
    } finally {
      setSending(false);
    }
  }

  async function copyReceipt() {
    if (!receipt) return;
    await navigator.clipboard?.writeText(receipt.external_event_id);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  const receiptTone = receipt?.status === 'processed' ? 'success' : receipt?.status === 'failed' ? 'danger' : 'warning';

  return (
    <section id="event-check" className="panel event-check" aria-labelledby="event-check-title">
      <div className="panel-header">
        <div>
          <h2 id="event-check-title">Send a test event</h2>
          <p>{scope ? 'Send one event through receive → process and confirm the worker picks it up.' : 'Your workspace membership is loading; verification will be available when it is connected.'}</p>
        </div>
      </div>
      <form className="panel-body form-stack" onSubmit={handleSubmit}>
        <div className="form-row">
          <label className="field">
            <span className="field-label">Event type</span>
            <input className="mono" value={form.event_type} onChange={(event) => setForm({ ...form, event_type: event.target.value })} required spellCheck={false} />
          </label>
          <label className="field">
            <span className="field-label">External user ID</span>
            <input className="mono" value={form.user_id} onChange={(event) => setForm({ ...form, user_id: event.target.value })} required spellCheck={false} />
          </label>
        </div>
        <label className="field">
          <span className="field-label">Payload <em>JSON object</em></span>
          <textarea className="code-input" rows={4} value={form.payload} onChange={(event) => setForm({ ...form, payload: event.target.value })} spellCheck={false} required />
        </label>
        {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span>{error}</span></p>}
        {receipt && (
          <div className="receipt" role="status">
            <span className={`badge capitalize badge-${receiptTone}`}><span className="dot" />{receipt.status}</span>
            <span className="receipt-copy"><strong>Receipt #{receipt.event_id}</strong><code>{receipt.external_event_id}</code></span>
            <button className="icon-button ghost" type="button" onClick={copyReceipt} aria-label={copied ? 'Event ID copied' : 'Copy event ID'}>
              {copied ? <IconCheck size={15} /> : <IconCopy size={15} />}
            </button>
          </div>
        )}
        <div className="form-footer">
          <span className="form-footer-note">{scope ? `Sends to ${scope.environment_name}` : 'No environment connected'}</span>
          <button className="primary-button" type="submit" disabled={sending || !scope}>
            <IconSend size={15} stroke={1.75} />
            {sending ? 'Sending…' : scope ? 'Send event' : 'Waiting for workspace'}
          </button>
        </div>
      </form>
    </section>
  );
}
