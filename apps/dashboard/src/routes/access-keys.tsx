import { createFileRoute } from '@tanstack/react-router';
import { AccessKeysPage } from '../pages/access-keys/ui/AccessKeysPage';

// The page stays mounted across the list, /new and /$id/edit and reads which one matched.
export const Route = createFileRoute('/access-keys')({ component: AccessKeysPage });
