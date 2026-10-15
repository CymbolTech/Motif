'use client';

import { useSyncExternalStore } from 'react';
import { ACHIEVEMENTS, type Achievement, type Level } from './catalog';
import {
  createRun,
  isRunState,
  processEvent,
  remainingForCompletion,
  type Effect,
  type IncomingEvent,
  type ProcessResult,
  type RunState,
} from './engine';

export type Toast =
  | { id: string; kind: 'achievement'; achievement: Achievement; effects: Effect[]; receipt?: string; ordinal: number }
  | { id: string; kind: 'level'; level: Level };

export type Snapshot = {
  run: RunState;
  toasts: Toast[];
  /** Increments on every accepted event; the liquid core listens to it. */
  pulse: number;
  hydrated: boolean;
  questLogOpen: boolean;
};

const STORAGE_KEY = 'motifengine.run.v1';
const listeners = new Set<() => void>();
const serverSnapshot: Snapshot = { run: createRun(), toasts: [], pulse: 0, hydrated: false, questLogOpen: false };
let snapshot: Snapshot = serverSnapshot;
let toastSeq = 0;

function publish(next: Partial<Snapshot>) {
  snapshot = { ...snapshot, ...next };
  listeners.forEach((listener) => listener());
}

function persist(run: RunState) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(run));
  } catch {
    // Storage can be unavailable in private windows; the run still works in memory.
  }
}

export function hydrateRun() {
  if (snapshot.hydrated) return;
  let run = createRun();
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    const parsed: unknown = stored ? JSON.parse(stored) : null;
    if (isRunState(parsed)) run = { ...createRun(), ...parsed };
  } catch {
    // A malformed or blocked store starts a fresh run.
  }
  publish({ run, hydrated: true });
}

function toastsFor(result: ProcessResult, run: RunState): Toast[] {
  const toasts: Toast[] = result.achievements.map((achievement) => ({
    id: `toast-${(toastSeq += 1)}`,
    kind: 'achievement',
    achievement,
    effects: result.effects.filter((effect) => effect.via.startsWith(achievement.id.replaceAll('-', '_'))),
    receipt: result.receipt,
    ordinal: Object.keys(run.unlocked).length,
  }));
  if (result.levelUp) toasts.push({ id: `toast-${(toastSeq += 1)}`, kind: 'level', level: result.levelUp });
  return toasts;
}

export function emit(event: IncomingEvent): ProcessResult {
  if (!snapshot.hydrated) hydrateRun();
  const { state, result } = processEvent(snapshot.run, event);
  let run = state;
  let toasts = [...snapshot.toasts, ...toastsFor(result, run)];

  if (result.achievements.length && !run.unlocked.completionist && remainingForCompletion(run) === 0) {
    const finale = processEvent(run, { id: 'run-completed', type: 'run.completed' });
    run = finale.state;
    toasts = [...toasts, ...toastsFor(finale.result, run)];
  }

  publish({ run, toasts: toasts.slice(-4), pulse: result.status === 'accepted' ? snapshot.pulse + 1 : snapshot.pulse });
  persist(run);
  return result;
}

export function updateRun(update: (run: RunState) => RunState) {
  const run = update(snapshot.run);
  if (run === snapshot.run) return;
  publish({ run });
  persist(run);
}

export function dismissToast(id: string) {
  publish({ toasts: snapshot.toasts.filter((toast) => toast.id !== id) });
}

export function setQuestLog(open: boolean | ((open: boolean) => boolean)) {
  const next = typeof open === 'function' ? open(snapshot.questLogOpen) : open;
  if (next !== snapshot.questLogOpen) publish({ questLogOpen: next });
}

export function resetRun() {
  const run = createRun();
  publish({ run, toasts: [], pulse: snapshot.pulse + 1 });
  persist(run);
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getSnapshot() {
  return snapshot;
}

export function useGame<T>(select: (snapshot: Snapshot) => T): T {
  return useSyncExternalStore(
    subscribe,
    () => select(snapshot),
    () => select(serverSnapshot),
  );
}

export function subscribeGame(listener: () => void) {
  return subscribe(listener);
}

export const TOTAL_ACHIEVEMENTS = ACHIEVEMENTS.length;
export const THEME_KEY = 'motifengine.theme';
