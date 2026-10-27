'use client';

import Lenis from 'lenis';
import { useEffect } from 'react';

// Page motion: Lenis smooth scrolling, reveals for [data-reveal] blocks, and
// a slow parallax on the relief plates. Everything stays visible without it;
// the preflight script in the layout only hides reveal targets once it knows
// this component will run, and reduced motion skips all of it.

declare global {
  interface Window {
    __motionReady?: boolean;
  }
}

export function Motion() {
  useEffect(() => {
    const root = document.documentElement;
    window.__motionReady = true;
    if (!root.classList.contains('motion')) return;

    const lenis = new Lenis({ lerp: 0.1, anchors: { offset: -24 } });

    // Relief plates drift against the scroll. Plates are tracked from well
    // before they enter the viewport, so the first frame they are visible
    // already carries the right offset; reads and writes are batched.
    const plates = new Set<HTMLElement>();
    const drift = () => {
      const view = window.innerHeight;
      const values: [HTMLElement, string][] = [];
      for (const plate of plates) {
        const box = plate.getBoundingClientRect();
        const progress = (box.top + box.height / 2 - view / 2) / (view / 2 + box.height / 2);
        values.push([plate, Math.max(-1, Math.min(1, progress)).toFixed(4)]);
      }
      for (const [plate, value] of values) plate.style.setProperty('--drift', value);
    };
    const watch = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) plates.add(entry.target as HTMLElement);
          else plates.delete(entry.target as HTMLElement);
        }
        drift();
      },
      { rootMargin: '50% 0px' },
    );
    document.querySelectorAll<HTMLElement>('.plate').forEach((plate) => {
      plates.add(plate);
      watch.observe(plate);
    });
    drift();

    let last = -1;
    let frame = requestAnimationFrame(function raf(time) {
      lenis.raf(time);
      if (lenis.animatedScroll !== last) {
        last = lenis.animatedScroll;
        drift();
      }
      frame = requestAnimationFrame(raf);
    });

    const reveal = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          entry.target.classList.add('is-in');
          reveal.unobserve(entry.target);
        }
      },
      { rootMargin: '0px 0px -12% 0px' },
    );
    document.querySelectorAll('[data-reveal]').forEach((element) => reveal.observe(element));

    return () => {
      cancelAnimationFrame(frame);
      reveal.disconnect();
      watch.disconnect();
      lenis.destroy();
    };
  }, []);

  return null;
}

/** Runs before first paint: opt in to motion, and give up if the client never starts. */
export const MOTION_PREFLIGHT = `try{if(!matchMedia('(prefers-reduced-motion: reduce)').matches){var d=document.documentElement;d.classList.add('motion');setTimeout(function(){if(!window.__motionReady)d.classList.remove('motion')},4000)}}catch(e){}`;
