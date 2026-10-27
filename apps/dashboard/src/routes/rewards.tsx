import { createFileRoute } from '@tanstack/react-router';
import { CatalogPage } from '../pages/catalog/ui/CatalogPage';

// The page stays mounted across its list, /new and /$id/edit child routes and reads which one matched.
export const Route = createFileRoute('/rewards')({ component: () => <CatalogPage kind="reward" /> });
