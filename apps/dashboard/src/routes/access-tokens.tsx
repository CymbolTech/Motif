import { createFileRoute } from '@tanstack/react-router';
import { AccessTokensPage } from '../pages/access-tokens/ui/AccessTokensPage';

// The page stays mounted across the list and /new and reads which one matched.
export const Route = createFileRoute('/access-tokens')({ component: AccessTokensPage });
