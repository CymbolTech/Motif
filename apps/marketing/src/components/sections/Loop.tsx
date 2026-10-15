'use client';

import { useEffect, useRef, useState } from 'react';
import { emit } from '@/game/store';
import { Coord } from '@/components/ui/Coord';
import { LOOP_STEPS } from '@/lib/content';

/** One event, five moves: the giant word holds while each move scrolls past and reports itself. */
export function Loop() {
  const [active, setActive] = useState(0);
  const stepsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = stepsRef.current;
    if (!container) return;
    const steps = Array.from(container.querySelectorAll<HTMLElement>('[data-step]'));
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          const index = Number((entry.target as HTMLElement).dataset.index);
          setActive(index);
          emit({ id: `loop-${LOOP_STEPS[index].id}`, type: 'loop.step_viewed', payload: { step: LOOP_STEPS[index].id } });
        });
      },
      { rootMargin: '-45% 0px -45% 0px' },
    );
    steps.forEach((step) => observer.observe(step));
    return () => observer.disconnect();
  }, []);

  return (
    <section className="loop section" id="loop" aria-labelledby="loop-title">
      <Coord />
      <div className="frame section-head">
        <h2 id="loop-title" className="section-title display">
          One event. Five moves.
        </h2>
        <p className="section-lede">Follow a single order from your backend to a reward the customer can hold. Each move is a real part of the engine, shown with the shape it has in the API.</p>
      </div>

      <div className="frame loop-body">
        <div className="loop-sticky" aria-hidden="true">
          <p className="loop-count mono">
            Move {String(active + 1).padStart(2, '0')}/{String(LOOP_STEPS.length).padStart(2, '0')}
          </p>
          <div className="loop-words">
            {LOOP_STEPS.map((step, index) => (
              <span key={step.id} className="loop-word display" data-state={index === active ? 'on' : index < active ? 'past' : 'next'}>
                {step.word}
              </span>
            ))}
          </div>
          <ol className="loop-rail mono">
            {LOOP_STEPS.map((step, index) => (
              <li key={step.id} data-state={index === active ? 'on' : index < active ? 'past' : undefined}>
                {step.word}
              </li>
            ))}
          </ol>
        </div>

        <div className="loop-steps" ref={stepsRef}>
          {LOOP_STEPS.map((step, index) => (
            <article key={step.id} className="loop-step" data-step={step.id} data-index={index} data-active={index === active || undefined}>
              <h3 className="loop-step-title">{step.title}</h3>
              <p className="loop-step-body">{step.body}</p>
              <figure className="specimen specimen-framed">
                <span className="ticks">
                  <i />
                </span>
                <figcaption className="specimen-head mono">
                  <span>Shape in the API</span>
                  <span>{step.endpoint}</span>
                </figcaption>
                <pre>{step.specimen}</pre>
              </figure>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
