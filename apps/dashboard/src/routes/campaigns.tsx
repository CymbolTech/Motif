import { createFileRoute } from '@tanstack/react-router';
import { EngagementPage } from '../pages/engagement/ui/EngagementPage';

// The page stays mounted across its list, /new and /$id/edit child routes and reads which one matched.
export const Route = createFileRoute('/campaigns')({ component: () => <EngagementPage kind="campaign" /> });
