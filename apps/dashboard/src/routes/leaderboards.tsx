import { createFileRoute } from '@tanstack/react-router';
import { LeaderboardsPage } from '../pages/leaderboards/ui/LeaderboardsPage';

// The page stays mounted across the list, /new, /$key and /$key/edit and reads which one matched.
export const Route = createFileRoute('/leaderboards')({ component: LeaderboardsPage });
