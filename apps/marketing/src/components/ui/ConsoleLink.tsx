'use client';

import type { ReactNode } from 'react';
import UseAnimations from 'react-useanimations';
import arrowUpRight from 'react-useanimations/lib/arrowUpRight';
import { emit } from '@/game/store';
import { DASHBOARD_URL } from '@/lib/content';

type ConsoleLinkProps = { className?: string; children: ReactNode; iconSize?: number };

/** The primary action everywhere: opens the console in a new tab so the run survives. */
export function ConsoleLink({ className = 'btn btn-signal', children, iconSize = 18 }: ConsoleLinkProps) {
  return (
    <UseAnimations
      animation={arrowUpRight}
      size={iconSize}
      render={(eventProps, animationProps) => (
        <a
          {...eventProps}
          className={className}
          href={DASHBOARD_URL}
          target="_blank"
          rel="noopener"
          onClick={(event) => {
            eventProps.onClick(event);
            emit({ id: 'console-opened', type: 'console.opened' });
          }}
        >
          {children}
          <span {...animationProps} />
          <span className="visually-hidden">(opens in a new tab)</span>
        </a>
      )}
    />
  );
}
