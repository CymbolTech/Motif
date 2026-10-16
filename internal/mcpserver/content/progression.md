# XP, levels, missions, achievements and streaks

## XP and levels

`award_xp` adds non-spendable experience. Levels are named cumulative thresholds (`level`, `name`, `xp_threshold`; level 1 usually at 0 XP). `GET /v1/users/{user_id}/progression` returns total XP, the current level and the next threshold.

## Missions

A mission counts events of one type toward `target_count` and caps there. Use it for checklists and challenges. The profile reports `progress`, `target_count`, `completed` and `completed_at`. A mission awards nothing by itself; add a rule on the same event to award XP or points.

## Achievements

An achievement unlocks once after `required_count` events of one type, with an immutable unlock time. Progress counts once per accepted event ID. Like missions, it awards nothing by itself.

A mission and an achievement can listen to the same event: the mission shows the live checklist, the achievement records the durable milestone.

## Streaks

`GET /v1/users/{user_id}/streaks?event_type=lesson.completed` returns `current_streak`, `longest_streak` and `last_active_date`, by UTC calendar day. Streaks are a read model and award nothing.
