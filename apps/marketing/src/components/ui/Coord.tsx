'use client';

import { useEffect, useRef } from 'react';

/** A bracketed readout of where its section sits on the page, in CSS pixels. */
export function Coord({ side = 'left' }: { side?: 'left' | 'right' }) {
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const element = ref.current;
    const section = element?.parentElement;
    if (!element || !section) return;
    const measure = () => {
      const box = section.getBoundingClientRect();
      const x = Math.round(side === 'left' ? box.left : box.right);
      element.textContent = `[${x}, ${Math.round(box.top + window.scrollY)}]`;
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(document.body);
    return () => observer.disconnect();
  }, [side]);

  return <span ref={ref} className={`coord coord-${side} mono`} aria-hidden="true" />;
}
