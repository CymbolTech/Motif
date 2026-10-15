import {
  ACHIEVEMENTS,
  CONVERSION,
  RATE_LIMIT,
  REWARDS,
  RULES,
  levelFor,
  type Achievement,
  type Currency,
  type Level,
  type Payload,
  type Reward,
} from './catalog';

// A pure, serializable model of one external user's state inside a workspace
// environment. It follows the engine's delivery semantics: one receipt per
// event id, one effect per (event, rule version, action ordinal), append-only
// ledger entries, and redemptions with explicit states.

export type LedgerEntry = {
  id: string;
  currency: Currency;
  amount: number;
  kind: 'award' | 'spend' | 'convert_out' | 'convert_in' | 'reserve' | 'release';
  key: string;
  source: string;
  at: number;
};

export type Effect = { label: string; via: string; key: string };

export type LogLine = {
  id: string;
  at: number;
  eventId: string;
  eventType: string;
  status: 'accepted' | 'duplicate' | 'rate_limited';
  summary: string;
};

export type RedemptionState = 'reserved' | 'delivering' | 'unknown' | 'fulfilled';

export type Redemption = { id: string; rewardId: Reward['id']; state: RedemptionState; attempts: number; at: number };

export type RunState = {
  version: 1;
  seq: number;
  receipts: Record<string, string>;
  progress: Record<string, number>;
  unlocked: Record<string, number>;
  xp: number;
  balances: Record<Currency, number>;
  ledger: LedgerEntry[];
  log: LogLine[];
  redemptions: Redemption[];
  unlocks: Partial<Record<Reward['id'], boolean>>;
  requestTimes: number[];
};

export type ProcessResult = {
  status: LogLine['status'];
  receipt?: string;
  effects: Effect[];
  achievements: Achievement[];
  levelUp?: Level;
};

export type IncomingEvent = { id: string; type: string; payload?: Payload; rateLimited?: boolean };

const LEDGER_LIMIT = 80;
const LOG_LIMIT = 40;

export function createRun(): RunState {
  return {
    version: 1,
    seq: 0,
    receipts: {},
    progress: {},
    unlocked: {},
    xp: 0,
    balances: { coins: 0, loyalty: 0 },
    ledger: [],
    log: [],
    redemptions: [],
    unlocks: {},
    requestTimes: [],
  };
}

function nextId(state: RunState, prefix: string) {
  state.seq += 1;
  return `${prefix}_${(state.seq * 7919).toString(16).padStart(4, '0').slice(-4)}`;
}

function appendLedger(state: RunState, entry: Omit<LedgerEntry, 'id' | 'at'>) {
  state.ledger = [{ ...entry, id: nextId(state, 'txn'), at: Date.now() }, ...state.ledger].slice(0, LEDGER_LIMIT);
}

function appendLog(state: RunState, line: Omit<LogLine, 'id' | 'at'>) {
  state.log = [{ ...line, id: nextId(state, 'log'), at: Date.now() }, ...state.log].slice(0, LOG_LIMIT);
}

function applyXp(state: RunState, amount: number, via: string, key: string, effects: Effect[]) {
  state.xp += amount;
  effects.push({ label: `+${amount} XP`, via, key });
}

function applyPoints(state: RunState, currency: Currency, amount: number, via: string, key: string, effects: Effect[]) {
  state.balances[currency] += amount;
  appendLedger(state, { currency, amount, kind: 'award', key, source: via });
  effects.push({ label: `+${amount} ${currency}`, via, key });
}

function unlock(state: RunState, achievement: Achievement, eventId: string, effects: Effect[]) {
  state.unlocked[achievement.id] = Date.now();
  const via = `${achievement.id.replaceAll('-', '_')}_bonus@v1`;
  applyXp(state, achievement.bonus.xp, via, `${eventId}:${via}:0`, effects);
  if (achievement.bonus.coins) applyPoints(state, 'coins', achievement.bonus.coins, via, `${eventId}:${via}:1`, effects);
}

/** Processes one event exactly as the worker would, returning a new state. */
export function processEvent(previous: RunState, event: IncomingEvent): { state: RunState; result: ProcessResult } {
  const state: RunState = structuredClone(previous);
  const payload = event.payload ?? {};

  const existing = state.receipts[event.id];
  if (existing) {
    appendLog(state, { eventId: event.id, eventType: event.type, status: 'duplicate', summary: `duplicate, receipt ${existing} reused` });
    return { state, result: { status: 'duplicate', receipt: existing, effects: [], achievements: [] } };
  }

  if (event.rateLimited) {
    const now = Date.now();
    state.requestTimes = state.requestTimes.filter((time) => now - time < RATE_LIMIT.windowMs);
    if (state.requestTimes.length >= RATE_LIMIT.requests) {
      appendLog(state, { eventId: event.id, eventType: event.type, status: 'rate_limited', summary: '429 rate_limited, retry later' });
      return { state, result: { status: 'rate_limited', effects: [], achievements: [] } };
    }
    state.requestTimes.push(now);
  }

  const receipt = nextId(state, 'evt');
  state.receipts[event.id] = receipt;
  const levelBefore = levelFor(state.xp);
  const effects: Effect[] = [];
  const unlockedNow: Achievement[] = [];

  for (const rule of RULES) {
    if (rule.eventType !== event.type || !rule.matches(payload)) continue;
    const via = `${rule.id}@v${rule.version}`;
    rule.actions.forEach((action, ordinal) => {
      const key = `${event.id}:${via}:${ordinal}`;
      if (action.type === 'award_xp') applyXp(state, action.amount, via, key, effects);
      else applyPoints(state, action.currency, action.amount, via, key, effects);
    });
  }

  for (const achievement of ACHIEVEMENTS) {
    if (state.unlocked[achievement.id] || !achievement.on.includes(event.type)) continue;
    if (achievement.where && !achievement.where(payload)) continue;
    const count = (state.progress[achievement.id] ?? 0) + 1;
    state.progress[achievement.id] = count;
    if (count >= achievement.required) {
      unlock(state, achievement, event.id, effects);
      unlockedNow.push(achievement);
    }
  }

  const levelAfter = levelFor(state.xp);
  const summary = effects.length ? effects.map((effect) => effect.label).join(' ') : 'accepted, no rule matched';
  appendLog(state, { eventId: event.id, eventType: event.type, status: 'accepted', summary });

  return {
    state,
    result: {
      status: 'accepted',
      receipt,
      effects,
      achievements: unlockedNow,
      levelUp: levelAfter.level > levelBefore.level ? levelAfter : undefined,
    },
  };
}

export function remainingForCompletion(state: RunState) {
  return ACHIEVEMENTS.filter((achievement) => achievement.id !== 'completionist' && !state.unlocked[achievement.id]).length;
}

/** Atomic conversion: both ledger rows carry the same conversion id. */
export function convertPoints(previous: RunState, fromAmount: number): { state: RunState; conversionId?: string; received: number } {
  const state: RunState = structuredClone(previous);
  const units = Math.floor(fromAmount / CONVERSION.fromAmount);
  if (units < 1 || state.balances[CONVERSION.from] < units * CONVERSION.fromAmount) return { state: previous, received: 0 };
  const spent = units * CONVERSION.fromAmount;
  const received = units * CONVERSION.toAmount;
  const conversionId = nextId(state, 'cnv');
  const source = `${CONVERSION.id}@v${CONVERSION.version}`;
  state.balances[CONVERSION.from] -= spent;
  state.balances[CONVERSION.to] += received;
  appendLedger(state, { currency: CONVERSION.from, amount: -spent, kind: 'convert_out', key: `${conversionId}:out`, source });
  appendLedger(state, { currency: CONVERSION.to, amount: received, kind: 'convert_in', key: `${conversionId}:in`, source });
  return { state, conversionId, received };
}

export function reserveReward(previous: RunState, rewardId: Reward['id']): { state: RunState; redemption?: Redemption } {
  const reward = REWARDS.find((candidate) => candidate.id === rewardId);
  if (!reward || previous.balances.coins < reward.cost) return { state: previous };
  const state: RunState = structuredClone(previous);
  const redemption: Redemption = { id: nextId(state, 'rdm'), rewardId, state: 'reserved', attempts: 0, at: Date.now() };
  state.balances.coins -= reward.cost;
  appendLedger(state, { currency: 'coins', amount: -reward.cost, kind: 'reserve', key: `${redemption.id}:reserve`, source: `reward:${rewardId}` });
  state.redemptions = [redemption, ...state.redemptions];
  return { state, redemption };
}

export function advanceRedemption(previous: RunState, redemptionId: string, next: RedemptionState): RunState {
  const state: RunState = structuredClone(previous);
  const redemption = state.redemptions.find((candidate) => candidate.id === redemptionId);
  if (!redemption) return previous;
  redemption.state = next;
  if (next === 'delivering') redemption.attempts += 1;
  if (next === 'fulfilled') {
    const reward = REWARDS.find((candidate) => candidate.id === redemption.rewardId);
    if (reward) appendLedger(state, { currency: 'coins', amount: 0, kind: 'spend', key: `${redemption.id}:settle`, source: `reward:${reward.id} settled` });
    state.unlocks[redemption.rewardId] = true;
  }
  return state;
}

export function isRunState(value: unknown): value is RunState {
  return typeof value === 'object' && value !== null && (value as RunState).version === 1 && typeof (value as RunState).xp === 'number';
}
