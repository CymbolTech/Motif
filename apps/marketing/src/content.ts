export const capabilities = [
  { eyebrow: '01 / Signals', title: 'Receive a signal once, then make it useful.', body: 'Validate bounded event payloads against versioned contracts. Idempotent receipts keep retries safe and every event remains traceable.', detail: 'Events · schemas · deduplication' },
  { eyebrow: '02 / Logic', title: 'Turn behavior into rules your team can explain.', body: 'Compose readable conditions and typed actions, publish immutable versions, and schedule a rule without hiding its runtime behavior.', detail: 'Conditions · actions · schedules' },
  { eyebrow: '03 / State', title: 'Keep progression and loyalty separate by design.', body: 'Model XP, achievements, missions, and independent point currencies without collapsing different kinds of user value into one balance.', detail: 'XP · missions · point ledgers' },
  { eyebrow: '04 / Recovery', title: 'Make reward delivery observable when the network is not.', body: 'Track redemption state, provider attempts, retries, and ambiguous outcomes so a temporary outage never silently spends a customer’s points.', detail: 'Rewards · providers · reconciliation' },
] as const;

export const scenarios = [
  { label: 'Commerce', title: 'Reward the second purchase.', body: 'A customer backend sends order.completed. A published rule checks the purchase count and awards loyalty points exactly once.' },
  { label: 'Learning', title: 'Make progress visible.', body: 'A lesson.completed event advances a mission, awards XP, and unlocks an achievement when the configured threshold is reached.' },
  { label: 'Community', title: 'Keep currencies intentional.', body: 'Convert one tenant-owned currency into another with a versioned rate and one atomic operation, preserving the ledger history.' },
] as const;

export const docs = [
  { title: 'Events and schemas', description: 'Define contracts, validate payloads, and follow a receipt through processing.', href: '/docs#events' },
  { title: 'Rules and schedules', description: 'Build conditions, typed actions, immutable versions, and live windows.', href: '/docs#rules' },
  { title: 'Points and progression', description: 'Understand currencies, ledger entries, expiration, XP, missions, and achievements.', href: '/docs#progression' },
  { title: 'Rewards and operations', description: 'Configure redemption, providers, retries, and operator diagnostics.', href: '/docs#rewards' },
] as const;
