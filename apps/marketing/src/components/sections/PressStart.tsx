import Link from 'next/link';
import { ConsoleLink } from '@/components/ui/ConsoleLink';
import { Coord } from '@/components/ui/Coord';
import { RunSummary } from './RunSummary';

export function PressStart() {
  return (
    <section className="press-start" aria-labelledby="press-start-title">
      <Coord />
      <div className="frame press-start-frame">
        <h2 id="press-start-title" className="press-start-title display">
          <span>Press</span> <span>start.</span>
        </h2>
        <div className="press-start-copy">
          <p>Create a workspace, register your first event, publish a rule. Your players are already waiting.</p>
          <div className="press-start-actions">
            <ConsoleLink className="btn btn-big btn-reward">Open the console</ConsoleLink>
            <Link className="btn btn-big btn-line-ink" href="/docs">
              Read the docs
            </Link>
          </div>
          <p className="mono press-start-local">
            Or run the whole stack locally: <code>make dev-docker</code>
          </p>
        </div>
        <RunSummary />
      </div>
    </section>
  );
}
