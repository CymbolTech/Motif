import { createFileRoute } from '@tanstack/react-router';
import { ConfigurePage } from '../pages/configure/ui/ConfigurePage';

// The page stays mounted across its list, /new and /$id/edit child routes and reads which one matched.
export const Route = createFileRoute('/events')({ component: () => <ConfigurePage area="events" /> });
