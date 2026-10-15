import { MeasurementPanel } from '../../../features/measurement/ui/MeasurementPanel';
import { useScope } from '../../../shared/scope/ScopeContext';
import { PageHeader } from '../../../shared/ui/PageHeader';

export function AnalyzePage() {
  const { scope } = useScope();

  return <main className="page page-reports">
    <PageHeader breadcrumb={['Workspace', 'Reports']} title="Reports" description="A shared view of what members are doing, earning, and completing." />
    <MeasurementPanel scope={scope} />
  </main>;
}
