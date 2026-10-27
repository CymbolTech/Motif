import { useNavigate } from '@tanstack/react-router';
import { viewPaths, type View } from './views';

/** Navigates to a section by its view name, for pages that link across sections. */
export function useNavigateToView() {
  const navigate = useNavigate();
  return (view: View) => void navigate({ to: viewPaths[view] });
}
