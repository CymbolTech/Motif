UPDATE plans SET entitlements = entitlements - 'max_leaderboards';

DROP TABLE IF EXISTS leaderboard_scores;
DROP TABLE IF EXISTS leaderboard_rounds;
DROP TABLE IF EXISTS leaderboards;
