import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/leaderboards/$key')({
  // ?round=43 shows a finished round; without it the board shows its current round.
  validateSearch: (search: Record<string, unknown>): { round?: string } => ({ round: typeof search.round === 'string' && /^\d+$/.test(search.round) ? search.round : undefined }),
});
