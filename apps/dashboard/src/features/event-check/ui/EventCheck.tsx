import { useState } from 'react';
import { IconBolt, IconCheck, IconCopy, IconSend, IconTerminal2 } from '@tabler/icons-react';
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
      if (!scope) throw new Error('Select a tenant environment before sending a test event.');
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

  return (
    <section className="panel event-panel" aria-labelledby="event-check-title">
      <div className="panel-heading">
        <div>
          <span className="section-label">Live verification</span>
          <h2 id="event-check-title">Send a test event</h2>
        </div>
        <div className="live-mark"><span /> {scope?.environment_name ?? 'scope pending'}</div>
      </div>
      <p className="panel-intro">Run one real event through the receipt path and watch it move from accepted to processed.</p>
      <form className="event-form" onSubmit={handleSubmit}>
        <div className="form-grid">
          <label>
            <span>Event type</span>
            <input value={form.event_type} onChange={(event) => setForm({ ...form, event_type: event.target.value })} />
          </label>
          <label>
            <span>External user ID</span>
            <input value={form.user_id} onChange={(event) => setForm({ ...form, user_id: event.target.value })} />
          </label>
        </div>
        <label>
          <span>Payload</span>
          <div className="code-field">
            <IconTerminal2 size={15} stroke={1.5} />
            <textarea value={form.payload} onChange={(event) => setForm({ ...form, payload: event.target.value })} spellCheck={false} />
          </div>
        </label>
        <div className="form-actions">
          {error && <span className="form-error" role="alert">{error}</span>}
          <button className="primary-button" type="submit" disabled={sending}>
            <IconSend size={16} stroke={1.8} />
            {sending ? 'Sending…' : 'Send event'}
          </button>
        </div>
      </form>
      {receipt && (
        <div className="receipt" role="status">
          <div className="receipt-icon"><IconCheck size={16} stroke={2} /></div>
          <div>
            <strong>Receipt #{receipt.event_id}</strong>
            <span>{receipt.external_event_id}</span>
          </div>
          <div className="receipt-status"><IconBolt size={14} stroke={1.8} /> {receipt.status}</div>
          <button className="icon-button" type="button" onClick={copyReceipt} aria-label="Copy event ID">
            {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
          </button>
        </div>
      )}
    </section>
  );
}
