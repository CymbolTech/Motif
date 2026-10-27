import { createFileRoute } from '@tanstack/react-router';
import { AnalyzePage } from '../pages/analyze/ui/AnalyzePage';

export const Route = createFileRoute('/reports')({ component: AnalyzePage });
