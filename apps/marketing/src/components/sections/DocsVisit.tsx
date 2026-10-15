'use client';

import { useEffect } from 'react';
import { emit } from '@/game/store';

export function DocsVisit() {
  useEffect(() => {
    emit({ id: 'docs-opened', type: 'docs.opened' });
  }, []);
  return null;
}
