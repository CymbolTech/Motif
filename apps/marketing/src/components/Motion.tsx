'use client';

import Lenis from 'lenis';
import { usePathname } from 'next/navigation';
import { useEffect, useRef } from 'react';

// Page motion: Lenis smooth scrolling, reveals for [data-reveal] blocks, and
// a slow parallax on the relief plates. Everything stays visible without it;
// the preflight script in the layout only hides reveal targets once it knows
// this component will run, and reduced motion skips all of it.
//
// This component lives in the root layout, which stays mounted across
// client-side navigations. Lenis runs once for the whole visit; reveals and
// plate tracking are rebuilt for every route, so a page reached through a
// link registers its own blocks instead of leaving them hidden.

declare global {
  interface Window {
    __motionReady?: boolean;
  }
}

export function Motion() {
  const pathname = usePathname();
  const lenisRef = useRef<Lenis | null>(null);
  const driftRef = useRef<() => void>(() => {});

  // Smooth scrolling, once per visit.
  useEffect(() => {
    window.__motionReady = true;
    if (!document.documentElement.classList.contains('motion')) return;

    const lenis = new Lenis({ lerp: 0.1, anchors: { offset: -24 } });
    lenisRef.current = lenis;
    let last = -1;
    let frame = requestAnimationFrame(function raf(time) {
      lenis.raf(time);
      if (lenis.animatedScroll !== last) {
        last = lenis.animatedScroll;
        driftRef.current();
      }
      frame = requestAnimationFrame(raf);
    });

    return () => {
      cancelAnimationFrame(frame);
      lenis.destroy();
      lenisRef.current = null;
    };
  }, []);

  // Reveals and plate drift, once per route.
  useEffect(() => {
    if (!document.documentElement.classList.contains('motion')) return;

    // The new page can be taller or shorter than the last one.
    lenisRef.current?.resize();

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
    document.querySelectorAll('[data-reveal]:not(.is-in)').forEach((element) => reveal.observe(element));

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
    driftRef.current = drift;
    drift();

    return () => {
      reveal.disconnect();
      watch.disconnect();
      driftRef.current = () => {};
    };
  }, [pathname]);

  return null;
}

/** Runs before first paint: opt in to motion, and give up if the client never starts. */
export const MOTION_PREFLIGHT = `try{if(!matchMedia('(prefers-reduced-motion: reduce)').matches){var d=document.documentElement;d.classList.add('motion');setTimeout(function(){if(!window.__motionReady)d.classList.remove('motion')},4000)}}catch(e){}`;
