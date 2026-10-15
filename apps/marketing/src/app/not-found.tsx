import Link from 'next/link';
import './landing.css';

export default function NotFound() {
  return (
    <main id="main" className="frame not-found">
      <p className="mono">404 · event_type not registered</p>
      <h1 className="display not-found-title">Off the map.</h1>
      <p>This page does not exist. The rest of the run is still where you left it.</p>
      <Link className="btn btn-signal btn-big" href="/">
        Back to the start
      </Link>
    </main>
  );
}
