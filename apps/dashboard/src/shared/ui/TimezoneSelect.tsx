import { useEffect, useMemo, useRef, useState } from 'react';
import { IconCheck, IconChevronDown, IconSearch } from '@tabler/icons-react';

const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

function zones(): string[] {
  const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.('timeZone') ?? [];
  return Array.from(new Set(['UTC', ...supported]));
}

/** "GMT+3:30" for a zone right now, so people can recognise their zone by offset. */
function offsetLabel(zone: string) {
  try {
    const part = new Intl.DateTimeFormat('en-US', { timeZone: zone, timeZoneName: 'shortOffset' }).formatToParts(new Date()).find((item) => item.type === 'timeZoneName');
    return part?.value === 'GMT' ? 'GMT+0' : part?.value ?? '';
  } catch {
    return '';
  }
}

/** A searchable list of IANA time zones in the console's select style. */
export function TimezoneSelect({ value, onChange, ariaLabel = 'Time zone' }: { value: string; onChange: (zone: string) => void; ariaLabel?: string }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const all = useMemo(() => zones().map((zone) => ({ zone, offset: offsetLabel(zone) })), []);
  const visible = useMemo(() => {
    const needle = query.trim().toLowerCase().replace(/\s+/g, '_');
    if (!needle) return [...all.filter((item) => item.zone === browserZone), ...all.filter((item) => item.zone !== browserZone)];
    // Cities that start with the search come first: "teh" finds Tehran before Whitehorse.
    const score = (item: { zone: string; offset: string }) => {
      const zone = item.zone.toLowerCase();
      if (zone.split('/').some((part) => part.startsWith(needle))) return 0;
      return zone.includes(needle) || item.offset.toLowerCase().includes(needle) ? 1 : -1;
    };
    return all.map((item) => ({ item, rank: score(item) })).filter(({ rank }) => rank >= 0).sort((a, b) => a.rank - b.rank).map(({ item }) => item);
  }, [all, query]);

  useEffect(() => {
    function closeOnOutsidePointer(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    return () => document.removeEventListener('pointerdown', closeOnOutsidePointer);
  }, []);
  useEffect(() => {
    if (!open) return;
    setQuery('');
    requestAnimationFrame(() => searchRef.current?.focus());
  }, [open]);

  function choose(zone: string) {
    onChange(zone);
    setOpen(false);
    requestAnimationFrame(() => triggerRef.current?.focus());
  }

  return <div className={`custom-select ${open ? 'is-open' : ''}`} ref={rootRef}>
    <button ref={triggerRef} className="custom-select-trigger" type="button" aria-label={ariaLabel} aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((current) => !current)} onKeyDown={(event) => { if (event.key === 'ArrowDown') { event.preventDefault(); setOpen(true); } }}>
      <span className="timezone-value">{value.replace(/_/g, ' ')}<small>{offsetLabel(value)}</small></span>
      <IconChevronDown size={15} aria-hidden="true" />
    </button>
    {open && <div className="custom-select-menu timezone-menu">
      <label className="search-field timezone-search"><IconSearch size={14} stroke={1.75} aria-hidden="true" /><input ref={searchRef} type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search city or offset" aria-label="Search time zones" onKeyDown={(event) => { if (event.key === 'Escape') setOpen(false); if (event.key === 'Enter' && visible[0]) { event.preventDefault(); choose(visible[0].zone); } }} /></label>
      <div role="listbox" aria-label={ariaLabel}>
        {visible.length === 0 ? <p className="timezone-empty">No matching time zone</p> : visible.map((item) => <button key={item.zone} className={`custom-select-option ${item.zone === value ? 'selected' : ''}`} type="button" role="option" aria-selected={item.zone === value} onClick={() => choose(item.zone)}>
          <span>{item.zone.replace(/_/g, ' ')}{item.zone === browserZone && <em> · your time zone</em>}</span>
          <span className="timezone-offset">{item.offset}{item.zone === value && <IconCheck size={14} aria-hidden="true" />}</span>
        </button>)}
      </div>
    </div>}
  </div>;
}
