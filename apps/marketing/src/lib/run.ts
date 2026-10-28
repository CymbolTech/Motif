'use client';

// The visitor's run: this site uses the same mechanics Motif sells. Reading a
// section, exploring a building block or opening a question is an event; a
// rule awards XP for each, once; achievements unlock at a count. State lives
// in this browser only (localStorage) and never leaves it.

export type AchievementId = 'first-steps' | 'explorer' | 'curious' | 'whole-pitch' | 'went-deeper' | 'pressed-start';

export const ACHIEVEMENTS: Record<AchievementId, { title: string; hint: string }> = {
  'first-steps': { title: 'First steps', hint: 'Earn your first XP' },
  explorer: { title: 'Explorer', hint: 'Look at all ten building blocks' },
  curious: { title: 'Curious', hint: 'Open three questions' },
  'whole-pitch': { title: 'Read the whole pitch', hint: 'Reach the end of the page' },
  'went-deeper': { title: 'Went deeper', hint: 'Open the docs' },
  'pressed-start': { title: 'Pressed start', hint: 'Start building' },
};

export const LEVELS = [
  { at: 0, name: 'Visitor' },
  { at: 75, name: 'Regular' },
  { at: 200, name: 'Builder' },
  { at: 350, name: 'Champion' },
];

export type Run = {
  xp: number;
  /** Event ids already rewarded, so every event pays once. */
  events: string[];
  unlocked: AchievementId[];
};

export type Award = { key: number; xp: number; achievement?: AchievementId };

const KEY = 'motif.run.v1';
const EMPTY: Run = { xp: 0, events: [], unlocked: [] };

let run: Run = EMPTY;
let loaded = false;
let awardKey = 0;
const listeners = new Set<() => void>();
const awardListeners = new Set<(award: Award) => void>();

function load() {
  if (loaded || typeof window === 'undefined') return;
  loaded = true;
  try {
    const stored = JSON.parse(localStorage.getItem(KEY) || 'null');
    if (stored && typeof stored.xp === 'number' && Array.isArray(stored.events) && Array.isArray(stored.unlocked)) run = stored;
  } catch {
    // Private windows and blocked storage start a fresh run.
  }
}

function save() {
  try {
    localStorage.setItem(KEY, JSON.stringify(run));
  } catch {
    // The run still works for this page view.
  }
}

function emit(award: Award) {
  for (const listener of listeners) listener();
  for (const listener of awardListeners) listener(award);
}

export function getRun(): Run {
  load();
  return run;
}

export const getServerRun = (): Run => EMPTY;

export function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function onAward(listener: (award: Award) => void) {
  awardListeners.add(listener);
  return () => {
    awardListeners.delete(listener);
  };
}

export function level(xp: number) {
  let index = 0;
  LEVELS.forEach((step, i) => {
    if (xp >= step.at) index = i;
  });
  const current = LEVELS[index];
  const next = LEVELS[index + 1];
  return {
    number: index + 1,
    name: current.name,
    progress: next ? (xp - current.at) / (next.at - current.at) : 1,
    next: next?.at,
  };
}

export function unlock(id: AchievementId) {
  load();
  if (run.unlocked.includes(id)) return;
  run = { ...run, unlocked: [...run.unlocked, id] };
  save();
  emit({ key: ++awardKey, xp: 0, achievement: id });
}

/** Rewards an event once. Returns whether it paid out. */
export function earn(eventId: string, xp: number) {
  load();
  if (run.events.includes(eventId)) return false;
  run = { ...run, xp: run.xp + xp, events: [...run.events, eventId] };
  save();
  emit({ key: ++awardKey, xp });
  if (!run.unlocked.includes('first-steps')) unlock('first-steps');
  return true;
}

export function countEvents(prefix: string) {
  return getRun().events.filter((id) => id.startsWith(prefix)).length;
}
