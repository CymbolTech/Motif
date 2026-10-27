// Site-wide constants. Both URLs are baked in at build time.

// A production build without NEXT_PUBLIC_SITE_URL falls back to the public domain,
// never to localhost, so canonical URLs, the sitemap and JSON-LD stay valid.
const FALLBACK_SITE_URL = process.env.NODE_ENV === 'production' ? 'https://motif.cymbol.ir' : 'http://127.0.0.1:4174';
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL || FALLBACK_SITE_URL).replace(/\/$/, '');
export const DASHBOARD_URL = process.env.NEXT_PUBLIC_DASHBOARD_URL || 'http://127.0.0.1:4173';

export const SITE_NAME = 'Motif';
export const COMPANY = 'Cymbol';

export const TITLE = 'Motif: Gamification and Loyalty API for XP, Badges, Rewards';
export const DESCRIPTION =
  'Motif is a gamification and loyalty API: add XP, levels, streaks, missions, badges, points, rewards and leaderboards to your product with one event call.';

export const KEYWORDS = [
  'gamification',
  'gamification API',
  'gamification engine',
  'gamification platform',
  'loyalty program API',
  'loyalty points',
  'points and rewards',
  'XP and levels',
  'achievements and badges',
  'missions',
  'streaks',
  'leaderboards',
  'rewards engine',
  'user engagement',
];

export const NAV = [
  { href: '/#building-blocks', label: 'Product' },
  { href: '/#use-cases', label: 'Use cases' },
  { href: '/#faq', label: 'FAQ' },
  { href: '/docs', label: 'Docs' },
];
