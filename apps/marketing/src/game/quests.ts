import { ACHIEVEMENTS, type Achievement } from './catalog';
import type { RunState } from './engine';

const TRACK_ORDER = ['first-signal', 'halfway', 'loop-walker', 'rule-smith', 'once-means-once', 'money-changer', 'cashed-in', 'world-tour', 'curious', 'all-the-way', 'manual', 'press-start', 'sharpshooter', 'cheat-code', 'completionist'];

/** The objective the HUD suggests next: the first locked achievement in story order. */
export function nextQuest(run: RunState): Achievement | undefined {
  return TRACK_ORDER.map((id) => ACHIEVEMENTS.find((achievement) => achievement.id === id)).find(
    (achievement): achievement is Achievement => Boolean(achievement && !run.unlocked[achievement.id]),
  );
}

export function questText(quest: Achievement | undefined) {
  if (!quest) return 'Every achievement unlocked. Legend.';
  return quest.secret ? quest.hint ?? quest.description : quest.description;
}
