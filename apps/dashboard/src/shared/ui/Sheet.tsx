import { useEffect, useRef, type ReactNode } from 'react';
import { IconX } from '@tabler/icons-react';

type SheetProps = {
  labelledBy: string;
  title: ReactNode;
  description?: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  size?: 'default' | 'wide';
};

/** A right-hand sheet for work that belongs beside the current task rather than replacing it. */
export function Sheet({ labelledBy, title, description, onClose, children, footer, size = 'default' }: SheetProps) {
  const panelRef = useRef<HTMLElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const previousFocus = document.activeElement as HTMLElement | null;
    // Defer one frame: a select that opened this sheet returns focus to its own trigger on the
    // current frame, and the sheet must win that race.
    const focusFrame = requestAnimationFrame(() => {
      const firstField = panelRef.current?.querySelector<HTMLElement>('input:not([disabled]), textarea, button.custom-select-trigger');
      (firstField ?? panelRef.current)?.focus();
    });
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !document.querySelector('.custom-select.is-open')) closeRef.current();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      cancelAnimationFrame(focusFrame);
      window.removeEventListener('keydown', closeOnEscape);
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus?.();
    };
  }, []);

  return (
    <div className="sheet-layer" role="presentation">
      <button className="sheet-scrim" type="button" tabIndex={-1} aria-label="Close" onClick={onClose} />
      <section ref={panelRef} className={`sheet ${size === 'wide' ? 'sheet-wide' : ''}`} role="dialog" aria-modal="true" aria-labelledby={labelledBy} tabIndex={-1}>
        <header className="sheet-header">
          <div>
            <h2 id={labelledBy}>{title}</h2>
            {description && <p>{description}</p>}
          </div>
          <button className="icon-button ghost" type="button" onClick={onClose} aria-label="Close"><IconX size={16} /></button>
        </header>
        <div className="sheet-body">{children}</div>
        {footer && <footer className="sheet-footer">{footer}</footer>}
      </section>
    </div>
  );
}
