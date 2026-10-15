'use client';

import Lenis from 'lenis';
import 'lenis/dist/lenis.css';
import { useEffect } from 'react';

/** Inertial page scrolling. Native scrolling stays in charge under reduced motion. */
export function SmoothScroll() {
  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const lenis = new Lenis({
      autoRaf: true,
      lerp: 0.09,
      wheelMultiplier: 0.95,
      // Lenis honours the page's scroll-padding-top, which already clears the HUD.
      anchors: true,
    });
    return () => lenis.destroy();
  }, []);
  return null;
}
