export type View = 'overview' | 'loyalty' | 'events' | 'automations' | 'currency' | 'level' | 'mission' | 'achievement' | 'reward' | 'goal' | 'campaign' | 'leaderboard' | 'analyze' | 'access';

/** The top-level path of each console section. Pages own the paths below theirs. */
export const viewPaths = {
  overview: '/',
  loyalty: '/loyalty',
  events: '/events',
  automations: '/automations',
  currency: '/currencies',
  level: '/levels',
  mission: '/missions',
  achievement: '/achievements',
  reward: '/rewards',
  goal: '/goals',
  campaign: '/campaigns',
  leaderboard: '/leaderboards',
  analyze: '/reports',
  access: '/access-tokens',
} as const satisfies Record<View, string>;

export function viewFromPath(pathname: string): View {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/';
  const entries = Object.entries(viewPaths) as [View, string][];
  const match = entries.find(([, path]) => path === normalizedPath) ?? entries.find(([, path]) => path !== '/' && normalizedPath.startsWith(`${path}/`));
  return match?.[0] ?? 'overview';
}
