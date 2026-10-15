import { Coord } from '@/components/ui/Coord';
import { HiddenTarget } from '@/components/ui/HiddenTarget';
import { CAPABILITIES } from '@/lib/content';

export function Capabilities() {
  return (
    <section className="capabilities section" id="engine" aria-labelledby="engine-title">
      <Coord />
      <div className="frame section-head">
        <h2 id="engine-title" className="section-title display">
          Everything in the engine.
        </h2>
        <p className="section-lede">Ten parts, one contract. Each row names the endpoint where it lives, so your engineers can check every word.</p>
      </div>
      <div className="frame">
        <ul className="cap-list">
          {CAPABILITIES.map((capability, index) => (
            <li key={capability.name}>
              <details>
                <summary>
                  <span className="cap-name display">{capability.name}</span>
                  <span className="cap-endpoint mono">[{capability.endpoint}]</span>
                  <span className="cap-toggle" aria-hidden="true" />
                </summary>
                <p className="cap-body">{capability.body}</p>
              </details>
              {index === 6 && <HiddenTarget id="bravo" className="target-cap" />}
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
