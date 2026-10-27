import { createFileRoute } from '@tanstack/react-router';
import { useNavigateToView } from '../app/navigation';
import { LoyaltyPage } from '../pages/loyalty/ui/LoyaltyPage';

export const Route = createFileRoute('/loyalty')({
  // ?user=sara keeps the inspected member across refreshes and shared links.
  validateSearch: (search: Record<string, unknown>): { user?: string } => ({ user: typeof search.user === 'string' && search.user.trim() ? search.user.trim() : undefined }),
  component: () => <LoyaltyPage onNavigate={useNavigateToView()} />,
});
