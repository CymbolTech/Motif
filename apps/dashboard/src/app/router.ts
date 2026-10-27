import { createRouter } from '@tanstack/react-router';
import { routeTree } from '../routeTree.gen';

// Plain query strings (?round=43, ?user=sara) instead of the router's JSON encoding.
function parseSearch(search: string) {
  return Object.fromEntries(new URLSearchParams(search));
}

function stringifySearch(search: Record<string, unknown>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(search)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value));
  }
  const query = params.toString();
  return query ? `?${query}` : '';
}

export const router = createRouter({
  routeTree,
  // The signed-in principal is supplied by <RouterProvider context>.
  context: { principal: undefined!, signOut: undefined! },
  parseSearch,
  stringifySearch,
  scrollRestoration: true,
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
