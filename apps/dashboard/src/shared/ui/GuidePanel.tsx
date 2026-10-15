import { useEffect, useRef, useState } from 'react';
import { IconChevronDown, IconHelpCircle } from '@tabler/icons-react';

type GuidePanelProps = {
  id: string;
  title: string;
  purpose: string;
  steps: readonly string[];
  example: string;
  result: string;
  /** Opens the guide on its own, e.g. once a list has loaded empty. The reader's own toggle always wins. */
  defaultOpen?: boolean;
};

export function GuidePanel({ id, title, purpose, steps, example, result, defaultOpen = false }: GuidePanelProps) {
  const [open, setOpen] = useState(defaultOpen);
  const touched = useRef(false);

  useEffect(() => {
    if (!touched.current) setOpen(defaultOpen);
  }, [defaultOpen]);

  return (
    <section className={`guide ${open ? 'is-open' : ''}`} aria-labelledby={`${id}-title`}>
      <button className="guide-toggle" type="button" aria-expanded={open} aria-controls={`${id}-body`} onClick={() => { touched.current = true; setOpen((current) => !current); }}>
        <IconHelpCircle size={16} stroke={1.75} aria-hidden="true" />
        <span id={`${id}-title`}>How {title.toLowerCase()} work</span>
        <span className="guide-toggle-hint">{purpose}</span>
        <IconChevronDown className="guide-chevron" size={16} stroke={1.75} aria-hidden="true" />
      </button>
      {open && (
        <div className="guide-body" id={`${id}-body`}>
          <div className="guide-block">
            <h3>Use it for</h3>
            <p>{purpose}</p>
          </div>
          <div className="guide-block">
            <h3>Set it up</h3>
            <ol>
              {steps.map((step) => <li key={step}>{step}</li>)}
            </ol>
          </div>
          <div className="guide-block guide-example">
            <h3>Example</h3>
            <p>{example}</p>
            <p className="guide-result"><span>Result</span>{result}</p>
          </div>
        </div>
      )}
    </section>
  );
}
