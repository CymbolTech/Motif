'use client';

import { useEffect, useRef, useState } from 'react';
import { emit } from '@/game/store';
import { Coord } from '@/components/ui/Coord';
import { FAQ } from '@/lib/content';

/** Answers that open like drawers: rows rise in on arrival, each answer slides open from its question. */
export function Faq() {
  const [open, setOpen] = useState<string | null>(null);
  const [arrived, setArrived] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    list.dataset.armed = '';
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) return;
        setArrived(true);
        observer.disconnect();
      },
      { threshold: 0.2 },
    );
    observer.observe(list);
    return () => observer.disconnect();
  }, []);

  const toggle = (id: string) => {
    const next = open === id ? null : id;
    setOpen(next);
    if (next) emit({ id: `faq-${id}`, type: 'faq.opened' });
  };

  return (
    <section className="faq section" id="faq" aria-labelledby="faq-title">
      <Coord />
      <div className="frame section-head">
        <h2 id="faq-title" className="section-title display">
          Straight answers.
        </h2>
      </div>
      <div className="frame">
        <div className="faq-list" ref={listRef} data-arrived={arrived || undefined}>
          {FAQ.map((item, index) => {
            const expanded = open === item.id;
            return (
              <div key={item.id} className="faq-item" data-open={expanded || undefined} style={{ ['--i' as string]: index }}>
                <h3 className="faq-question">
                  <button type="button" aria-expanded={expanded} aria-controls={`faq-answer-${item.id}`} id={`faq-q-${item.id}`} onClick={() => toggle(item.id)}>
                    <span className="faq-text">{item.question}</span>
                    <span className="faq-toggle" aria-hidden="true" />
                  </button>
                </h3>
                <div className="faq-answer" id={`faq-answer-${item.id}`} role="region" aria-labelledby={`faq-q-${item.id}`} inert={!expanded}>
                  <div className="faq-answer-inner">
                    <p>{item.answer}</p>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
