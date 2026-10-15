import { createContext, Fragment, useContext, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

type PageHeaderProps = {
  breadcrumb: string[];
  /** Page-level status and refresh controls, rendered in the shell's top bar. */
  topbarActions?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
};

// The shell owns the top bar; each page hands its breadcrumb to it through this slot
// so the trail sits beside the environment chip instead of above the page title.
export type TopbarSlots = { crumbs: HTMLElement | null; actions: HTMLElement | null };
const TopbarSlotContext = createContext<TopbarSlots>({ crumbs: null, actions: null });
export const TopbarSlotProvider = TopbarSlotContext.Provider;

export function PageHeader({ breadcrumb, topbarActions, title, description, actions, className = '' }: PageHeaderProps) {
  const slots = useContext(TopbarSlotContext);
  const trail = <ol className="breadcrumb">{breadcrumb.map((item, index) => <Fragment key={`${item}-${index}`}><li aria-current={index === breadcrumb.length - 1 ? 'page' : undefined}>{item}</li></Fragment>)}</ol>;

  return (
    <>
      {slots.crumbs && createPortal(trail, slots.crumbs)}
      {slots.actions && topbarActions && createPortal(topbarActions, slots.actions)}
      <header className={`page-header ${className}`.trim()}>
        <div className="page-header-copy">
          <h1>{title}</h1>
          {description && <p>{description}</p>}
        </div>
        {actions && <div className="page-header-actions">{actions}</div>}
      </header>
    </>
  );
}
