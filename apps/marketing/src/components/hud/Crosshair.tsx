'use client';

import { useEffect, useRef } from 'react';

/** Light-gun crosshair: hairlines that follow a fine pointer with live page coordinates. */
export function Crosshair() {
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = root.current;
    if (!element || !window.matchMedia('(pointer: fine)').matches) return;
    const x = element.querySelector<HTMLElement>('.cross-x');
    const y = element.querySelector<HTMLElement>('.cross-y');
    const label = element.querySelector<HTMLElement>('.cross-label');
    let frame = 0;
    let pointer = { x: -100, y: -100 };

    const draw = () => {
      frame = 0;
      if (x) x.style.transform = `translate3d(0, ${pointer.y}px, 0)`;
      if (y) y.style.transform = `translate3d(${pointer.x}px, 0, 0)`;
      if (label) {
        const flipX = pointer.x > window.innerWidth - 150;
        const flipY = pointer.y > window.innerHeight - 60;
        label.style.transform = `translate3d(${pointer.x + (flipX ? -136 : 14)}px, ${pointer.y + (flipY ? -44 : 14)}px, 0)`;
        label.textContent = `X ${String(Math.round(pointer.x)).padStart(4, '0')} PX\nY ${String(Math.round(pointer.y + window.scrollY)).padStart(5, '0')} PX`;
      }
    };

    const onMove = (event: PointerEvent) => {
      if (event.pointerType !== 'mouse') return;
      pointer = { x: event.clientX, y: event.clientY };
      element.dataset.active = '1';
      if (!frame) frame = requestAnimationFrame(draw);
    };
    const onLeave = () => {
      delete element.dataset.active;
    };
    const onDown = (event: PointerEvent) => {
      if (event.pointerType !== 'mouse') return;
      const shot = document.createElement('span');
      shot.className = 'cross-shot';
      shot.style.left = `${event.clientX}px`;
      shot.style.top = `${event.clientY}px`;
      element.appendChild(shot);
      window.setTimeout(() => shot.remove(), 600);
    };

    window.addEventListener('pointermove', onMove, { passive: true });
    window.addEventListener('pointerdown', onDown, { passive: true });
    document.documentElement.addEventListener('pointerleave', onLeave);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerdown', onDown);
      document.documentElement.removeEventListener('pointerleave', onLeave);
    };
  }, []);

  return (
    <div className="crosshair" ref={root} aria-hidden="true">
      <span className="cross-x" />
      <span className="cross-y" />
      <span className="cross-label mono" />
    </div>
  );
}
