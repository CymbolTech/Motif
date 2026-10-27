import { createFileRoute } from '@tanstack/react-router';
import { useNavigateToView } from '../app/navigation';
import { OverviewPage } from '../pages/overview/ui/OverviewPage';

export const Route = createFileRoute('/')({ component: Overview });

function Overview() {
  const { principal } = Route.useRouteContext();
  return <OverviewPage principal={principal} onNavigate={useNavigateToView()} />;
}
