import { Coord } from '@/components/ui/Coord';
import { HiddenTarget } from '@/components/ui/HiddenTarget';

export function Manifesto() {
  return (
    <section className="manifesto" aria-label="Why MotifEngine">
      <Coord />
      <div className="frame grid">
        <p className="manifesto-text">
          Engagement logic usually hides in cron jobs, spreadsheets and one-off scripts. MotifEngine gives it one home:{' '}
          <mark>trusted events</mark> in, <mark>published rules</mark> in the middle, <mark>ledgered rewards</mark> out.
          <HiddenTarget id="alpha" className="target-inline" />
        </p>
        <ul className="manifesto-facts mono">
          <li>One receipt per event id</li>
          <li>One effect per rule version and action</li>
          <li>One append-only ledger per currency</li>
        </ul>
      </div>
    </section>
  );
}
