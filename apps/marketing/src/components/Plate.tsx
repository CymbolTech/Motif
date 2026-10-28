import type { ReactNode } from 'react';

type PlateProps = {
  /** Relief image under public/relief, without extension. */
  relief: 'hero' | 'track' | 'decide' | 'agents';
  className?: string;
  priority?: boolean;
  children: ReactNode;
};

/** A greyscale relief plate with film grain that carries one console card. */
export function Plate({ relief, className = '', priority = false, children }: PlateProps) {
  return (
    <div className={`plate ${className}`} data-reveal={priority ? undefined : ''}>
      <img
        className="plate-relief"
        src={`/relief/${relief}.webp`}
        alt=""
        width={relief === 'hero' ? 1200 : 1800}
        height={relief === 'hero' ? 800 : 680}
        loading={priority ? 'eager' : 'lazy'}
        decoding="async"
        fetchPriority={priority ? 'high' : 'auto'}
      />
      <div className="plate-body">{children}</div>
    </div>
  );
}
