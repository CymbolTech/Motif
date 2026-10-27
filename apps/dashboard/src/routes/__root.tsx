import { createRootRouteWithContext, Navigate } from '@tanstack/react-router';
import { ConsoleShell } from '../app/ConsoleShell';
import type { AdminPrincipal } from '../shared/api/client';

export type ShellContext = { principal: AdminPrincipal; signOut: () => void };

export const Route = createRootRouteWithContext<ShellContext>()({
  component: ConsoleShell,
  // Unknown paths land on the overview instead of an empty page.
  notFoundComponent: () => <Navigate to="/" replace />,
});
