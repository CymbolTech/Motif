'use client';

import UseAnimations from 'react-useanimations';
import scrollDown from 'react-useanimations/lib/scrollDown';

export function ScrollCue() {
  return (
    <a className="hero-scroll mono" href="#loop">
      <UseAnimations animation={scrollDown} size={22} />
      [Scroll to continue]
    </a>
  );
}
