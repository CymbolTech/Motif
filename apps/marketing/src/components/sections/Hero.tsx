import Link from 'next/link';
import { LiquidCore } from '@/components/core/LiquidCore';
import { ConsoleLink } from '@/components/ui/ConsoleLink';
import { Coord } from '@/components/ui/Coord';
import { EventTrigger } from './EventTrigger';
import { ScrollCue } from './ScrollCue';

export function Hero() {
  return (
    <section className="hero" aria-labelledby="hero-title">
      <Coord />
      <Coord side="right" />
      <div className="hero-frame frame">
        <span className="ticks hero-ticks">
          <i />
        </span>
        <h1 id="hero-title" className="hero-title display">
          <span>Your product</span> <span>
            already has players<em>.</em>
          </span>
        </h1>

        <LiquidCore className="hero-core" label="The MotifEngine core: a lattice cage around a liquid heart. Each event you fire threads through the lattice into the heart, and each achievement makes the heart grow. Drag to spin it." />
        <p className="hero-core-label mono" aria-hidden="true">
          Core · drag to spin
        </p>

        <div className="hero-copy">
          <p className="hero-lede">
            MotifEngine turns what your users already do into XP, achievements, missions, points and rewards. Your backend sends events, your team publishes the rules, and every reward lands in a ledger that never counts twice.
          </p>
          <div className="hero-actions">
            <ConsoleLink className="btn btn-signal btn-big">Open the console</ConsoleLink>
            <Link className="btn btn-big" href="/docs">
              Read the docs
            </Link>
          </div>
          <dl className="telemetry mono">
            <div>
              <dt>Type</dt>
              <dd>Engagement and loyalty engine</dd>
            </div>
            <div>
              <dt>License</dt>
              <dd>Open source</dd>
            </div>
            <div>
              <dt>Integration</dt>
              <dd>REST · OpenAPI 3.0</dd>
            </div>
            <div>
              <dt>Scope</dt>
              <dd>Environment → user</dd>
            </div>
          </dl>
        </div>

        <EventTrigger />
        <ScrollCue />
      </div>
    </section>
  );
}
