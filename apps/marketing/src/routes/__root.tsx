import type { ReactNode } from 'react';
import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import '../styles.css';

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'MotifEngine — Open-source engagement engine' },
      { name: 'description', content: 'An open-source engine for events, rules, progression, points, and rewards behind one dependable API.' },
    ],
  }),
  component: RootDocument,
});

function RootDocument() {
  return <Document><Outlet /></Document>;
}

function Document({ children }: Readonly<{ children: ReactNode }>) {
  return <html lang="en"><head><HeadContent /></head><body>{children}<Scripts /></body></html>;
}
